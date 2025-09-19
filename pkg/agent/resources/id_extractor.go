package resources

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
)

// IDExtractor handles resource ID extraction using configuration
type IDExtractor struct {
	creationPatterns     []config.ExtractionPattern
	modificationPatterns []config.ExtractionPattern
	associationPatterns  []config.ExtractionPattern
	deletionPatterns     []config.ExtractionPattern
	queryPatterns        []config.ExtractionPattern
	actionClassifier     *ActionClassifier
	fallbackStrategies   *config.FallbackStrategies
	mu                   sync.RWMutex
}

// ActionClassifier classifies tools into action types
type ActionClassifier struct {
	creationPatterns     []*regexp.Regexp
	modificationPatterns []*regexp.Regexp
	associationPatterns  []*regexp.Regexp
	deletionPatterns     []*regexp.Regexp
	queryPatterns        []*regexp.Regexp
}

// NewIDExtractor creates a new ID extractor from configuration
func NewIDExtractor(cfg *config.ResourceExtractionConfig) (*IDExtractor, error) {
	extractor := &IDExtractor{
		creationPatterns:     cfg.ResourceIDExtraction.CreationTools.Patterns,
		modificationPatterns: cfg.ResourceIDExtraction.ModificationTools.Patterns,
		associationPatterns:  cfg.ResourceIDExtraction.AssociationTools.Patterns,
		deletionPatterns:     cfg.ResourceIDExtraction.DeletionTools.Patterns,
		queryPatterns:        cfg.ResourceIDExtraction.QueryTools.Patterns,
		fallbackStrategies:   &cfg.FallbackStrategies,
	}

	// Create action classifier
	classifier, err := NewActionClassifier(&cfg.ToolActionTypes)
	if err != nil {
		return nil, err
	}
	extractor.actionClassifier = classifier

	return extractor, nil
}

// NewActionClassifier creates a new action classifier
func NewActionClassifier(cfg *config.ToolActionTypes) (*ActionClassifier, error) {
	classifier := &ActionClassifier{}

	// Compile creation patterns
	for _, pattern := range cfg.CreationTools.Patterns {
		compiledPattern, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile creation pattern %s: %w", pattern, err)
		}
		classifier.creationPatterns = append(classifier.creationPatterns, compiledPattern)
	}

	// Compile modification patterns
	for _, pattern := range cfg.ModificationTools.Patterns {
		compiledPattern, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile modification pattern %s: %w", pattern, err)
		}
		classifier.modificationPatterns = append(classifier.modificationPatterns, compiledPattern)
	}

	// Compile association patterns
	for _, pattern := range cfg.AssociationTools.Patterns {
		compiledPattern, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile association pattern %s: %w", pattern, err)
		}
		classifier.associationPatterns = append(classifier.associationPatterns, compiledPattern)
	}

	// Compile deletion patterns
	for _, pattern := range cfg.DeletionTools.Patterns {
		compiledPattern, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile deletion pattern %s: %w", pattern, err)
		}
		classifier.deletionPatterns = append(classifier.deletionPatterns, compiledPattern)
	}

	// Compile query patterns
	for _, pattern := range cfg.QueryTools.Patterns {
		compiledPattern, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile query pattern %s: %w", pattern, err)
		}
		classifier.queryPatterns = append(classifier.queryPatterns, compiledPattern)
	}

	return classifier, nil
}

// ClassifyTool determines the action type of a tool
func (ac *ActionClassifier) ClassifyTool(toolName string) string {
	// Try creation patterns first
	for _, pattern := range ac.creationPatterns {
		if pattern.MatchString(toolName) {
			return "creation"
		}
	}

	// Try modification patterns
	for _, pattern := range ac.modificationPatterns {
		if pattern.MatchString(toolName) {
			return "modification"
		}
	}

	// Try association patterns
	for _, pattern := range ac.associationPatterns {
		if pattern.MatchString(toolName) {
			return "association"
		}
	}

	// Try deletion patterns
	for _, pattern := range ac.deletionPatterns {
		if pattern.MatchString(toolName) {
			return "deletion"
		}
	}

	// Try query patterns
	for _, pattern := range ac.queryPatterns {
		if pattern.MatchString(toolName) {
			return "query"
		}
	}

	return "unknown"
}

// ExtractResourceID extracts resource ID from tool parameters and results
func (e *IDExtractor) ExtractResourceID(toolName string, resourceType string, parameters, result map[string]interface{}) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	actionType := e.actionClassifier.ClassifyTool(toolName)

	var patterns []config.ExtractionPattern
	var dataSource map[string]interface{}

	switch actionType {
	case "creation":
		patterns = e.creationPatterns
		dataSource = result // Created resources return IDs in result
	case "modification":
		patterns = e.modificationPatterns
		dataSource = parameters // Modified resources have IDs in parameters
	case "association":
		patterns = e.associationPatterns
		dataSource = result // Association operations return new association IDs in result
	case "deletion":
		patterns = e.deletionPatterns
		dataSource = parameters // Deletion operations specify IDs in parameters
	case "query":
		patterns = e.queryPatterns
		dataSource = mergeData(parameters, result) // Could be in either
	default:
		// Try all patterns with both data sources
		if id, err := e.tryAllPatterns(resourceType, parameters, result); err == nil {
			return id, nil
		}
		return e.fallbackExtraction(parameters, result)
	}

	// Try patterns for this resource type and action
	for _, pattern := range patterns {
		if e.matchesResourceType(pattern.ResourceTypes, resourceType) {
			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(dataSource, fieldPath); value != "" {
					return value, nil
				}
			}
		}
	}

	// Fallback extraction
	return e.fallbackExtraction(parameters, result)
}

// tryAllPatterns tries all pattern types when action type is unknown
func (e *IDExtractor) tryAllPatterns(resourceType string, parameters, result map[string]interface{}) (string, error) {
	// Try creation patterns with result data
	for _, pattern := range e.creationPatterns {
		if e.matchesResourceType(pattern.ResourceTypes, resourceType) {
			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(result, fieldPath); value != "" {
					return value, nil
				}
			}
		}
	}

	// Try modification patterns with parameters
	for _, pattern := range e.modificationPatterns {
		if e.matchesResourceType(pattern.ResourceTypes, resourceType) {
			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(parameters, fieldPath); value != "" {
					return value, nil
				}
			}
		}
	}

	// Try association patterns with result data
	for _, pattern := range e.associationPatterns {
		if e.matchesResourceType(pattern.ResourceTypes, resourceType) {
			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(result, fieldPath); value != "" {
					return value, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no patterns matched for resource type %s", resourceType)
}

// matchesResourceType checks if a resource type matches the pattern's resource types
func (e *IDExtractor) matchesResourceType(patternTypes []string, resourceType string) bool {
	for _, patternType := range patternTypes {
		if patternType == "*" || patternType == resourceType {
			return true
		}
	}
	return false
}

// extractFromPath extracts a value from nested data using dot notation
func (e *IDExtractor) extractFromPath(data map[string]interface{}, path string) string {
	if data == nil {
		return ""
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		// Handle array access like "resources[0]"
		if strings.Contains(part, "[") {
			arrayName, index, err := parseArrayAccessForIDExtractor(part)
			if err != nil {
				return ""
			}

			if array, ok := current[arrayName].([]interface{}); ok && len(array) > index {
				if i == len(parts)-1 {
					// Last part of path - return the array element
					return fmt.Sprintf("%v", array[index])
				}
				// Continue navigation
				if nextLevel, ok := array[index].(map[string]interface{}); ok {
					current = nextLevel
					continue
				}
			}
			return ""
		}

		// Regular field access
		if i == len(parts)-1 {
			// Last part - return the value
			if value, ok := current[part]; ok {
				return fmt.Sprintf("%v", value)
			}
		} else {
			// Intermediate part - navigate deeper
			if nextLevel, ok := current[part].(map[string]interface{}); ok {
				current = nextLevel
			} else {
				return ""
			}
		}
	}

	return ""
}

// parseArrayAccessForIDExtractor parses array access syntax like "resources[0]"
func parseArrayAccessForIDExtractor(part string) (string, int, error) {
	leftBracket := strings.Index(part, "[")
	rightBracket := strings.Index(part, "]")

	if leftBracket == -1 || rightBracket == -1 || rightBracket <= leftBracket {
		return "", 0, fmt.Errorf("invalid array access syntax: %s", part)
	}

	arrayName := part[:leftBracket]
	indexStr := part[leftBracket+1 : rightBracket]

	// Handle wildcard or special array access
	if indexStr == "*" {
		return arrayName, 0, nil // Return first element for wildcard
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid array index: %s", indexStr)
	}

	return arrayName, index, nil
}

// fallbackExtraction uses fallback strategies when patterns fail
func (e *IDExtractor) fallbackExtraction(parameters, result map[string]interface{}) (string, error) {
	// Try common ID fields
	for _, field := range e.fallbackStrategies.CommonIDFields {
		// Check in result first
		if value := e.findValueInMap(result, field); value != "" {
			return value, nil
		}
		// Then check in parameters
		if value := e.findValueInMap(parameters, field); value != "" {
			return value, nil
		}
	}

	// Try array extraction patterns
	for _, path := range e.fallbackStrategies.ArrayExtraction {
		if value := e.extractFromPath(result, path); value != "" {
			return value, nil
		}
	}

	// Try nested extraction patterns
	for _, path := range e.fallbackStrategies.NestedExtraction {
		if value := e.extractFromPath(result, path); value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("could not extract resource ID using any fallback strategy")
}

// findValueInMap finds a value in a map, case-insensitive
func (e *IDExtractor) findValueInMap(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}

	// Try exact match first
	if value, ok := data[key]; ok {
		return fmt.Sprintf("%v", value)
	}

	// Try case-insensitive match
	for k, v := range data {
		if strings.EqualFold(k, key) {
			return fmt.Sprintf("%v", v)
		}
	}

	return ""
}

// mergeData merges parameters and result maps
func mergeData(parameters, result map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy parameters first
	for k, v := range parameters {
		merged[k] = v
	}

	// Add result (overwrites parameters if same key)
	for k, v := range result {
		merged[k] = v
	}

	return merged
}

// ExtractAllResourceIDs extracts all possible resource IDs from tool data
func (e *IDExtractor) ExtractAllResourceIDs(toolName string, parameters, result map[string]interface{}) map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make(map[string]string)

	actionType := e.actionClassifier.ClassifyTool(toolName)

	var patterns []config.ExtractionPattern
	var dataSource map[string]interface{}

	switch actionType {
	case "creation":
		patterns = e.creationPatterns
		dataSource = result
	case "modification", "deletion":
		patterns = e.modificationPatterns
		dataSource = parameters
	case "query":
		patterns = e.queryPatterns
		dataSource = mergeData(parameters, result)
	default:
		dataSource = mergeData(parameters, result)
	}

	// Extract IDs for all resource types
	for _, pattern := range patterns {
		for _, resourceType := range pattern.ResourceTypes {
			if resourceType == "*" {
				continue // Skip wildcard for this function
			}

			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(dataSource, fieldPath); value != "" {
					ids[resourceType] = value
					break // Take first match for this resource type
				}
			}
		}
	}

	return ids
}
