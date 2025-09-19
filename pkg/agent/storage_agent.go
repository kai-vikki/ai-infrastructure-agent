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
// STORAGE AGENT IMPLEMENTATION
// =============================================================================

// StorageAgent handles storage-related infrastructure tasks
type StorageAgent struct {
	*BaseAgent
	taskQueue     chan *Task
	rdsTools      map[string]interfaces.MCPTool
	ebsTools      map[string]interfaces.MCPTool
	s3Tools       map[string]interfaces.MCPTool
	snapshotTools map[string]interfaces.MCPTool
	awsClient     *aws.Client
	logger        *logging.Logger
}

// NewStorageAgent creates a new storage agent
func NewStorageAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *StorageAgent {
	agent := &StorageAgent{
		BaseAgent:     baseAgent,
		taskQueue:     make(chan *Task, 50),
		rdsTools:      make(map[string]interfaces.MCPTool),
		ebsTools:      make(map[string]interfaces.MCPTool),
		s3Tools:       make(map[string]interfaces.MCPTool),
		snapshotTools: make(map[string]interfaces.MCPTool),
		awsClient:     awsClient,
		logger:        logger,
	}

	// Set agent type
	agent.agentType = AgentTypeStorage
	agent.name = "Storage Agent"
	agent.description = "Handles RDS databases, EBS volumes, S3 buckets, and storage infrastructure"

	// Set capabilities
	agent.SetCapability(AgentCapability{
		Name:        "rds_management",
		Description: "Create and manage RDS databases",
		Tools:       []string{"create-db-instance", "list-db-instances", "start-db-instance", "stop-db-instance"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "ebs_management",
		Description: "Create and manage EBS volumes",
		Tools:       []string{"create-volume", "attach-volume", "detach-volume", "list-volumes"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "s3_management",
		Description: "Create and manage S3 buckets",
		Tools:       []string{"create-bucket", "list-buckets", "delete-bucket"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "snapshot_management",
		Description: "Create and manage snapshots",
		Tools:       []string{"create-snapshot", "list-snapshots", "delete-snapshot"},
	})

	// Initialize storage tools
	agent.initializeStorageTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

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

	// Add RDS tools
	for _, tool := range sa.rdsTools {
		storageTools = append(storageTools, tool)
	}

	// Add EBS tools
	for _, tool := range sa.ebsTools {
		storageTools = append(storageTools, tool)
	}

	// Add S3 tools
	for _, tool := range sa.s3Tools {
		storageTools = append(storageTools, tool)
	}

	// Add snapshot tools
	for _, tool := range sa.snapshotTools {
		storageTools = append(storageTools, tool)
	}

	return storageTools
}

// ProcessTask processes a storage-related task
func (sa *StorageAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":  sa.id,
		"task_id":   task.ID,
		"task_type": task.Type,
	}).Info("Processing storage task")

	// Update task status
	task.Status = TaskStatusInProgress
	now := time.Now()
	task.StartedAt = &now

	// Process based on task type
	switch task.Type {
	case "create_db_instance":
		return sa.createDBInstance(ctx, task)
	case "create_db_subnet_group":
		return sa.createDBSubnetGroup(ctx, task)
	case "create_volume":
		return sa.createVolume(ctx, task)
	case "attach_volume":
		return sa.attachVolume(ctx, task)
	case "detach_volume":
		return sa.detachVolume(ctx, task)
	case "create_snapshot":
		return sa.createSnapshot(ctx, task)
	case "create_bucket":
		return sa.createBucket(ctx, task)
	case "start_db_instance":
		return sa.startDBInstance(ctx, task)
	case "stop_db_instance":
		return sa.stopDBInstance(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (sa *StorageAgent) CanHandleTask(task *Task) bool {
	storageTaskTypes := []string{
		"create_db_instance", "create_db_subnet_group", "start_db_instance", "stop_db_instance",
		"create_volume", "attach_volume", "detach_volume", "create_snapshot",
		"create_bucket", "delete_bucket", "list_buckets",
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
	// Storage agents coordinate with compute and network agents
	switch otherAgent.GetAgentType() {
	case AgentTypeCompute:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":    sa.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent for storage configuration")

		// Storage agent provides volume information to compute agent
		return sa.provideStorageInfoToCompute(otherAgent)

	case AgentTypeNetwork:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":    sa.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with network agent for storage networking")

		// Storage agent requests subnet information from network agent
		return sa.requestNetworkInfoFromNetwork(otherAgent)

	default:
		sa.logger.WithFields(map[string]interface{}{
			"agent_id":    sa.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
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
// STORAGE TOOL INITIALIZATION
// =============================================================================

// initializeStorageTools initializes all storage-related tools
func (sa *StorageAgent) initializeStorageTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(sa.awsClient, sa.logger)

	// Initialize RDS tools
	sa.initializeRDSTools(toolFactory)

	// Initialize EBS tools
	sa.initializeEBSTools(toolFactory)

	// Initialize S3 tools
	sa.initializeS3Tools(toolFactory)

	// Initialize snapshot tools
	sa.initializeSnapshotTools(toolFactory)
}

// initializeRDSTools initializes RDS-related tools
func (sa *StorageAgent) initializeRDSTools(toolFactory interfaces.ToolFactory) {
	rdsToolTypes := []string{
		"create-db-subnet-group",
		"create-db-instance",
		"start-db-instance",
		"stop-db-instance",
		"delete-db-instance",
		"create-db-snapshot",
		"list-db-instances",
		"list-db-snapshots",
	}

	for _, toolType := range rdsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create RDS tool")
			continue
		}

		sa.rdsTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeEBSTools initializes EBS-related tools
func (sa *StorageAgent) initializeEBSTools(toolFactory interfaces.ToolFactory) {
	ebsToolTypes := []string{
		"create-volume",
		"attach-volume",
		"detach-volume",
		"list-volumes",
	}

	for _, toolType := range ebsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create EBS tool")
			continue
		}

		sa.ebsTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeS3Tools initializes S3-related tools
func (sa *StorageAgent) initializeS3Tools(toolFactory interfaces.ToolFactory) {
	s3ToolTypes := []string{
		"create-bucket",
		"list-buckets",
		"delete-bucket",
	}

	for _, toolType := range s3ToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create S3 tool")
			continue
		}

		sa.s3Tools[toolType] = tool
		sa.AddTool(tool)
	}
}

// initializeSnapshotTools initializes snapshot-related tools
func (sa *StorageAgent) initializeSnapshotTools(toolFactory interfaces.ToolFactory) {
	snapshotToolTypes := []string{
		"create-snapshot",
		"list-snapshots",
		"delete-snapshot",
	}

	for _, toolType := range snapshotToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: sa.awsClient,
		})
		if err != nil {
			sa.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create snapshot tool")
			continue
		}

		sa.snapshotTools[toolType] = tool
		sa.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createDBInstance creates a new RDS database instance
func (sa *StorageAgent) createDBInstance(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating RDS database instance")

	// Extract parameters
	dbInstanceIdentifier := "storage-agent-db"
	if identifier, exists := task.Parameters["dbInstanceIdentifier"]; exists {
		if identifierStr, ok := identifier.(string); ok {
			dbInstanceIdentifier = identifierStr
		}
	}

	engine := "mysql"
	if engineParam, exists := task.Parameters["engine"]; exists {
		if engineStr, ok := engineParam.(string); ok {
			engine = engineStr
		}
	}

	engineVersion := "8.0"
	if version, exists := task.Parameters["engineVersion"]; exists {
		if versionStr, ok := version.(string); ok {
			engineVersion = versionStr
		}
	}

	instanceClass := "db.t3.micro"
	if instanceClassParam, exists := task.Parameters["instanceClass"]; exists {
		if instanceClassStr, ok := instanceClassParam.(string); ok {
			instanceClass = instanceClassStr
		}
	}

	allocatedStorage := 20
	if storage, exists := task.Parameters["allocatedStorage"]; exists {
		if storageInt, ok := storage.(int); ok {
			allocatedStorage = storageInt
		}
	}

	masterUsername := "admin"
	if username, exists := task.Parameters["masterUsername"]; exists {
		if usernameStr, ok := username.(string); ok {
			masterUsername = usernameStr
		}
	}

	masterPassword := ""
	if password, exists := task.Parameters["masterPassword"]; exists {
		if passwordStr, ok := password.(string); ok {
			masterPassword = passwordStr
		}
	}

	dbSubnetGroupName := ""
	if subnetGroup, exists := task.Parameters["dbSubnetGroupName"]; exists {
		if subnetGroupStr, ok := subnetGroup.(string); ok {
			dbSubnetGroupName = subnetGroupStr
		}
	}

	vpcSecurityGroupIds := []string{}
	if securityGroups, exists := task.Parameters["vpcSecurityGroupIds"]; exists {
		if sgList, ok := securityGroups.([]interface{}); ok {
			for _, sg := range sgList {
				if sgStr, ok := sg.(string); ok {
					vpcSecurityGroupIds = append(vpcSecurityGroupIds, sgStr)
				}
			}
		}
	}

	// Use RDS tool to create database instance
	tool, exists := sa.rdsTools["create-db-instance"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "RDS database instance creation tool not available"
		return task, fmt.Errorf("RDS database instance creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"engine":               engine,
		"engineVersion":        engineVersion,
		"instanceClass":        instanceClass,
		"allocatedStorage":     allocatedStorage,
		"masterUsername":       masterUsername,
		"masterPassword":       masterPassword,
		"dbSubnetGroupName":    dbSubnetGroupName,
		"vpcSecurityGroupIds":  vpcSecurityGroupIds,
		"tags": map[string]string{
			"Name":      dbInstanceIdentifier,
			"CreatedBy": "storage-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create RDS database instance: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"db_instance_identifier": dbInstanceIdentifier,
		"engine":                 engine,
		"engine_version":         engineVersion,
		"instance_class":         instanceClass,
		"allocated_storage":      allocatedStorage,
		"master_username":        masterUsername,
		"db_subnet_group_name":   dbSubnetGroupName,
		"vpc_security_group_ids": vpcSecurityGroupIds,
		"status":                 "created",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":                task.ID,
		"db_instance_identifier": dbInstanceIdentifier,
	}).Info("RDS database instance created successfully")

	return task, nil
}

// createDBSubnetGroup creates a new RDS database subnet group
func (sa *StorageAgent) createDBSubnetGroup(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating RDS database subnet group")

	// Extract parameters
	dbSubnetGroupName := "storage-agent-db-subnet-group"
	if name, exists := task.Parameters["dbSubnetGroupName"]; exists {
		if nameStr, ok := name.(string); ok {
			dbSubnetGroupName = nameStr
		}
	}

	description := "Database subnet group created by storage agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
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

	if len(subnetIds) == 0 {
		task.Status = TaskStatusFailed
		task.Error = "Subnet IDs are required for database subnet group creation"
		return task, fmt.Errorf("subnet IDs are required for database subnet group creation")
	}

	// Use RDS tool to create database subnet group
	tool, exists := sa.rdsTools["create-db-subnet-group"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "RDS database subnet group creation tool not available"
		return task, fmt.Errorf("RDS database subnet group creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"dbSubnetGroupName": dbSubnetGroupName,
		"description":       description,
		"subnetIds":         subnetIds,
		"tags": map[string]string{
			"Name":      dbSubnetGroupName,
			"CreatedBy": "storage-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create RDS database subnet group: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"db_subnet_group_name": dbSubnetGroupName,
		"description":          description,
		"subnet_ids":           subnetIds,
		"status":               "created",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":              task.ID,
		"db_subnet_group_name": dbSubnetGroupName,
	}).Info("RDS database subnet group created successfully")

	return task, nil
}

// createVolume creates a new EBS volume
func (sa *StorageAgent) createVolume(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating EBS volume")

	// Extract parameters
	size := 10
	if sizeParam, exists := task.Parameters["size"]; exists {
		if sizeInt, ok := sizeParam.(int); ok {
			size = sizeInt
		}
	}

	volumeType := "gp3"
	if volumeTypeParam, exists := task.Parameters["volumeType"]; exists {
		if volumeTypeStr, ok := volumeTypeParam.(string); ok {
			volumeType = volumeTypeStr
		}
	}

	availabilityZone := ""
	if az, exists := task.Parameters["availabilityZone"]; exists {
		if azStr, ok := az.(string); ok {
			availabilityZone = azStr
		}
	}

	encrypted := false
	if encryptedParam, exists := task.Parameters["encrypted"]; exists {
		if encryptedBool, ok := encryptedParam.(bool); ok {
			encrypted = encryptedBool
		}
	}

	name := "storage-agent-volume"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}

	// Use EBS tool to create volume
	tool, exists := sa.ebsTools["create-volume"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "EBS volume creation tool not available"
		return task, fmt.Errorf("EBS volume creation tool not available")
	}

	// Execute tool
	volumeID, err := tool.Execute(ctx, map[string]interface{}{
		"size":             size,
		"volumeType":       volumeType,
		"availabilityZone": availabilityZone,
		"encrypted":        encrypted,
		"name":             name,
		"tags": map[string]string{
			"Name":      name,
			"CreatedBy": "storage-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create EBS volume: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"volume_id":         volumeID,
		"size":              size,
		"volume_type":       volumeType,
		"availability_zone": availabilityZone,
		"encrypted":         encrypted,
		"name":              name,
		"status":            "created",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"volume_id": volumeID,
	}).Info("EBS volume created successfully")

	return task, nil
}

// attachVolume attaches an EBS volume to an EC2 instance
func (sa *StorageAgent) attachVolume(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Attaching EBS volume")

	// Extract parameters
	volumeId := ""
	if volume, exists := task.Parameters["volumeId"]; exists {
		if volumeStr, ok := volume.(string); ok {
			volumeId = volumeStr
		}
	}

	if volumeId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Volume ID is required for volume attachment"
		return task, fmt.Errorf("volume ID is required for volume attachment")
	}

	instanceId := ""
	if instance, exists := task.Parameters["instanceId"]; exists {
		if instanceStr, ok := instance.(string); ok {
			instanceId = instanceStr
		}
	}

	if instanceId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Instance ID is required for volume attachment"
		return task, fmt.Errorf("instance ID is required for volume attachment")
	}

	device := "/dev/sdf"
	if deviceParam, exists := task.Parameters["device"]; exists {
		if deviceStr, ok := deviceParam.(string); ok {
			device = deviceStr
		}
	}

	// Use EBS tool to attach volume
	tool, exists := sa.ebsTools["attach-volume"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "EBS volume attachment tool not available"
		return task, fmt.Errorf("EBS volume attachment tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"volumeId":   volumeId,
		"instanceId": instanceId,
		"device":     device,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to attach EBS volume: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"volume_id":   volumeId,
		"instance_id": instanceId,
		"device":      device,
		"status":      "attached",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"volume_id":   volumeId,
		"instance_id": instanceId,
	}).Info("EBS volume attached successfully")

	return task, nil
}

// detachVolume detaches an EBS volume from an EC2 instance
func (sa *StorageAgent) detachVolume(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Detaching EBS volume")

	// Extract parameters
	volumeId := ""
	if volume, exists := task.Parameters["volumeId"]; exists {
		if volumeStr, ok := volume.(string); ok {
			volumeId = volumeStr
		}
	}

	if volumeId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Volume ID is required for volume detachment"
		return task, fmt.Errorf("volume ID is required for volume detachment")
	}

	instanceId := ""
	if instance, exists := task.Parameters["instanceId"]; exists {
		if instanceStr, ok := instance.(string); ok {
			instanceId = instanceStr
		}
	}

	if instanceId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Instance ID is required for volume detachment"
		return task, fmt.Errorf("instance ID is required for volume detachment")
	}

	// Use EBS tool to detach volume
	tool, exists := sa.ebsTools["detach-volume"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "EBS volume detachment tool not available"
		return task, fmt.Errorf("EBS volume detachment tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"volumeId":   volumeId,
		"instanceId": instanceId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to detach EBS volume: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"volume_id":   volumeId,
		"instance_id": instanceId,
		"status":      "detached",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"volume_id":   volumeId,
		"instance_id": instanceId,
	}).Info("EBS volume detached successfully")

	return task, nil
}

// createSnapshot creates a new EBS snapshot
func (sa *StorageAgent) createSnapshot(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating EBS snapshot")

	// Extract parameters
	volumeId := ""
	if volume, exists := task.Parameters["volumeId"]; exists {
		if volumeStr, ok := volume.(string); ok {
			volumeId = volumeStr
		}
	}

	if volumeId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Volume ID is required for snapshot creation"
		return task, fmt.Errorf("volume ID is required for snapshot creation")
	}

	description := "Snapshot created by storage agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}

	name := "storage-agent-snapshot"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}

	// Use snapshot tool to create snapshot
	tool, exists := sa.snapshotTools["create-snapshot"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "EBS snapshot creation tool not available"
		return task, fmt.Errorf("EBS snapshot creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"volumeId":    volumeId,
		"description": description,
		"name":        name,
		"tags": map[string]string{
			"Name":      name,
			"CreatedBy": "storage-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create EBS snapshot: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"snapshot_id": name, // placeholder ID since tool return removed
		"volume_id":   volumeId,
		"description": description,
		"name":        name,
		"status":      "created",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"snapshot_id": name,
		"volume_id":   volumeId,
	}).Info("EBS snapshot created successfully")

	return task, nil
}

// createBucket creates a new S3 bucket
func (sa *StorageAgent) createBucket(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Creating S3 bucket")

	// Extract parameters
	bucketName := "storage-agent-bucket"
	if name, exists := task.Parameters["bucketName"]; exists {
		if nameStr, ok := name.(string); ok {
			bucketName = nameStr
		}
	}

	region := "us-west-2"
	if regionParam, exists := task.Parameters["region"]; exists {
		if regionStr, ok := regionParam.(string); ok {
			region = regionStr
		}
	}

	// Use S3 tool to create bucket
	tool, exists := sa.s3Tools["create-bucket"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "S3 bucket creation tool not available"
		return task, fmt.Errorf("S3 bucket creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"bucketName": bucketName,
		"region":     region,
		"tags": map[string]string{
			"Name":      bucketName,
			"CreatedBy": "storage-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"bucket_name": bucketName,
		"region":      region,
		"status":      "created",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"bucket_name": bucketName,
	}).Info("S3 bucket created successfully")

	return task, nil
}

// startDBInstance starts an RDS database instance
func (sa *StorageAgent) startDBInstance(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Starting RDS database instance")

	// Extract parameters
	dbInstanceIdentifier := ""
	if identifier, exists := task.Parameters["dbInstanceIdentifier"]; exists {
		if identifierStr, ok := identifier.(string); ok {
			dbInstanceIdentifier = identifierStr
		}
	}

	if dbInstanceIdentifier == "" {
		task.Status = TaskStatusFailed
		task.Error = "Database instance identifier is required"
		return task, fmt.Errorf("database instance identifier is required")
	}

	// Use RDS tool to start database instance
	tool, exists := sa.rdsTools["start-db-instance"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "RDS database instance start tool not available"
		return task, fmt.Errorf("RDS database instance start tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to start RDS database instance: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"db_instance_identifier": dbInstanceIdentifier,
		"status":                 "started",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":                task.ID,
		"db_instance_identifier": dbInstanceIdentifier,
	}).Info("RDS database instance started successfully")

	return task, nil
}

// stopDBInstance stops an RDS database instance
func (sa *StorageAgent) stopDBInstance(ctx context.Context, task *Task) (*Task, error) {
	sa.logger.WithField("task_id", task.ID).Info("Stopping RDS database instance")

	// Extract parameters
	dbInstanceIdentifier := ""
	if identifier, exists := task.Parameters["dbInstanceIdentifier"]; exists {
		if identifierStr, ok := identifier.(string); ok {
			dbInstanceIdentifier = identifierStr
		}
	}

	if dbInstanceIdentifier == "" {
		task.Status = TaskStatusFailed
		task.Error = "Database instance identifier is required"
		return task, fmt.Errorf("database instance identifier is required")
	}

	// Use RDS tool to stop database instance
	tool, exists := sa.rdsTools["stop-db-instance"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "RDS database instance stop tool not available"
		return task, fmt.Errorf("RDS database instance stop tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to stop RDS database instance: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"db_instance_identifier": dbInstanceIdentifier,
		"status":                 "stopped",
	}

	sa.logger.WithFields(map[string]interface{}{
		"task_id":                task.ID,
		"db_instance_identifier": dbInstanceIdentifier,
	}).Info("RDS database instance stopped successfully")

	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// provideStorageInfoToCompute provides storage information to compute agent
func (sa *StorageAgent) provideStorageInfoToCompute(computeAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with storage information
	// For now, we'll log the coordination
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"compute_agent": computeAgent.GetInfo().ID,
	}).Info("Providing storage information to compute agent")

	return nil
}

// requestNetworkInfoFromNetwork requests network information from network agent
func (sa *StorageAgent) requestNetworkInfoFromNetwork(networkAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message requesting network information
	// For now, we'll log the coordination
	sa.logger.WithFields(map[string]interface{}{
		"agent_id":      sa.id,
		"network_agent": networkAgent.GetInfo().ID,
	}).Info("Requesting network information from network agent")

	return nil
}
