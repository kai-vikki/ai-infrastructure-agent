package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// =============================================================================
// SPECIALIZED AGENT IMPLEMENTATIONS
// =============================================================================

// NetworkAgent handles network-related infrastructure tasks
type NetworkAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewNetworkAgent creates a new network agent
func NewNetworkAgent(baseAgent *BaseAgent) *NetworkAgent {
	agent := &NetworkAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeNetwork
	agent.name = "Network Agent"
	agent.description = "Handles VPC, subnets, route tables, and networking infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "vpc_management",
		Description: "Create and manage VPCs",
		Tools:       []string{"create-vpc", "list-vpcs", "delete-vpc"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "subnet_management",
		Description: "Create and manage subnets",
		Tools:       []string{"create-subnet", "list-subnets", "delete-subnet"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "routing_management",
		Description: "Manage route tables and routing",
		Tools:       []string{"create-route-table", "add-route", "associate-route-table"},
	})

	return agent
}

// GetAgentType returns the agent type
func (na *NetworkAgent) GetAgentType() AgentType {
	return AgentTypeNetwork
}

// GetCapabilities returns the agent's capabilities
func (na *NetworkAgent) GetCapabilities() []AgentCapability {
	return na.capabilities
}

// GetSpecializedTools returns tools specific to network management
func (na *NetworkAgent) GetSpecializedTools() []interfaces.MCPTool {
	// Filter tools to only include network-related ones
	var networkTools []interfaces.MCPTool
	for _, tool := range na.tools {
		toolName := tool.Name()
		if isNetworkTool(toolName) {
			networkTools = append(networkTools, tool)
		}
	}
	return networkTools
}

// ProcessTask processes a network-related task
func (na *NetworkAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithFields(map[string]interface{}{
		"agent_id":   na.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing network task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_vpc":
		return na.createVPC(ctx, task)
	case "create_subnet":
		return na.createSubnet(ctx, task)
	case "create_route_table":
		return na.createRouteTable(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (na *NetworkAgent) CanHandleTask(task *Task) bool {
	networkTaskTypes := []string{
		"create_vpc", "create_subnet", "create_route_table",
		"create_internet_gateway", "create_nat_gateway",
		"associate_route_table", "add_route",
	}

	for _, taskType := range networkTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (na *NetworkAgent) GetTaskQueue() chan *Task {
	return na.taskQueue
}

// CoordinateWith coordinates with another agent
func (na *NetworkAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Network agents typically coordinate with compute agents
	if otherAgent.GetAgentType() == AgentTypeCompute {
		na.logger.WithFields(map[string]interface{}{
			"agent_id":      na.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent")
	}
	return nil
}

// RequestHelp requests help from another agent
func (na *NetworkAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	// Network agents might need help from security agents for security group management
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      na.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from network agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (na *NetworkAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	// Network agents can provide networking expertise
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      na.id,
		To:        request.From,
		Success:   true,
		Content:   "Network expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// COMPUTE AGENT
// =============================================================================

// ComputeAgent handles compute-related infrastructure tasks
type ComputeAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewComputeAgent creates a new compute agent
func NewComputeAgent(baseAgent *BaseAgent) *ComputeAgent {
	agent := &ComputeAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeCompute
	agent.name = "Compute Agent"
	agent.description = "Handles EC2 instances, auto scaling groups, and load balancers"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "ec2_management",
		Description: "Create and manage EC2 instances",
		Tools:       []string{"create-ec2-instance", "list-ec2-instances", "start-ec2-instance", "stop-ec2-instance"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "asg_management",
		Description: "Create and manage auto scaling groups",
		Tools:       []string{"create-auto-scaling-group", "list-auto-scaling-groups"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "alb_management",
		Description: "Create and manage application load balancers",
		Tools:       []string{"create-load-balancer", "create-target-group", "create-listener"},
	})

	return agent
}

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
	for _, tool := range ca.tools {
		toolName := tool.Name()
		if isComputeTool(toolName) {
			computeTools = append(computeTools, tool)
		}
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
	case "create_load_balancer":
		return ca.createLoadBalancer(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ca *ComputeAgent) CanHandleTask(task *Task) bool {
	computeTaskTypes := []string{
		"create_ec2_instance", "create_auto_scaling_group", "create_load_balancer",
		"create_target_group", "create_listener", "create_launch_template",
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
		}).Info("Coordinating with network agent")
	case AgentTypeSecurity:
		ca.logger.WithFields(map[string]interface{}{
			"agent_id":      ca.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with security agent")
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
// STORAGE AGENT
// =============================================================================

// StorageAgent handles storage-related infrastructure tasks
type StorageAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewStorageAgent creates a new storage agent
func NewStorageAgent(baseAgent *BaseAgent) *StorageAgent {
	agent := &StorageAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeStorage
	agent.name = "Storage Agent"
	agent.description = "Handles RDS databases, EBS volumes, and storage infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "rds_management",
		Description: "Create and manage RDS databases",
		Tools:       []string{"create-db-instance", "list-db-instances", "start-db-instance", "stop-db-instance"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "ebs_management",
		Description: "Create and manage EBS volumes",
		Tools:       []string{"create-volume", "attach-volume", "detach-volume"},
	})

	return agent
}

// GetAgentType returns the agent type
func (sa *StorageAgent) GetAgentType() AgentType {
	return AgentTypeStorage
}

// GetCapabilities returns the agent's capabilities
func (sa *StorageAgent) GetCapabilities() []AgentCapability {
	return sa.capabilities
}

// GetSpecializedTools returns tools specific to storage management
func (sa *StorageAgent) GetSpecializedTools() []interfaces.MCPTool {
	var storageTools []interfaces.MCPTool
	for _, tool := range sa.tools {
		toolName := tool.Name()
		if isStorageTool(toolName) {
			storageTools = append(storageTools, tool)
		}
	}
	return storageTools
}

// ProcessTask processes a storage-related task
func (sa *StorageAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":   sa.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing storage task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_db_instance":
		return sa.createDBInstance(ctx, task)
	case "create_volume":
		return sa.createVolume(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (sa *StorageAgent) CanHandleTask(task *Task) bool {
	storageTaskTypes := []string{
		"create_db_instance", "create_volume", "create_snapshot",
		"attach_volume", "detach_volume",
	}

	for _, taskType := range storageTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (sa *StorageAgent) GetTaskQueue() chan *Task {
	return sa.taskQueue
}

// CoordinateWith coordinates with another agent
func (sa *StorageAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Storage agents coordinate with compute agents for volume attachments
	if otherAgent.GetAgentType() == AgentTypeCompute {
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":      sa.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent")
	}
	return nil
}

// RequestHelp requests help from another agent
func (sa *StorageAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      sa.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from storage agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (sa *StorageAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      sa.id,
		To:        request.From,
		Success:   true,
		Content:   "Storage expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// SECURITY AGENT
// =============================================================================

// SecurityAgent handles security-related infrastructure tasks
type SecurityAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewSecurityAgent creates a new security agent
func NewSecurityAgent(baseAgent *BaseAgent) *SecurityAgent {
	agent := &SecurityAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeSecurity
	agent.name = "Security Agent"
	agent.description = "Handles security groups, IAM policies, and security infrastructure"

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

	return agent
}

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
	for _, tool := range sa.tools {
		toolName := tool.Name()
		if isSecurityTool(toolName) {
			securityTools = append(securityTools, tool)
		}
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
	case "add_security_group_rule":
		return sa.addSecurityGroupRule(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (sa *SecurityAgent) CanHandleTask(task *Task) bool {
	securityTaskTypes := []string{
		"create_security_group", "add_security_group_rule", "delete_security_group",
		"create_iam_role", "attach_iam_policy",
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
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"other_agent":   otherAgent.GetInfo().ID,
		"other_type":    otherAgent.GetAgentType(),
	}).Info("Coordinating with other agent")
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
// MONITORING AGENT
// =============================================================================

// MonitoringAgent handles monitoring-related infrastructure tasks
type MonitoringAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewMonitoringAgent creates a new monitoring agent
func NewMonitoringAgent(baseAgent *BaseAgent) *MonitoringAgent {
	agent := &MonitoringAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeMonitoring
	agent.name = "Monitoring Agent"
	agent.description = "Handles CloudWatch, logging, and monitoring infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "cloudwatch_management",
		Description: "Create and manage CloudWatch alarms and dashboards",
		Tools:       []string{"create-alarm", "create-dashboard", "put-metric"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "logging_management",
		Description: "Create and manage log groups and streams",
		Tools:       []string{"create-log-group", "create-log-stream"},
	})

	return agent
}

// GetAgentType returns the agent type
func (ma *MonitoringAgent) GetAgentType() AgentType {
	return AgentTypeMonitoring
}

// GetCapabilities returns the agent's capabilities
func (ma *MonitoringAgent) GetCapabilities() []AgentCapability {
	return ma.capabilities
}

// GetSpecializedTools returns tools specific to monitoring management
func (ma *MonitoringAgent) GetSpecializedTools() []interfaces.MCPTool {
	var monitoringTools []interfaces.MCPTool
	for _, tool := range ma.tools {
		toolName := tool.Name()
		if isMonitoringTool(toolName) {
			monitoringTools = append(monitoringTools, tool)
		}
	}
	return monitoringTools
}

// ProcessTask processes a monitoring-related task
func (ma *MonitoringAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":   ma.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing monitoring task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_alarm":
		return ma.createAlarm(ctx, task)
	case "create_dashboard":
		return ma.createDashboard(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ma *MonitoringAgent) CanHandleTask(task *Task) bool {
	monitoringTaskTypes := []string{
		"create_alarm", "create_dashboard", "create_log_group",
		"create_log_stream", "put_metric",
	}

	for _, taskType := range monitoringTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (ma *MonitoringAgent) GetTaskQueue() chan *Task {
	return ma.taskQueue
}

// CoordinateWith coordinates with another agent
func (ma *MonitoringAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Monitoring agents coordinate with all other agents
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":      ma.id,
		"other_agent":   otherAgent.GetInfo().ID,
		"other_type":    otherAgent.GetAgentType(),
	}).Info("Coordinating with other agent")
	return nil
}

// RequestHelp requests help from another agent
func (ma *MonitoringAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ma.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from monitoring agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (ma *MonitoringAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ma.id,
		To:        request.From,
		Success:   true,
		Content:   "Monitoring expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// BACKUP AGENT
// =============================================================================

// BackupAgent handles backup-related infrastructure tasks
type BackupAgent struct {
	*BaseAgent
	taskQueue chan *Task
}

// NewBackupAgent creates a new backup agent
func NewBackupAgent(baseAgent *BaseAgent) *BackupAgent {
	agent := &BackupAgent{
		BaseAgent: baseAgent,
		taskQueue: make(chan *Task, 50),
	}

	// Set agent type
	agent.agentType = AgentTypeBackup
	agent.name = "Backup Agent"
	agent.description = "Handles snapshots, backups, and disaster recovery"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "snapshot_management",
		Description: "Create and manage snapshots",
		Tools:       []string{"create-snapshot", "list-snapshots", "delete-snapshot"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "backup_management",
		Description: "Create and manage backups",
		Tools:       []string{"create-backup", "restore-backup", "list-backups"},
	})

	return agent
}

// GetAgentType returns the agent type
func (ba *BackupAgent) GetAgentType() AgentType {
	return AgentTypeBackup
}

// GetCapabilities returns the agent's capabilities
func (ba *BackupAgent) GetCapabilities() []AgentCapability {
	return ba.capabilities
}

// GetSpecializedTools returns tools specific to backup management
func (ba *BackupAgent) GetSpecializedTools() []interfaces.MCPTool {
	var backupTools []interfaces.MCPTool
	for _, tool := range ba.tools {
		toolName := tool.Name()
		if isBackupTool(toolName) {
			backupTools = append(backupTools, tool)
		}
	}
	return backupTools
}

// ProcessTask processes a backup-related task
func (ba *BackupAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithFields(map[string]interface{}{
		"agent_id":   ba.id,
		"task_id":    task.ID,
		"task_type":  task.Type,
	}).Info("Processing backup task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_snapshot":
		return ba.createSnapshot(ctx, task)
	case "create_backup":
		return ba.createBackup(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ba *BackupAgent) CanHandleTask(task *Task) bool {
	backupTaskTypes := []string{
		"create_snapshot", "create_backup", "restore_backup",
		"delete_snapshot", "delete_backup",
	}

	for _, taskType := range backupTaskTypes {
		if task.Type == taskType {
			return true
		}
	}
	return false
}

// GetTaskQueue returns the task queue
func (ba *BackupAgent) GetTaskQueue() chan *Task {
	return ba.taskQueue
}

// CoordinateWith coordinates with another agent
func (ba *BackupAgent) CoordinateWith(otherAgent SpecializedAgentInterface) error {
	// Backup agents coordinate with storage and compute agents
	switch otherAgent.GetAgentType() {
	case AgentTypeStorage, AgentTypeCompute:
		ba.logger.WithFields(map[string]interface{}{
			"agent_id":      ba.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent")
	}
	return nil
}

// RequestHelp requests help from another agent
func (ba *BackupAgent) RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ba.id,
		To:        request.To,
		Success:   true,
		Content:   "Help requested from backup agent",
		Timestamp: time.Now(),
	}, nil
}

// ProvideHelp provides help to another agent
func (ba *BackupAgent) ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ba.id,
		To:        request.From,
		Success:   true,
		Content:   "Backup expertise provided",
		Timestamp: time.Now(),
	}, nil
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// Tool classification functions
func isNetworkTool(toolName string) bool {
	networkTools := []string{
		"create-vpc", "list-vpcs", "create-subnet", "list-subnets",
		"create-route-table", "add-route", "associate-route-table",
		"create-internet-gateway", "create-nat-gateway",
	}
	return contains(networkTools, toolName)
}

func isComputeTool(toolName string) bool {
	computeTools := []string{
		"create-ec2-instance", "list-ec2-instances", "start-ec2-instance", "stop-ec2-instance",
		"create-auto-scaling-group", "list-auto-scaling-groups", "create-launch-template",
		"create-load-balancer", "create-target-group", "create-listener",
	}
	return contains(computeTools, toolName)
}

func isStorageTool(toolName string) bool {
	storageTools := []string{
		"create-db-instance", "list-db-instances", "start-db-instance", "stop-db-instance",
		"create-volume", "attach-volume", "detach-volume", "create-snapshot",
	}
	return contains(storageTools, toolName)
}

func isSecurityTool(toolName string) bool {
	securityTools := []string{
		"create-security-group", "list-security-groups", "add-security-group-ingress-rule",
		"add-security-group-egress-rule", "delete-security-group",
	}
	return contains(securityTools, toolName)
}

func isMonitoringTool(toolName string) bool {
	monitoringTools := []string{
		"create-alarm", "create-dashboard", "create-log-group", "create-log-stream",
		"put-metric", "list-alarms", "list-dashboards",
	}
	return contains(monitoringTools, toolName)
}

func isBackupTool(toolName string) bool {
	backupTools := []string{
		"create-snapshot", "list-snapshots", "delete-snapshot",
		"create-backup", "restore-backup", "list-backups",
	}
	return contains(backupTools, toolName)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// =============================================================================
// PLACEHOLDER TASK PROCESSING METHODS
// =============================================================================

// These methods are placeholders for actual task processing logic
// They would be implemented with real AWS API calls in a production system

func (na *NetworkAgent) createVPC(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"vpc_id": "vpc-12345678",
		"status": "created",
	}
	return task, nil
}

func (na *NetworkAgent) createSubnet(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"subnet_id": "subnet-12345678",
		"status":    "created",
	}
	return task, nil
}

func (na *NetworkAgent) createRouteTable(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"route_table_id": "rtb-12345678",
		"status":         "created",
	}
	return task, nil
}

func (ca *ComputeAgent) createEC2Instance(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"instance_id": "i-12345678",
		"status":      "created",
	}
	return task, nil
}

func (ca *ComputeAgent) createAutoScalingGroup(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"asg_name": "asg-12345678",
		"status":   "created",
	}
	return task, nil
}

func (ca *ComputeAgent) createLoadBalancer(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/1234567890123456",
		"status":            "created",
	}
	return task, nil
}

func (sa *StorageAgent) createDBInstance(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"db_instance_id": "db-12345678",
		"status":         "created",
	}
	return task, nil
}

func (sa *StorageAgent) createVolume(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"volume_id": "vol-12345678",
		"status":    "created",
	}
	return task, nil
}

func (sa *SecurityAgent) createSecurityGroup(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"security_group_id": "sg-12345678",
		"status":            "created",
	}
	return task, nil
}

func (sa *SecurityAgent) addSecurityGroupRule(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"rule_id": "rule-12345678",
		"status":  "created",
	}
	return task, nil
}

func (ma *MonitoringAgent) createAlarm(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"alarm_name": "alarm-12345678",
		"status":     "created",
	}
	return task, nil
}

func (ma *MonitoringAgent) createDashboard(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"dashboard_name": "dashboard-12345678",
		"status":         "created",
	}
	return task, nil
}

func (ba *BackupAgent) createSnapshot(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"snapshot_id": "snap-12345678",
		"status":      "created",
	}
	return task, nil
}

func (ba *BackupAgent) createBackup(ctx context.Context, task *Task) (*Task, error) {
	// Placeholder implementation
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"backup_id": "backup-12345678",
		"status":    "created",
	}
	return task, nil
}
