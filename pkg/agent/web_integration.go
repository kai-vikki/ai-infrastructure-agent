package agent

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// WEB INTEGRATION FOR MULTI-AGENT SYSTEM
// =============================================================================

// WebIntegration provides HTTP endpoints for the multi-agent system
type WebIntegration struct {
	multiAgentSystem *MultiAgentSystem
	logger           *logging.Logger
	router           *mux.Router
}

// NewWebIntegration creates a new web integration for the multi-agent system
func NewWebIntegration(multiAgentSystem *MultiAgentSystem, logger *logging.Logger) *WebIntegration {
	integration := &WebIntegration{
		multiAgentSystem: multiAgentSystem,
		logger:           logger,
		router:           mux.NewRouter(),
	}

	// Setup routes
	integration.setupRoutes()

	return integration
}

// =============================================================================
// ROUTE SETUP
// =============================================================================

// setupRoutes sets up all HTTP routes for the multi-agent system
func (wi *WebIntegration) setupRoutes() {
	// System management routes
	wi.router.HandleFunc("/api/multi-agent/status", wi.getSystemStatus).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/metrics", wi.getSystemMetrics).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/health", wi.getSystemHealth).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/diagnostics", wi.getSystemDiagnostics).Methods("GET")

	// Agent management routes
	wi.router.HandleFunc("/api/multi-agent/agents", wi.getAllAgents).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/agents/{agentId}", wi.getAgent).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/agents/{agentId}/metrics", wi.getAgentMetrics).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/agents/{agentId}/health", wi.getAgentHealth).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/agents/by-type/{agentType}", wi.getAgentsByType).Methods("GET")

	// Task management routes
	wi.router.HandleFunc("/api/multi-agent/tasks", wi.getAllTasks).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/tasks/{taskId}", wi.getTask).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/tasks/{taskId}/status", wi.getTaskStatus).Methods("GET")

	// Request processing routes
	wi.router.HandleFunc("/api/multi-agent/process", wi.processRequest).Methods("POST")
	wi.router.HandleFunc("/api/multi-agent/coordinate", wi.coordinateExecution).Methods("POST")

	// Message bus routes
	wi.router.HandleFunc("/api/multi-agent/messages/stats", wi.getMessageBusStats).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/messages/{agentId}/queue", wi.getAgentMessageQueue).Methods("GET")

	// Agent registry routes
	wi.router.HandleFunc("/api/multi-agent/registry/agents", wi.getRegistryAgents).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/registry/health", wi.getRegistryHealth).Methods("GET")
	wi.router.HandleFunc("/api/multi-agent/registry/statistics", wi.getRegistryStatistics).Methods("GET")
}

// =============================================================================
// SYSTEM MANAGEMENT ENDPOINTS
// =============================================================================

// getSystemStatus returns the current status of the multi-agent system
func (wi *WebIntegration) getSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := wi.multiAgentSystem.GetSystemStatus()
	wi.writeJSONResponse(w, http.StatusOK, status)
}

// getSystemMetrics returns detailed metrics for the multi-agent system
func (wi *WebIntegration) getSystemMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := wi.multiAgentSystem.GetSystemMetrics()
	wi.writeJSONResponse(w, http.StatusOK, metrics)
}

// getSystemHealth performs a health check on the entire system
func (wi *WebIntegration) getSystemHealth(w http.ResponseWriter, r *http.Request) {
	err := wi.multiAgentSystem.HealthCheck()
	if err != nil {
		wi.writeJSONResponse(w, http.StatusServiceUnavailable, map[string]interface{}{
			"healthy": false,
			"error":   err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"healthy": true,
		"status":  "all systems operational",
	})
}

// getSystemDiagnostics returns detailed diagnostics for the system
func (wi *WebIntegration) getSystemDiagnostics(w http.ResponseWriter, r *http.Request) {
	diagnostics := wi.multiAgentSystem.GetSystemDiagnostics()
	wi.writeJSONResponse(w, http.StatusOK, diagnostics)
}

// =============================================================================
// AGENT MANAGEMENT ENDPOINTS
// =============================================================================

// getAllAgents returns all agents in the system
func (wi *WebIntegration) getAllAgents(w http.ResponseWriter, r *http.Request) {
	agents := wi.multiAgentSystem.agentRegistry.GetAllAgents()

	agentList := make([]map[string]interface{}, 0, len(agents))
	for _, agent := range agents {
		agentInfo := agent.GetInfo()
		agentList = append(agentList, map[string]interface{}{
			"id":           agentInfo.ID,
			"type":         agentInfo.Type,
			"name":         agentInfo.Name,
			"description":  agentInfo.Description,
			"status":       agentInfo.Status,
			"capabilities": agentInfo.Capabilities,
			"tools":        agentInfo.Tools,
			"lastSeen":     agentInfo.LastSeen,
			"metadata":     agentInfo.Metadata,
		})
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"agents": agentList,
		"count":  len(agentList),
	})
}

// getAgent returns a specific agent by ID
func (wi *WebIntegration) getAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	agent, exists := wi.multiAgentSystem.GetAgent(agentID)
	if !exists {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": "agent not found",
		})
		return
	}

	agentInfo := agent.GetInfo()
	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":           agentInfo.ID,
		"type":         agentInfo.Type,
		"name":         agentInfo.Name,
		"description":  agentInfo.Description,
		"status":       agentInfo.Status,
		"capabilities": agentInfo.Capabilities,
		"tools":        agentInfo.Tools,
		"lastSeen":     agentInfo.LastSeen,
		"metadata":     agentInfo.Metadata,
	})
}

// getAgentMetrics returns metrics for a specific agent
func (wi *WebIntegration) getAgentMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	agent, exists := wi.multiAgentSystem.GetAgent(agentID)
	if !exists {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": "agent not found",
		})
		return
	}

	metrics := agent.GetMetrics()
	wi.writeJSONResponse(w, http.StatusOK, metrics)
}

// getAgentHealth performs a health check on a specific agent
func (wi *WebIntegration) getAgentHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	agent, exists := wi.multiAgentSystem.GetAgent(agentID)
	if !exists {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": "agent not found",
		})
		return
	}

	err := agent.HealthCheck()
	if err != nil {
		wi.writeJSONResponse(w, http.StatusServiceUnavailable, map[string]interface{}{
			"healthy": false,
			"error":   err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"healthy": true,
		"status":  "agent operational",
	})
}

// getAgentsByType returns all agents of a specific type
func (wi *WebIntegration) getAgentsByType(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentTypeStr := vars["agentType"]

	agentType := AgentType(agentTypeStr)
	agents := wi.multiAgentSystem.GetAgentsByType(agentType)

	agentList := make([]map[string]interface{}, 0, len(agents))
	for _, agent := range agents {
		agentInfo := agent.GetInfo()
		agentList = append(agentList, map[string]interface{}{
			"id":           agentInfo.ID,
			"type":         agentInfo.Type,
			"name":         agentInfo.Name,
			"description":  agentInfo.Description,
			"status":       agentInfo.Status,
			"capabilities": agentInfo.Capabilities,
			"tools":        agentInfo.Tools,
			"lastSeen":     agentInfo.LastSeen,
			"metadata":     agentInfo.Metadata,
		})
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"agents":     agentList,
		"count":      len(agentList),
		"agent_type": agentType,
	})
}

// =============================================================================
// TASK MANAGEMENT ENDPOINTS
// =============================================================================

// getAllTasks returns all tasks in the system
func (wi *WebIntegration) getAllTasks(w http.ResponseWriter, r *http.Request) {
	tasks := wi.multiAgentSystem.GetAllTasks()

	taskList := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, map[string]interface{}{
			"id":            task.ID,
			"type":          task.Type,
			"description":   task.Description,
			"assignedAgent": task.AssignedAgent,
			"requiredAgent": task.RequiredAgent,
			"parameters":    task.Parameters,
			"priority":      task.Priority,
			"status":        task.Status,
			"createdAt":     task.CreatedAt,
			"startedAt":     task.StartedAt,
			"completedAt":   task.CompletedAt,
			"result":        task.Result,
			"error":         task.Error,
			"dependencies":  task.Dependencies,
			"dependents":    task.Dependents,
		})
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"tasks": taskList,
		"count": len(taskList),
	})
}

// getTask returns a specific task by ID
func (wi *WebIntegration) getTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskId"]

	task, err := wi.multiAgentSystem.GetTaskStatus(taskID)
	if err != nil {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":            task.ID,
		"type":          task.Type,
		"description":   task.Description,
		"assignedAgent": task.AssignedAgent,
		"requiredAgent": task.RequiredAgent,
		"parameters":    task.Parameters,
		"priority":      task.Priority,
		"status":        task.Status,
		"createdAt":     task.CreatedAt,
		"startedAt":     task.StartedAt,
		"completedAt":   task.CompletedAt,
		"result":        task.Result,
		"error":         task.Error,
		"dependencies":  task.Dependencies,
		"dependents":    task.Dependents,
	})
}

// getTaskStatus returns the status of a specific task
func (wi *WebIntegration) getTaskStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskId"]

	task, err := wi.multiAgentSystem.GetTaskStatus(taskID)
	if err != nil {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":     task.ID,
		"status": task.Status,
		"error":  task.Error,
	})
}

// =============================================================================
// REQUEST PROCESSING ENDPOINTS
// =============================================================================

// processRequest processes a natural language infrastructure request
func (wi *WebIntegration) processRequest(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Request string `json:"request"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		wi.writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request format",
		})
		return
	}

	if request.Request == "" {
		wi.writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "request cannot be empty",
		})
		return
	}

	// Process the request
	decision, err := wi.multiAgentSystem.ProcessRequest(r.Context(), request.Request)
	if err != nil {
		wi.writeJSONResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"decision": decision,
		"success":  true,
	})
}

// coordinateExecution coordinates the execution of multiple tasks
func (wi *WebIntegration) coordinateExecution(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Tasks []*Task `json:"tasks"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		wi.writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request format",
		})
		return
	}

	if len(request.Tasks) == 0 {
		wi.writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "tasks cannot be empty",
		})
		return
	}

	// Coordinate execution
	err := wi.multiAgentSystem.coordinator.CoordinateExecution(request.Tasks)
	if err != nil {
		wi.writeJSONResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "execution coordinated successfully",
	})
}

// =============================================================================
// MESSAGE BUS ENDPOINTS
// =============================================================================

// getMessageBusStats returns statistics about the message bus
func (wi *WebIntegration) getMessageBusStats(w http.ResponseWriter, r *http.Request) {
	stats := wi.multiAgentSystem.messageBus.GetStatistics()
	wi.writeJSONResponse(w, http.StatusOK, stats)
}

// getAgentMessageQueue returns the message queue for a specific agent
func (wi *WebIntegration) getAgentMessageQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	queue, err := wi.multiAgentSystem.messageBus.GetMessageQueue(agentID)
	if err != nil {
		wi.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"agent_id": agentID,
		"queue":    queue,
		"count":    len(queue),
	})
}

// =============================================================================
// AGENT REGISTRY ENDPOINTS
// =============================================================================

// getRegistryAgents returns all agents from the registry
func (wi *WebIntegration) getRegistryAgents(w http.ResponseWriter, r *http.Request) {
	agents := wi.multiAgentSystem.agentRegistry.GetAllAgents()

	agentList := make([]map[string]interface{}, 0, len(agents))
	for _, agent := range agents {
		agentInfo := agent.GetInfo()
		agentList = append(agentList, map[string]interface{}{
			"id":           agentInfo.ID,
			"type":         agentInfo.Type,
			"name":         agentInfo.Name,
			"description":  agentInfo.Description,
			"status":       agentInfo.Status,
			"capabilities": agentInfo.Capabilities,
			"tools":        agentInfo.Tools,
			"lastSeen":     agentInfo.LastSeen,
			"metadata":     agentInfo.Metadata,
		})
	}

	wi.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"agents": agentList,
		"count":  len(agentList),
	})
}

// getRegistryHealth returns the health status of all agents in the registry
func (wi *WebIntegration) getRegistryHealth(w http.ResponseWriter, r *http.Request) {
	health := wi.multiAgentSystem.agentRegistry.GetAllAgentHealth()
	wi.writeJSONResponse(w, http.StatusOK, health)
}

// getRegistryStatistics returns statistics about the agent registry
func (wi *WebIntegration) getRegistryStatistics(w http.ResponseWriter, r *http.Request) {
	// This would need to be implemented in the registry
	stats := map[string]interface{}{
		"total_agents": len(wi.multiAgentSystem.agentRegistry.GetAllAgents()),
		"message":      "registry statistics not yet implemented",
	}
	wi.writeJSONResponse(w, http.StatusOK, stats)
}

// =============================================================================
// UTILITY METHODS
// =============================================================================

// writeJSONResponse writes a JSON response to the HTTP response writer
func (wi *WebIntegration) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		wi.logger.WithError(err).Error("Failed to encode JSON response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetRouter returns the HTTP router for the web integration
func (wi *WebIntegration) GetRouter() *mux.Router {
	return wi.router
}

// =============================================================================
// WEBSOCKET INTEGRATION (PLACEHOLDER)
// =============================================================================

// WebSocketIntegration provides WebSocket support for real-time updates
type WebSocketIntegration struct {
	multiAgentSystem *MultiAgentSystem
	logger           *logging.Logger
	clients          map[*websocket.Conn]bool
	broadcast        chan []byte
	register         chan *websocket.Conn
	unregister       chan *websocket.Conn
}

// NewWebSocketIntegration creates a new WebSocket integration
func NewWebSocketIntegration(multiAgentSystem *MultiAgentSystem, logger *logging.Logger) *WebSocketIntegration {
	return &WebSocketIntegration{
		multiAgentSystem: multiAgentSystem,
		logger:           logger,
		clients:          make(map[*websocket.Conn]bool),
		broadcast:        make(chan []byte),
		register:         make(chan *websocket.Conn),
		unregister:       make(chan *websocket.Conn),
	}
}

// Start starts the WebSocket integration
func (wsi *WebSocketIntegration) Start() {
	go wsi.run()
}

// run runs the WebSocket integration
func (wsi *WebSocketIntegration) run() {
	for {
		select {
		case conn := <-wsi.register:
			wsi.clients[conn] = true
			wsi.logger.Info("WebSocket client connected")

		case conn := <-wsi.unregister:
			if _, ok := wsi.clients[conn]; ok {
				delete(wsi.clients, conn)
				conn.Close()
				wsi.logger.Info("WebSocket client disconnected")
			}

		case message := <-wsi.broadcast:
			for conn := range wsi.clients {
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					conn.Close()
					delete(wsi.clients, conn)
				}
			}
		}
	}
}

// BroadcastMessage broadcasts a message to all connected WebSocket clients
func (wsi *WebSocketIntegration) BroadcastMessage(message []byte) {
	wsi.broadcast <- message
}

// HandleWebSocket handles WebSocket connections
func (wsi *WebSocketIntegration) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsi.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}

	wsi.register <- conn

	// Keep connection alive
	go func() {
		defer func() {
			wsi.unregister <- conn
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					wsi.logger.WithError(err).Error("WebSocket error")
				}
				break
			}
		}
	}()
}
