package tools

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	awsclient "github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// =============================================================================
// SECRETS MANAGER TOOLS
// =============================================================================

// CreateSecretTool creates a new secret in AWS Secrets Manager
type CreateSecretTool struct {
	*BaseTool
	awsClient *awsclient.Client
}

// NewCreateSecretTool creates a new create secret tool
func NewCreateSecretTool(awsClient *awsclient.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the secret",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the secret",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "Secret value",
			},
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Tags for the secret",
			},
		},
		"required": []string{"name"},
	}

	examples := []interfaces.ToolExample{
		{
			Description: "Create a database password secret",
			Arguments: map[string]interface{}{
				"name":        "db-password",
				"description": "Database password for production",
				"value":       "my-secret-password",
			},
			Expected: "arn:aws:secretsmanager:us-west-2:123456789012:secret:db-password-AbCdEf",
		},
	}

	baseTool := NewBaseTool("create-secret", "Creates a new secret in AWS Secrets Manager", "secrets", inputSchema, logger)
	baseTool.examples = examples

	return &CreateSecretTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

// Execute executes the create secret tool
func (t *CreateSecretTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, ok := args["name"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: name parameter is required"),
			},
		}, fmt.Errorf("name parameter is required")
	}

	description, _ := args["description"].(string)
	value, _ := args["value"].(string)

	// Default values
	if description == "" {
		description = "Secret created by AI Infrastructure Agent"
	}
	if value == "" {
		value = "default-secret-value"
	}

	// Create secret
	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		Description:  aws.String(description),
		SecretString: aws.String(value),
	}

	// Add tags if provided
	if tags, ok := args["tags"].(map[string]string); ok {
		var tagList []types.Tag
		for key, val := range tags {
			tagList = append(tagList, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(val),
			})
		}
		input.Tags = tagList
	}

	result, err := t.awsClient.SecretsManager.CreateSecret(ctx, input)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to create secret: %v", err)),
			},
		}, fmt.Errorf("failed to create secret: %w", err)
	}

	t.logger.WithFields(map[string]interface{}{
		"secret_name": name,
		"secret_arn":  *result.ARN,
	}).Info("Secret created successfully")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(*result.ARN),
		},
	}, nil
}

// GetSecretTool retrieves a secret value from AWS Secrets Manager
type GetSecretTool struct {
	*BaseTool
	awsClient *awsclient.Client
}

// NewGetSecretTool creates a new get secret tool
func NewGetSecretTool(awsClient *awsclient.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the secret to retrieve",
			},
		},
		"required": []string{"name"},
	}

	baseTool := NewBaseTool("get-secret", "Retrieves a secret value from AWS Secrets Manager", "secrets", inputSchema, logger)

	return &GetSecretTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

// Execute executes the get secret tool
func (t *GetSecretTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, ok := args["name"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: name parameter is required"),
			},
		}, fmt.Errorf("name parameter is required")
	}

	// Get secret
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	result, err := t.awsClient.SecretsManager.GetSecretValue(ctx, input)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to get secret: %v", err)),
			},
		}, fmt.Errorf("failed to get secret: %w", err)
	}

	t.logger.WithField("secret_name", name).Info("Secret retrieved successfully")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(*result.SecretString),
		},
	}, nil
}

// UpdateSecretTool updates a secret value in AWS Secrets Manager
type UpdateSecretTool struct {
	*BaseTool
	awsClient *awsclient.Client
}

// NewUpdateSecretTool creates a new update secret tool
func NewUpdateSecretTool(awsClient *awsclient.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the secret to update",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "New secret value",
			},
		},
		"required": []string{"name", "value"},
	}

	baseTool := NewBaseTool("update-secret", "Updates a secret value in AWS Secrets Manager", "secrets", inputSchema, logger)

	return &UpdateSecretTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

// Execute executes the update secret tool
func (t *UpdateSecretTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, ok := args["name"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: name parameter is required"),
			},
		}, fmt.Errorf("name parameter is required")
	}

	value, ok := args["value"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: value parameter is required"),
			},
		}, fmt.Errorf("value parameter is required")
	}

	// Update secret
	input := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(value),
	}

	result, err := t.awsClient.SecretsManager.UpdateSecret(ctx, input)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to update secret: %v", err)),
			},
		}, fmt.Errorf("failed to update secret: %w", err)
	}

	t.logger.WithFields(map[string]interface{}{
		"secret_name": name,
		"secret_arn":  *result.ARN,
	}).Info("Secret updated successfully")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(*result.ARN),
		},
	}, nil
}

// ListSecretsTool lists all secrets in AWS Secrets Manager
type ListSecretsTool struct {
	*BaseTool
	awsClient *awsclient.Client
}

// NewListSecretsTool creates a new list secrets tool
func NewListSecretsTool(awsClient *awsclient.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool("list-secrets", "Lists all secrets in AWS Secrets Manager", "secrets", inputSchema, logger)

	return &ListSecretsTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

// Execute executes the list secrets tool
func (t *ListSecretsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// List secrets
	input := &secretsmanager.ListSecretsInput{}

	result, err := t.awsClient.SecretsManager.ListSecrets(ctx, input)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to list secrets: %v", err)),
			},
		}, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Extract secret names and ARNs
	secrets := make([]map[string]interface{}, len(result.SecretList))
	for i, secret := range result.SecretList {
		secrets[i] = map[string]interface{}{
			"name":        *secret.Name,
			"arn":         *secret.ARN,
			"description": *secret.Description,
		}
	}

	t.logger.WithField("secret_count", len(secrets)).Info("Secrets listed successfully")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Found %d secrets", len(secrets))),
		},
	}, nil
}

// DeleteSecretTool deletes a secret from AWS Secrets Manager
type DeleteSecretTool struct {
	*BaseTool
	awsClient *awsclient.Client
}

// NewDeleteSecretTool creates a new delete secret tool
func NewDeleteSecretTool(awsClient *awsclient.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the secret to delete",
			},
		},
		"required": []string{"name"},
	}

	baseTool := NewBaseTool("delete-secret", "Deletes a secret from AWS Secrets Manager", "secrets", inputSchema, logger)

	return &DeleteSecretTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

// Execute executes the delete secret tool
func (t *DeleteSecretTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, ok := args["name"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: name parameter is required"),
			},
		}, fmt.Errorf("name parameter is required")
	}

	// Delete secret
	input := &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(name),
	}

	_, err := t.awsClient.SecretsManager.DeleteSecret(ctx, input)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Failed to delete secret: %v", err)),
			},
		}, fmt.Errorf("failed to delete secret: %w", err)
	}

	t.logger.WithField("secret_name", name).Info("Secret deleted successfully")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("Secret deleted successfully"),
		},
	}, nil
}
