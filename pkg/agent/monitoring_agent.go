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
// MONITORING AGENT IMPLEMENTATION
// =============================================================================

// MonitoringAgent handles monitoring-related infrastructure tasks
type MonitoringAgent struct {
	*BaseAgent
	taskQueue       chan *Task
	cloudWatchTools map[string]interfaces.MCPTool
	logsTools       map[string]interfaces.MCPTool
	metricsTools    map[string]interfaces.MCPTool
	alarmTools      map[string]interfaces.MCPTool
	dashboardTools  map[string]interfaces.MCPTool
	awsClient       *aws.Client
	logger          *logging.Logger
}

// NewMonitoringAgent creates a new monitoring agent
func NewMonitoringAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *MonitoringAgent {
	agent := &MonitoringAgent{
		BaseAgent:       baseAgent,
		taskQueue:       make(chan *Task, 50),
		cloudWatchTools: make(map[string]interfaces.MCPTool),
		logsTools:       make(map[string]interfaces.MCPTool),
		metricsTools:    make(map[string]interfaces.MCPTool),
		alarmTools:      make(map[string]interfaces.MCPTool),
		dashboardTools:  make(map[string]interfaces.MCPTool),
		awsClient:       awsClient,
		logger:          logger,
	}

	// Set agent type
	agent.agentType = AgentTypeMonitoring
	agent.name = "Monitoring Agent"
	agent.description = "Handles CloudWatch, logging, metrics, alarms, and monitoring infrastructure"

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

	agent.SetCapability(AgentCapability{
		Name:        "metrics_management",
		Description: "Create and manage custom metrics",
		Tools:       []string{"put-metric", "list-metrics", "get-metric-statistics"},
	})

	agent.SetCapability(AgentCapability{
		Name:        "alarm_management",
		Description: "Create and manage CloudWatch alarms",
		Tools:       []string{"create-alarm", "list-alarms", "delete-alarm"},
	})

	// Initialize monitoring tools
	agent.initializeMonitoringTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

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

	// Add CloudWatch tools
	for _, tool := range ma.cloudWatchTools {
		monitoringTools = append(monitoringTools, tool)
	}

	// Add logs tools
	for _, tool := range ma.logsTools {
		monitoringTools = append(monitoringTools, tool)
	}

	// Add metrics tools
	for _, tool := range ma.metricsTools {
		monitoringTools = append(monitoringTools, tool)
	}

	// Add alarm tools
	for _, tool := range ma.alarmTools {
		monitoringTools = append(monitoringTools, tool)
	}

	// Add dashboard tools
	for _, tool := range ma.dashboardTools {
		monitoringTools = append(monitoringTools, tool)
	}

	return monitoringTools
}

// ProcessTask processes a monitoring-related task
func (ma *MonitoringAgent) ProcessTask(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":  ma.id,
		"task_id":   task.ID,
		"task_type": task.Type,
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
	case "create_log_group":
		return ma.createLogGroup(ctx, task)
	case "create_log_stream":
		return ma.createLogStream(ctx, task)
	case "put_metric":
		return ma.putMetric(ctx, task)
	case "list_metrics":
		return ma.listMetrics(ctx, task)
	case "get_metric_statistics":
		return ma.getMetricStatistics(ctx, task)
	case "list_alarms":
		return ma.listAlarms(ctx, task)
	case "delete_alarm":
		return ma.deleteAlarm(ctx, task)
	default:
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		return task, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// CanHandleTask checks if the agent can handle a specific task
func (ma *MonitoringAgent) CanHandleTask(task *Task) bool {
	monitoringTaskTypes := []string{
		"create_alarm", "create_dashboard", "create_log_group", "create_log_stream",
		"put_metric", "list_metrics", "get_metric_statistics", "list_alarms",
		"delete_alarm", "update_alarm", "enable_alarm", "disable_alarm",
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
	switch otherAgent.GetAgentType() {
	case AgentTypeCompute:
		ma.logger.WithFields(map[string]interface{}{
			"agent_id":    ma.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent for monitoring configuration")

		// Monitoring agent provides monitoring setup for compute resources
		return ma.provideMonitoringForCompute(otherAgent)

	case AgentTypeStorage:
		ma.logger.WithFields(map[string]interface{}{
			"agent_id":    ma.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with storage agent for monitoring configuration")

		// Monitoring agent provides monitoring setup for storage resources
		return ma.provideMonitoringForStorage(otherAgent)

	case AgentTypeNetwork:
		ma.logger.WithFields(map[string]interface{}{
			"agent_id":    ma.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with network agent for monitoring configuration")

		// Monitoring agent provides monitoring setup for network resources
		return ma.provideMonitoringForNetwork(otherAgent)

	default:
		ma.logger.WithFields(map[string]interface{}{
			"agent_id":    ma.id,
			"other_agent": otherAgent.GetInfo().ID,
			"other_type":  otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
	}
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
// MONITORING TOOL INITIALIZATION
// =============================================================================

// initializeMonitoringTools initializes all monitoring-related tools
func (ma *MonitoringAgent) initializeMonitoringTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(ma.awsClient, ma.logger)

	// Initialize CloudWatch tools
	ma.initializeCloudWatchTools(toolFactory)

	// Initialize logs tools
	ma.initializeLogsTools(toolFactory)

	// Initialize metrics tools
	ma.initializeMetricsTools(toolFactory)

	// Initialize alarm tools
	ma.initializeAlarmTools(toolFactory)

	// Initialize dashboard tools
	ma.initializeDashboardTools(toolFactory)
}

// initializeCloudWatchTools initializes CloudWatch-related tools
func (ma *MonitoringAgent) initializeCloudWatchTools(toolFactory interfaces.ToolFactory) {
	cloudWatchToolTypes := []string{
		"put-metric",
		"list-metrics",
		"get-metric-statistics",
	}

	for _, toolType := range cloudWatchToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ma.awsClient,
		})
		if err != nil {
			ma.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create CloudWatch tool")
			continue
		}

		ma.cloudWatchTools[toolType] = tool
		ma.AddTool(tool)
	}
}

// initializeLogsTools initializes logs-related tools
func (ma *MonitoringAgent) initializeLogsTools(toolFactory interfaces.ToolFactory) {
	logsToolTypes := []string{
		"create-log-group",
		"create-log-stream",
		"list-log-groups",
		"list-log-streams",
	}

	for _, toolType := range logsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ma.awsClient,
		})
		if err != nil {
			ma.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create logs tool")
			continue
		}

		ma.logsTools[toolType] = tool
		ma.AddTool(tool)
	}
}

// initializeMetricsTools initializes metrics-related tools
func (ma *MonitoringAgent) initializeMetricsTools(toolFactory interfaces.ToolFactory) {
	metricsToolTypes := []string{
		"put-metric",
		"list-metrics",
		"get-metric-statistics",
	}

	for _, toolType := range metricsToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ma.awsClient,
		})
		if err != nil {
			ma.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create metrics tool")
			continue
		}

		ma.metricsTools[toolType] = tool
		ma.AddTool(tool)
	}
}

// initializeAlarmTools initializes alarm-related tools
func (ma *MonitoringAgent) initializeAlarmTools(toolFactory interfaces.ToolFactory) {
	alarmToolTypes := []string{
		"create-alarm",
		"list-alarms",
		"delete-alarm",
		"update-alarm",
		"enable-alarm",
		"disable-alarm",
	}

	for _, toolType := range alarmToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ma.awsClient,
		})
		if err != nil {
			ma.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create alarm tool")
			continue
		}

		ma.alarmTools[toolType] = tool
		ma.AddTool(tool)
	}
}

// initializeDashboardTools initializes dashboard-related tools
func (ma *MonitoringAgent) initializeDashboardTools(toolFactory interfaces.ToolFactory) {
	dashboardToolTypes := []string{
		"create-dashboard",
		"list-dashboards",
		"delete-dashboard",
		"update-dashboard",
	}

	for _, toolType := range dashboardToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: ma.awsClient,
		})
		if err != nil {
			ma.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create dashboard tool")
			continue
		}

		ma.dashboardTools[toolType] = tool
		ma.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createAlarm creates a new CloudWatch alarm
func (ma *MonitoringAgent) createAlarm(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Creating CloudWatch alarm")

	// Extract parameters
	alarmName := "monitoring-agent-alarm"
	if name, exists := task.Parameters["alarmName"]; exists {
		if nameStr, ok := name.(string); ok {
			alarmName = nameStr
		}
	}

	metricName := "CPUUtilization"
	if metric, exists := task.Parameters["metricName"]; exists {
		if metricStr, ok := metric.(string); ok {
			metricName = metricStr
		}
	}

	namespace := "AWS/EC2"
	if ns, exists := task.Parameters["namespace"]; exists {
		if nsStr, ok := ns.(string); ok {
			namespace = nsStr
		}
	}

	statistic := "Average"
	if stat, exists := task.Parameters["statistic"]; exists {
		if statStr, ok := stat.(string); ok {
			statistic = statStr
		}
	}

	period := 300
	if periodParam, exists := task.Parameters["period"]; exists {
		if periodInt, ok := periodParam.(int); ok {
			period = periodInt
		}
	}

	threshold := 80.0
	if thresholdParam, exists := task.Parameters["threshold"]; exists {
		if thresholdFloat, ok := thresholdParam.(float64); ok {
			threshold = thresholdFloat
		}
	}

	comparisonOperator := "GreaterThanThreshold"
	if comparison, exists := task.Parameters["comparisonOperator"]; exists {
		if comparisonStr, ok := comparison.(string); ok {
			comparisonOperator = comparisonStr
		}
	}

	evaluationPeriods := 2
	if evaluation, exists := task.Parameters["evaluationPeriods"]; exists {
		if evaluationInt, ok := evaluation.(int); ok {
			evaluationPeriods = evaluationInt
		}
	}

	// Use alarm tool to create alarm
	tool, exists := ma.alarmTools["create-alarm"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch alarm creation tool not available"
		return task, fmt.Errorf("CloudWatch alarm creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"alarmName":          alarmName,
		"metricName":         metricName,
		"namespace":          namespace,
		"statistic":          statistic,
		"period":             period,
		"threshold":          threshold,
		"comparisonOperator": comparisonOperator,
		"evaluationPeriods":  evaluationPeriods,
		"tags": map[string]string{
			"Name":      alarmName,
			"CreatedBy": "monitoring-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create CloudWatch alarm: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"alarm_name":          alarmName,
		"metric_name":         metricName,
		"namespace":           namespace,
		"statistic":           statistic,
		"period":              period,
		"threshold":           threshold,
		"comparison_operator": comparisonOperator,
		"evaluation_periods":  evaluationPeriods,
		"status":              "created",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":    task.ID,
		"alarm_name": alarmName,
	}).Info("CloudWatch alarm created successfully")

	return task, nil
}

// createDashboard creates a new CloudWatch dashboard
func (ma *MonitoringAgent) createDashboard(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Creating CloudWatch dashboard")

	// Extract parameters
	dashboardName := "monitoring-agent-dashboard"
	if name, exists := task.Parameters["dashboardName"]; exists {
		if nameStr, ok := name.(string); ok {
			dashboardName = nameStr
		}
	}

	dashboardBody := `{
		"widgets": [
			{
				"type": "metric",
				"x": 0,
				"y": 0,
				"width": 12,
				"height": 6,
				"properties": {
					"metrics": [
						["AWS/EC2", "CPUUtilization", "InstanceId", "i-1234567890abcdef0"]
					],
					"period": 300,
					"stat": "Average",
					"region": "us-west-2",
					"title": "EC2 CPU Utilization"
				}
			}
		]
	}`

	if body, exists := task.Parameters["dashboardBody"]; exists {
		if bodyStr, ok := body.(string); ok {
			dashboardBody = bodyStr
		}
	}

	// Use dashboard tool to create dashboard
	tool, exists := ma.dashboardTools["create-dashboard"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch dashboard creation tool not available"
		return task, fmt.Errorf("CloudWatch dashboard creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"dashboardName": dashboardName,
		"dashboardBody": dashboardBody,
		"tags": map[string]string{
			"Name":      dashboardName,
			"CreatedBy": "monitoring-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create CloudWatch dashboard: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"dashboard_name": dashboardName,
		"dashboard_body": dashboardBody,
		"status":         "created",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":        task.ID,
		"dashboard_name": dashboardName,
	}).Info("CloudWatch dashboard created successfully")

	return task, nil
}

// createLogGroup creates a new CloudWatch log group
func (ma *MonitoringAgent) createLogGroup(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Creating CloudWatch log group")

	// Extract parameters
	logGroupName := "monitoring-agent-logs"
	if name, exists := task.Parameters["logGroupName"]; exists {
		if nameStr, ok := name.(string); ok {
			logGroupName = nameStr
		}
	}

	retentionInDays := 14
	if retention, exists := task.Parameters["retentionInDays"]; exists {
		if retentionInt, ok := retention.(int); ok {
			retentionInDays = retentionInt
		}
	}

	// Use logs tool to create log group
	tool, exists := ma.logsTools["create-log-group"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch log group creation tool not available"
		return task, fmt.Errorf("CloudWatch log group creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"logGroupName":    logGroupName,
		"retentionInDays": retentionInDays,
		"tags": map[string]string{
			"Name":      logGroupName,
			"CreatedBy": "monitoring-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create CloudWatch log group: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"log_group_name":    logGroupName,
		"retention_in_days": retentionInDays,
		"status":            "created",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":        task.ID,
		"log_group_name": logGroupName,
	}).Info("CloudWatch log group created successfully")

	return task, nil
}

// createLogStream creates a new CloudWatch log stream
func (ma *MonitoringAgent) createLogStream(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Creating CloudWatch log stream")

	// Extract parameters
	logGroupName := "monitoring-agent-logs"
	if groupName, exists := task.Parameters["logGroupName"]; exists {
		if groupNameStr, ok := groupName.(string); ok {
			logGroupName = groupNameStr
		}
	}

	logStreamName := "monitoring-agent-stream"
	if streamName, exists := task.Parameters["logStreamName"]; exists {
		if streamNameStr, ok := streamName.(string); ok {
			logStreamName = streamNameStr
		}
	}

	// Use logs tool to create log stream
	tool, exists := ma.logsTools["create-log-stream"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch log stream creation tool not available"
		return task, fmt.Errorf("CloudWatch log stream creation tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"logGroupName":  logGroupName,
		"logStreamName": logStreamName,
		"tags": map[string]string{
			"Name":      logStreamName,
			"CreatedBy": "monitoring-agent",
			"TaskID":    task.ID,
		},
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create CloudWatch log stream: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"log_group_name":  logGroupName,
		"log_stream_name": logStreamName,
		"status":          "created",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":         task.ID,
		"log_group_name":  logGroupName,
		"log_stream_name": logStreamName,
	}).Info("CloudWatch log stream created successfully")

	return task, nil
}

// putMetric puts a custom metric to CloudWatch
func (ma *MonitoringAgent) putMetric(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Putting custom metric to CloudWatch")

	// Extract parameters
	namespace := "Custom/MonitoringAgent"
	if ns, exists := task.Parameters["namespace"]; exists {
		if nsStr, ok := ns.(string); ok {
			namespace = nsStr
		}
	}

	metricName := "CustomMetric"
	if metric, exists := task.Parameters["metricName"]; exists {
		if metricStr, ok := metric.(string); ok {
			metricName = metricStr
		}
	}

	value := 1.0
	if valueParam, exists := task.Parameters["value"]; exists {
		if valueFloat, ok := valueParam.(float64); ok {
			value = valueFloat
		}
	}

	unit := "Count"
	if unitParam, exists := task.Parameters["unit"]; exists {
		if unitStr, ok := unitParam.(string); ok {
			unit = unitStr
		}
	}

	dimensions := map[string]string{}
	if dims, exists := task.Parameters["dimensions"]; exists {
		if dimsMap, ok := dims.(map[string]interface{}); ok {
			for key, val := range dimsMap {
				if valStr, ok := val.(string); ok {
					dimensions[key] = valStr
				}
			}
		}
	}

	// Use metrics tool to put metric
	tool, exists := ma.metricsTools["put-metric"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch put metric tool not available"
		return task, fmt.Errorf("CloudWatch put metric tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"namespace":  namespace,
		"metricName": metricName,
		"value":      value,
		"unit":       unit,
		"dimensions": dimensions,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to put custom metric to CloudWatch: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"namespace":   namespace,
		"metric_name": metricName,
		"value":       value,
		"unit":        unit,
		"dimensions":  dimensions,
		"status":      "put",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"namespace":   namespace,
		"metric_name": metricName,
		"value":       value,
	}).Info("Custom metric put to CloudWatch successfully")

	return task, nil
}

// listMetrics lists CloudWatch metrics
func (ma *MonitoringAgent) listMetrics(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Listing CloudWatch metrics")

	// Extract parameters
	namespace := ""
	if ns, exists := task.Parameters["namespace"]; exists {
		if nsStr, ok := ns.(string); ok {
			namespace = nsStr
		}
	}

	metricName := ""
	if metric, exists := task.Parameters["metricName"]; exists {
		if metricStr, ok := metric.(string); ok {
			metricName = metricStr
		}
	}

	// Use metrics tool to list metrics
	tool, exists := ma.metricsTools["list-metrics"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch list metrics tool not available"
		return task, fmt.Errorf("CloudWatch list metrics tool not available")
	}

	// Execute tool
	data, err := tool.Execute(ctx, map[string]interface{}{
		"namespace":  namespace,
		"metricName": metricName,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to list CloudWatch metrics: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"namespace":   namespace,
		"metric_name": metricName,
		"metrics":     data,
		"status":      "listed",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"namespace": namespace,
	}).Info("CloudWatch metrics listed successfully")

	return task, nil
}

// getMetricStatistics gets CloudWatch metric statistics
func (ma *MonitoringAgent) getMetricStatistics(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Getting CloudWatch metric statistics")

	// Extract parameters
	namespace := "AWS/EC2"
	if ns, exists := task.Parameters["namespace"]; exists {
		if nsStr, ok := ns.(string); ok {
			namespace = nsStr
		}
	}

	metricName := "CPUUtilization"
	if metric, exists := task.Parameters["metricName"]; exists {
		if metricStr, ok := metric.(string); ok {
			metricName = metricStr
		}
	}

	statistics := []string{"Average"}
	if stats, exists := task.Parameters["statistics"]; exists {
		if statsList, ok := stats.([]interface{}); ok {
			statistics = []string{}
			for _, stat := range statsList {
				if statStr, ok := stat.(string); ok {
					statistics = append(statistics, statStr)
				}
			}
		}
	}

	period := 300
	if periodParam, exists := task.Parameters["period"]; exists {
		if periodInt, ok := periodParam.(int); ok {
			period = periodInt
		}
	}

	// Use metrics tool to get metric statistics
	tool, exists := ma.metricsTools["get-metric-statistics"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch get metric statistics tool not available"
		return task, fmt.Errorf("CloudWatch get metric statistics tool not available")
	}

	// Execute tool
	data, err := tool.Execute(ctx, map[string]interface{}{
		"namespace":  namespace,
		"metricName": metricName,
		"statistics": statistics,
		"period":     period,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to get CloudWatch metric statistics: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"namespace":   namespace,
		"metric_name": metricName,
		"statistics":  statistics,
		"period":      period,
		"data":        data,
		"status":      "retrieved",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"namespace":   namespace,
		"metric_name": metricName,
	}).Info("CloudWatch metric statistics retrieved successfully")

	return task, nil
}

// listAlarms lists CloudWatch alarms
func (ma *MonitoringAgent) listAlarms(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Listing CloudWatch alarms")

	// Extract parameters
	alarmNames := []string{}
	if names, exists := task.Parameters["alarmNames"]; exists {
		if namesList, ok := names.([]interface{}); ok {
			for _, name := range namesList {
				if nameStr, ok := name.(string); ok {
					alarmNames = append(alarmNames, nameStr)
				}
			}
		}
	}

	// Use alarm tool to list alarms
	tool, exists := ma.alarmTools["list-alarms"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch list alarms tool not available"
		return task, fmt.Errorf("CloudWatch list alarms tool not available")
	}

	// Execute tool
	data, err := tool.Execute(ctx, map[string]interface{}{
		"alarmNames": alarmNames,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to list CloudWatch alarms: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"alarm_names": alarmNames,
		"alarms":      data,
		"status":      "listed",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"alarm_names": alarmNames,
	}).Info("CloudWatch alarms listed successfully")

	return task, nil
}

// deleteAlarm deletes a CloudWatch alarm
func (ma *MonitoringAgent) deleteAlarm(ctx context.Context, task *Task) (*Task, error) {
	ma.logger.WithField("task_id", task.ID).Info("Deleting CloudWatch alarm")

	// Extract parameters
	alarmName := ""
	if name, exists := task.Parameters["alarmName"]; exists {
		if nameStr, ok := name.(string); ok {
			alarmName = nameStr
		}
	}

	if alarmName == "" {
		task.Status = TaskStatusFailed
		task.Error = "Alarm name is required for alarm deletion"
		return task, fmt.Errorf("alarm name is required for alarm deletion")
	}

	// Use alarm tool to delete alarm
	tool, exists := ma.alarmTools["delete-alarm"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "CloudWatch delete alarm tool not available"
		return task, fmt.Errorf("CloudWatch delete alarm tool not available")
	}

	// Execute tool
	_, err := tool.Execute(ctx, map[string]interface{}{
		"alarmName": alarmName,
	})

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to delete CloudWatch alarm: %w", err)
	}

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"alarm_name": alarmName,
		"status":     "deleted",
	}

	ma.logger.WithFields(map[string]interface{}{
		"task_id":    task.ID,
		"alarm_name": alarmName,
	}).Info("CloudWatch alarm deleted successfully")

	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// provideMonitoringForCompute provides monitoring setup for compute resources
func (ma *MonitoringAgent) provideMonitoringForCompute(computeAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with monitoring setup
	// For now, we'll log the coordination
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":      ma.id,
		"compute_agent": computeAgent.GetInfo().ID,
	}).Info("Providing monitoring setup for compute resources")

	return nil
}

// provideMonitoringForStorage provides monitoring setup for storage resources
func (ma *MonitoringAgent) provideMonitoringForStorage(storageAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with monitoring setup
	// For now, we'll log the coordination
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":      ma.id,
		"storage_agent": storageAgent.GetInfo().ID,
	}).Info("Providing monitoring setup for storage resources")

	return nil
}

// provideMonitoringForNetwork provides monitoring setup for network resources
func (ma *MonitoringAgent) provideMonitoringForNetwork(networkAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with monitoring setup
	// For now, we'll log the coordination
	ma.logger.WithFields(map[string]interface{}{
		"agent_id":      ma.id,
		"network_agent": networkAgent.GetInfo().ID,
	}).Info("Providing monitoring setup for network resources")

	return nil
}
