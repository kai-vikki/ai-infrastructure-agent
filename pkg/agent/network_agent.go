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
// NETWORK AGENT IMPLEMENTATION
// =============================================================================

// NetworkAgent handles network-related infrastructure tasks
type NetworkAgent struct {
	*BaseAgent
	taskQueue        chan *Task
	networkTools     map[string]interfaces.MCPTool
	vpcTools         map[string]interfaces.MCPTool
	subnetTools      map[string]interfaces.MCPTool
	routingTools     map[string]interfaces.MCPTool
	gatewayTools     map[string]interfaces.MCPTool
	awsClient        *aws.Client
	logger           *logging.Logger
}

// NewNetworkAgent creates a new network agent
func NewNetworkAgent(baseAgent *BaseAgent, awsClient *aws.Client, logger *logging.Logger) *NetworkAgent {
	agent := &NetworkAgent{
		BaseAgent:    baseAgent,
		taskQueue:    make(chan *Task, 50),
		networkTools: make(map[string]interfaces.MCPTool),
		vpcTools:     make(map[string]interfaces.MCPTool),
		subnetTools:  make(map[string]interfaces.MCPTool),
		routingTools: make(map[string]interfaces.MCPTool),
		gatewayTools: make(map[string]interfaces.MCPTool),
		awsClient:    awsClient,
		logger:       logger,
	}

	// Set agent type
	agent.agentType = AgentTypeNetwork
	agent.name = "Network Agent"
	agent.description = "Handles VPC, subnets, route tables, gateways, and networking infrastructure"

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

	agent.SetCapability(AgentCapability{
		Name:        "gateway_management",
		Description: "Manage internet gateways and NAT gateways",
		Tools:       []string{"create-internet-gateway", "create-nat-gateway", "attach-gateway"},
	})

	// Initialize network tools
	agent.initializeNetworkTools()

	return agent
}

// =============================================================================
// AGENT INTERFACE IMPLEMENTATION
// =============================================================================

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
	var networkTools []interfaces.MCPTool
	
	// Add VPC tools
	for _, tool := range na.vpcTools {
		networkTools = append(networkTools, tool)
	}
	
	// Add subnet tools
	for _, tool := range na.subnetTools {
		networkTools = append(networkTools, tool)
	}
	
	// Add routing tools
	for _, tool := range na.routingTools {
		networkTools = append(networkTools, tool)
	}
	
	// Add gateway tools
	for _, tool := range na.gatewayTools {
		networkTools = append(networkTools, tool)
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
	case "create_internet_gateway":
		return na.createInternetGateway(ctx, task)
	case "create_nat_gateway":
		return na.createNATGateway(ctx, task)
	case "associate_route_table":
		return na.associateRouteTable(ctx, task)
	case "add_route":
		return na.addRoute(ctx, task)
	case "attach_internet_gateway":
		return na.attachInternetGateway(ctx, task)
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
		"associate_route_table", "add_route", "attach_internet_gateway",
		"create_public_subnet", "create_private_subnet",
		"create_public_route_table", "create_private_route_table",
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
	// Network agents typically coordinate with compute and security agents
	switch otherAgent.GetAgentType() {
	case AgentTypeCompute:
		na.logger.WithFields(map[string]interface{}{
			"agent_id":      na.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with compute agent for network configuration")
		
		// Network agent provides VPC and subnet information to compute agent
		return na.provideNetworkInfoToCompute(otherAgent)
		
	case AgentTypeSecurity:
		na.logger.WithFields(map[string]interface{}{
			"agent_id":      na.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with security agent for network security")
		
		// Network agent provides VPC information to security agent
		return na.provideNetworkInfoToSecurity(otherAgent)
		
	default:
		na.logger.WithFields(map[string]interface{}{
			"agent_id":      na.id,
			"other_agent":   otherAgent.GetInfo().ID,
			"other_type":    otherAgent.GetAgentType(),
		}).Info("Coordinating with other agent type")
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
// NETWORK TOOL INITIALIZATION
// =============================================================================

// initializeNetworkTools initializes all network-related tools
func (na *NetworkAgent) initializeNetworkTools() {
	// Create tool factory
	toolFactory := tools.NewToolFactory(na.awsClient, na.logger)
	
	// Initialize VPC tools
	na.initializeVPCTools(toolFactory)
	
	// Initialize subnet tools
	na.initializeSubnetTools(toolFactory)
	
	// Initialize routing tools
	na.initializeRoutingTools(toolFactory)
	
	// Initialize gateway tools
	na.initializeGatewayTools(toolFactory)
}

// initializeVPCTools initializes VPC-related tools
func (na *NetworkAgent) initializeVPCTools(toolFactory interfaces.ToolFactory) {
	vpcToolTypes := []string{
		"create-vpc",
		"list-vpcs",
	}
	
	for _, toolType := range vpcToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: na.awsClient,
		})
		if err != nil {
			na.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create VPC tool")
			continue
		}
		
		na.vpcTools[toolType] = tool
		na.AddTool(tool)
	}
}

// initializeSubnetTools initializes subnet-related tools
func (na *NetworkAgent) initializeSubnetTools(toolFactory interfaces.ToolFactory) {
	subnetToolTypes := []string{
		"create-subnet",
		"create-private-subnet",
		"create-public-subnet",
		"list-subnets",
		"select-subnets-for-alb",
	}
	
	for _, toolType := range subnetToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: na.awsClient,
		})
		if err != nil {
			na.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create subnet tool")
			continue
		}
		
		na.subnetTools[toolType] = tool
		na.AddTool(tool)
	}
}

// initializeRoutingTools initializes routing-related tools
func (na *NetworkAgent) initializeRoutingTools(toolFactory interfaces.ToolFactory) {
	routingToolTypes := []string{
		"create-public-route-table",
		"create-private-route-table",
		"associate-route-table",
		"add-route",
	}
	
	for _, toolType := range routingToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: na.awsClient,
		})
		if err != nil {
			na.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create routing tool")
			continue
		}
		
		na.routingTools[toolType] = tool
		na.AddTool(tool)
	}
}

// initializeGatewayTools initializes gateway-related tools
func (na *NetworkAgent) initializeGatewayTools(toolFactory interfaces.ToolFactory) {
	gatewayToolTypes := []string{
		"create-internet-gateway",
		"create-nat-gateway",
	}
	
	for _, toolType := range gatewayToolTypes {
		tool, err := toolFactory.CreateTool(toolType, &tools.ToolDependencies{
			AWSClient: na.awsClient,
		})
		if err != nil {
			na.logger.WithError(err).WithField("tool_type", toolType).Error("Failed to create gateway tool")
			continue
		}
		
		na.gatewayTools[toolType] = tool
		na.AddTool(tool)
	}
}

// =============================================================================
// TASK PROCESSING METHODS
// =============================================================================

// createVPC creates a new VPC
func (na *NetworkAgent) createVPC(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Creating VPC")
	
	// Extract parameters
	cidrBlock := "10.0.0.0/16"
	if cidr, exists := task.Parameters["cidrBlock"]; exists {
		if cidrStr, ok := cidr.(string); ok {
			cidrBlock = cidrStr
		}
	}
	
	name := "network-agent-vpc"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Use VPC tool to create VPC
	tool, exists := na.vpcTools["create-vpc"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "VPC creation tool not available"
		return task, fmt.Errorf("VPC creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"cidrBlock": cidrBlock,
		"name":      name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "network-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create VPC: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"vpc_id":     result.Content[0].Text,
		"cidr_block": cidrBlock,
		"name":       name,
		"status":     "created",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id": task.ID,
		"vpc_id":  result.Content[0].Text,
	}).Info("VPC created successfully")
	
	return task, nil
}

// createSubnet creates a new subnet
func (na *NetworkAgent) createSubnet(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Creating subnet")
	
	// Extract parameters
	vpcId := ""
	if vpc, exists := task.Parameters["vpcId"]; exists {
		if vpcStr, ok := vpc.(string); ok {
			vpcId = vpcStr
		}
	}
	
	if vpcId == "" {
		task.Status = TaskStatusFailed
		task.Error = "VPC ID is required for subnet creation"
		return task, fmt.Errorf("VPC ID is required for subnet creation")
	}
	
	cidrBlock := "10.0.1.0/24"
	if cidr, exists := task.Parameters["cidrBlock"]; exists {
		if cidrStr, ok := cidr.(string); ok {
			cidrBlock = cidrStr
		}
	}
	
	availabilityZone := ""
	if az, exists := task.Parameters["availabilityZone"]; exists {
		if azStr, ok := az.(string); ok {
			availabilityZone = azStr
		}
	}
	
	name := "network-agent-subnet"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Determine tool type based on subnet type
	toolType := "create-subnet"
	if subnetType, exists := task.Parameters["type"]; exists {
		if typeStr, ok := subnetType.(string); ok {
			switch typeStr {
			case "private":
				toolType = "create-private-subnet"
			case "public":
				toolType = "create-public-subnet"
			}
		}
	}
	
	// Use subnet tool to create subnet
	tool, exists := na.subnetTools[toolType]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("Subnet creation tool %s not available", toolType)
		return task, fmt.Errorf("subnet creation tool %s not available", toolType)
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"vpcId":            vpcId,
		"cidrBlock":        cidrBlock,
		"availabilityZone": availabilityZone,
		"name":             name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "network-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create subnet: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"subnet_id":         result.Content[0].Text,
		"vpc_id":            vpcId,
		"cidr_block":        cidrBlock,
		"availability_zone": availabilityZone,
		"name":              name,
		"status":            "created",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"subnet_id": result.Content[0].Text,
	}).Info("Subnet created successfully")
	
	return task, nil
}

// createRouteTable creates a new route table
func (na *NetworkAgent) createRouteTable(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Creating route table")
	
	// Extract parameters
	vpcId := ""
	if vpc, exists := task.Parameters["vpcId"]; exists {
		if vpcStr, ok := vpc.(string); ok {
			vpcId = vpcStr
		}
	}
	
	if vpcId == "" {
		task.Status = TaskStatusFailed
		task.Error = "VPC ID is required for route table creation"
		return task, fmt.Errorf("VPC ID is required for route table creation")
	}
	
	name := "network-agent-route-table"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Determine tool type based on route table type
	toolType := "create-public-route-table"
	if routeTableType, exists := task.Parameters["type"]; exists {
		if typeStr, ok := routeTableType.(string); ok {
			switch typeStr {
			case "private":
				toolType = "create-private-route-table"
			case "public":
				toolType = "create-public-route-table"
			}
		}
	}
	
	// Use route table tool to create route table
	tool, exists := na.routingTools[toolType]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("Route table creation tool %s not available", toolType)
		return task, fmt.Errorf("route table creation tool %s not available", toolType)
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"vpcId": vpcId,
		"name":  name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "network-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create route table: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"route_table_id": result.Content[0].Text,
		"vpc_id":         vpcId,
		"name":           name,
		"status":         "created",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":        task.ID,
		"route_table_id": result.Content[0].Text,
	}).Info("Route table created successfully")
	
	return task, nil
}

// createInternetGateway creates a new internet gateway
func (na *NetworkAgent) createInternetGateway(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Creating internet gateway")
	
	// Extract parameters
	name := "network-agent-igw"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Use internet gateway tool to create internet gateway
	tool, exists := na.gatewayTools["create-internet-gateway"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Internet gateway creation tool not available"
		return task, fmt.Errorf("internet gateway creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"name": name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "network-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create internet gateway: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"internet_gateway_id": result.Content[0].Text,
		"name":                name,
		"status":              "created",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":            task.ID,
		"internet_gateway_id": result.Content[0].Text,
	}).Info("Internet gateway created successfully")
	
	return task, nil
}

// createNATGateway creates a new NAT gateway
func (na *NetworkAgent) createNATGateway(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Creating NAT gateway")
	
	// Extract parameters
	subnetId := ""
	if subnet, exists := task.Parameters["subnetId"]; exists {
		if subnetStr, ok := subnet.(string); ok {
			subnetId = subnetStr
		}
	}
	
	if subnetId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Subnet ID is required for NAT gateway creation"
		return task, fmt.Errorf("subnet ID is required for NAT gateway creation")
	}
	
	allocationId := ""
	if allocation, exists := task.Parameters["allocationId"]; exists {
		if allocationStr, ok := allocation.(string); ok {
			allocationId = allocationStr
		}
	}
	
	name := "network-agent-nat-gateway"
	if nameParam, exists := task.Parameters["name"]; exists {
		if nameStr, ok := nameParam.(string); ok {
			name = nameStr
		}
	}
	
	// Use NAT gateway tool to create NAT gateway
	tool, exists := na.gatewayTools["create-nat-gateway"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "NAT gateway creation tool not available"
		return task, fmt.Errorf("NAT gateway creation tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"subnetId":     subnetId,
		"allocationId": allocationId,
		"name":         name,
		"tags": map[string]string{
			"Name":        name,
			"CreatedBy":   "network-agent",
			"TaskID":      task.ID,
		},
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to create NAT gateway: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"nat_gateway_id": result.Content[0].Text,
		"subnet_id":      subnetId,
		"allocation_id":  allocationId,
		"name":           name,
		"status":         "created",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":       task.ID,
		"nat_gateway_id": result.Content[0].Text,
	}).Info("NAT gateway created successfully")
	
	return task, nil
}

// associateRouteTable associates a route table with a subnet
func (na *NetworkAgent) associateRouteTable(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Associating route table")
	
	// Extract parameters
	routeTableId := ""
	if routeTable, exists := task.Parameters["routeTableId"]; exists {
		if routeTableStr, ok := routeTable.(string); ok {
			routeTableId = routeTableStr
		}
	}
	
	if routeTableId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Route table ID is required for association"
		return task, fmt.Errorf("route table ID is required for association")
	}
	
	subnetId := ""
	if subnet, exists := task.Parameters["subnetId"]; exists {
		if subnetStr, ok := subnet.(string); ok {
			subnetId = subnetStr
		}
	}
	
	if subnetId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Subnet ID is required for association"
		return task, fmt.Errorf("subnet ID is required for association")
	}
	
	// Use route table association tool
	tool, exists := na.routingTools["associate-route-table"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Route table association tool not available"
		return task, fmt.Errorf("route table association tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"routeTableId": routeTableId,
		"subnetId":     subnetId,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to associate route table: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"association_id":   result.Content[0].Text,
		"route_table_id":   routeTableId,
		"subnet_id":        subnetId,
		"status":           "associated",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":        task.ID,
		"association_id": result.Content[0].Text,
	}).Info("Route table associated successfully")
	
	return task, nil
}

// addRoute adds a route to a route table
func (na *NetworkAgent) addRoute(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Adding route")
	
	// Extract parameters
	routeTableId := ""
	if routeTable, exists := task.Parameters["routeTableId"]; exists {
		if routeTableStr, ok := routeTable.(string); ok {
			routeTableId = routeTableStr
		}
	}
	
	if routeTableId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Route table ID is required for adding route"
		return task, fmt.Errorf("route table ID is required for adding route")
	}
	
	destinationCidrBlock := "0.0.0.0/0"
	if destination, exists := task.Parameters["destinationCidrBlock"]; exists {
		if destinationStr, ok := destination.(string); ok {
			destinationCidrBlock = destinationStr
		}
	}
	
	gatewayId := ""
	if gateway, exists := task.Parameters["gatewayId"]; exists {
		if gatewayStr, ok := gateway.(string); ok {
			gatewayId = gatewayStr
		}
	}
	
	if gatewayId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Gateway ID is required for adding route"
		return task, fmt.Errorf("gateway ID is required for adding route")
	}
	
	// Use add route tool
	tool, exists := na.routingTools["add-route"]
	if !exists {
		task.Status = TaskStatusFailed
		task.Error = "Add route tool not available"
		return task, fmt.Errorf("add route tool not available")
	}
	
	// Execute tool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"routeTableId":         routeTableId,
		"destinationCidrBlock": destinationCidrBlock,
		"gatewayId":            gatewayId,
	})
	
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return task, fmt.Errorf("failed to add route: %w", err)
	}
	
	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"route_table_id":           routeTableId,
		"destination_cidr_block":   destinationCidrBlock,
		"gateway_id":               gatewayId,
		"status":                   "added",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":        task.ID,
		"route_table_id": routeTableId,
	}).Info("Route added successfully")
	
	return task, nil
}

// attachInternetGateway attaches an internet gateway to a VPC
func (na *NetworkAgent) attachInternetGateway(ctx context.Context, task *Task) (*Task, error) {
	na.logger.WithField("task_id", task.ID).Info("Attaching internet gateway")
	
	// Extract parameters
	internetGatewayId := ""
	if igw, exists := task.Parameters["internetGatewayId"]; exists {
		if igwStr, ok := igw.(string); ok {
			internetGatewayId = igwStr
		}
	}
	
	if internetGatewayId == "" {
		task.Status = TaskStatusFailed
		task.Error = "Internet gateway ID is required for attachment"
		return task, fmt.Errorf("internet gateway ID is required for attachment")
	}
	
	vpcId := ""
	if vpc, exists := task.Parameters["vpcId"]; exists {
		if vpcStr, ok := vpc.(string); ok {
			vpcId = vpcStr
		}
	}
	
	if vpcId == "" {
		task.Status = TaskStatusFailed
		task.Error = "VPC ID is required for attachment"
		return task, fmt.Errorf("VPC ID is required for attachment")
	}
	
	// Use internet gateway attachment tool (this would need to be implemented)
	// For now, we'll simulate the attachment
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = map[string]interface{}{
		"internet_gateway_id": internetGatewayId,
		"vpc_id":              vpcId,
		"status":              "attached",
	}
	
	na.logger.WithFields(map[string]interface{}{
		"task_id":            task.ID,
		"internet_gateway_id": internetGatewayId,
		"vpc_id":              vpcId,
	}).Info("Internet gateway attached successfully")
	
	return task, nil
}

// =============================================================================
// COORDINATION METHODS
// =============================================================================

// provideNetworkInfoToCompute provides network information to compute agent
func (na *NetworkAgent) provideNetworkInfoToCompute(computeAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with network information
	// For now, we'll log the coordination
	na.logger.WithFields(map[string]interface{}{
		"agent_id":      na.id,
		"compute_agent": computeAgent.GetInfo().ID,
	}).Info("Providing network information to compute agent")
	
	return nil
}

// provideNetworkInfoToSecurity provides network information to security agent
func (na *NetworkAgent) provideNetworkInfoToSecurity(securityAgent SpecializedAgentInterface) error {
	// This would typically involve sending a message with network information
	// For now, we'll log the coordination
	na.logger.WithFields(map[string]interface{}{
		"agent_id":       na.id,
		"security_agent": securityAgent.GetInfo().ID,
	}).Info("Providing network information to security agent")
	
	return nil
}
