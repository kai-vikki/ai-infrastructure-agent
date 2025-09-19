package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// RequestProcessorInterface defines core request processing functionality
//
// Available Functions:
//   - ProcessRequest()                : Process natural language infrastructure requests
//   - gatherDecisionContext()         : Gather context for decision-making
//   - generateDecisionWithPlan()      : Generate AI decision with detailed execution plan
//   - validateDecision()              : Validate agent decisions for safety and consistency
//   - buildDecisionWithPlanPrompt()   : Build comprehensive prompts for AI decision making
//   - parseAIResponseWithPlan()       : Parse AI responses into structured execution plans
//
// This file handles the core request processing pipeline from natural language
// input to validated execution plans ready for infrastructure operations.
//
// Usage Example:
//   1. decision, err := agent.ProcessRequest(ctx, "Create a web server with load balancer")
//   2. // Decision contains validated execution plan ready for deployment

// ProcessRequest processes a natural language infrastructure request and generates a plan
func (a *StateAwareAgent) ProcessRequest(ctx context.Context, request string) (*types.AgentDecision, error) {
	a.Logger.WithField("request", request).Info("Processing infrastructure request")

	// Create decision ID
	decisionID := uuid.New().String()

	// Gather context
	decisionContext, err := a.gatherDecisionContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to gather decision context: %w", err)
	}

	// Generate AI decision with detailed execution plan
	decision, err := a.generateDecisionWithPlan(ctx, decisionID, request, decisionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate decision: %w", err)
	}

	// Validate decision
	if err := a.validateDecision(decision, decisionContext); err != nil {
		return nil, fmt.Errorf("decision validation failed: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"action":      decision.Action,
		"confidence":  decision.Confidence,
		"plan_steps":  len(decision.ExecutionPlan),
	}).Info("Infrastructure request processed successfully")

	return decision, nil
}

// gatherDecisionContext gathers context for decision-making
func (a *StateAwareAgent) gatherDecisionContext(ctx context.Context, request string) (*DecisionContext, error) {
	a.Logger.Debug("Gathering decision context")

	// Use MCP server to analyze infrastructure state
	currentState, discoveredResources, _, err := a.AnalyzeInfrastructureState(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze infrastructure state: %w", err)
	}

	// Use MCP server to detect conflicts
	conflicts, err := a.DetectInfrastructureConflicts(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	// Use MCP server to get deployment order
	deploymentOrder, _, err := a.PlanInfrastructureDeployment(ctx, nil, false)
	if err != nil {
		// Non-fatal error - continue without deployment order
		a.Logger.WithError(err).Warn("Failed to calculate deployment order")
		deploymentOrder = []string{}
	}

	// Analyze resource correlation for better decision making
	resourceCorrelation := a.analyzeResourceCorrelation(currentState, discoveredResources)

	return &DecisionContext{
		Request:             request,
		CurrentState:        currentState,
		DiscoveredState:     discoveredResources,
		Conflicts:           conflicts,
		DependencyGraph:     nil, // Will be handled by MCP server
		DeploymentOrder:     deploymentOrder,
		ResourceCorrelation: resourceCorrelation,
	}, nil
}

// generateDecisionWithPlan uses AI to generate a decision with detailed execution plan
func (a *StateAwareAgent) generateDecisionWithPlan(ctx context.Context, decisionID, request string, context *DecisionContext) (*types.AgentDecision, error) {
	a.Logger.Debug("Generating AI decision with execution plan")

	// Create prompt for the AI that includes plan generation
	prompt := a.buildDecisionWithPlanPrompt(request, context)

	// Log prompt details for debugging
	a.Logger.WithFields(map[string]interface{}{
		"prompt_length":  len(prompt),
		"max_tokens":     a.config.MaxTokens,
		"temperature":    a.config.Temperature,
		"provider":       a.config.Provider,
		"model":          a.config.Model,
		"prompt_preview": prompt[:min(500, len(prompt))],
	}).Info("Calling LLM with prompt")

	// Call the LLM using the new recommended method
	response, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt,
		llms.WithTemperature(a.config.Temperature),
		llms.WithMaxTokens(a.config.MaxTokens))

	// Enhanced error handling
	if err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"provider":      a.config.Provider,
			"model":         a.config.Model,
			"prompt_length": len(prompt),
		}).Error("LLM call failed")
		return nil, fmt.Errorf("failed to generate AI response: %w", err)
	}

	if a.config.EnableDebug {
		// Comprehensive response logging
		a.Logger.WithFields(map[string]interface{}{
			"response_length":  len(response),
			"response_empty":   len(response) == 0,
			"response_content": response, // Log full response for debugging
		}).Info("LLM Response received")
	}

	// Handle empty response immediately
	if len(response) == 0 {
		a.Logger.Error("LLM returned empty response - check API key, model availability, and prompt")
		return nil, fmt.Errorf("LLM returned empty response - possible API key, model, or prompt issue")
	}

	if a.config.EnableDebug {
		// Log response characteristics for debugging
		a.Logger.WithFields(map[string]interface{}{
			"response_length":     len(response),
			"max_tokens_config":   a.config.MaxTokens,
			"starts_with_brace":   strings.HasPrefix(response, "{"),
			"ends_with_brace":     strings.HasSuffix(response, "}"),
			"probable_truncation": strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}"),
		}).Debug("LLM Response Analysis")
	}

	// Check for potential token limit issues
	if len(response) > 0 && strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}") {
		a.Logger.WithFields(map[string]interface{}{
			"response_length": len(response),
			"max_tokens":      a.config.MaxTokens,
			"last_100_chars":  response[max(0, len(response)-100):],
		}).Warn("Response appears truncated - consider increasing max_tokens in config")
	}

	// Parse the AI response with execution plan
	decision, err := a.parseAIResponseWithPlan(decisionID, request, response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// validateDecision validates an agent decision
func (a *StateAwareAgent) validateDecision(decision *types.AgentDecision, context *DecisionContext) error {
	a.Logger.Debug("Validating agent decision")

	// Check confidence threshold
	if decision.Confidence < 0.7 {
		return fmt.Errorf("decision confidence too low: %f", decision.Confidence)
	}

	// Validate action
	validActions := map[string]bool{
		"create_infrastructure": true,
		"update_infrastructure": true,
		"delete_infrastructure": true,
		"resolve_conflicts":     true,
		"no_action":             true,
	}

	if !validActions[decision.Action] {
		return fmt.Errorf("invalid action: %s", decision.Action)
	}

	planValidActions := map[string]bool{
		"create":              true,
		"update":              true,
		"add":                 true,
		"delete":              true,
		"validate":            true,
		"api_value_retrieval": true,
	}

	for _, planStep := range decision.ExecutionPlan {
		if !planValidActions[planStep.Action] {
			return fmt.Errorf("invalid plan action: %s", planStep.Action)
		}
	}

	// Check for critical conflicts if auto-resolve is disabled
	if !a.config.AutoResolveConflicts && len(context.Conflicts) > 0 {
		for _, conflict := range context.Conflicts {
			if conflict.ConflictType == "dependency" {
				return fmt.Errorf("critical dependency conflict detected, manual resolution required")
			}
		}
	}

	// if a.config.EnableDebug {
	// 	// Validate execution plan for hardcoded AWS resource IDs
	// 	if err := a.validateExecutionPlanDependencies(decision.ExecutionPlan); err != nil {
	// 		return fmt.Errorf("execution plan validation failed: %w", err)
	// 	}
	// }

	return nil
}

// buildDecisionWithPlanPrompt builds a prompt for AI decision-making with execution plan
func (a *StateAwareAgent) buildDecisionWithPlanPrompt(request string, context *DecisionContext) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert AWS infrastructure automation agent with comprehensive state management capabilities.\n\n")

	// Add available tools context
	prompt.WriteString(a.getAvailableToolsContext())
	prompt.WriteString("\n")

	prompt.WriteString("USER REQUEST: " + request + "\n\n")

	// === INFRASTRUCTURE STATE OVERVIEW ===
	prompt.WriteString("📊 INFRASTRUCTURE STATE OVERVIEW:\n")
	prompt.WriteString("Analyze ALL available resources from the state file to make informed decisions.\n\n")

	// Show current managed resources from state file
	if len(context.CurrentState.Resources) > 0 {
		prompt.WriteString("🏗️ MANAGED RESOURCES (from state file):\n")
		for resourceID, resource := range context.CurrentState.Resources {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resourceID, resource.Type, resource.Status))

			// Extract and show key properties from state file
			if resource.Properties != nil {
				var properties []string

				// Extract from direct properties
				for key, value := range resource.Properties {
					if key == "mcp_response" {
						// Extract from nested mcp_response
						if mcpMap, ok := value.(map[string]interface{}); ok {
							for mcpKey, mcpValue := range mcpMap {
								if mcpKey != "success" && mcpKey != "timestamp" && mcpKey != "message" {
									properties = append(properties, fmt.Sprintf("%s:%v", mcpKey, mcpValue))
								}
							}
						}
					} else if key != "status" {
						properties = append(properties, fmt.Sprintf("%s:%v", key, value))
					}
				}

				if len(properties) > 0 {
					prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
				}
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}

	// Show discovered AWS resources (not in state file)
	if len(context.DiscoveredState) > 0 {
		prompt.WriteString("🔍 DISCOVERED AWS RESOURCES (not managed in state file):\n")
		for _, resource := range context.DiscoveredState {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", resource.ID, resource.Type, resource.Status))

			if resource.Properties != nil {
				var properties []string

				// Show most relevant properties for each resource type
				relevantKeys := []string{"vpcId", "groupName", "instanceType", "cidrBlock", "name", "state", "availabilityZone"}
				for _, key := range relevantKeys {
					if value, exists := resource.Properties[key]; exists {
						properties = append(properties, fmt.Sprintf("%s:%v", key, value))
					}
				}

				if len(properties) > 0 {
					prompt.WriteString(fmt.Sprintf(" [%s]", strings.Join(properties, ", ")))
				}
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}

	// Show resource correlations if any
	if len(context.ResourceCorrelation) > 0 {
		prompt.WriteString("🔗 RESOURCE CORRELATIONS:\n")
		for managedID, correlation := range context.ResourceCorrelation {
			prompt.WriteString(fmt.Sprintf("- State file resource '%s' correlates with AWS resource '%s' (confidence: %.2f)\n",
				managedID, correlation.DiscoveredResource.ID, correlation.MatchConfidence))
		}
		prompt.WriteString("\n")
	}

	// Show any conflicts
	if len(context.Conflicts) > 0 {
		prompt.WriteString("⚠️ DETECTED CONFLICTS:\n")
		for _, conflict := range context.Conflicts {
			prompt.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n", conflict.ConflictType, conflict.Details, conflict.ResourceID))
		}
		prompt.WriteString("\n")
	}

	// === DECISION GUIDELINES ===
	prompt.WriteString("🎯 DECISION-MAKING GUIDELINES:\n")
	prompt.WriteString("1. RESOURCE REUSE: Always prefer existing AWS resources over creating new ones\n")
	prompt.WriteString("2. STATE AWARENESS: Consider all resources in the state file for dependencies and conflicts\n")
	prompt.WriteString("3. INTELLIGENT PLANNING: Create execution plans that leverage existing infrastructure\n")
	prompt.WriteString("4. MINIMAL CHANGES: Make only necessary changes to achieve the user's request\n")
	prompt.WriteString("5. DEPENDENCY MANAGEMENT: Ensure proper dependency ordering in execution plans\n\n")

	// === AI DECISION PROMPT ===
	prompt.WriteString("📋 YOUR TASK:\n")
	prompt.WriteString("Based on the user request and ALL infrastructure state information above:\n")
	prompt.WriteString("1. Analyze what already exists in both managed and discovered resources\n")
	prompt.WriteString("2. Determine the minimal set of actions needed to fulfill the request\n")
	prompt.WriteString("3. Create an execution plan using available MCP tools\n")
	prompt.WriteString("4. Provide clear reasoning for your decisions\n\n")

	// === JSON RESPONSE SCHEMA ===
	prompt.WriteString("🔧 REQUIRED JSON RESPONSE FORMAT:\n")
	prompt.WriteString("Respond with ONLY valid JSON in this exact format:\n\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"action\": \"create_infrastructure|update_infrastructure|delete_infrastructure|no_action\",\n")
	prompt.WriteString("  \"reasoning\": \"Detailed explanation of your analysis and decision-making process\",\n")
	prompt.WriteString("  \"confidence\": 0.0-1.0,\n")
	prompt.WriteString("  \"resourcesAnalyzed\": {\n")
	prompt.WriteString("    \"managedCount\": 0,\n")
	prompt.WriteString("    \"discoveredCount\": 0,\n")
	prompt.WriteString("    \"reusableResources\": [\"list of resources that can be reused\"]\n")
	prompt.WriteString("  },\n")
	prompt.WriteString("  \"executionPlan\": [\n")
	prompt.WriteString("    {\n")
	prompt.WriteString("      \"id\": \"step-1\",\n")
	prompt.WriteString("      \"name\": \"Step Description\",\n")
	prompt.WriteString("      \"description\": \"Detailed step description\",\n")
	prompt.WriteString("      \"action\": \"create|update|delete|validate|api_value_retrieval\",\n")
	prompt.WriteString("      \"resourceId\": \"logical-resource-id\",\n")
	prompt.WriteString("      \"mcpTool\": \"exact-mcp-tool-name\",\n")
	prompt.WriteString("      \"toolParameters\": {\n")
	prompt.WriteString("        \"parameter\": \"value\"\n")
	prompt.WriteString("      },\n")
	prompt.WriteString("      \"dependsOn\": [\"list-of-step-ids\"],\n")
	prompt.WriteString("      \"estimatedDuration\": \"10s\",\n")
	prompt.WriteString("      \"status\": \"pending\"\n")
	prompt.WriteString("    }\n")
	prompt.WriteString("  ]\n")
	prompt.WriteString("}\n\n")

	// === CRITICAL INSTRUCTIONS ===
	prompt.WriteString("🚨 STEP DEPENDENCY REQUIREMENTS:\n")
	prompt.WriteString("NEVER use hardcoded AWS resource IDs like sg-12345678, vpc-abcdef, ami-987654, etc.\n")
	prompt.WriteString("ALWAYS create step dependencies and use {{step-id.resourceId}} references:\n\n")
	prompt.WriteString("✅ CORRECT Load Balancer Pattern:\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"id\": \"step-create-lb-sg\",\n")
	prompt.WriteString("  \"action\": \"create\",\n")
	prompt.WriteString("  \"mcpTool\": \"create-security-group\",\n")
	prompt.WriteString("  \"toolParameters\": {\n")
	prompt.WriteString("    \"groupName\": \"web-alb-sg\",\n")
	prompt.WriteString("    \"description\": \"Security group for load balancer\",\n")
	prompt.WriteString("    \"vpcId\": \"{{step-vpc.resourceId}}\"\n")
	prompt.WriteString("  }\n")
	prompt.WriteString("},\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"id\": \"step-create-alb\",\n")
	prompt.WriteString("  \"action\": \"create\",\n")
	prompt.WriteString("  \"mcpTool\": \"create-load-balancer\",\n")
	prompt.WriteString("  \"toolParameters\": {\n")
	prompt.WriteString("    \"name\": \"web-app-alb\",\n")
	prompt.WriteString("    \"securityGroupIds\": [\"{{step-create-lb-sg.resourceId}}\"]\n")
	prompt.WriteString("  },\n")
	prompt.WriteString("  \"dependsOn\": [\"step-create-lb-sg\"]\n")
	prompt.WriteString("}\n\n")
	prompt.WriteString("❌ WRONG - Hardcoded IDs will cause failures:\n")
	prompt.WriteString("\"securityGroupIds\": [\"sg-05dc049424690e203\"]  // This ID may not exist!\n\n")

	prompt.WriteString("� IMPORTANT: Distinguish between HARDCODED AWS IDs vs LEGITIMATE PARAMETERS:\n")
	prompt.WriteString("✅ LEGITIMATE parameter values (these are fine):\n")
	prompt.WriteString("  - Resource names: \"web-app-alb\", \"production-vpc\", \"public-subnet-1\"\n")
	prompt.WriteString("  - Parameter types: \"vpc_id\", \"subnet_id\", \"security_group_id\", \"existing_resource\"\n")
	prompt.WriteString("  - Schemes/types: \"internet-facing\", \"application\", \"HTTP\", \"HTTPS\"\n")
	prompt.WriteString("  - CIDR blocks: \"10.0.0.0/16\", \"192.168.1.0/24\"\n")
	prompt.WriteString("❌ HARDCODED AWS resource IDs (NEVER use these):\n")
	prompt.WriteString("  - VPC IDs: \"vpc-12345678abcdef\"\n")
	prompt.WriteString("  - Subnet IDs: \"subnet-87654321fedcba\"\n")
	prompt.WriteString("  - Security Group IDs: \"sg-05dc049424690e203\"\n")
	prompt.WriteString("  - Instance IDs: \"i-1234567890abcdef0\"\n")
	prompt.WriteString("  - AMI IDs: \"ami-0abcdef1234567890\"\n\n")

	prompt.WriteString("�🚨 CRITICAL INSTRUCTIONS:\n")
	prompt.WriteString("1. ANALYZE ALL RESOURCES: Consider every resource shown above before making decisions\n")
	prompt.WriteString("2. REUSE FIRST: Always check if existing resources can fulfill the request\n")
	prompt.WriteString("3. USE EXACT TOOL NAMES: Only use MCP tool names shown in the tools context above\n")
	prompt.WriteString("4. PARAMETER ACCURACY: Use correct parameter names and types for each tool\n")
	prompt.WriteString("5. DEPENDENCY REFERENCES: Use {{step-id.resourceId}} format for dependencies\n")
	prompt.WriteString("6. JSON ONLY: Return only valid JSON - no markdown, no explanations, no extra text\n")
	prompt.WriteString("7. STATE FILE AWARENESS: Remember that managed resources exist in the state file\n")
	prompt.WriteString("8. ACTION TYPE USAGE:\n")
	prompt.WriteString("   - create: For new AWS resources that don't exist (VPC, subnets, route table associations, routes, security rules, etc.)\n")
	prompt.WriteString("   - update: For modifying properties of existing resources (changing tags, descriptions)\n")
	prompt.WriteString("   - delete: For removing AWS resources\n")
	prompt.WriteString("   - validate: For checking resource states or configurations\n")
	prompt.WriteString("   - api_value_retrieval: For fetching real AWS values to replace placeholders\n")
	prompt.WriteString("   🚨 ONLY use these exact actions: create, update, add, delete, validate, api_value_retrieval\n")
	prompt.WriteString("   🚨 NEVER use: associate, attach, connect, link, join, bind, or any other action names\n\n")

	// === EXAMPLES ===
	prompt.WriteString("💡 DECISION EXAMPLES:\n")
	prompt.WriteString("Example 1 - Resource Reuse: If user wants a web server and you see existing VPC and security groups, reuse them\n")
	prompt.WriteString("Example 2 - Minimal Changes: If user wants to add a database and VPC exists, only create database resources\n")
	prompt.WriteString("Example 3 - No Action: If user requests something that already exists, return action: \"no_action\"\n\n")

	prompt.WriteString("💡 ACTION EXAMPLES:\n")
	prompt.WriteString("✅ CREATE: \"action\": \"create\" for new VPC, subnets, security groups, route table associations, routes, etc.\n")
	prompt.WriteString("✅ CREATE: \"action\": \"create\" for associating route tables with subnets (creates new association resource)\n")
	prompt.WriteString("✅ CREATE: \"action\": \"create\" for adding routes to route tables (creates new route resource)\n")
	prompt.WriteString("❌ NEVER USE: associate, attach, connect, link, join, bind\n\n")

	prompt.WriteString("BEGIN YOUR ANALYSIS AND PROVIDE YOUR JSON RESPONSE:\n")

	return prompt.String()
}

// parseAIResponseWithPlan parses the AI response into an AgentDecision with execution plan
func (a *StateAwareAgent) parseAIResponseWithPlan(decisionID, request, response string) (*types.AgentDecision, error) {
	a.Logger.Debug("Parsing AI response for execution plan")

	// Log the raw response for debugging - ALWAYS log this for troubleshooting
	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"raw_response_length": len(response),
			"raw_response":        response,
		}).Info("AI Response received")
	}

	// Check if response appears to be truncated JSON
	if strings.HasPrefix(response, "{") && !strings.HasSuffix(response, "}") {
		a.Logger.WithFields(map[string]interface{}{
			"response_starts_with": response[:min(100, len(response))],
			"response_ends_with":   response[max(0, len(response)-100):],
		}).Warn("Response appears to be truncated JSON")
	}

	// Try multiple JSON extraction methods
	jsonStr := a.extractJSON(response)
	if jsonStr == "" {
		// Try alternative extraction methods
		jsonStr = a.extractJSONAlternative(response)
	}

	// Special handling for potentially truncated responses
	if jsonStr == "" && strings.HasPrefix(response, "{") {
		a.Logger.Warn("Attempting to parse potentially truncated JSON response")
		jsonStr = a.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		a.Logger.WithFields(map[string]interface{}{
			"response_preview":  response[:min(500, len(response))],
			"response_length":   len(response),
			"starts_with_brace": strings.HasPrefix(response, "{"),
			"ends_with_brace":   strings.HasSuffix(response, "}"),
		}).Error("No valid JSON found in AI response")
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"extracted_json_length": len(jsonStr),
			"extracted_json":        jsonStr,
		}).Info("Successfully extracted JSON from AI response")
	}

	// Parse JSON with execution plan - updated for native MCP tool support
	var parsed struct {
		Action        string                 `json:"action"`
		Reasoning     string                 `json:"reasoning"`
		Confidence    float64                `json:"confidence"`
		Parameters    map[string]interface{} `json:"parameters"`
		ExecutionPlan []struct {
			ID                string                 `json:"id"`
			Name              string                 `json:"name"`
			Description       string                 `json:"description"`
			Action            string                 `json:"action"`
			ResourceID        string                 `json:"resourceId"`
			MCPTool           string                 `json:"mcpTool"`        // New: Direct MCP tool name
			ToolParameters    map[string]interface{} `json:"toolParameters"` // New: Direct tool parameters
			Parameters        map[string]interface{} `json:"parameters"`     // Legacy fallback
			DependsOn         []string               `json:"dependsOn"`
			EstimatedDuration string                 `json:"estimatedDuration"`
			Status            string                 `json:"status"`
		} `json:"executionPlan"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		a.Logger.WithError(err).WithField("json", jsonStr).Error("Failed to parse AI response JSON")

		// Try fallback parsing without execution plan
		var simpleParsed struct {
			Action     string                 `json:"action"`
			Reasoning  string                 `json:"reasoning"`
			Confidence float64                `json:"confidence"`
			Parameters map[string]interface{} `json:"parameters"`
		}

		if fallbackErr := json.Unmarshal([]byte(jsonStr), &simpleParsed); fallbackErr != nil {
			return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
		}

		a.Logger.Warn("Using fallback parsing - no execution plan available")
		return &types.AgentDecision{
			ID:            decisionID,
			Action:        simpleParsed.Action,
			Resource:      request,
			Reasoning:     simpleParsed.Reasoning,
			Confidence:    simpleParsed.Confidence,
			Parameters:    simpleParsed.Parameters,
			ExecutionPlan: []*types.ExecutionPlanStep{}, // Empty plan
			Timestamp:     time.Now(),
		}, nil
	}

	// Convert execution plan with native MCP support
	var executionPlan []*types.ExecutionPlanStep
	for _, step := range parsed.ExecutionPlan {
		planStep := &types.ExecutionPlanStep{
			ID:                step.ID,
			Name:              step.Name,
			Description:       step.Description,
			Action:            step.Action,
			ResourceID:        step.ResourceID,
			MCPTool:           step.MCPTool,
			ToolParameters:    step.ToolParameters,
			Parameters:        step.Parameters,
			DependsOn:         step.DependsOn,
			EstimatedDuration: step.EstimatedDuration,
			Status:            step.Status,
		}

		// Ensure we have parameters - use ToolParameters if available, otherwise Parameters
		if len(planStep.ToolParameters) > 0 {
			// Native MCP mode - use ToolParameters as primary
			if planStep.Parameters == nil {
				planStep.Parameters = make(map[string]interface{})
			}
			// Copy tool parameters to legacy parameters for compatibility
			for key, value := range planStep.ToolParameters {
				planStep.Parameters[key] = value
			}
		}

		executionPlan = append(executionPlan, planStep)
	}

	return &types.AgentDecision{
		ID:            decisionID,
		Action:        parsed.Action,
		Resource:      request,
		Reasoning:     parsed.Reasoning,
		Confidence:    parsed.Confidence,
		Parameters:    parsed.Parameters,
		ExecutionPlan: executionPlan,
		Timestamp:     time.Now(),
	}, nil
}

// validateExecutionPlanDependencies validates execution plan to detect hardcoded AWS resource IDs
func (a *StateAwareAgent) validateExecutionPlanDependencies(executionPlan []*types.ExecutionPlanStep) error {
	a.Logger.Debug("Validating execution plan for hardcoded AWS resource IDs")

	// AWS resource ID patterns to detect - ONLY match actual AWS resource IDs, not parameter values
	awsResourcePatterns := map[string]string{
		"vpc":              `^vpc-[0-9a-f]{8,17}$`,
		"subnet":           `^subnet-[0-9a-f]{8,17}$`,
		"security-group":   `^sg-[0-9a-f]{8,17}$`,
		"instance":         `^i-[0-9a-f]{8,17}$`,
		"ami":              `^ami-[0-9a-f]{8,17}$`,
		"load-balancer":    `^arn:aws:elasticloadbalancing:.*:.*:loadbalancer/.*$`,
		"target-group":     `^arn:aws:elasticloadbalancing:.*:.*:targetgroup/.*$`,
		"launch-template":  `^lt-[0-9a-f]{8,17}$`,
		"route-table":      `^rtb-[0-9a-f]{8,17}$`,
		"internet-gateway": `^igw-[0-9a-f]{8,17}$`,
		"nat-gateway":      `^nat-[0-9a-f]{8,17}$`,
		"elastic-ip":       `^eipalloc-[0-9a-f]{8,17}$`,
	}

	var hardcodedResources []string

	for _, step := range executionPlan {
		// Check tool parameters for hardcoded AWS resource IDs
		if step.Parameters != nil {
			hardcoded := a.detectHardcodedResourceIDs(step.Parameters, awsResourcePatterns, step.ID)
			hardcodedResources = append(hardcodedResources, hardcoded...)
		}
	}

	if len(hardcodedResources) > 0 {
		a.Logger.WithFields(map[string]interface{}{
			"hardcoded_resources": hardcodedResources,
		}).Error("Detected hardcoded AWS resource IDs in execution plan")

		return fmt.Errorf("execution plan contains hardcoded AWS resource IDs that may not exist: %s. Use step dependency references like {{step-create-sg.resourceId}} instead", strings.Join(hardcodedResources, ", "))
	}

	return nil
}

// detectHardcodedResourceIDs recursively checks for hardcoded AWS resource IDs in parameters
func (a *StateAwareAgent) detectHardcodedResourceIDs(params map[string]interface{}, patterns map[string]string, stepID string) []string {
	var hardcodedIDs []string

	for key, value := range params {
		switch v := value.(type) {
		case string:
			// Skip step references (they start with {{ and end with }})
			if strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") {
				continue
			}

			// Check against AWS resource patterns
			for resourceType, pattern := range patterns {
				if matched, _ := regexp.MatchString(pattern, v); matched {
					hardcodedID := fmt.Sprintf("%s:%s (in %s.%s)", resourceType, v, stepID, key)
					hardcodedIDs = append(hardcodedIDs, hardcodedID)

					a.Logger.WithFields(map[string]interface{}{
						"step_id":       stepID,
						"parameter":     key,
						"value":         v,
						"resource_type": resourceType,
					}).Warn("Detected hardcoded AWS resource ID")
				}
			}

		case []interface{}:
			// Handle arrays (like securityGroupIds)
			for i, item := range v {
				if itemStr, ok := item.(string); ok {
					// Skip step references
					if strings.HasPrefix(itemStr, "{{") && strings.HasSuffix(itemStr, "}}") {
						continue
					}

					// Check against AWS resource patterns
					for resourceType, pattern := range patterns {
						if matched, _ := regexp.MatchString(pattern, itemStr); matched {
							hardcodedID := fmt.Sprintf("%s:%s (in %s.%s[%d])", resourceType, itemStr, stepID, key, i)
							hardcodedIDs = append(hardcodedIDs, hardcodedID)

							a.Logger.WithFields(map[string]interface{}{
								"step_id":       stepID,
								"parameter":     key,
								"array_index":   i,
								"value":         itemStr,
								"resource_type": resourceType,
							}).Warn("Detected hardcoded AWS resource ID in array")
						}
					}
				}
			}

		case map[string]interface{}:
			// Recursive check for nested maps
			nestedHardcoded := a.detectHardcodedResourceIDs(v, patterns, stepID)
			hardcodedIDs = append(hardcodedIDs, nestedHardcoded...)
		}
	}

	return hardcodedIDs
}
