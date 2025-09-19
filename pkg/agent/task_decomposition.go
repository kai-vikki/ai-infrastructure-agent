package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

// =============================================================================
// TASK DECOMPOSITION ENGINE
// =============================================================================

// TaskDecompositionEngine handles intelligent task decomposition
type TaskDecompositionEngine struct {
	coordinator *CoordinatorAgent
	logger      *logging.Logger
	patterns    *TaskPatterns
	templates   *TaskTemplates
}

// TaskPatterns contains patterns for identifying infrastructure requirements
type TaskPatterns struct {
	networkPatterns   []*regexp.Regexp
	computePatterns   []*regexp.Regexp
	storagePatterns   []*regexp.Regexp
	securityPatterns  []*regexp.Regexp
	monitoringPatterns []*regexp.Regexp
	backupPatterns    []*regexp.Regexp
}

// TaskTemplates contains templates for common infrastructure patterns
type TaskTemplates struct {
	webAppTemplate     []map[string]interface{}
	databaseTemplate   []map[string]interface{}
	loadBalancerTemplate []map[string]interface{}
	monitoringTemplate []map[string]interface{}
	backupTemplate     []map[string]interface{}
}

// DecompositionResult contains the result of task decomposition
type DecompositionResult struct {
	OriginalRequest    string                   `json:"original_request"`
	IdentifiedPatterns []string                 `json:"identified_patterns"`
	DecomposedTasks    []map[string]interface{} `json:"decomposed_tasks"`
	TaskDependencies   map[string][]string      `json:"task_dependencies"`
	ExecutionPlan      []string                 `json:"execution_plan"`
	EstimatedDuration  string                   `json:"estimated_duration"`
	RiskAssessment     map[string]interface{}   `json:"risk_assessment"`
}

// NewTaskDecompositionEngine creates a new task decomposition engine
func NewTaskDecompositionEngine(coordinator *CoordinatorAgent, logger *logging.Logger) *TaskDecompositionEngine {
	engine := &TaskDecompositionEngine{
		coordinator: coordinator,
		logger:      logger,
		patterns:    NewTaskPatterns(),
		templates:   NewTaskTemplates(),
	}
	return engine
}

// NewTaskPatterns creates task patterns for infrastructure identification
func NewTaskPatterns() *TaskPatterns {
	return &TaskPatterns{
		networkPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(vpc|virtual private cloud|network|subnet|route table|gateway)`),
			regexp.MustCompile(`(?i)(public|private|subnet|availability zone)`),
			regexp.MustCompile(`(?i)(internet gateway|nat gateway|route table)`),
		},
		computePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ec2|instance|server|compute|virtual machine)`),
			regexp.MustCompile(`(?i)(auto scaling|asg|load balancer|alb|elb)`),
			regexp.MustCompile(`(?i)(launch template|ami|instance type)`),
		},
		storagePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(rds|database|mysql|postgresql|aurora)`),
			regexp.MustCompile(`(?i)(ebs|volume|storage|s3|bucket)`),
			regexp.MustCompile(`(?i)(snapshot|backup|restore)`),
		},
		securityPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(security group|firewall|iam|role|policy)`),
			regexp.MustCompile(`(?i)(kms|encryption|key|secret)`),
			regexp.MustCompile(`(?i)(ssl|tls|certificate|https)`),
		},
		monitoringPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(cloudwatch|monitoring|alarm|dashboard|log)`),
			regexp.MustCompile(`(?i)(metric|alert|notification|sns)`),
			regexp.MustCompile(`(?i)(health check|status|uptime)`),
		},
		backupPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(backup|snapshot|disaster recovery|dr)`),
			regexp.MustCompile(`(?i)(restore|recovery|point in time)`),
			regexp.MustCompile(`(?i)(retention|archive|tape)`),
		},
	}
}

// NewTaskTemplates creates task templates for common infrastructure patterns
func NewTaskTemplates() *TaskTemplates {
	return &TaskTemplates{
		webAppTemplate: []map[string]interface{}{
			{
				"type":        "create_vpc",
				"agent_type":  "Network",
				"parameters": map[string]interface{}{
					"cidrBlock": "10.0.0.0/16",
					"name":      "web-app-vpc",
				},
				"dependencies": []string{},
				"priority":     5,
			},
			{
				"type":        "create_public_subnet",
				"agent_type":  "Network",
				"parameters": map[string]interface{}{
					"vpcId":            "{{vpc_id}}",
					"cidrBlock":        "10.0.1.0/24",
					"availabilityZone": "{{az_1}}",
					"name":             "web-app-public-subnet",
				},
				"dependencies": []string{"create_vpc"},
				"priority":     4,
			},
			{
				"type":        "create_private_subnet",
				"agent_type":  "Network",
				"parameters": map[string]interface{}{
					"vpcId":            "{{vpc_id}}",
					"cidrBlock":        "10.0.2.0/24",
					"availabilityZone": "{{az_2}}",
					"name":             "web-app-private-subnet",
				},
				"dependencies": []string{"create_vpc"},
				"priority":     4,
			},
			{
				"type":        "create_internet_gateway",
				"agent_type":  "Network",
				"parameters": map[string]interface{}{
					"name": "web-app-igw",
				},
				"dependencies": []string{},
				"priority":     3,
			},
			{
				"type":        "create_security_group",
				"agent_type":  "Security",
				"parameters": map[string]interface{}{
					"groupName":   "web-app-sg",
					"description": "Security group for web application",
					"vpcId":       "{{vpc_id}}",
				},
				"dependencies": []string{"create_vpc"},
				"priority":     3,
			},
			{
				"type":        "create_ec2_instance",
				"agent_type":  "Compute",
				"parameters": map[string]interface{}{
					"instanceType": "t3.micro",
					"subnetId":     "{{public_subnet_id}}",
					"securityGroupIds": []string{"{{security_group_id}}"},
					"name":         "web-app-instance",
				},
				"dependencies": []string{"create_public_subnet", "create_security_group"},
				"priority":     2,
			},
			{
				"type":        "create_load_balancer",
				"agent_type":  "Compute",
				"parameters": map[string]interface{}{
					"lbName":           "web-app-alb",
					"subnetIds":        []string{"{{public_subnet_id}}"},
					"securityGroupIds": []string{"{{security_group_id}}"},
					"scheme":           "internet-facing",
				},
				"dependencies": []string{"create_public_subnet", "create_security_group"},
				"priority":     2,
			},
		},
		databaseTemplate: []map[string]interface{}{
			{
				"type":        "create_db_subnet_group",
				"agent_type":  "Storage",
				"parameters": map[string]interface{}{
					"dbSubnetGroupName": "web-app-db-subnet-group",
					"description":       "Subnet group for web app database",
					"subnetIds":         []string{"{{private_subnet_id}}"},
				},
				"dependencies": []string{"create_private_subnet"},
				"priority":     3,
			},
			{
				"type":        "create_db_instance",
				"agent_type":  "Storage",
				"parameters": map[string]interface{}{
					"dbInstanceIdentifier": "web-app-db",
					"engine":               "mysql",
					"engineVersion":        "8.0",
					"instanceClass":        "db.t3.micro",
					"allocatedStorage":     20,
					"masterUsername":       "admin",
					"dbSubnetGroupName":    "{{db_subnet_group_name}}",
					"vpcSecurityGroupIds":  []string{"{{db_security_group_id}}"},
				},
				"dependencies": []string{"create_db_subnet_group", "create_db_security_group"},
				"priority":     1,
			},
		},
		loadBalancerTemplate: []map[string]interface{}{
			{
				"type":        "create_target_group",
				"agent_type":  "Compute",
				"parameters": map[string]interface{}{
					"tgName":   "web-app-tg",
					"vpcId":    "{{vpc_id}}",
					"port":     80,
					"protocol": "HTTP",
				},
				"dependencies": []string{"create_vpc"},
				"priority":     3,
			},
			{
				"type":        "create_listener",
				"agent_type":  "Compute",
				"parameters": map[string]interface{}{
					"loadBalancerArn": "{{load_balancer_arn}}",
					"targetGroupArn":  "{{target_group_arn}}",
					"port":            80,
					"protocol":        "HTTP",
				},
				"dependencies": []string{"create_load_balancer", "create_target_group"},
				"priority":     1,
			},
		},
		monitoringTemplate: []map[string]interface{}{
			{
				"type":        "create_alarm",
				"agent_type":  "Monitoring",
				"parameters": map[string]interface{}{
					"alarmName":           "web-app-cpu-alarm",
					"metricName":          "CPUUtilization",
					"namespace":           "AWS/EC2",
					"statistic":           "Average",
					"period":              300,
					"threshold":           80.0,
					"comparisonOperator":  "GreaterThanThreshold",
					"evaluationPeriods":   2,
				},
				"dependencies": []string{"create_ec2_instance"},
				"priority":     1,
			},
			{
				"type":        "create_dashboard",
				"agent_type":  "Monitoring",
				"parameters": map[string]interface{}{
					"dashboardName": "web-app-dashboard",
					"dashboardBody": `{
						"widgets": [
							{
								"type": "metric",
								"x": 0,
								"y": 0,
								"width": 12,
								"height": 6,
								"properties": {
									"metrics": [
										["AWS/EC2", "CPUUtilization", "InstanceId", "{{instance_id}}"]
									],
									"period": 300,
									"stat": "Average",
									"region": "us-west-2",
									"title": "EC2 CPU Utilization"
								}
							}
						]
					}`,
				},
				"dependencies": []string{"create_ec2_instance"},
				"priority":     1,
			},
		},
		backupTemplate: []map[string]interface{}{
			{
				"type":        "create_snapshot",
				"agent_type":  "Backup",
				"parameters": map[string]interface{}{
					"volumeId":    "{{volume_id}}",
					"description": "Daily backup snapshot",
					"name":        "web-app-daily-snapshot",
				},
				"dependencies": []string{"create_ec2_instance"},
				"priority":     1,
			},
			{
				"type":        "create_backup",
				"agent_type":  "Backup",
				"parameters": map[string]interface{}{
					"resourceId":   "{{db_instance_id}}",
					"resourceType": "RDS",
					"backupName":   "web-app-db-backup",
					"description":  "Daily database backup",
				},
				"dependencies": []string{"create_db_instance"},
				"priority":     1,
			},
		},
	}
}

// =============================================================================
// TASK DECOMPOSITION METHODS
// =============================================================================

// DecomposeRequest decomposes a natural language request into tasks
func (tde *TaskDecompositionEngine) DecomposeRequest(ctx context.Context, request string) (*DecompositionResult, error) {
	tde.logger.WithField("request", request).Info("Starting task decomposition")

	// 1. Identify patterns in the request
	patterns := tde.identifyPatterns(request)
	tde.logger.WithField("patterns", patterns).Info("Identified patterns")

	// 2. Determine infrastructure type
	infrastructureType := tde.determineInfrastructureType(patterns)
	tde.logger.WithField("infrastructure_type", infrastructureType).Info("Determined infrastructure type")

	// 3. Use LLM for intelligent decomposition
	llmTasks, err := tde.decomposeWithLLM(ctx, request, patterns, infrastructureType)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose with LLM: %w", err)
	}

	// 4. Apply templates if applicable
	templateTasks := tde.applyTemplates(infrastructureType, patterns)

	// 5. Merge and optimize tasks
	mergedTasks := tde.mergeAndOptimizeTasks(llmTasks, templateTasks)

	// 6. Build dependency graph
	dependencies := tde.buildDependencyGraph(mergedTasks)

	// 7. Create execution plan
	executionPlan := tde.createExecutionPlan(mergedTasks, dependencies)

	// 8. Assess risks
	riskAssessment := tde.assessRisks(mergedTasks, patterns)

	// 9. Estimate duration
	estimatedDuration := tde.estimateDuration(mergedTasks)

	result := &DecompositionResult{
		OriginalRequest:    request,
		IdentifiedPatterns: patterns,
		DecomposedTasks:    mergedTasks,
		TaskDependencies:   dependencies,
		ExecutionPlan:      executionPlan,
		EstimatedDuration:  estimatedDuration,
		RiskAssessment:     riskAssessment,
	}

	tde.logger.WithFields(map[string]interface{}{
		"task_count":         len(mergedTasks),
		"infrastructure_type": infrastructureType,
		"estimated_duration": estimatedDuration,
	}).Info("Task decomposition completed")

	return result, nil
}

// identifyPatterns identifies infrastructure patterns in the request
func (tde *TaskDecompositionEngine) identifyPatterns(request string) []string {
	var patterns []string
	requestLower := strings.ToLower(request)

	// Check network patterns
	for _, pattern := range tde.patterns.networkPatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "network")
			break
		}
	}

	// Check compute patterns
	for _, pattern := range tde.patterns.computePatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "compute")
			break
		}
	}

	// Check storage patterns
	for _, pattern := range tde.patterns.storagePatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "storage")
			break
		}
	}

	// Check security patterns
	for _, pattern := range tde.patterns.securityPatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "security")
			break
		}
	}

	// Check monitoring patterns
	for _, pattern := range tde.patterns.monitoringPatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "monitoring")
			break
		}
	}

	// Check backup patterns
	for _, pattern := range tde.patterns.backupPatterns {
		if pattern.MatchString(requestLower) {
			patterns = append(patterns, "backup")
			break
		}
	}

	return patterns
}

// determineInfrastructureType determines the type of infrastructure being requested
func (tde *TaskDecompositionEngine) determineInfrastructureType(patterns []string) string {
	// Simple heuristic to determine infrastructure type
	if contains(patterns, "network") && contains(patterns, "compute") && contains(patterns, "storage") {
		return "web_application"
	}
	if contains(patterns, "database") || contains(patterns, "storage") {
		return "database"
	}
	if contains(patterns, "load balancer") || contains(patterns, "compute") {
		return "load_balancer"
	}
	if contains(patterns, "monitoring") {
		return "monitoring"
	}
	if contains(patterns, "backup") {
		return "backup"
	}
	return "custom"
}

// decomposeWithLLM uses LLM for intelligent task decomposition
func (tde *TaskDecompositionEngine) decomposeWithLLM(ctx context.Context, request string, patterns []string, infrastructureType string) ([]map[string]interface{}, error) {
	prompt := fmt.Sprintf(`
You are an expert AWS infrastructure architect. Decompose the following request into specific, actionable tasks.

Request: "%s"
Identified Patterns: %v
Infrastructure Type: %s

Available agent types and their capabilities:
- Network Agent: VPC, subnets, route tables, gateways, networking
- Compute Agent: EC2 instances, auto scaling groups, load balancers, AMIs
- Storage Agent: RDS databases, EBS volumes, S3 buckets, snapshots
- Security Agent: Security groups, IAM roles/policies, KMS keys, secrets
- Monitoring Agent: CloudWatch alarms, dashboards, logs, metrics
- Backup Agent: Snapshots, backups, disaster recovery

Consider:
- AWS best practices
- Security requirements
- High availability
- Cost optimization
- Scalability
- Dependencies between resources

Return a JSON array of tasks, where each task has:
- type: the task type (e.g., "create_vpc", "create_ec2_instance")
- agent_type: the agent type that should handle this task
- parameters: a map of parameters needed for the task
- dependencies: array of task types this task depends on
- priority: priority level (1-5, 5 being highest)
- description: brief description of what this task does

Example format:
[
  {
    "type": "create_vpc",
    "agent_type": "Network",
    "parameters": {
      "cidrBlock": "10.0.0.0/16",
      "name": "web-app-vpc"
    },
    "dependencies": [],
    "priority": 5,
    "description": "Create VPC for web application"
  }
]

Return only the JSON array, no additional text.
`, request, patterns, infrastructureType)

	// Call LLM
	response, err := tde.coordinator.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart(prompt)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}

	// Parse LLM response
	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(response.Choices[0].Content), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return tasks, nil
}

// applyTemplates applies predefined templates based on infrastructure type
func (tde *TaskDecompositionEngine) applyTemplates(infrastructureType string, patterns []string) []map[string]interface{} {
	var templateTasks []map[string]interface{}

	switch infrastructureType {
	case "web_application":
		templateTasks = append(templateTasks, tde.templates.webAppTemplate...)
		if contains(patterns, "database") {
			templateTasks = append(templateTasks, tde.templates.databaseTemplate...)
		}
		if contains(patterns, "load balancer") {
			templateTasks = append(templateTasks, tde.templates.loadBalancerTemplate...)
		}
		if contains(patterns, "monitoring") {
			templateTasks = append(templateTasks, tde.templates.monitoringTemplate...)
		}
		if contains(patterns, "backup") {
			templateTasks = append(templateTasks, tde.templates.backupTemplate...)
		}
	case "database":
		templateTasks = append(templateTasks, tde.templates.databaseTemplate...)
	case "load_balancer":
		templateTasks = append(templateTasks, tde.templates.loadBalancerTemplate...)
	case "monitoring":
		templateTasks = append(templateTasks, tde.templates.monitoringTemplate...)
	case "backup":
		templateTasks = append(templateTasks, tde.templates.backupTemplate...)
	}

	return templateTasks
}

// mergeAndOptimizeTasks merges LLM tasks with template tasks and optimizes
func (tde *TaskDecompositionEngine) mergeAndOptimizeTasks(llmTasks, templateTasks []map[string]interface{}) []map[string]interface{} {
	// Simple merge strategy - prefer LLM tasks over template tasks
	mergedTasks := make([]map[string]interface{}, 0, len(llmTasks)+len(templateTasks))
	
	// Add LLM tasks first
	mergedTasks = append(mergedTasks, llmTasks...)
	
	// Add template tasks that don't conflict
	for _, templateTask := range templateTasks {
		conflict := false
		for _, llmTask := range llmTasks {
			if llmTask["type"] == templateTask["type"] {
				conflict = true
				break
			}
		}
		if !conflict {
			mergedTasks = append(mergedTasks, templateTask)
		}
	}

	// Optimize tasks (remove duplicates, merge similar tasks, etc.)
	optimizedTasks := tde.optimizeTasks(mergedTasks)

	return optimizedTasks
}

// optimizeTasks optimizes the task list
func (tde *TaskDecompositionEngine) optimizeTasks(tasks []map[string]interface{}) []map[string]interface{} {
	// Remove duplicate tasks
	uniqueTasks := make([]map[string]interface{}, 0, len(tasks))
	seenTypes := make(map[string]bool)

	for _, task := range tasks {
		taskType, ok := task["type"].(string)
		if !ok {
			continue
		}
		
		if !seenTypes[taskType] {
			seenTypes[taskType] = true
			uniqueTasks = append(uniqueTasks, task)
		}
	}

	return uniqueTasks
}

// buildDependencyGraph builds a dependency graph for tasks
func (tde *TaskDecompositionEngine) buildDependencyGraph(tasks []map[string]interface{}) map[string][]string {
	dependencies := make(map[string][]string)

	for _, task := range tasks {
		taskType, ok := task["type"].(string)
		if !ok {
			continue
		}

		if deps, exists := task["dependencies"]; exists {
			if depsList, ok := deps.([]interface{}); ok {
				var depStrings []string
				for _, dep := range depsList {
					if depStr, ok := dep.(string); ok {
						depStrings = append(depStrings, depStr)
					}
				}
				dependencies[taskType] = depStrings
			}
		}
	}

	return dependencies
}

// createExecutionPlan creates an execution plan based on dependencies
func (tde *TaskDecompositionEngine) createExecutionPlan(tasks []map[string]interface{}, dependencies map[string][]string) []string {
	// Simple topological sort implementation
	var executionPlan []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(taskType string) error {
		if visiting[taskType] {
			return fmt.Errorf("circular dependency detected")
		}
		if visited[taskType] {
			return nil
		}

		visiting[taskType] = true
		for _, dep := range dependencies[taskType] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[taskType] = false
		visited[taskType] = true
		executionPlan = append(executionPlan, taskType)
		return nil
	}

	for _, task := range tasks {
		taskType, ok := task["type"].(string)
		if !ok {
			continue
		}
		
		if !visited[taskType] {
			if err := visit(taskType); err != nil {
				tde.logger.WithError(err).Error("Failed to create execution plan")
				continue
			}
		}
	}

	return executionPlan
}

// assessRisks assesses risks associated with the tasks
func (tde *TaskDecompositionEngine) assessRisks(tasks []map[string]interface{}, patterns []string) map[string]interface{} {
	risks := map[string]interface{}{
		"overall_risk_level": "medium",
		"identified_risks":   []string{},
		"mitigation_strategies": []string{},
	}

	var identifiedRisks []string
	var mitigationStrategies []string

	// Check for high-risk patterns
	if contains(patterns, "network") && contains(patterns, "compute") {
		identifiedRisks = append(identifiedRisks, "Complex network configuration may lead to connectivity issues")
		mitigationStrategies = append(mitigationStrategies, "Test network connectivity after each step")
	}

	if contains(patterns, "database") {
		identifiedRisks = append(identifiedRisks, "Database configuration requires careful security setup")
		mitigationStrategies = append(mitigationStrategies, "Use private subnets and security groups for database")
	}

	if contains(patterns, "load balancer") {
		identifiedRisks = append(identifiedRisks, "Load balancer configuration affects application availability")
		mitigationStrategies = append(mitigationStrategies, "Configure health checks and multiple availability zones")
	}

	// Determine overall risk level
	if len(identifiedRisks) > 3 {
		risks["overall_risk_level"] = "high"
	} else if len(identifiedRisks) > 1 {
		risks["overall_risk_level"] = "medium"
	} else {
		risks["overall_risk_level"] = "low"
	}

	risks["identified_risks"] = identifiedRisks
	risks["mitigation_strategies"] = mitigationStrategies

	return risks
}

// estimateDuration estimates the duration for task execution
func (tde *TaskDecompositionEngine) estimateDuration(tasks []map[string]interface{}) string {
	// Simple estimation based on task count and types
	taskCount := len(tasks)
	
	// Base time per task (in minutes)
	baseTimePerTask := 5
	
	// Adjust based on task types
	for _, task := range tasks {
		taskType, ok := task["type"].(string)
		if !ok {
			continue
		}
		
		switch {
		case strings.Contains(taskType, "database"):
			baseTimePerTask += 10 // Databases take longer
		case strings.Contains(taskType, "load_balancer"):
			baseTimePerTask += 5 // Load balancers take longer
		case strings.Contains(taskType, "vpc"):
			baseTimePerTask += 3 // VPCs take longer
		}
	}
	
	totalMinutes := taskCount * baseTimePerTask
	
	if totalMinutes < 60 {
		return fmt.Sprintf("%d minutes", totalMinutes)
	} else if totalMinutes < 1440 {
		hours := totalMinutes / 60
		minutes := totalMinutes % 60
		if minutes == 0 {
			return fmt.Sprintf("%d hours", hours)
		}
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	} else {
		days := totalMinutes / 1440
		hours := (totalMinutes % 1440) / 60
		if hours == 0 {
			return fmt.Sprintf("%d days", days)
		}
		return fmt.Sprintf("%d days %d hours", days, hours)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
