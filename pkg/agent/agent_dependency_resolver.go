package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ========== Interface defines ==========

// DependencyResolverInterface defines dependency resolution and parameter management functionality
//
// Available Functions:
//   - resolveDependencyReference()      : Resolve references like {{step-1.resourceId}}
//   - resolveDefaultValue()                 : Provide intelligent default values
//   - addMissingRequiredParameters()    : Add defaults for missing required parameters
//   - validateNativeMCPArguments()      : Validate arguments against tool schema
//
// This file handles dependency resolution between plan steps, parameter
// validation, and intelligent default value provisioning for infrastructure operations.
//
// Usage Example:
//   1. resolvedValue := agent.resolveDependencyReference("{{step-1.resourceId}}")
//   2. defaultValue := agent.resolveDefaultValue("create-ec2-instance", "instanceType", params)
//   3. // Use resolved values in infrastructure operations

// ========== Dependency Resolution and Parameter Management Functions ==========

// resolveDependencyReference resolves references like {{step-1.resourceId}} to actual resource IDs
func (a *StateAwareAgent) resolveDependencyReference(reference string) (string, error) {
	a.Logger.WithField("reference", reference).Debug("Starting dependency reference resolution")

	// Extract step ID from reference like {{step-1.resourceId}} or {{step-1.resourceId}}[0]
	if !strings.HasPrefix(reference, "{{") || (!strings.HasSuffix(reference, "}}") && !strings.Contains(reference, "}[")) {
		return reference, nil // Not a reference
	}

	// Handle bracket notation first: {{step-1.resourceId}}[0] -> convert to {{step-1.resourceId.0}}
	if strings.Contains(reference, "}[") {
		// Pattern: {{step-1.resourceId}}[0]
		bracketPos := strings.Index(reference, "}[")
		if bracketPos > 0 {
			beforeBracket := reference[:bracketPos+1] // {{step-1.resourceId}}
			afterBracket := reference[bracketPos+2:]  // 0]

			if strings.HasSuffix(afterBracket, "]") {
				indexStr := strings.TrimSuffix(afterBracket, "]")
				if _, err := strconv.Atoi(indexStr); err == nil {
					// Convert {{step-1.resourceId}}[0] to {{step-1.resourceId.0}}
					convertedRef := strings.TrimSuffix(beforeBracket, "}}") + "." + indexStr + "}}"
					a.Logger.WithFields(map[string]interface{}{
						"original_reference":  reference,
						"converted_reference": convertedRef,
					}).Info("Converted bracket notation to dot notation")
					return a.resolveDependencyReference(convertedRef)
				}
			}
		}
	}

	refContent := strings.TrimSuffix(strings.TrimPrefix(reference, "{{"), "}}")

	parts := strings.Split(refContent, ".")

	// Support multiple reference formats: {{step-1.resourceId}}, {{step-1}}, {{step-1.targetGroupArn}}, {{step-1.resourceId.0}}, etc.
	var stepID string
	var requestedField string
	var arrayIndex int = -1

	if len(parts) == 3 {
		// Format: {{step-1.resourceId.0}} - array indexing
		stepID = parts[0]
		requestedField = parts[1]
		if idx, err := strconv.Atoi(parts[2]); err == nil {
			arrayIndex = idx
		} else {
			return "", fmt.Errorf("invalid array index in reference: %s (expected numeric index)", reference)
		}
	} else if len(parts) == 2 {
		stepID = parts[0]
		requestedField = parts[1]
	} else if len(parts) == 1 {
		stepID = parts[0]
		requestedField = "resourceId" // Default to resourceId for backward compatibility
	} else {
		return "", fmt.Errorf("invalid reference format: %s (expected {{step-id.field}}, {{step-id.field.index}}, or {{step-id}})", reference)
	}

	a.Logger.WithFields(map[string]interface{}{
		"reference":       reference,
		"step_id":         stepID,
		"requested_field": requestedField,
		"array_index":     arrayIndex,
	}).Debug("Parsed dependency reference")

	a.mappingsMutex.RLock()
	resourceID, exists := a.resourceMappings[stepID]

	// Log available mappings for debugging
	a.Logger.WithFields(map[string]interface{}{
		"step_id":            stepID,
		"mapping_exists":     exists,
		"available_mappings": len(a.resourceMappings),
	}).Debug("Checking resource mappings")

	// Handle array indexing - check for specific indexed mapping first
	if arrayIndex >= 0 {
		indexedKey := fmt.Sprintf("%s.%d", stepID, arrayIndex)
		if indexedValue, indexedExists := a.resourceMappings[indexedKey]; indexedExists {
			a.mappingsMutex.RUnlock()

			a.Logger.WithFields(map[string]interface{}{
				"reference":   reference,
				"step_id":     stepID,
				"array_index": arrayIndex,
				"resource_id": indexedValue,
				"source":      "indexed_mapping",
			}).Info("Resolved indexed dependency reference")

			return indexedValue, nil
		} else {
			a.Logger.WithFields(map[string]interface{}{
				"indexed_key":    indexedKey,
				"mapping_exists": false,
			}).Debug("Indexed mapping not found")
		}
	}
	a.mappingsMutex.RUnlock()

	if !exists {
		// In test mode, avoid accessing real state - rely only on stored mappings - will update with state mocking
		if a.testMode {
			return "", fmt.Errorf("dependency reference not found in test mode: %s (step ID: %s not found in resource mappings)", reference, stepID)
		}

		// Fallback: try to get state via MCP and extract the resource ID
		if resolvedID, err := a.resolveFromInfrastructureState(stepID, requestedField, reference, arrayIndex); err == nil {
			return resolvedID, nil
		}

		return "", fmt.Errorf("resource ID not found for step: %s", stepID)
	}

	a.Logger.WithFields(map[string]interface{}{
		"reference":   reference,
		"step_id":     stepID,
		"resource_id": resourceID,
	}).Debug("Resolved dependency reference")

	return resourceID, nil
}

// resolveFromInfrastructureState attempts to resolve a dependency reference by parsing the infrastructure state
func (a *StateAwareAgent) resolveFromInfrastructureState(stepID, requestedField, reference string, arrayIndex int) (string, error) {
	// Parse the state and look for the step ID
	stateJSON, err := a.ExportInfrastructureState(context.Background(), false) // Only managed state
	if err != nil {
		return "", fmt.Errorf("failed to export infrastructure state: %w", err)
	}

	var stateData map[string]interface{}
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		return "", fmt.Errorf("failed to parse state JSON: %w", err)
	}

	managedState, ok := stateData["managed_state"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("managed_state not found in state data")
	}

	resources, ok := managedState["resources"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resources not found in managed_state")
	}

	resource, ok := resources[stepID].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resource not found for step ID: %s", stepID)
	}

	// Extract AWS resource ID from the resource properties
	properties, ok := resource["properties"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("properties not found in resource")
	}

	mcpResponse, ok := properties["mcp_response"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("mcp_response not found in properties")
	}

	// First, try to find the specifically requested field
	if requestedField != "resourceId" {
		// Check for array indexing
		if arrayIndex >= 0 {
			if arrayField, ok := mcpResponse[requestedField].([]interface{}); ok {
				if arrayIndex < len(arrayField) {
					if id, ok := arrayField[arrayIndex].(string); ok && id != "" {
						// Cache it for future use
						a.mappingsMutex.Lock()
						a.resourceMappings[stepID] = id
						a.mappingsMutex.Unlock()

						a.Logger.WithFields(map[string]interface{}{
							"reference":       reference,
							"step_id":         stepID,
							"resource_id":     id,
							"source":          "state_fallback",
							"requested_field": requestedField,
							"array_index":     arrayIndex,
						}).Info("Resolved array field dependency from state")

						return id, nil
					}
				} else {
					return "", fmt.Errorf("array index %d out of bounds for field %s (length: %d)", arrayIndex, requestedField, len(arrayField))
				}
			}
			// If it's not an array but we have an index, check if the field maps to all_zones
			if requestedField == "resourceId" && arrayIndex >= 0 {
				if allZones, ok := mcpResponse["all_zones"].([]interface{}); ok && arrayIndex < len(allZones) {
					if id, ok := allZones[arrayIndex].(string); ok && id != "" {
						// Cache it for future use
						a.mappingsMutex.Lock()
						a.resourceMappings[stepID] = id
						a.mappingsMutex.Unlock()

						a.Logger.WithFields(map[string]interface{}{
							"reference":   reference,
							"step_id":     stepID,
							"resource_id": id,
							"source":      "state_fallback_all_zones",
							"array_index": arrayIndex,
						}).Info("Resolved availability zone from all_zones array")

						return id, nil
					}
				}
			}
		} else {
			if id, ok := mcpResponse[requestedField].(string); ok && id != "" {
				// Cache it for future use
				a.mappingsMutex.Lock()
				a.resourceMappings[stepID] = id
				a.mappingsMutex.Unlock()

				a.Logger.WithFields(map[string]interface{}{
					"reference":       reference,
					"step_id":         stepID,
					"resource_id":     id,
					"source":          "state_fallback",
					"requested_field": requestedField,
				}).Info("Resolved specific field dependency from state")

				return id, nil
			}
		}
	}

	// Use configuration-driven field resolver with resource type detection
	fieldsToTry := a.fieldResolver.GetFieldsForRequestWithContext(requestedField, mcpResponse)

	for _, field := range fieldsToTry {
		if id, ok := mcpResponse[field].(string); ok && id != "" {
			// Cache it for future use
			a.mappingsMutex.Lock()
			a.resourceMappings[stepID] = id
			a.mappingsMutex.Unlock()

			a.Logger.WithFields(map[string]interface{}{
				"reference":   reference,
				"step_id":     stepID,
				"resource_id": id,
				"source":      "state_fallback",
				"field":       field,
			}).Debug("Resolved dependency reference from state")

			return id, nil
		}
	}

	// Final fallback: try to extract from nested resource.id field
	if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
		if resourceID, ok := resource["id"].(string); ok && resourceID != "" {
			// Cache it for future use
			a.mappingsMutex.Lock()
			a.resourceMappings[stepID] = resourceID
			a.mappingsMutex.Unlock()

			a.Logger.WithFields(map[string]interface{}{
				"reference":   reference,
				"step_id":     stepID,
				"resource_id": resourceID,
				"source":      "state_fallback_nested",
				"field":       "resource.id",
			}).Debug("Resolved dependency reference from nested resource.id in state")

			return resourceID, nil
		}
	}

	return "", fmt.Errorf("resource ID not found in mcp_response for step: %s", stepID)
}

// LEGACY FUNCTIONS - Using native MCP integration approach

// resolveDefaultValue provides default values for required parameters
func (a *StateAwareAgent) resolveDefaultValue(toolName, paramName string, params map[string]interface{}) interface{} {
	switch toolName {
	case "create-ec2-instance":
		switch paramName {
		case "instanceType":
			// Use params to choose appropriate instance type based on workload
			if workload, exists := params["workload_type"]; exists {
				switch workload {
				case "compute-intensive":
					return "c5.large"
				case "memory-intensive":
					return "r5.large"
				case "storage-intensive":
					return "i3.large"
				default:
					return "t3.micro"
				}
			}
			return "t3.micro"
		case "imageId":
			// First, try to find AMI from a previous API retrieval step
			if amiStepRef, exists := params["ami_step_ref"]; exists {
				stepRef := fmt.Sprintf("%v", amiStepRef)
				if amiID, err := a.resolveDependencyReference(stepRef); err == nil {
					a.Logger.WithFields(map[string]interface{}{
						"ami_id":   amiID,
						"step_ref": stepRef,
						"source":   "api_retrieval_step",
					}).Info("Using AMI ID from API retrieval step")
					return amiID
				} else {
					a.Logger.WithError(err).WithField("step_ref", stepRef).Warn("Failed to resolve AMI step reference, falling back to direct API call")
				}
			}

			// Fallback to direct API call (legacy approach)
			amiID := a.retrieveDefaultAMIForRegion()
			if amiID != "" {
				a.Logger.WithFields(map[string]interface{}{
					"ami_id": amiID,
					"source": "direct_api_call",
				}).Info("Using AMI ID from direct API call")
				return amiID
			}

			// If all else fails, return empty string to trigger an error
			a.Logger.Warn("No AMI ID available from API retrieval step or direct call")
			return ""
		case "keyName":
			// Try to use key name from params if available
			if keyName, exists := params["ssh_key"]; exists {
				return keyName
			}
			return nil // Let AWS use account default
		}
	case "create-vpc":
		switch paramName {
		case "cidrBlock":
			// Use params to determine appropriate CIDR block
			if cidr, exists := params["cidr"]; exists {
				return cidr
			}
			if environment, exists := params["environment"]; exists {
				switch environment {
				case "production":
					return "10.0.0.0/16"
				case "staging":
					return "10.1.0.0/16"
				case "development":
					return "10.2.0.0/16"
				default:
					return "10.0.0.0/16"
				}
			}
			return "10.0.0.0/16"
		case "name":
			// Generate name based on params
			if name, exists := params["resource_name"]; exists {
				return name
			}
			if environment, exists := params["environment"]; exists {
				return fmt.Sprintf("vpc-%s", environment)
			}
			return "ai-agent-vpc"
		}
	case "create-security-group":
		switch paramName {
		case "description":
			// Generate description based on params
			if desc, exists := params["description"]; exists {
				return desc
			}
			if purpose, exists := params["purpose"]; exists {
				return fmt.Sprintf("Security group for %s", purpose)
			}
			return "Security group created by AI Agent"
		}
	}
	return nil
}

// addMissingRequiredParameters adds intelligent defaults for missing required parameters
func (a *StateAwareAgent) addMissingRequiredParameters(toolName string, arguments map[string]interface{}, toolInfo MCPToolInfo) error {
	if toolInfo.InputSchema == nil {
		return nil // No schema to validate against
	}

	properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldStr, ok := field.(string); ok {
				requiredFields[fieldStr] = true
			}
		}
	}

	// Add defaults for missing required fields
	for paramName := range properties {
		if requiredFields[paramName] {
			if _, exists := arguments[paramName]; !exists {
				// Parameter is required but missing, add default
				if defaultValue := a.resolveDefaultValue(toolName, paramName, arguments); defaultValue != nil {
					arguments[paramName] = defaultValue
					a.Logger.WithFields(map[string]interface{}{
						"tool_name":  toolName,
						"param_name": paramName,
						"default":    defaultValue,
					}).Debug("Added default value for missing required parameter")
				}
			}
		}
	}

	return nil
}

// validateNativeMCPArguments validates arguments against the tool's schema
func (a *StateAwareAgent) validateNativeMCPArguments(toolName string, arguments map[string]interface{}, toolInfo MCPToolInfo) error {
	if toolInfo.InputSchema == nil {
		return nil // No schema to validate against
	}

	properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldStr, ok := field.(string); ok {
				requiredFields[fieldStr] = true
			}
		}
	}

	// Validate required fields are present and non-empty
	for paramName := range properties {
		if requiredFields[paramName] {
			val, exists := arguments[paramName]
			if !exists || val == nil {
				return fmt.Errorf("required parameter %s is missing for tool %s", paramName, toolName)
			}
			// Check for empty strings
			if strVal, ok := val.(string); ok && strVal == "" {
				return fmt.Errorf("required parameter %s is empty for tool %s", paramName, toolName)
			}
		}
	}

	return nil
}
