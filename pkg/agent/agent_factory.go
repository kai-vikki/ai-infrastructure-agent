package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/retrieval"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
)

// ========== Interface defines ==========

// AgentFactoryInterface defines agent creation and initialization functionality
//
// Available Functions:
//   - NewStateAwareAgent()        : Create a new state-aware AI agent instance
//   - Initialize()                : Initialize agent and test connectivity
//   - Cleanup()                   : Clean up agent resources and connections
//
//   - initializeLLM()             : Initialize the Language Model (OpenAI, Gemini, etc.)
//   - testLLMConnectivity()       : Test LLM connection and basic functionality
//
// This file handles all agent creation, initialization, and cleanup operations.
// It ensures proper setup of LLM connections, MCP processes, and resource mappings.
//
// Usage Example:
//   1. agent := NewStateAwareAgent(config, awsClient, stateFile, region, logger, awsConfig)
//   2. agent.Initialize(ctx)
//   3. defer agent.Cleanup()

// ========== Agent Factory and Initialization Functions ==========

// NewStateAwareAgent creates a new state-aware AI agent
func NewStateAwareAgent(agentConfig *config.AgentConfig, awsClient *aws.Client, stateFilePath, region string, logger *logging.Logger, awsConfig *config.AWSConfig) (*StateAwareAgent, error) {
	// Initialize LLM based on provider
	llm, err := initializeLLM(agentConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	// Initialize configuration loader
	configLoader := config.NewConfigLoader("./settings")

	// Load field mapping configuration
	fieldMappingConfig, err := configLoader.LoadFieldMappings()
	if err != nil {
		return nil, fmt.Errorf("failed to load field mapping config: %w", err)
	}

	// Load extraction configuration
	extractionConfig, err := configLoader.LoadResourceExtraction()
	if err != nil {
		return nil, fmt.Errorf("failed to load resource extraction config: %w", err)
	}

	idExtractor, err := resources.NewIDExtractor(extractionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ID extractor: %w", err)
	}

	// Load resource pattern configuration
	resourcePatternConfig, err := configLoader.LoadResourcePatterns()
	if err != nil {
		return nil, fmt.Errorf("failed to load resource pattern config: %w", err)
	}

	// Initialize pattern matcher
	patternMatcher, err := resources.NewPatternMatcher(resourcePatternConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pattern matcher: %w", err)
	}

	// Initialize value type inferrer
	valueTypeInferrer, err := resources.NewValueTypeInferrer(resourcePatternConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create value type inferrer: %w", err)
	}

	fieldResolver := resources.NewFieldResolver(fieldMappingConfig)
	fieldResolver.SetPatternMatcher(patternMatcher)

	// Get global retrieval registry
	registry := retrieval.GetGlobalRegistry()

	agent := &StateAwareAgent{
		// Common properties
		llm:       llm,
		config:    agentConfig,
		awsConfig: awsConfig,
		awsClient: awsClient,
		Logger:    logger,

		// MCP properties
		mcpProcess:       nil, // Will be initialized when needed
		resourceMappings: make(map[string]string),
		mcpTools:         make(map[string]MCPToolInfo),
		mcpResources:     make(map[string]MCPResourceInfo),

		// Lock properties
		capabilityMutex: sync.RWMutex{},
		mappingsMutex:   sync.RWMutex{},

		// Configuration-driven components
		patternMatcher:    patternMatcher,
		valueTypeInferrer: valueTypeInferrer,
		fieldResolver:     fieldResolver,

		// Extractor for resource identification
		extractionConfig: extractionConfig,
		idExtractor:      idExtractor,

		registry: registry,
	}

	return agent, nil
}

// Initialize initializes the agent and loads existing state
func (a *StateAwareAgent) Initialize(ctx context.Context) error {
	a.Logger.Info("Initializing state-aware AI agent")

	// Test LLM connectivity
	if err := a.testLLMConnectivity(ctx); err != nil {
		a.Logger.WithError(err).Error("LLM connectivity test failed")
		return fmt.Errorf("LLM connectivity test failed: %w", err)
	}

	// Initialize retrieval registry and register all retrieval functions
	a.initializeRetrievalRegistry()

	// Start MCP process and discover capabilities early
	if err := a.startMCPProcess(); err != nil {
		a.Logger.WithError(err).Warn("Failed to start MCP process during initialization, will retry later")
		// Don't fail initialization if MCP process fails, it will be retried when needed
	}

	a.Logger.Info("State-aware AI agent initialized successfully")
	return nil
}

// Cleanup ensures proper cleanup of resources
func (a *StateAwareAgent) Cleanup() {
	a.stopMCPProcess()
}

// initializeLLM initializes the appropriate LLM based on the provider configuration
func initializeLLM(agentConfig *config.AgentConfig, logger *logging.Logger) (llms.Model, error) {
	provider := strings.ToLower(agentConfig.Provider)

	logger.WithFields(map[string]interface{}{
		"provider": provider,
		"model":    agentConfig.Model,
	}).Info("Initializing LLM")

	switch provider {
	case "openai":
		if agentConfig.OpenAIAPIKey == "" {
			logger.Error("OpenAI API key is missing")
			return nil, fmt.Errorf("OpenAI API key is required for provider 'openai'")
		}
		logger.WithFields(map[string]interface{}{
			"api_key_length": len(agentConfig.OpenAIAPIKey),
			"api_key_prefix": agentConfig.OpenAIAPIKey[:min(8, len(agentConfig.OpenAIAPIKey))],
		}).Debug("OpenAI configuration")

		llm, err := openai.New(
			openai.WithToken(agentConfig.OpenAIAPIKey),
			openai.WithModel(agentConfig.Model),
		)
		if err != nil {
			logger.WithError(err).Error("Failed to initialize OpenAI client")
			return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
		}
		logger.Info("OpenAI client initialized successfully")
		return llm, nil

	case "gemini", "googleai":
		if agentConfig.GeminiAPIKey == "" {
			logger.Error("Gemini API key is missing")
			return nil, fmt.Errorf("GeminiAI API key is required for provider 'gemini'")
		}
		logger.WithFields(map[string]interface{}{
			"api_key_length": len(agentConfig.GeminiAPIKey),
			"api_key_prefix": agentConfig.GeminiAPIKey[:min(8, len(agentConfig.GeminiAPIKey))],
		}).Debug("Gemini configuration")

		ctx := context.Background()
		llm, err := googleai.New(
			ctx,
			googleai.WithAPIKey(agentConfig.GeminiAPIKey),
			googleai.WithDefaultModel(agentConfig.Model),
		)
		if err != nil {
			logger.WithError(err).Error("Failed to initialize Gemini client")
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		logger.Info("Gemini client initialized successfully")
		return llm, nil

	case "anthropic":
		return nil, fmt.Errorf("AnthropicAI provider not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s. Supported providers: openai, gemini", provider)
	}
}

// testLLMConnectivity tests basic connectivity to the LLM
func (a *StateAwareAgent) testLLMConnectivity(ctx context.Context) error {
	a.Logger.Debug("Testing LLM connectivity")

	testPrompt := "Respond with exactly this JSON: {\"status\": \"ok\", \"message\": \"connectivity test successful\"}"

	response, err := llms.GenerateFromSinglePrompt(ctx, a.llm, testPrompt,
		llms.WithTemperature(0.1),
		llms.WithMaxTokens(100))

	if err != nil {
		return fmt.Errorf("LLM test call failed: %w", err)
	}

	if len(response) == 0 {
		return fmt.Errorf("LLM returned empty response during connectivity test")
	}

	a.Logger.WithFields(map[string]interface{}{
		"test_response_length": len(response),
		"test_response":        response,
	}).Info("LLM connectivity test successful")

	return nil
}
