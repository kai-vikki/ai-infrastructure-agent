package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
)

// =============================================================================
// SECURITY AGENT IMPLEMENTATION
// =============================================================================

// SecurityAgent handles security-related infrastructure tasks
type SecurityAgent struct {
	*BaseAgent
	taskQueue        chan *Task
	securityGroupTools map[string]interfaces.MCPTool
	iamTools         map[string]interfaces.MCPTool
	kmsTools         map[string]interfaces.MCPTool
	secretsTools     map[string]interfaces.MCPTool
	awsClient        *aws.Client
	logger           *logging.Logger
}

// NewSecurityAgent creates a new security agent
func NewSecurityAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *SecurityAgent {
	agent := &SecurityAgent{
		BaseAgent:         baseAgent,
		taskQueue:         make(chan *Task, 50),
		securityGroupTools: make(map[string]interfaces.MCPTool),
		iamTools:          make(map[string]interfaces.MCPTool),
		kmsTools:          make(map[string]interfaces.MCPTool),
		secretsTools:      make(map[string]interfaces.MCPTool),
		awsClient:         awsClient,
		logger:            logger,
	}

	// Set agent type
	agent.agentType = AgentTypeSecurity
	agent.name = "Security Agent"
	agent.description = "Handles security groups, IAM policies, KMS keys, and security infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "security_group_management",
		Description: "Create and manage security groups",
		Tools:       []string{"create-security-group", "add-security-group-ingress-rule", "add-security-group-egress-rule"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "iam_management",
		Description: "Create and manage IAM policies and roles",
		Tools:       []string{"create-role", "attach-policy", "create-policy"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "kms_management",
		Description: "Create and manage KMS keys",
		Tools:       []string{"create-kms-key", "list-kms-keys", "enable-kms-key"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "secrets_management",
		Description: "Create and manage secrets",
		Tools:       []string{"create-secret", "get-secret", "update-secret"},
	})

	// Initialize security tools
	agent.initializeSecurityTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

// GetAgentType returns the agent type
func (sa *SecurityAgent) GetAgentType() AgentType {
	return AgentTypeSecurity
}

// GetCapabilities returns the agent's capabilities
func (sa *SecurityAgent) GetCapabilities() []AgentCapability {
	return sa.capabilities
}

// GetSpecializedTools returns tools specific to security management
func (sa *SecurityAgent) GetSpecializedTools() []interfaces.MCPTool {
	var securityTools []interfaces.MCPTool
	
	// Add security group tools
	for _, tool := range sa.securityGroupTools {
		securityTools = append(securityTools, tool)
	}
	
	// Add IAM tools
	for _, tool := range sa.iamTools {
		securityTools = append(securityTools, tool)
	}
	
	// Add KMS tools
	for _, tool := range sa.kmsTools {
		securityTools = append(securityTools, tool)
	}
	
	// Add secrets tools
	for _, tool := range sa.secretsTools {
		securityTools = append(securityTools, tool)
	}
	
	return securityTools
}

// ProcessTask processes a security-related task
func (sa *SecurityAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":   sa.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing security task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_security_group":
		return sa.createSecurityGroup(ctx, task)
	case "add_security_group_ingress_rule":
		return sa.addSecurityGroupIngressRule(ctx, task)
	case "add_security_group_egress_rule":
		return sa.addSecurityGroupEgressRule(ctx, task)
	case "create_iam_role":
		return sa.createIAMRole(ctx, task)
	case "create_iam_policy":
		return sa.createIAMPolicy(ctx, task)
	case "attach_iam_policy":
		return sa.attachIAMPolicy(ctx, task)
	case "create_kms_key":
		return sa.createKMSKey(ctx, task)
	case "create_secret":
		return sa.createSecret(ctx, task)
	case "get_secret":
		return sa.getSecret(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (sa *SecurityAgent) CanHandleTask(task *Task) bool {
	securityTaskTypes := []string{
		"create_security_group", "add_security_group_ingress_rule", "add_security_group_egress_rule",
		"create_iam_role", "create_iam_policy", "attach_iam_policy",
		"create_kms_key", "create_secret", "get_secret", "update_secret",
	}

	for _, taskType := range securityTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (sa *SecurityAgent) GetTaskQueue() chan *Task {
	return sa.taskQueue
}

// CoordinateWith coordinates with another agent
func (sa *SecurityAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Security agents coordinate with all other agents
	switch otherAgent.GetAgentType() {
	case AgentTypeNetwork:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":      sa.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with network agent for security configuration")
		
		// Security agent provides security group information to network agent
		return sa.provideSecurityInfoToNetwork(otherAgent)
		
	case AgentTypeCompute:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":      sa.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent for security configuration")
		
		// Security agent provides security group information to compute agent
		return sa.provideSecurityInfoToCompute(otherAgent)
		
	case AgentTypeStorage:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":      sa.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with storage agent for security configuration")
		
		// Security agent provides security group information to storage agent
		return sa.provideSecurityInfoToStorage(otherAgent)
		
	default:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":      sa.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
	}
	return nil
}

// RequestHelp requests help from another agent
func (sa *SecurityAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      sa.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from security agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (sa *SecurityAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      sa.id,
		To:        request.From,
		Success:   true,
		Content:   "Security expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// SECURITY TOOL INITIALIZATION
// =============================================================================

// initializeSecurityTools initializes all security-related tools
func (sa *SecurityAgent) initializeSecurityTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(sa.awsClient, sa.logger)
	
	// Initialize security group tools
	sa.initializeSecurityGroupTools(toolFactory)
	
	// Initialize IAM tools
	sa.initializeIAMTools(toolFactory)
	
	// Initialize KMS tools
	sa.initializeKMSTools(toolFactory)
	
	// Initialize secrets tools
	sa.initializeSecretsTools(toolFactory)
}

// initializeSecurityGroupTools initializes security group-related tools
func (sa *SecurityAgent) initializeSecurityGroupTools(toolFactory interfaces.ToolFactory) {
	securityGroupToolTypes := []string{
		"create-security-group",
		"add-security-group-ingress-rule",
		"add-security-group-egress-rule",
		"list-security-groups",
		"delete-security-group",
	}
	
	for _, toolType := range securityGroupToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create security group tool")
			continue
		}
		
		sa.securityGroupTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeIAMTools initializes IAM-related tools
func (sa *SecurityAgent) initializeIAMTools(toolFactory interfaces.ToolFactory) {
	iamToolTypes := []string{
		"create-role",
		"create-policy",
		"attach-policy",
		"list-roles",
		"list-policies",
	}
	
	for _, toolType := range iamToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create IAM tool")
			continue
		}
		
		sa.iamTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeKMSTools initializes KMS-related tools
func (sa *SecurityAgent) initializeKMSTools(toolFactory interfaces.ToolFactory) {
	kmsToolTypes := []string{
		"create-kms-key",
		"list-kms-keys",
		"enable-kms-key",
		"disable-kms-key",
	}
	
	for _, toolType := range kmsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create KMS tool")
			continue
		}
		
		sa.kmsTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeSecretsTools initializes secrets-related tools
func (sa *SecurityAgent) initializeSecretsTools(toolFactory interfaces.ToolFactory) {
	secretsToolTypes := []string{
		"create-secret",
		"get-secret",
		"update-secret",
		"list-secrets",
		"delete-secret",
	}
	
	for _, toolType := range secretsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create secrets tool")
			continue
		}
		
		sa.secretsTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createSecurityGroup creates a new security group
func (sa *SecurityAgent) createSecurityGroup(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating security group")
	
	// Extract parameters
	groupName := "security-agent-sg"
	if name, exists := task.Parameters["groupName"]; exists {
		if nameStr, ok := name.(string); ok {
			groupName = nameStr
		}
	}
	
	description := "Security group created by security agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}
	
	vpcId := ""
	if vpc, exists := task.Parameters["vpcId"]; exists {
		if vpcStr, ok := vpc.(string); ok {
			vpcId = vpcStr
		}
	}
	
	if vpcId == "" {
		task.Status = TaskStatusFailed
		task.Error = "VPC ID is required for security group creation"
		return task, fmt.Errorf("VPC ID is required for security group creation")
	}
	
	// Use security group tool to create security group
	tool, exists := sa.securityGroupTools["create-security-group"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Security group creation tool not available"
		return task, fmt.Errorf("security group creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"groupName":   groupName,
		"description": description,
		"vpcId":       vpcId,
		"tags": map[string]string{
			"Name":        groupName,
			"CreatedBy":   "security-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create security group: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"security_group_id": result.Content[0].Text,
		"group_name":        groupName,
		"description":       description,
		"vpc_id":            vpcId,
		"status":            "created",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":          task.ID,
		"security_group_id": result.Content[0].Text,
	}).Info("Security group created successfully")
	
	return task, nil
}

// addSecurityGroupIngressRule adds an ingress rule to a security group
func (sa *SecurityAgent) addSecurityGroupIngressRule(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Adding security group ingress rule")
	
	// Extract parameters
	groupId := ""
	if group, exists := task.Parameters["groupId"]; exists {
		if groupStr, ok := group.(string); ok {
			groupId = groupStr
		}
	}
	
	if groupId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Security group ID is required for adding ingress rule"
		return task, fmt.Errorf("security group ID is required for adding ingress rule")
	}
	
	protocol := "tcp"
	if protocolParam, exists := task.Parameters["protocol"]; exists {
		if protocolStr, ok := protocolParam.(string); ok {
			protocol = protocolStr
		}
	}
	
	port := 80
	if portParam, exists := task.Parameters["port"]; exists {
		if portInt, ok := portParam.(int); ok {
			port = portInt
		}
	}
	
	cidrBlocks := []string{"0.0.0.0/0"}
	if cidr, exists := task.Parameters["cidrBlocks"]; exists {
		if cidrList, ok := cidr.([]interface{}); ok {
			cidrBlocks = []string{}
			for _, cidrItem := range cidrList {
				if cidrStr, ok := cidrItem.(string); ok {
					cidrBlocks = append(cidrBlocks, cidrStr)
				}
			}
		}
	}
	
	// Use security group tool to add ingress rule
	tool, exists := sa.securityGroupTools["add-security-group-ingress-rule"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Security group ingress rule tool not available"
		return task, fmt.Errorf("security group ingress rule tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"groupId":    groupId,
		"protocol":   protocol,
		"port":       port,
		"cidrBlocks": cidrBlocks,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to add security group ingress rule: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"security_group_id": groupId,
		"protocol":          protocol,
		"port":              port,
		"cidr_blocks":       cidrBlocks,
		"status":            "added",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":          task.ID,
		"security_group_id": groupId,
		"protocol":         protocol,
		"port":             port,
	}).Info("Security group ingress rule added successfully")
	
	return task, nil
}

// addSecurityGroupEgressRule adds an egress rule to a security group
func (sa *SecurityAgent) addSecurityGroupEgressRule(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Adding security group egress rule")
	
	// Extract parameters
	groupId := ""
	if group, exists := task.Parameters["groupId"]; exists {
		if groupStr, ok := group.(string); ok {
			groupId = groupStr
		}
	}
	
	if groupId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Security group ID is required for adding egress rule"
		return task, fmt.Errorf("security group ID is required for adding egress rule")
	}
	
	protocol := "tcp"
	if protocolParam, exists := task.Parameters["protocol"]; exists {
		if protocolStr, ok := protocolParam.(string); ok {
			protocol = protocolStr
		}
	}
	
	port := 80
	if portParam, exists := task.Parameters["port"]; exists {
		if portInt, ok := portParam.(int); ok {
			port = portInt
		}
	}
	
	cidrBlocks := []string{"0.0.0.0/0"}
	if cidr, exists := task.Parameters["cidrBlocks"]; exists {
		if cidrList, ok := cidr.([]interface{}); ok {
			cidrBlocks = []string{}
			for _, cidrItem := range cidrList {
				if cidrStr, ok := cidrItem.(string); ok {
					cidrBlocks = append(cidrBlocks, cidrStr)
				}
			}
		}
	}
	
	// Use security group tool to add egress rule
	tool, exists := sa.securityGroupTools["add-security-group-egress-rule"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Security group egress rule tool not available"
		return task, fmt.Errorf("security group egress rule tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"groupId":    groupId,
		"protocol":   protocol,
		"port":       port,
		"cidrBlocks": cidrBlocks,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to add security group egress rule: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"security_group_id": groupId,
		"protocol":          protocol,
		"port":              port,
		"cidr_blocks":       cidrBlocks,
		"status":            "added",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":          task.ID,
		"security_group_id": groupId,
		"protocol":         protocol,
		"port":             port,
	}).Info("Security group egress rule added successfully")
	
	return task, nil
}

// createIAMRole creates a new IAM role
func (sa *SecurityAgent) createIAMRole(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating IAM role")
	
	// Extract parameters
	roleName := "security-agent-role"
	if name, exists := task.Parameters["roleName"]; exists {
		if nameStr, ok := name.(string); ok {
			roleName = nameStr
		}
	}
	
	assumeRolePolicyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`
	
	if policy, exists := task.Parameters["assumeRolePolicyDocument"]; exists {
		if policyStr, ok := policy.(string); ok {
			assumeRolePolicyDocument = policyStr
		}
	}
	
	description := "IAM role created by security agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}
	
	// Use IAM tool to create role
	tool, exists := sa.iamTools["create-role"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "IAM role creation tool not available"
		return task, fmt.Errorf("IAM role creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"roleName":                 roleName,
		"assumeRolePolicyDocument": assumeRolePolicyDocument,
		"description":              description,
		"tags": map[string]string{
			"Name":        roleName,
			"CreatedBy":   "security-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create IAM role: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"role_arn":  result.Content[0].Text,
		"role_name": roleName,
		"description": description,
		"status":     "created",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"role_arn":  result.Content[0].Text,
		"role_name": roleName,
	}).Info("IAM role created successfully")
	
	return task, nil
}

// createIAMPolicy creates a new IAM policy
func (sa *SecurityAgent) createIAMPolicy(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating IAM policy")
	
	// Extract parameters
	policyName := "security-agent-policy"
	if name, exists := task.Parameters["policyName"]; exists {
		if nameStr, ok := name.(string); ok {
			policyName = nameStr
		}
	}
	
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"s3:GetObject",
					"s3:PutObject"
				],
				"Resource": "*"
			}
		]
	}`
	
	if policy, exists := task.Parameters["policyDocument"]; exists {
		if policyStr, ok := policy.(string); ok {
			policyDocument = policyStr
		}
	}
	
	description := "IAM policy created by security agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}
	
	// Use IAM tool to create policy
	tool, exists := sa.iamTools["create-policy"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "IAM policy creation tool not available"
		return task, fmt.Errorf("IAM policy creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"policyName":     policyName,
		"policyDocument": policyDocument,
		"description":    description,
		"tags": map[string]string{
			"Name":        policyName,
			"CreatedBy":   "security-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create IAM policy: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"policy_arn":  result.Content[0].Text,
		"policy_name": policyName,
		"description": description,
		"status":      "created",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"policy_arn":  result.Content[0].Text,
		"policy_name": policyName,
	}).Info("IAM policy created successfully")
	
	return task, nil
}

// attachIAMPolicy attaches an IAM policy to a role
func (sa *SecurityAgent) attachIAMPolicy(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Attaching IAM policy")
	
	// Extract parameters
	roleName := ""
	if role, exists := task.Parameters["roleName"]; exists {
		if roleStr, ok := role.(string); ok {
			roleName = roleStr
		}
	}
	
	if roleName == "" {
		task.Status = TaskStatusFailed
		task.Error = "Role name is required for policy attachment"
		return task, fmt.Errorf("role name is required for policy attachment")
	}
	
	policyArn := ""
	if policy, exists := task.Parameters["policyArn"]; exists {
		if policyStr, ok := policy.(string); ok {
			policyArn = policyStr
		}
	}
	
	if policyArn == "" {
		task.Status = TaskStatusFailed
		task.Error = "Policy ARN is required for policy attachment"
		return task, fmt.Errorf("policy ARN is required for policy attachment")
	}
	
	// Use IAM tool to attach policy
	tool, exists := sa.iamTools["attach-policy"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "IAM policy attachment tool not available"
		return task, fmt.Errorf("IAM policy attachment tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"roleName":  roleName,
		"policyArn": policyArn,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to attach IAM policy: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"role_name":  roleName,
		"policy_arn": policyArn,
		"status":     "attached",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"role_name":   roleName,
		"policy_arn":  policyArn,
	}).Info("IAM policy attached successfully")
	
	return task, nil
}

// createKMSKey creates a new KMS key
func (sa *SecurityAgent) createKMSKey(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating KMS key")
	
	// Extract parameters
	description := "KMS key created by security agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}
	
	keyUsage := "ENCRYPT_DECRYPT"
	if usage, exists := task.Parameters["keyUsage"]; exists {
		if usageStr, ok := usage.(string); ok {
			keyUsage = usageStr
		}
	}
	
	keySpec := "SYMMETRIC_DEFAULT"
	if spec, exists := task.Parameters["keySpec"]; exists {
		if specStr, ok := spec.(string); ok {
			keySpec = specStr
		}
	}
	
	// Use KMS tool to create key
	tool, exists := sa.kmsTools["create-kms-key"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "KMS key creation tool not available"
		return task, fmt.Errorf("KMS key creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"description": description,
		"keyUsage":    keyUsage,
		"keySpec":     keySpec,
		"tags": map[string]string{
			"Name":        "security-agent-kms-key",
			"CreatedBy":   "security-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create KMS key: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"key_id":      result.Content[0].Text,
		"description": description,
		"key_usage":   keyUsage,
		"key_spec":    keySpec,
		"status":      "created",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id": task.ID,
		"key_id":  result.Content[0].Text,
	}).Info("KMS key created successfully")
	
	return task, nil
}

// createSecret creates a new secret
func (sa *SecurityAgent) createSecret(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating secret")
	
	// Extract parameters
	secretName := "security-agent-secret"
	if name, exists := task.Parameters["secretName"]; exists {
		if nameStr, ok := name.(string); ok {
			secretName = nameStr
		}
	}
	
	secretValue := "default-secret-value"
	if value, exists := task.Parameters["secretValue"]; exists {
		if valueStr, ok := value.(string); ok {
			secretValue = valueStr
		}
	}
	
	description := "Secret created by security agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}
	
	// Use secrets tool to create secret
	tool, exists := sa.secretsTools["create-secret"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Secret creation tool not available"
		return task, fmt.Errorf("secret creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"secretName":  secretName,
		"secretValue": secretValue,
		"description": description,
		"tags": map[string]string{
			"Name":        secretName,
			"CreatedBy":   "security-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create secret: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"secret_arn":  result.Content[0].Text,
		"secret_name": secretName,
		"description": description,
		"status":      "created",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"secret_arn":  result.Content[0].Text,
		"secret_name": secretName,
	}).Info("Secret created successfully")
	
	return task, nil
}

// getSecret retrieves a secret value
func (sa *SecurityAgent) getSecret(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Getting secret")
	
	// Extract parameters
	secretName := ""
	if name, exists := task.Parameters["secretName"]; exists {
		if nameStr, ok := name.(string); ok {
			secretName = nameStr
		}
	}
	
	if secretName == "" {
		task.Status = TaskStatusFailed
		task.Error = "Secret name is required for retrieving secret"
		return task, fmt.Errorf("secret name is required for retrieving secret")
	}
	
	// Use secrets tool to get secret
	tool, exists := sa.secretsTools["get-secret"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Secret retrieval tool not available"
		return task, fmt.Errorf("secret retrieval tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"secretName": secretName,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to get secret: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"secret_name":  secretName,
		"secret_value": result.Content[0].Text,
		"status":       "retrieved",
	}
	
	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"secret_name": secretName,
	}).Info("Secret retrieved successfully")
	
	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// provideSecurityInfoToNetwork provides security information to network agent
func (sa *SecurityAgent) provideSecurityInfoToNetwork(networkAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with security information
	// For now, we'll log the coordination
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"network_agent": networkAgent.GetInfo().ID,
	}).Info("Providing security information to network agent")
	
	return nil
}

// provideSecurityInfoToCompute provides security information to compute agent
func (sa *SecurityAgent) provideSecurityInfoToCompute(computeAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with security information
	// For now, we'll log the coordination
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"compute_agent": computeAgent.GetInfo().ID,
	}).Info("Providing security information to compute agent")
	
	return nil
}

// provideSecurityInfoToStorage provides security information to storage agent
func (sa *SecurityAgent) provideSecurityInfoToStorage(storageAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with security information
	// For now, we'll log the coordination
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"storage_agent": storageAgent.GetInfo().ID,
	}).Info("Providing security information to storage agent")
	
	return nil
}
