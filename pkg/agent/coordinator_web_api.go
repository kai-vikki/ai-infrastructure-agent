package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// COORDINATOR WEB API
// =============================================================================

// CoordinatorWebAPI provides HTTP API endpoints for the coordinator agent
type CoordinatorWebAPI struct {
	coordinator *CoordinatorAgent
	logger      *logging.Logger
	upgrader    websocket.Upgrader
}

// APIRequest represents a request to the coordinator API
type APIRequest struct {
	Request    string                 `json:"request"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Timestamp  time.Time              `json:"timestamp,omitempty"`
}

// APIResponse represents a response from the coordinator API
type APIResponse struct {
	RequestID     string                 `json:"request_id"`
	Status        string                 `json:"status"`
	Message       string                 `json:"message,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	ExecutionTime time.Duration          `json:"execution_time,omitempty"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewCoordinatorWebAPI creates a new coordinator web API
func NewCoordinatorWebAPI(coordinator *CoordinatorAgent, logger *logging.Logger) *CoordinatorWebAPI {
	return &CoordinatorWebAPI{
		coordinator: coordinator,
		logger:      logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development, restrict in production
				return true
			},
		},
	}
}

// =============================================================================
// HTTP HANDLERS
// =============================================================================

// HandleNaturalLanguageRequest handles natural language requests
func (cwa *CoordinatorWebAPI) HandleNaturalLanguageRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling natural language request")

	// Parse request
	var apiReq APIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		cwa.sendErrorResponse(w, requestID, "Invalid request payload", err)
		return
	}

	// Set request ID if not provided
	if apiReq.RequestID == "" {
		apiReq.RequestID = requestID
	}
	apiReq.Timestamp = time.Now()

	// Validate request
	if apiReq.Request == "" {
		cwa.sendErrorResponse(w, requestID, "Request field is required", nil)
		return
	}

	// Process the request
	ctx := context.Background()
	result, err := cwa.processNaturalLanguageRequest(ctx, &apiReq)
	if err != nil {
		cwa.sendErrorResponse(w, requestID, "Failed to process request", err)
		return
	}

	// Send response
	response := APIResponse{
		RequestID:     requestID,
		Status:        "success",
		Message:       "Request processed successfully",
		Data:          result,
		Timestamp:     time.Now(),
		ExecutionTime: time.Since(startTime),
	}

	cwa.sendJSONResponse(w, http.StatusOK, response)
}

// HandleTaskDecomposition handles task decomposition requests
func (cwa *CoordinatorWebAPI) HandleTaskDecomposition(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling task decomposition request")

	// Parse request
	var apiReq APIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		cwa.sendErrorResponse(w, requestID, "Invalid request payload", err)
		return
	}

	// Set request ID if not provided
	if apiReq.RequestID == "" {
		apiReq.RequestID = requestID
	}
	apiReq.Timestamp = time.Now()

	// Validate request
	if apiReq.Request == "" {
		cwa.sendErrorResponse(w, requestID, "Request field is required", nil)
		return
	}

	// Process the request
	ctx := context.Background()
	result, err := cwa.processTaskDecomposition(ctx, &apiReq)
	if err != nil {
		cwa.sendErrorResponse(w, requestID, "Failed to decompose request", err)
		return
	}

	// Send response
	response := APIResponse{
		RequestID:     requestID,
		Status:        "success",
		Message:       "Request decomposed successfully",
		Data:          result,
		Timestamp:     time.Now(),
		ExecutionTime: time.Since(startTime),
	}

	cwa.sendJSONResponse(w, http.StatusOK, response)
}

// HandleTaskExecution handles task execution requests
func (cwa *CoordinatorWebAPI) HandleTaskExecution(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling task execution request")

	// Parse request
	var apiReq APIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		cwa.sendErrorResponse(w, requestID, "Invalid request payload", err)
		return
	}

	// Set request ID if not provided
	if apiReq.RequestID == "" {
		apiReq.RequestID = requestID
	}
	apiReq.Timestamp = time.Now()

	// Validate request
	if apiReq.Request == "" {
		cwa.sendErrorResponse(w, requestID, "Request field is required", nil)
		return
	}

	// Process the request
	ctx := context.Background()
	result, err := cwa.processTaskExecution(ctx, &apiReq)
	if err != nil {
		cwa.sendErrorResponse(w, requestID, "Failed to execute tasks", err)
		return
	}

	// Send response
	response := APIResponse{
		RequestID:     requestID,
		Status:        "success",
		Message:       "Tasks executed successfully",
		Data:          result,
		Timestamp:     time.Now(),
		ExecutionTime: time.Since(startTime),
	}

	cwa.sendJSONResponse(w, http.StatusOK, response)
}

// HandleIntelligentDecision handles intelligent decision requests
func (cwa *CoordinatorWebAPI) HandleIntelligentDecision(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling intelligent decision request")

	// Parse request
	var apiReq APIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		cwa.sendErrorResponse(w, requestID, "Invalid request payload", err)
		return
	}

	// Set request ID if not provided
	if apiReq.RequestID == "" {
		apiReq.RequestID = requestID
	}
	apiReq.Timestamp = time.Now()

	// Validate request
	if apiReq.Request == "" {
		cwa.sendErrorResponse(w, requestID, "Request field is required", nil)
		return
	}

	// Process the request
	ctx := context.Background()
	result, err := cwa.processIntelligentDecision(ctx, &apiReq)
	if err != nil {
		cwa.sendErrorResponse(w, requestID, "Failed to make intelligent decision", err)
		return
	}

	// Send response
	response := APIResponse{
		RequestID:     requestID,
		Status:        "success",
		Message:       "Intelligent decision made successfully",
		Data:          result,
		Timestamp:     time.Now(),
		ExecutionTime: time.Since(startTime),
	}

	cwa.sendJSONResponse(w, http.StatusOK, response)
}

// HandleStatus handles status requests
func (cwa *CoordinatorWebAPI) HandleStatus(w http.ResponseWriter, r *http.Request) {
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling status request")

	// Get coordinator status
	status := cwa.getCoordinatorStatus()

	// Send response
	response := APIResponse{
		RequestID: requestID,
		Status:    "success",
		Message:   "Status retrieved successfully",
		Data:      status,
		Timestamp: time.Now(),
	}

	cwa.sendJSONResponse(w, http.StatusOK, response)
}

// HandleWebSocket handles WebSocket connections
func (cwa *CoordinatorWebAPI) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	requestID := uuid.New().String()

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     r.Method,
		"path":       r.URL.Path,
	}).Info("Handling WebSocket connection")

	// Upgrade connection to WebSocket
	conn, err := cwa.upgrader.Upgrade(w, r, nil)
	if err != nil {
		cwa.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	cwa.logger.WithField("request_id", requestID).Info("WebSocket connection established")

	// Handle WebSocket messages
	for {
		var message WebSocketMessage
		if err := conn.ReadJSON(&message); err != nil {
			cwa.logger.WithError(err).Error("Failed to read WebSocket message")
			break
		}

		// Process WebSocket message
		response, err := cwa.processWebSocketMessage(&message)
		if err != nil {
			cwa.logger.WithError(err).Error("Failed to process WebSocket message")
			response = &WebSocketMessage{
				Type: "error",
				Data: map[string]interface{}{
					"error": err.Error(),
				},
				Timestamp: time.Now(),
			}
		}

		// Send response
		if err := conn.WriteJSON(response); err != nil {
			cwa.logger.WithError(err).Error("Failed to write WebSocket message")
			break
		}
	}

	cwa.logger.WithField("request_id", requestID).Info("WebSocket connection closed")
}

// =============================================================================
// REQUEST PROCESSING METHODS
// =============================================================================

// processNaturalLanguageRequest processes natural language requests
func (cwa *CoordinatorWebAPI) processNaturalLanguageRequest(ctx context.Context, req *APIRequest) (map[string]interface{}, error) {
	cwa.logger.WithField("request_id", req.RequestID).Info("Processing natural language request")

	// 1. Decompose the request
	decompositionResult, err := cwa.coordinator.decomposeWithLLM(ctx, req.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose request: %w", err)
	}

	// 2. Create execution plan
	executionPlan, err := cwa.createExecutionPlan(ctx, decompositionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution plan: %w", err)
	}

	// 3. Execute tasks if requested
	var executionResults map[string]interface{}
	if execute, exists := req.Options["execute"]; exists {
		if shouldExecute, ok := execute.(bool); ok && shouldExecute {
			executionResults, err = cwa.executeTasks(ctx, decompositionResult)
			if err != nil {
				return nil, fmt.Errorf("failed to execute tasks: %w", err)
			}
		}
	}

	result := map[string]interface{}{
		"original_request":  req.Request,
		"decomposed_tasks":  decompositionResult,
		"execution_plan":    executionPlan,
		"execution_results": executionResults,
		"status":            "processed",
	}

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": req.RequestID,
		"task_count": len(decompositionResult),
		"executed":   executionResults != nil,
	}).Info("Natural language request processed successfully")

	return result, nil
}

// processTaskDecomposition processes task decomposition requests
func (cwa *CoordinatorWebAPI) processTaskDecomposition(ctx context.Context, req *APIRequest) (map[string]interface{}, error) {
	cwa.logger.WithField("request_id", req.RequestID).Info("Processing task decomposition request")

	// Use the task decomposition engine
	decompositionEngine := NewTaskDecompositionEngine(cwa.coordinator, cwa.logger)
	result, err := decompositionEngine.DecomposeRequest(ctx, req.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose request: %w", err)
	}

	// Convert result to map
	resultMap := map[string]interface{}{
		"original_request":    result.OriginalRequest,
		"identified_patterns": result.IdentifiedPatterns,
		"decomposed_tasks":    result.DecomposedTasks,
		"task_dependencies":   result.TaskDependencies,
		"execution_plan":      result.ExecutionPlan,
		"estimated_duration":  result.EstimatedDuration,
		"risk_assessment":     result.RiskAssessment,
		"status":              "decomposed",
	}

	cwa.logger.WithFields(map[string]interface{}{
		"request_id":         req.RequestID,
		"task_count":         len(result.DecomposedTasks),
		"estimated_duration": result.EstimatedDuration,
	}).Info("Task decomposition request processed successfully")

	return resultMap, nil
}

// processTaskExecution processes task execution requests
func (cwa *CoordinatorWebAPI) processTaskExecution(ctx context.Context, req *APIRequest) (map[string]interface{}, error) {
	cwa.logger.WithField("request_id", req.RequestID).Info("Processing task execution request")

	// Extract tasks from request
	tasks, exists := req.Parameters["tasks"]
	if !exists {
		return nil, fmt.Errorf("tasks parameter is required")
	}

	tasksList, ok := tasks.([]interface{})
	if !ok {
		return nil, fmt.Errorf("tasks parameter must be a list")
	}

	// Convert to Task objects
	executionTasks := make([]*Task, 0, len(tasksList))
	for _, taskData := range tasksList {
		taskMap, ok := taskData.(map[string]interface{})
		if !ok {
			continue
		}

		task := &Task{
			ID:         uuid.New().String(),
			Type:       taskMap["type"].(string),
			Parameters: taskMap["parameters"].(map[string]interface{}),
			Status:     TaskStatusPending,
			CreatedAt:  time.Now(),
		}
		executionTasks = append(executionTasks, task)
	}

	// Execute tasks using orchestration engine
	results, err := cwa.coordinator.orchestrationEngine.ExecuteTasks(ctx, executionTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasks: %w", err)
	}

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": req.RequestID,
		"task_count": len(executionTasks),
	}).Info("Task execution request processed successfully")

	return results, nil
}

// processIntelligentDecision processes intelligent decision requests
func (cwa *CoordinatorWebAPI) processIntelligentDecision(ctx context.Context, req *APIRequest) (map[string]interface{}, error) {
	cwa.logger.WithField("request_id", req.RequestID).Info("Processing intelligent decision request")

	// Use LLM to make intelligent decision
	decision, err := cwa.coordinator.makeDecisionWithLLM(ctx, req.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to make intelligent decision: %w", err)
	}

	result := map[string]interface{}{
		"context":  req.Request,
		"decision": decision,
		"status":   "decision_made",
	}

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": req.RequestID,
		"context":    req.Request,
	}).Info("Intelligent decision request processed successfully")

	return result, nil
}

// processWebSocketMessage processes WebSocket messages
func (cwa *CoordinatorWebAPI) processWebSocketMessage(message *WebSocketMessage) (*WebSocketMessage, error) {
	cwa.logger.WithField("message_type", message.Type).Info("Processing WebSocket message")

	switch message.Type {
	case "status":
		status := cwa.getCoordinatorStatus()
		return &WebSocketMessage{
			Type:      "status_response",
			Data:      status,
			Timestamp: time.Now(),
		}, nil
	case "decompose":
		request, exists := message.Data["request"]
		if !exists {
			return nil, fmt.Errorf("request field is required")
		}

		requestStr, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("request must be a string")
		}

		ctx := context.Background()
		result, err := cwa.coordinator.decomposeWithLLM(ctx, requestStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decompose request: %w", err)
		}

		return &WebSocketMessage{
			Type: "decomposition_response",
			Data: map[string]interface{}{
				"decomposed_tasks": result,
			},
			Timestamp: time.Now(),
		}, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// createExecutionPlan creates an execution plan from decomposed tasks
func (cwa *CoordinatorWebAPI) createExecutionPlan(ctx context.Context, tasks []map[string]interface{}) (map[string]interface{}, error) {
	// Convert to Task objects
	executionTasks := make([]*Task, 0, len(tasks))
	for _, taskData := range tasks {
		task := &Task{
			ID:         uuid.New().String(),
			Type:       taskData["type"].(string),
			Parameters: taskData["parameters"].(map[string]interface{}),
			Status:     TaskStatusPending,
			CreatedAt:  time.Now(),
		}
		executionTasks = append(executionTasks, task)
	}

	// Create execution plan using orchestration engine
	plan, err := cwa.coordinator.orchestrationEngine.createExecutionPlan(ctx, executionTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution plan: %w", err)
	}

	// Convert plan to map
	planMap := map[string]interface{}{
		"plan_id":            plan.PlanID,
		"tasks":              plan.Tasks,
		"dependencies":       plan.Dependencies,
		"execution_order":    plan.ExecutionOrder,
		"estimated_duration": plan.EstimatedDuration,
		"risk_assessment":    plan.RiskAssessment,
		"created_at":         plan.CreatedAt,
		"status":             plan.Status,
	}

	return planMap, nil
}

// executeTasks executes tasks using the orchestration engine
func (cwa *CoordinatorWebAPI) executeTasks(ctx context.Context, tasks []map[string]interface{}) (map[string]interface{}, error) {
	// Convert to Task objects
	executionTasks := make([]*Task, 0, len(tasks))
	for _, taskData := range tasks {
		task := &Task{
			ID:         uuid.New().String(),
			Type:       taskData["type"].(string),
			Parameters: taskData["parameters"].(map[string]interface{}),
			Status:     TaskStatusPending,
			CreatedAt:  time.Now(),
		}
		executionTasks = append(executionTasks, task)
	}

	// Execute tasks using orchestration engine
	results, err := cwa.coordinator.orchestrationEngine.ExecuteTasks(ctx, executionTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasks: %w", err)
	}

	return results, nil
}

// getCoordinatorStatus gets the current status of the coordinator
func (cwa *CoordinatorWebAPI) getCoordinatorStatus() map[string]interface{} {
	status := map[string]interface{}{
		"coordinator_id":   cwa.coordinator.GetInfo().ID,
		"coordinator_type": cwa.coordinator.GetAgentType(),
		"status":           cwa.coordinator.GetStatus(),
		"capabilities":     cwa.coordinator.GetCapabilities(),
		"active_tasks":     len(cwa.coordinator.activeTasks),
		"completed_tasks":  len(cwa.coordinator.completedTasks),
		"failed_tasks":     len(cwa.coordinator.failedTasks),
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	return status
}

// sendJSONResponse sends a JSON response
func (cwa *CoordinatorWebAPI) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendErrorResponse sends an error response
func (cwa *CoordinatorWebAPI) sendErrorResponse(w http.ResponseWriter, requestID, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}

	response := APIResponse{
		RequestID: requestID,
		Status:    "error",
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	cwa.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"error":      errorMsg,
	}).Error("Sending error response")

	cwa.sendJSONResponse(w, http.StatusBadRequest, response)
}

// =============================================================================
// ROUTE SETUP
// =============================================================================

// SetupRoutes sets up the HTTP routes for the coordinator API
func (cwa *CoordinatorWebAPI) SetupRoutes(router *mux.Router) {
	// API routes
	router.HandleFunc("/api/coordinator/request", cwa.HandleNaturalLanguageRequest).Methods("POST")
	router.HandleFunc("/api/coordinator/decompose", cwa.HandleTaskDecomposition).Methods("POST")
	router.HandleFunc("/api/coordinator/execute", cwa.HandleTaskExecution).Methods("POST")
	router.HandleFunc("/api/coordinator/decision", cwa.HandleIntelligentDecision).Methods("POST")
	router.HandleFunc("/api/coordinator/status", cwa.HandleStatus).Methods("GET")

	// WebSocket route
	router.HandleFunc("/api/coordinator/ws", cwa.HandleWebSocket).Methods("GET")

	cwa.logger.Info("Coordinator Web API routes registered")
}
