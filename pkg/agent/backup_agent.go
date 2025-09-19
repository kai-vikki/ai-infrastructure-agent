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
// BACKUP AGENT IMPLEMENTATION
// =============================================================================

// BackupAgent handles backup-related infrastructure tasks
type BackupAgent struct {
	*BaseAgent
	taskQueue     chan *Task
	snapshotTools map[string]interfaces.MCPTool
	backupTools   map[string]interfaces.MCPTool
	recoveryTools map[string]interfaces.MCPTool
	s3BackupTools map[string]interfaces.MCPTool
	awsClient     *aws.Client
	logger        *logging.Logger
}

// NewBackupAgent creates a new backup agent
func NewBackupAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *BackupAgent {
	agent := &BackupAgent{
		BaseAgent:     baseAgent,
		taskQueue:     make(chan *Task, 50),
		snapshotTools: make(map[string]interfaces.MCPTool),
		backupTools:   make(map[string]interfaces.MCPTool),
		recoveryTools: make(map[string]interfaces.MCPTool),
		s3BackupTools: make(map[string]interfaces.MCPTool),
		awsClient:     awsClient,
		logger:        logger,
	}

	// Set agent type
	agent.agentType = AgentTypeBackup
	agent.name = "Backup Agent"
	agent.description = "Handles snapshots, backups, disaster recovery, and backup infrastructure"

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

	agent.SetCapability(AgentCapability{
		Name:        "recovery_management",
		Description: "Manage disaster recovery",
		Tools:       []string{"restore-from-snapshot", "restore-from-backup", "validate-backup"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "s3_backup_management",
		Description: "Manage S3-based backups",
		Tools:       []string{"backup-to-s3", "restore-from-s3", "list-s3-backups"},
	})

	// Initialize backup tools
	agent.initializeBackupTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

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

	// Add snapshot tools
	for _, tool := range ba.snapshotTools {
		backupTools = append(backupTools, tool)
	}

	// Add backup tools
	for _, tool := range ba.backupTools {
		backupTools = append(backupTools, tool)
	}

	// Add recovery tools
	for _, tool := range ba.recoveryTools {
		backupTools = append(backupTools, tool)
	}

	// Add S3 backup tools
	for _, tool := range ba.s3BackupTools {
		backupTools = append(backupTools, tool)
	}

	return backupTools
}

// ProcessTask processes a backup-related task
func (ba *BackupAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithFields(map[string]interface{}{
		"agent_id":  ba.id,
		"task_id":   task.ID,
		"task_type": task.Type,
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
	case "restore_from_snapshot":
		return ba.restoreFromSnapshot(ctx, task)
	case "restore_from_backup":
		return ba.restoreFromBackup(ctx, task)
	case "backup_to_s3":
		return ba.backupToS3(ctx, task)
	case "restore_from_s3":
		return ba.restoreFromS3(ctx, task)
	case "list_snapshots":
		return ba.listSnapshots(ctx, task)
	case "list_backups":
		return ba.listBackups(ctx, task)
	case "validate_backup":
		return ba.validateBackup(ctx, task)
	case "delete_snapshot":
		return ba.deleteSnapshot(ctx, task)
	case "delete_backup":
		return ba.deleteBackup(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ba *BackupAgent) CanHandleTask(task *Task) bool {
	backupTaskTypes := []string{
		"create_snapshot", "create_backup", "restore_from_snapshot", "restore_from_backup",
		"backup_to_s3", "restore_from_s3", "list_snapshots", "list_backups",
		"validate_backup", "delete_snapshot", "delete_backup", "schedule_backup",
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
	case AgentTypeStorage:
		ba.logger.WithFields(map[string]interface{}{
			"agent_id":    ba.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with storage agent for backup configuration")

		// Backup agent provides backup setup for storage resources
		return ba.provideBackupForStorage(otherAgent)

	case AgentTypeCompute:
		ba.logger.WithFields(map[string]interface{}{
			"agent_id":    ba.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent for backup configuration")

		// Backup agent provides backup setup for compute resources
		return ba.provideBackupForCompute(otherAgent)

	default:
		ba.logger.WithFields(map[string]interface{}{
			"agent_id":    ba.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
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
// BACKUP TOOL INITIALIZATION
// =============================================================================

// initializeBackupTools initializes all backup-related tools
func (ba *BackupAgent) initializeBackupTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(ba.awsClient, ba.logger)

	// Initialize snapshot tools
	ba.initializeSnapshotTools(toolFactory)

	// Initialize backup tools
	ba.initializeBackupToolsInternal(toolFactory)

	// Initialize recovery tools
	ba.initializeRecoveryTools(toolFactory)

	// Initialize S3 backup tools
	ba.initializeS3BackupTools(toolFactory)
}

// initializeSnapshotTools initializes snapshot-related tools
func (ba *BackupAgent) initializeSnapshotTools(toolFactory interfaces.ToolFactory) {
	snapshotToolTypes := []string{
		"create-snapshot",
		"list-snapshots",
		"delete-snapshot",
		"copy-snapshot",
	}

	for _, toolType := range snapshotToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ba.awsClient,
		})
		if err != nil {
			ba.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create snapshot tool")
			continue
		}

		ba.snapshotTools[toolType] = tool
		ba.AddTool(tool)
	}
}

// initializeBackupToolsInternal initializes backup-related tools
func (ba *BackupAgent) initializeBackupToolsInternal(toolFactory interfaces.ToolFactory) {
	backupToolTypes := []string{
		"create-backup",
		"list-backups",
		"delete-backup",
		"restore-backup",
	}

	for _, toolType := range backupToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ba.awsClient,
		})
		if err != nil {
			ba.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create backup tool")
			continue
		}

		ba.backupTools[toolType] = tool
		ba.AddTool(tool)
	}
}

// initializeRecoveryTools initializes recovery-related tools
func (ba *BackupAgent) initializeRecoveryTools(toolFactory interfaces.ToolFactory) {
	recoveryToolTypes := []string{
		"restore-from-snapshot",
		"restore-from-backup",
		"validate-backup",
		"test-restore",
	}

	for _, toolType := range recoveryToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ba.awsClient,
		})
		if err != nil {
			ba.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create recovery tool")
			continue
		}

		ba.recoveryTools[toolType] = tool
		ba.AddTool(tool)
	}
}

// initializeS3BackupTools initializes S3 backup-related tools
func (ba *BackupAgent) initializeS3BackupTools(toolFactory interfaces.ToolFactory) {
	s3BackupToolTypes := []string{
		"backup-to-s3",
		"restore-from-s3",
		"list-s3-backups",
		"delete-s3-backup",
	}

	for _, toolType := range s3BackupToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ba.awsClient,
		})
		if err != nil {
			ba.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create S3 backup tool")
			continue
		}

		ba.s3BackupTools[toolType] = tool
		ba.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createSnapshot creates a new EBS snapshot
func (ba *BackupAgent) createSnapshot(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Creating EBS snapshot")

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

	description := "Snapshot created by backup agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}

	name := "backup-agent-snapshot"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}

	// Use snapshot tool to create snapshot
	tool, exists := ba.snapshotTools["create-snapshot"]
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
			"CreatedBy": "backup-agent",
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
	// Best-effort: no output parsed in this stub implementation
	var snapshotID interface{}
	task.Result = map[string]interface{}{
		"snapshot_id": snapshotID,
		"volume_id":   volumeId,
		"description": description,
		"name":        name,
		"status":      "created",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"snapshot_id": snapshotID,
		"volume_id":   volumeId,
	}).Info("EBS snapshot created successfully")

	return task, nil
}

// createBackup creates a new backup
func (ba *BackupAgent) createBackup(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Creating backup")

	// Extract parameters
	resourceId := ""
	if resource, exists := task.Parameters["resourceId"]; exists {
		if resourceStr, ok := resource.(string); ok {
			resourceId = resourceStr
		}
	}

	if resourceId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Resource ID is required for backup creation"
		return task, fmt.Errorf("resource ID is required for backup creation")
	}

	resourceType := "EBS"
	if resourceTypeParam, exists := task.Parameters["resourceType"]; exists {
		if resourceTypeStr, ok := resourceTypeParam.(string); ok {
			resourceType = resourceTypeStr
		}
	}

	backupName := "backup-agent-backup"
	if name, exists := task.Parameters["backupName"]; exists {
		if nameStr, ok := name.(string); ok {
			backupName = nameStr
		}
	}

	description := "Backup created by backup agent"
	if desc, exists := task.Parameters["description"]; exists {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}

	// Use backup tool to create backup
	tool, exists := ba.backupTools["create-backup"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Backup creation tool not available"
		return task, fmt.Errorf("backup creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"resourceId":   resourceId,
		"resourceType": resourceType,
		"backupName":   backupName,
		"description":  description,
		"tags": map[string]string{
			"Name":      backupName,
			"CreatedBy": "backup-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create backup: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	// Best-effort: no output parsed in this stub implementation
	var backupID interface{}
	task.Result = map[string]interface{}{
		"backup_id":     backupID,
		"resource_id":   resourceId,
		"resource_type": resourceType,
		"backup_name":   backupName,
		"description":   description,
		"status":        "created",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"backup_id":   backupID,
		"resource_id": resourceId,
	}).Info("Backup created successfully")

	return task, nil
}

// restoreFromSnapshot restores from a snapshot
func (ba *BackupAgent) restoreFromSnapshot(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Restoring from snapshot")

	// Extract parameters
	snapshotId := ""
	if snapshot, exists := task.Parameters["snapshotId"]; exists {
		if snapshotStr, ok := snapshot.(string); ok {
			snapshotId = snapshotStr
		}
	}

	if snapshotId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Snapshot ID is required for restoration"
		return task, fmt.Errorf("snapshot ID is required for restoration")
	}

	availabilityZone := ""
	if az, exists := task.Parameters["availabilityZone"]; exists {
		if azStr, ok := az.(string); ok {
			availabilityZone = azStr
		}
	}

	volumeType := "gp3"
	if volumeTypeParam, exists := task.Parameters["volumeType"]; exists {
		if volumeTypeStr, ok := volumeTypeParam.(string); ok {
			volumeType = volumeTypeStr
		}
	}

	name := "backup-agent-restored-volume"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}

	// Use recovery tool to restore from snapshot
	tool, exists := ba.recoveryTools["restore-from-snapshot"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Snapshot restoration tool not available"
		return task, fmt.Errorf("snapshot restoration tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"snapshotId":       snapshotId,
		"availabilityZone": availabilityZone,
		"volumeType":       volumeType,
		"name":             name,
		"tags": map[string]string{
			"Name":      name,
			"CreatedBy": "backup-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var volumeID interface{}
	if len(result.Content) > 0 {
		volumeID = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"volume_id":         volumeID,
		"snapshot_id":       snapshotId,
		"availability_zone": availabilityZone,
		"volume_type":       volumeType,
		"name":              name,
		"status":            "restored",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"volume_id":   volumeID,
		"snapshot_id": snapshotId,
	}).Info("Volume restored from snapshot successfully")

	return task, nil
}

// restoreFromBackup restores from a backup
func (ba *BackupAgent) restoreFromBackup(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Restoring from backup")

	// Extract parameters
	backupId := ""
	if backup, exists := task.Parameters["backupId"]; exists {
		if backupStr, ok := backup.(string); ok {
			backupId = backupStr
		}
	}

	if backupId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Backup ID is required for restoration"
		return task, fmt.Errorf("backup ID is required for restoration")
	}

	resourceType := "EBS"
	if resourceTypeParam, exists := task.Parameters["resourceType"]; exists {
		if resourceTypeStr, ok := resourceTypeParam.(string); ok {
			resourceType = resourceTypeStr
		}
	}

	name := "backup-agent-restored-resource"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}

	// Use recovery tool to restore from backup
	tool, exists := ba.recoveryTools["restore-from-backup"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Backup restoration tool not available"
		return task, fmt.Errorf("backup restoration tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"backupId":     backupId,
		"resourceType": resourceType,
		"name":         name,
		"tags": map[string]string{
			"Name":      name,
			"CreatedBy": "backup-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to restore from backup: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var resourceID interface{}
	if len(result.Content) > 0 {
		resourceID = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"resource_id":   resourceID,
		"backup_id":     backupId,
		"resource_type": resourceType,
		"name":          name,
		"status":        "restored",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"resource_id": resourceID,
		"backup_id":   backupId,
	}).Info("Resource restored from backup successfully")

	return task, nil
}

// backupToS3 backs up data to S3
func (ba *BackupAgent) backupToS3(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Backing up to S3")

	// Extract parameters
	sourcePath := ""
	if source, exists := task.Parameters["sourcePath"]; exists {
		if sourceStr, ok := source.(string); ok {
			sourcePath = sourceStr
		}
	}

	if sourcePath == "" {
		task.Status = TaskStatusFailed
		task.Error = "Source path is required for S3 backup"
		return task, fmt.Errorf("source path is required for S3 backup")
	}

	bucketName := ""
	if bucket, exists := task.Parameters["bucketName"]; exists {
		if bucketStr, ok := bucket.(string); ok {
			bucketName = bucketStr
		}
	}

	if bucketName == "" {
		task.Status = TaskStatusFailed
		task.Error = "Bucket name is required for S3 backup"
		return task, fmt.Errorf("bucket name is required for S3 backup")
	}

	backupName := "backup-agent-s3-backup"
	if name, exists := task.Parameters["backupName"]; exists {
		if nameStr, ok := name.(string); ok {
			backupName = nameStr
		}
	}

	// Use S3 backup tool to backup to S3
	tool, exists := ba.s3BackupTools["backup-to-s3"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "S3 backup tool not available"
		return task, fmt.Errorf("S3 backup tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"sourcePath": sourcePath,
		"bucketName": bucketName,
		"backupName": backupName,
		"tags": map[string]string{
			"Name":      backupName,
			"CreatedBy": "backup-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to backup to S3: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var backupLocation interface{}
	if len(result.Content) > 0 {
		backupLocation = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"backup_location": backupLocation,
		"source_path":     sourcePath,
		"bucket_name":     bucketName,
		"backup_name":     backupName,
		"status":          "backed_up",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":         task.ID,
		"backup_location": backupLocation,
		"source_path":     sourcePath,
		"bucket_name":     bucketName,
	}).Info("Data backed up to S3 successfully")

	return task, nil
}

// restoreFromS3 restores data from S3
func (ba *BackupAgent) restoreFromS3(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Restoring from S3")

	// Extract parameters
	bucketName := ""
	if bucket, exists := task.Parameters["bucketName"]; exists {
		if bucketStr, ok := bucket.(string); ok {
			bucketName = bucketStr
		}
	}

	if bucketName == "" {
		task.Status = TaskStatusFailed
		task.Error = "Bucket name is required for S3 restoration"
		return task, fmt.Errorf("bucket name is required for S3 restoration")
	}

	backupKey := ""
	if key, exists := task.Parameters["backupKey"]; exists {
		if keyStr, ok := key.(string); ok {
			backupKey = keyStr
		}
	}

	if backupKey == "" {
		task.Status = TaskStatusFailed
		task.Error = "Backup key is required for S3 restoration"
		return task, fmt.Errorf("backup key is required for S3 restoration")
	}

	destinationPath := ""
	if destination, exists := task.Parameters["destinationPath"]; exists {
		if destinationStr, ok := destination.(string); ok {
			destinationPath = destinationStr
		}
	}

	if destinationPath == "" {
		task.Status = TaskStatusFailed
		task.Error = "Destination path is required for S3 restoration"
		return task, fmt.Errorf("destination path is required for S3 restoration")
	}

	// Use S3 backup tool to restore from S3
	tool, exists := ba.s3BackupTools["restore-from-s3"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "S3 restoration tool not available"
		return task, fmt.Errorf("S3 restoration tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"bucketName":      bucketName,
		"backupKey":       backupKey,
		"destinationPath": destinationPath,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to restore from S3: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"bucket_name":      bucketName,
		"backup_key":       backupKey,
		"destination_path": destinationPath,
		"status":           "restored",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":          task.ID,
		"bucket_name":      bucketName,
		"backup_key":       backupKey,
		"destination_path": destinationPath,
	}).Info("Data restored from S3 successfully")

	return task, nil
}

// listSnapshots lists available snapshots
func (ba *BackupAgent) listSnapshots(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Listing snapshots")

	// Extract parameters
	volumeId := ""
	if volume, exists := task.Parameters["volumeId"]; exists {
		if volumeStr, ok := volume.(string); ok {
			volumeId = volumeStr
		}
	}

	// Use snapshot tool to list snapshots
	tool, exists := ba.snapshotTools["list-snapshots"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "List snapshots tool not available"
		return task, fmt.Errorf("list snapshots tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"volumeId": volumeId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to list snapshots: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var snapshots interface{}
	if len(result.Content) > 0 {
		snapshots = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"volume_id": volumeId,
		"snapshots": snapshots,
		"status":    "listed",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"volume_id": volumeId,
	}).Info("Snapshots listed successfully")

	return task, nil
}

// listBackups lists available backups
func (ba *BackupAgent) listBackups(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Listing backups")

	// Extract parameters
	resourceId := ""
	if resource, exists := task.Parameters["resourceId"]; exists {
		if resourceStr, ok := resource.(string); ok {
			resourceId = resourceStr
		}
	}

	// Use backup tool to list backups
	tool, exists := ba.backupTools["list-backups"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "List backups tool not available"
		return task, fmt.Errorf("list backups tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"resourceId": resourceId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to list backups: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var backups interface{}
	if len(result.Content) > 0 {
		backups = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"resource_id": resourceId,
		"backups":     backups,
		"status":      "listed",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"resource_id": resourceId,
	}).Info("Backups listed successfully")

	return task, nil
}

// validateBackup validates a backup
func (ba *BackupAgent) validateBackup(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Validating backup")

	// Extract parameters
	backupId := ""
	if backup, exists := task.Parameters["backupId"]; exists {
		if backupStr, ok := backup.(string); ok {
			backupId = backupStr
		}
	}

	if backupId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Backup ID is required for backup validation"
		return task, fmt.Errorf("backup ID is required for backup validation")
	}

	// Use recovery tool to validate backup
	tool, exists := ba.recoveryTools["validate-backup"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Backup validation tool not available"
		return task, fmt.Errorf("backup validation tool not available")
	}

	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"backupId": backupId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to validate backup: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	var validation interface{}
	if len(result.Content) > 0 {
		validation = result.Content[0]
	}
	task.Result = map[string]interface{}{
		"backup_id":         backupId,
		"validation_result": validation,
		"status":            "validated",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"backup_id": backupId,
	}).Info("Backup validated successfully")

	return task, nil
}

// deleteSnapshot deletes a snapshot
func (ba *BackupAgent) deleteSnapshot(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Deleting snapshot")

	// Extract parameters
	snapshotId := ""
	if snapshot, exists := task.Parameters["snapshotId"]; exists {
		if snapshotStr, ok := snapshot.(string); ok {
			snapshotId = snapshotStr
		}
	}

	if snapshotId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Snapshot ID is required for snapshot deletion"
		return task, fmt.Errorf("snapshot ID is required for snapshot deletion")
	}

	// Use snapshot tool to delete snapshot
	tool, exists := ba.snapshotTools["delete-snapshot"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Delete snapshot tool not available"
		return task, fmt.Errorf("delete snapshot tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"snapshotId": snapshotId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to delete snapshot: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"snapshot_id": snapshotId,
		"status":      "deleted",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"snapshot_id": snapshotId,
	}).Info("Snapshot deleted successfully")

	return task, nil
}

// deleteBackup deletes a backup
func (ba *BackupAgent) deleteBackup(ctx context.Context, task *Task) (*Task, error) {
	ba.logger.WithField("task_id", task.ID).Info("Deleting backup")

	// Extract parameters
	backupId := ""
	if backup, exists := task.Parameters["backupId"]; exists {
		if backupStr, ok := backup.(string); ok {
			backupId = backupStr
		}
	}

	if backupId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Backup ID is required for backup deletion"
		return task, fmt.Errorf("backup ID is required for backup deletion")
	}

	// Use backup tool to delete backup
	tool, exists := ba.backupTools["delete-backup"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Delete backup tool not available"
		return task, fmt.Errorf("delete backup tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"backupId": backupId,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to delete backup: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"backup_id": backupId,
		"status":    "deleted",
	}

	ba.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"backup_id": backupId,
	}).Info("Backup deleted successfully")

	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// provideBackupForStorage provides backup setup for storage resources
func (ba *BackupAgent) provideBackupForStorage(storageAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with backup setup
	// For now, we'll log the coordination
	ba.logger.WithFields(map[string]interface{}{
		"agent_id":      ba.id,
		"storage_agent": storageAgent.GetInfo().ID,
	}).Info("Providing backup setup for storage resources")

	return nil
}

// provideBackupForCompute provides backup setup for compute resources
func (ba *BackupAgent) provideBackupForCompute(computeAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with backup setup
	// For now, we'll log the coordination
	ba.logger.WithFields(map[string]interface{}{
		"agent_id":      ba.id,
		"compute_agent": computeAgent.GetInfo().ID,
	}).Info("Providing backup setup for compute resources")

	return nil
}
