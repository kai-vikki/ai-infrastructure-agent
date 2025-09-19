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
// COMPUTE AGENT IMPLEMENTATION
// =============================================================================

// ComputeAgent handles compute-related infrastructure tasks
type ComputeAgent struct {
	*BaseAgent
	taskQueue        chan *Task
	ec2Tools         map[string]interfaces.MCPTool
	asgTools         map[string]interfaces.MCPTool
	albTools         map[string]interfaces.MCPTool
	amiTools         map[string]interfaces.MCPTool
	zoneTools        map[string]interfaces.MCPTool
	awsClient        *aws.Client
	logger           *logging.Logger
}

// NewComputeAgent creates a new compute agent
func NewComputeAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *ComputeAgent {
	agent := &ComputeAgent{
		BaseAgent:    baseAgent,
		taskQueue:    make(chan *Task, 50),
		ec2Tools:     make(map[string]interfaces.MCPTool),
		asgTools:     make(map[string]interfaces.MCPTool),
		albTools:     make(map[string]interfaces.MCPTool),
		amiTools:     make(map[string]interfaces.MCPTool),
		zoneTools:    make(map[string]interfaces.MCPTool),
		awsClient:    awsClient,
		logger:       logger,
	}

	// Set agent type
	agent.agentType = AgentTypeCompute
	agent.name = "Compute Agent"
	agent.description = "Handles EC2 instances, auto scaling groups, load balancers, and compute infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "ec2_management",
		Description: "Create and manage EC2 instances",
		Tools:       []string{"create-ec2-instance", "list-ec2-instances", "start-ec2-instance", "stop-ec2-instance"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "asg_management",
		Description: "Create and manage auto scaling groups",
		Tools:       []string{"create-auto-scaling-group", "list-auto-scaling-groups", "create-launch-template"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "alb_management",
		Description: "Create and manage application load balancers",
		Tools:       []string{"create-load-balancer", "create-target-group", "create-listener"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "ami_management",
		Description: "Manage AMIs and instance images",
		Tools:       []string{"get-latest-amazon-linux-ami", "get-latest-ubuntu-ami", "list-amis"},
	})

	// Initialize compute tools
	agent.initializeComputeTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

// GetAgentType returns the agent type
func (ca *ComputeAgent) GetAgentType() AgentType {
	return AgentTypeCompute
}

// GetCapabilities returns the agent's capabilities
func (ca *ComputeAgent) GetCapabilities() []AgentCapability {
	return ca.capabilities
}

// GetSpecializedTools returns tools specific to compute management
func (ca *ComputeAgent) GetSpecializedTools() []interfaces.MCPTool {
	var computeTools []interfaces.MCPTool
	
	// Add EC2 tools
	for _, tool := range ca.ec2Tools {
		computeTools = append(computeTools, tool)
	}
	
	// Add ASG tools
	for _, tool := range ca.asgTools {
		computeTools = append(computeTools, tool)
	}
	
	// Add ALB tools
	for _, tool := range ca.albTools {
		computeTools = append(computeTools, tool)
	}
	
	// Add AMI tools
	for _, tool := range ca.amiTools {
		computeTools = append(computeTools, tool)
	}
	
	// Add zone tools
	for _, tool := range ca.zoneTools {
		computeTools = append(computeTools, tool)
	}
	
	return computeTools
}

// ProcessTask processes a compute-related task
func (ca *ComputeAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithFields(map[string]interface{}{
		"agent_id":   ca.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing compute task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_ec2_instance":
		return ca.createEC2Instance(ctx, task)
	case "create_auto_scaling_group":
		return ca.createAutoScalingGroup(ctx, task)
	case "create_launch_template":
		return ca.createLaunchTemplate(ctx, task)
	case "create_load_balancer":
		return ca.createLoadBalancer(ctx, task)
	case "create_target_group":
		return ca.createTargetGroup(ctx, task)
	case "create_listener":
		return ca.createListener(ctx, task)
	case "get_latest_ami":
		return ca.getLatestAMI(ctx, task)
	case "get_availability_zones":
		return ca.getAvailabilityZones(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ca *ComputeAgent) CanHandleTask(task *Task) bool {
	computeTaskTypes := []string{
		"create_ec2_instance", "create_auto_scaling_group", "create_launch_template",
		"create_load_balancer", "create_target_group", "create_listener",
		"get_latest_ami", "get_availability_zones", "start_ec2_instance",
		"stop_ec2_instance", "terminate_ec2_instance", "create_ami_from_instance",
	}

	for _, taskType := range computeTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (ca *ComputeAgent) GetTaskQueue() chan *Task {
	return ca.taskQueue
}

// CoordinateWith coordinates with another agent
func (ca *ComputeAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Compute agents coordinate with network and security agents
	switch otherAgent.GetAgentType() {
	case AgentTypeNetwork:
		ca.logger.WithFields(map[string]interface{}{
			"agent_id":      ca.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with network agent for compute configuration")
		
		// Compute agent requests network information from network agent
		return ca.requestNetworkInfoFromNetwork(otherAgent)
		
	case AgentTypeSecurity:
		ca.logger.WithFields(map[string]interface{}{
			"agent_id":      ca.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with security agent for compute security")
		
		// Compute agent requests security group information from security agent
		return ca.requestSecurityInfoFromSecurity(otherAgent)
		
	default:
		ca.logger.WithFields(map[string]interface{}{
			"agent_id":      ca.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
	}
	return nil
}

// RequestHelp requests help from another agent
func (ca *ComputeAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ca.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from compute agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (ca *ComputeAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ca.id,
		To:        request.From,
		Success:   true,
		Content:   "Compute expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// COMPUTE TOOL INITIALIZATION
// =============================================================================

// initializeComputeTools initializes all compute-related tools
func (ca *ComputeAgent) initializeComputeTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(ca.awsClient, ca.logger)
	
	// Initialize EC2 tools
	ca.initializeEC2Tools(toolFactory)
	
	// Initialize ASG tools
	ca.initializeASGTools(toolFactory)
	
	// Initialize ALB tools
	ca.initializeALBTools(toolFactory)
	
	// Initialize AMI tools
	ca.initializeAMITools(toolFactory)
	
	// Initialize zone tools
	ca.initializeZoneTools(toolFactory)
}

// initializeEC2Tools initializes EC2-related tools
func (ca *ComputeAgent) initializeEC2Tools(toolFactory interfaces.ToolFactory) {
	ec2ToolTypes := []string{
		"create-ec2-instance",
		"list-ec2-instances",
		"start-ec2-instance",
		"stop-ec2-instance",
		"terminate-ec2-instance",
		"create-ami-from-instance",
	}
	
	for _, toolType := range ec2ToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ca.awsClient,
		})
		if err != nil {
			ca.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create EC2 tool")
			continue
		}
		
		ca.ec2Tools[toolType] = tool
		ca.AddTool(tool)
	}
}

// initializeASGTools initializes ASG-related tools
func (ca *ComputeAgent) initializeASGTools(toolFactory interfaces.ToolFactory) {
	asgToolTypes := []string{
		"create-launch-template",
		"create-auto-scaling-group",
		"list-auto-scaling-groups",
		"list-launch-templates",
	}
	
	for _, toolType := range asgToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ca.awsClient,
		})
		if err != nil {
			ca.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create ASG tool")
			continue
		}
		
		ca.asgTools[toolType] = tool
		ca.AddTool(tool)
	}
}

// initializeALBTools initializes ALB-related tools
func (ca *ComputeAgent) initializeALBTools(toolFactory interfaces.ToolFactory) {
	albToolTypes := []string{
		"create-load-balancer",
		"create-target-group",
		"create-listener",
		"list-load-balancers",
		"list-target-groups",
		"register-targets",
		"deregister-targets",
	}
	
	for _, toolType := range albToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ca.awsClient,
		})
		if err != nil {
			ca.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create ALB tool")
			continue
		}
		
		ca.albTools[toolType] = tool
		ca.AddTool(tool)
	}
}

// initializeAMITools initializes AMI-related tools
func (ca *ComputeAgent) initializeAMITools(toolFactory interfaces.ToolFactory) {
	amiToolTypes := []string{
		"get-latest-amazon-linux-ami",
		"get-latest-ubuntu-ami",
		"get-latest-windows-ami",
		"list-amis",
	}
	
	for _, toolType := range amiToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ca.awsClient,
		})
		if err != nil {
			ca.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create AMI tool")
			continue
		}
		
		ca.amiTools[toolType] = tool
		ca.AddTool(tool)
	}
}

// initializeZoneTools initializes zone-related tools
func (ca *ComputeAgent) initializeZoneTools(toolFactory interfaces.ToolFactory) {
	zoneToolTypes := []string{
		"get-availability-zones",
	}
	
	for _, toolType := range zoneToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ca.awsClient,
		})
		if err != nil {
			ca.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create zone tool")
			continue
		}
		
		ca.zoneTools[toolType] = tool
		ca.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createEC2Instance creates a new EC2 instance
func (ca *ComputeAgent) createEC2Instance(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating EC2 instance")
	
	// Extract parameters
	instanceType := "t3.micro"
	if instanceTypeParam, exists := task.Parameters["instanceType"]; exists {
		if instanceTypeStr, ok := instanceTypeParam.(string); ok {
			instanceType = instanceTypeStr
		}
	}
	
	amiId := ""
	if ami, exists := task.Parameters["amiId"]; exists {
		if amiStr, ok := ami.(string); ok {
			amiId = amiStr
		}
	}
	
	// If no AMI specified, get latest Amazon Linux AMI
	if amiId == "" {
		amiTool, exists := ca.amiTools["get-latest-amazon-linux-ami"]
		if exists {
			result, err := amiTool.Execute(ctx, map[string]interface{}{})
			if err == nil && len(result.Content) > 0 {
				amiId = result.Content[0].Text
			}
		}
	}
	
	if amiId == "" {
		task.Status = TaskStatusFailed
		task.Error = "AMI ID is required for EC2 instance creation"
		return task, fmt.Errorf("AMI ID is required for EC2 instance creation")
	}
	
	subnetId := ""
	if subnet, exists := task.Parameters["subnetId"]; exists {
		if subnetStr, ok := subnet.(string); ok {
			subnetId = subnetStr
		}
	}
	
	securityGroupIds := []string{}
	if securityGroups, exists := task.Parameters["securityGroupIds"]; exists {
		if sgList, ok := securityGroups.([]interface{}); ok {
			for _, sg := range sgList {
				if sgStr, ok := sg.(string); ok {
					securityGroupIds = append(securityGroupIds, sgStr)
				}
			}
		}
	}
	
	keyName := ""
	if key, exists := task.Parameters["keyName"]; exists {
		if keyStr, ok := key.(string); ok {
			keyName = keyStr
		}
	}
	
	name := "compute-agent-instance"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Use EC2 tool to create instance
	tool, exists := ca.ec2Tools["create-ec2-instance"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "EC2 instance creation tool not available"
		return task, fmt.Errorf("EC2 instance creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"instanceType":      instanceType,
		"amiId":            amiId,
		"subnetId":         subnetId,
		"securityGroupIds": securityGroupIds,
		"keyName":          keyName,
		"name":             name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "compute-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create EC2 instance: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"instance_id":       result.Content[0].Text,
		"instance_type":     instanceType,
		"ami_id":            amiId,
		"subnet_id":         subnetId,
		"security_group_ids": securityGroupIds,
		"key_name":          keyName,
		"name":              name,
		"status":            "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":      task.ID,
		"instance_id":  result.Content[0].Text,
	}).Info("EC2 instance created successfully")
	
	return task, nil
}

// createAutoScalingGroup creates a new auto scaling group
func (ca *ComputeAgent) createAutoScalingGroup(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating auto scaling group")
	
	// Extract parameters
	asgName := "compute-agent-asg"
	if nameParam, exists := task.Parameters["asgName"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			asgName = nameStr
		}
	}
	
	launchTemplateId := ""
	if launchTemplate, exists := task.Parameters["launchTemplateId"]; exists {
		if launchTemplateStr, ok := launchTemplate.(string); ok {
			launchTemplateId = launchTemplateStr
		}
	}
	
	if launchTemplateId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Launch template ID is required for ASG creation"
		return task, fmt.Errorf("launch template ID is required for ASG creation")
	}
	
	minSize := 1
	if min, exists := task.Parameters["minSize"]; exists {
		if minInt, ok := min.(int); ok {
			minSize = minInt
		}
	}
	
	maxSize := 3
	if max, exists := task.Parameters["maxSize"]; exists {
		if maxInt, ok := max.(int); ok {
			maxSize = maxInt
		}
	}
	
	desiredCapacity := 2
	if desired, exists := task.Parameters["desiredCapacity"]; exists {
		if desiredInt, ok := desired.(int); ok {
			desiredCapacity = desiredInt
		}
	}
	
	subnetIds := []string{}
	if subnets, exists := task.Parameters["subnetIds"]; exists {
		if subnetList, ok := subnets.([]interface{}); ok {
			for _, subnet := range subnetList {
				if subnetStr, ok := subnet.(string); ok {
					subnetIds = append(subnetIds, subnetStr)
				}
			}
		}
	}
	
	// Use ASG tool to create auto scaling group
	tool, exists := ca.asgTools["create-auto-scaling-group"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "ASG creation tool not available"
		return task, fmt.Errorf("ASG creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"asgName":         asgName,
		"launchTemplateId": launchTemplateId,
		"minSize":         minSize,
		"maxSize":         maxSize,
		"desiredCapacity": desiredCapacity,
		"subnetIds":       subnetIds,
		"tags": map[string]string{
			"Name":        asgName,
			"CreatedBy":   "compute-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create auto scaling group: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"asg_name":          asgName,
		"launch_template_id": launchTemplateId,
		"min_size":          minSize,
		"max_size":          maxSize,
		"desired_capacity":  desiredCapacity,
		"subnet_ids":        subnetIds,
		"status":            "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":  task.ID,
		"asg_name": asgName,
	}).Info("Auto scaling group created successfully")
	
	return task, nil
}

// createLaunchTemplate creates a new launch template
func (ca *ComputeAgent) createLaunchTemplate(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating launch template")
	
	// Extract parameters
	templateName := "compute-agent-launch-template"
	if nameParam, exists := task.Parameters["templateName"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			templateName = nameStr
		}
	}
	
	instanceType := "t3.micro"
	if instanceTypeParam, exists := task.Parameters["instanceType"]; exists {
		if instanceTypeStr, ok := instanceTypeParam.(string); ok {
			instanceType = instanceTypeStr
		}
	}
	
	amiId := ""
	if ami, exists := task.Parameters["amiId"]; exists {
		if amiStr, ok := ami.(string); ok {
			amiId = amiStr
		}
	}
	
	// If no AMI specified, get latest Amazon Linux AMI
	if amiId == "" {
		amiTool, exists := ca.amiTools["get-latest-amazon-linux-ami"]
		if exists {
			result, err := amiTool.Execute(ctx, map[string]interface{}{})
			if err == nil && len(result.Content) > 0 {
				amiId = result.Content[0].Text
			}
		}
	}
	
	if amiId == "" {
		task.Status = TaskStatusFailed
		task.Error = "AMI ID is required for launch template creation"
		return task, fmt.Errorf("AMI ID is required for launch template creation")
	}
	
	securityGroupIds := []string{}
	if securityGroups, exists := task.Parameters["securityGroupIds"]; exists {
		if sgList, ok := securityGroups.([]interface{}); ok {
			for _, sg := range sgList {
				if sgStr, ok := sg.(string); ok {
					securityGroupIds = append(securityGroupIds, sgStr)
				}
			}
		}
	}
	
	keyName := ""
	if key, exists := task.Parameters["keyName"]; exists {
		if keyStr, ok := key.(string); ok {
			keyName = keyStr
		}
	}
	
	// Use launch template tool to create launch template
	tool, exists := ca.asgTools["create-launch-template"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Launch template creation tool not available"
		return task, fmt.Errorf("launch template creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"templateName":     templateName,
		"instanceType":     instanceType,
		"amiId":            amiId,
		"securityGroupIds": securityGroupIds,
		"keyName":          keyName,
		"tags": map[string]string{
			"Name":        templateName,
			"CreatedBy":   "compute-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create launch template: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"launch_template_id": result.Content[0].Text,
		"template_name":      templateName,
		"instance_type":      instanceType,
		"ami_id":             amiId,
		"security_group_ids": securityGroupIds,
		"key_name":           keyName,
		"status":             "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":           task.ID,
		"launch_template_id": result.Content[0].Text,
	}).Info("Launch template created successfully")
	
	return task, nil
}

// createLoadBalancer creates a new application load balancer
func (ca *ComputeAgent) createLoadBalancer(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating load balancer")
	
	// Extract parameters
	lbName := "compute-agent-alb"
	if nameParam, exists := task.Parameters["lbName"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			lbName = nameStr
		}
	}
	
	subnetIds := []string{}
	if subnets, exists := task.Parameters["subnetIds"]; exists {
		if subnetList, ok := subnets.([]interface{}); ok {
			for _, subnet := range subnetList {
				if subnetStr, ok := subnet.(string); ok {
					subnetIds = append(subnetIds, subnetStr)
				}
			}
		}
	}
	
	securityGroupIds := []string{}
	if securityGroups, exists := task.Parameters["securityGroupIds"]; exists {
		if sgList, ok := securityGroups.([]interface{}); ok {
			for _, sg := range sgList {
				if sgStr, ok := sg.(string); ok {
					securityGroupIds = append(securityGroupIds, sgStr)
				}
			}
		}
	}
	
	scheme := "internet-facing"
	if schemeParam, exists := task.Parameters["scheme"]; exists {
		if schemeStr, ok := schemeParam.(string); ok {
			scheme = schemeStr
		}
	}
	
	// Use ALB tool to create load balancer
	tool, exists := ca.albTools["create-load-balancer"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Load balancer creation tool not available"
		return task, fmt.Errorf("load balancer creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"lbName":           lbName,
		"subnetIds":        subnetIds,
		"securityGroupIds": securityGroupIds,
		"scheme":           scheme,
		"tags": map[string]string{
			"Name":        lbName,
			"CreatedBy":   "compute-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create load balancer: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"load_balancer_arn": result.Content[0].Text,
		"lb_name":           lbName,
		"subnet_ids":        subnetIds,
		"security_group_ids": securityGroupIds,
		"scheme":            scheme,
		"status":            "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":           task.ID,
		"load_balancer_arn": result.Content[0].Text,
	}).Info("Load balancer created successfully")
	
	return task, nil
}

// createTargetGroup creates a new target group
func (ca *ComputeAgent) createTargetGroup(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating target group")
	
	// Extract parameters
	tgName := "compute-agent-tg"
	if nameParam, exists := task.Parameters["tgName"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			tgName = nameStr
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
		task.Error = "VPC ID is required for target group creation"
		return task, fmt.Errorf("VPC ID is required for target group creation")
	}
	
	port := 80
	if portParam, exists := task.Parameters["port"]; exists {
		if portInt, ok := portParam.(int); ok {
			port = portInt
		}
	}
	
	protocol := "HTTP"
	if protocolParam, exists := task.Parameters["protocol"]; exists {
		if protocolStr, ok := protocolParam.(string); ok {
			protocol = protocolStr
		}
	}
	
	// Use target group tool to create target group
	tool, exists := ca.albTools["create-target-group"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Target group creation tool not available"
		return task, fmt.Errorf("target group creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"tgName":   tgName,
		"vpcId":    vpcId,
		"port":     port,
		"protocol": protocol,
		"tags": map[string]string{
			"Name":        tgName,
			"CreatedBy":   "compute-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create target group: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"target_group_arn": result.Content[0].Text,
		"tg_name":          tgName,
		"vpc_id":           vpcId,
		"port":             port,
		"protocol":         protocol,
		"status":           "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":          task.ID,
		"target_group_arn": result.Content[0].Text,
	}).Info("Target group created successfully")
	
	return task, nil
}

// createListener creates a new listener
func (ca *ComputeAgent) createListener(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Creating listener")
	
	// Extract parameters
	loadBalancerArn := ""
	if lbArn, exists := task.Parameters["loadBalancerArn"]; exists {
		if lbArnStr, ok := lbArn.(string); ok {
			loadBalancerArn = lbArnStr
		}
	}
	
	if loadBalancerArn == "" {
		task.Status = TaskStatusFailed
		task.Error = "Load balancer ARN is required for listener creation"
		return task, fmt.Errorf("load balancer ARN is required for listener creation")
	}
	
	targetGroupArn := ""
	if tgArn, exists := task.Parameters["targetGroupArn"]; exists {
		if tgArnStr, ok := tgArn.(string); ok {
			targetGroupArn = tgArnStr
		}
	}
	
	if targetGroupArn == "" {
		task.Status = TaskStatusFailed
		task.Error = "Target group ARN is required for listener creation"
		return task, fmt.Errorf("target group ARN is required for listener creation")
	}
	
	port := 80
	if portParam, exists := task.Parameters["port"]; exists {
		if portInt, ok := portParam.(int); ok {
			port = portInt
		}
	}
	
	protocol := "HTTP"
	if protocolParam, exists := task.Parameters["protocol"]; exists {
		if protocolStr, ok := protocolParam.(string); ok {
			protocol = protocolStr
		}
	}
	
	// Use listener tool to create listener
	tool, exists := ca.albTools["create-listener"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Listener creation tool not available"
		return task, fmt.Errorf("listener creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"loadBalancerArn": loadBalancerArn,
		"targetGroupArn":  targetGroupArn,
		"port":            port,
		"protocol":        protocol,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create listener: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"listener_arn":      result.Content[0].Text,
		"load_balancer_arn": loadBalancerArn,
		"target_group_arn":  targetGroupArn,
		"port":              port,
		"protocol":          protocol,
		"status":            "created",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id":       task.ID,
		"listener_arn":  result.Content[0].Text,
	}).Info("Listener created successfully")
	
	return task, nil
}

// getLatestAMI gets the latest AMI for a specific OS
func (ca *ComputeAgent) getLatestAMI(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Getting latest AMI")
	
	// Extract parameters
	osType := "amazon-linux"
	if os, exists := task.Parameters["osType"]; exists {
		if osStr, ok := os.(string); ok {
			osType = osStr
		}
	}
	
	// Determine tool based on OS type
	var toolType string
	switch osType {
	case "amazon-linux":
		toolType = "get-latest-amazon-linux-ami"
	case "ubuntu":
		toolType = "get-latest-ubuntu-ami"
	case "windows":
		toolType = "get-latest-windows-ami"
	default:
		toolType = "get-latest-amazon-linux-ami"
	}
	
	// Use AMI tool to get latest AMI
	tool, exists := ca.amiTools[toolType]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("AMI tool %s not available", toolType)
		return task, fmt.Errorf("AMI tool %s not available", toolType)
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to get latest AMI: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"ami_id":  result.Content[0].Text,
		"os_type": osType,
		"status":  "retrieved",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id": task.ID,
		"ami_id":  result.Content[0].Text,
		"os_type": osType,
	}).Info("Latest AMI retrieved successfully")
	
	return task, nil
}

// getAvailabilityZones gets available availability zones
func (ca *ComputeAgent) getAvailabilityZones(ctx context.Context, task *Task) (*Task, error) {
	ca.logger.WithField("task_id", task.ID).Info("Getting availability zones")
	
	// Use zone tool to get availability zones
	tool, exists := ca.zoneTools["get-availability-zones"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Availability zone tool not available"
		return task, fmt.Errorf("availability zone tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to get availability zones: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"availability_zones": result.Content[0].Text,
		"status":             "retrieved",
	}
	
	ca.logger.WithFields(map[string]interface{}{
		"task_id": task.ID,
	}).Info("Availability zones retrieved successfully")
	
	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// requestNetworkInfoFromNetwork requests network information from network agent
func (ca *ComputeAgent) requestNetworkInfoFromNetwork(networkAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message requesting network information
	// For now, we'll log the coordination
	ca.logger.WithFields(map[string]interface{}{
		"agent_id":       ca.id,
		"network_agent":  networkAgent.GetInfo().ID,
	}).Info("Requesting network information from network agent")
	
	return nil
}

// requestSecurityInfoFromSecurity requests security information from security agent
func (ca *ComputeAgent) requestSecurityInfoFromSecurity(securityAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message requesting security information
	// For now, we'll log the coordination
	ca.logger.WithFields(map[string]interface{}{
		"agent_id":       ca.id,
		"security_agent": securityAgent.GetInfo().ID,
	}).Info("Requesting security information from security agent")
	
	return nil
}
