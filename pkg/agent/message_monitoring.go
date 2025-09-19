package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// MESSAGE MONITORING AND ANALYTICS
// =============================================================================

// MessageMonitoringSystem monitors and analyzes message flow
type MessageMonitoringSystem struct {
	messageLogs        *MessageLog
	analyticsEngine    *AnalyticsEngine
	alertManager       *AlertManager
	performanceTracker *PerformanceTracker
	logger             *logging.Logger
	mu                 sync.RWMutex
	started            bool
	ctx                context.Context
	cancel             context.CancelFunc
}

// MessageLog stores message logs for analysis
type MessageLog struct {
	entries []*MessageLogEntry
	mu      sync.RWMutex
}

// MessageLogEntry represents a message log entry
type MessageLogEntry struct {
	ID             string                 `json:"id"`
	MessageID      string                 `json:"message_id"`
	MessageType    string                 `json:"message_type"`
	Source         string                 `json:"source"`
	Destination    string                 `json:"destination"`
	Topic          string                 `json:"topic"`
	Status         MessageStatus          `json:"status"`
	Timestamp      time.Time              `json:"timestamp"`
	ProcessingTime time.Duration          `json:"processing_time"`
	Size           int                    `json:"size"`
	Priority       MessagePriority        `json:"priority"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// AnalyticsEngine analyzes message patterns and trends
type AnalyticsEngine struct {
	patterns  map[string]*MessagePattern
	trends    map[string]*MessageTrend
	anomalies []*MessageAnomaly
	logger    *logging.Logger
	mu        sync.RWMutex
}

// MessagePattern represents a message pattern
type MessagePattern struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Pattern   string                 `json:"pattern"`
	Frequency int                    `json:"frequency"`
	LastSeen  time.Time              `json:"last_seen"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// MessageTrend represents a message trend
type MessageTrend struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Metric      string                 `json:"metric"`
	Value       float64                `json:"value"`
	Change      float64                `json:"change"`
	Period      time.Duration          `json:"period"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// MessageAnomaly represents a message anomaly
type MessageAnomaly struct {
	ID          string                 `json:"id"`
	Type        AnomalyType            `json:"type"`
	Severity    AnomalySeverity        `json:"severity"`
	Description string                 `json:"description"`
	DetectedAt  time.Time              `json:"detected_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AlertManager manages alerts and notifications
type AlertManager struct {
	alerts        map[string]*Alert
	alertRules    map[string]*AlertRule
	notifications []*Notification
	logger        *logging.Logger
	mu            sync.RWMutex
}

// Alert represents an alert
type Alert struct {
	ID             string                 `json:"id"`
	RuleID         string                 `json:"rule_id"`
	Type           AlertType              `json:"type"`
	Severity       AlertSeverity          `json:"severity"`
	Message        string                 `json:"message"`
	Status         AlertStatus            `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time             `json:"resolved_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// AlertRule represents an alert rule
type AlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Condition   string                 `json:"condition"`
	Threshold   float64                `json:"threshold"`
	Severity    AlertSeverity          `json:"severity"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Notification represents a notification
type Notification struct {
	ID          string                 `json:"id"`
	AlertID     string                 `json:"alert_id"`
	Type        NotificationType       `json:"type"`
	Recipient   string                 `json:"recipient"`
	Message     string                 `json:"message"`
	Status      NotificationStatus     `json:"status"`
	SentAt      time.Time              `json:"sent_at"`
	DeliveredAt *time.Time             `json:"delivered_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PerformanceTracker tracks performance metrics
type PerformanceTracker struct {
	metrics      map[string]*PerformanceMetric
	aggregations map[string]*MetricAggregation
	logger       *logging.Logger
	mu           sync.RWMutex
}

// PerformanceMetric represents a performance metric
type PerformanceMetric struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Timestamp time.Time              `json:"timestamp"`
	Tags      map[string]string      `json:"tags"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// MetricAggregation represents a metric aggregation
type MetricAggregation struct {
	ID          string                 `json:"id"`
	MetricName  string                 `json:"metric_name"`
	Aggregation AggregationType        `json:"aggregation"`
	Value       float64                `json:"value"`
	Period      time.Duration          `json:"period"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AnomalyType represents the type of anomaly
type AnomalyType string

const (
	AnomalyTypeVolume   AnomalyType = "volume"
	AnomalyTypeLatency  AnomalyType = "latency"
	AnomalyTypeError    AnomalyType = "error"
	AnomalyTypePattern  AnomalyType = "pattern"
	AnomalyTypeSecurity AnomalyType = "security"
)

// AnomalySeverity represents the severity of an anomaly
type AnomalySeverity string

const (
	AnomalySeverityLow      AnomalySeverity = "low"
	AnomalySeverityMedium   AnomalySeverity = "medium"
	AnomalySeverityHigh     AnomalySeverity = "high"
	AnomalySeverityCritical AnomalySeverity = "critical"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeThreshold   AlertType = "threshold"
	AlertTypeAnomaly     AlertType = "anomaly"
	AlertTypeError       AlertType = "error"
	AlertTypePerformance AlertType = "performance"
	AlertTypeSecurity    AlertType = "security"
)

// AlertSeverity represents the severity of an alert
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusSuppressed   AlertStatus = "suppressed"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeEmail   NotificationType = "email"
	NotificationTypeSMS     NotificationType = "sms"
	NotificationTypeWebhook NotificationType = "webhook"
	NotificationTypeSlack   NotificationType = "slack"
	NotificationTypeTeams   NotificationType = "teams"
)

// NotificationStatus represents the status of a notification
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed    NotificationStatus = "failed"
)

// AggregationType represents the type of aggregation
type AggregationType string

const (
	AggregationTypeSum     AggregationType = "sum"
	AggregationTypeAverage AggregationType = "average"
	AggregationTypeMin     AggregationType = "min"
	AggregationTypeMax     AggregationType = "max"
	AggregationTypeCount   AggregationType = "count"
)

// NewMessageMonitoringSystem creates a new message monitoring system
func NewMessageMonitoringSystem(logger *logging.Logger) *MessageMonitoringSystem {
	ctx, cancel := context.WithCancel(context.Background())

	return &MessageMonitoringSystem{
		messageLogs: &MessageLog{
			entries: make([]*MessageLogEntry, 0),
		},
		analyticsEngine: &AnalyticsEngine{
			patterns:  make(map[string]*MessagePattern),
			trends:    make(map[string]*MessageTrend),
			anomalies: make([]*MessageAnomaly, 0),
			logger:    logger,
		},
		alertManager: &AlertManager{
			alerts:        make(map[string]*Alert),
			alertRules:    make(map[string]*AlertRule),
			notifications: make([]*Notification, 0),
			logger:        logger,
		},
		performanceTracker: &PerformanceTracker{
			metrics:      make(map[string]*PerformanceMetric),
			aggregations: make(map[string]*MetricAggregation),
			logger:       logger,
		},
		logger:  logger,
		started: false,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// =============================================================================
// MESSAGE MONITORING SYSTEM MANAGEMENT
// =============================================================================

// Start starts the message monitoring system
func (mms *MessageMonitoringSystem) Start() error {
	mms.mu.Lock()
	defer mms.mu.Unlock()

	if mms.started {
		return fmt.Errorf("message monitoring system is already started")
	}

	mms.started = true
	mms.logger.Info("Starting message monitoring system")

	// Start analytics processing goroutine
	go mms.processAnalytics()

	// Start alert processing goroutine
	go mms.processAlerts()

	// Start performance tracking goroutine
	go mms.trackPerformance()

	mms.logger.Info("Message monitoring system started successfully")
	return nil
}

// Stop stops the message monitoring system
func (mms *MessageMonitoringSystem) Stop() error {
	mms.mu.Lock()
	defer mms.mu.Unlock()

	if !mms.started {
		return fmt.Errorf("message monitoring system is not started")
	}

	mms.started = false
	mms.cancel()

	mms.logger.Info("Message monitoring system stopped")
	return nil
}

// =============================================================================
// MESSAGE LOGGING
// =============================================================================

// LogMessage logs a message for monitoring
func (mms *MessageMonitoringSystem) LogMessage(message *Message, processingTime time.Duration) error {
	mms.mu.RLock()
	defer mms.mu.RUnlock()

	if !mms.started {
		return fmt.Errorf("message monitoring system is not started")
	}

	// Create log entry
	entry := &MessageLogEntry{
		ID:             uuid.New().String(),
		MessageID:      message.ID,
		MessageType:    message.Type,
		Source:         message.From,
		Destination:    message.To,
		Topic:          message.Topic,
		Status:         message.Status,
		Timestamp:      time.Now(),
		ProcessingTime: processingTime,
		Size:           len(fmt.Sprintf("%v", message.Content)),
		Priority:       message.Priority,
		Metadata:       make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range message.Metadata {
		entry.Metadata[k] = v
	}

	// Add to log
	mms.messageLogs.mu.Lock()
	mms.messageLogs.entries = append(mms.messageLogs.entries, entry)
	mms.messageLogs.mu.Unlock()

	// Update performance metrics
	mms.updatePerformanceMetrics(entry)

	// Check for anomalies
	mms.checkForAnomalies(entry)

	// Check alert rules
	mms.checkAlertRules(entry)

	mms.logger.WithFields(map[string]interface{}{
		"message_id":      message.ID,
		"processing_time": processingTime,
		"status":          message.Status,
	}).Debug("Message logged for monitoring")

	return nil
}

// =============================================================================
// ANALYTICS PROCESSING
// =============================================================================

// processAnalytics processes message analytics
func (mms *MessageMonitoringSystem) processAnalytics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mms.analyzeMessagePatterns()
			mms.updateMessageTrends()
			mms.detectAnomalies()
		case <-mms.ctx.Done():
			return
		}
	}
}

// analyzeMessagePatterns analyzes message patterns
func (mms *MessageMonitoringSystem) analyzeMessagePatterns() {
	mms.analyticsEngine.mu.Lock()
	defer mms.analyticsEngine.mu.Unlock()

	// Get recent messages
	recentMessages := mms.getRecentMessages(5 * time.Minute)

	// Analyze patterns
	patternCounts := make(map[string]int)
	for _, entry := range recentMessages {
		pattern := fmt.Sprintf("%s->%s:%s", entry.Source, entry.Destination, entry.MessageType)
		patternCounts[pattern]++
	}

	// Update or create patterns
	for pattern, count := range patternCounts {
		if existingPattern, exists := mms.analyticsEngine.patterns[pattern]; exists {
			existingPattern.Frequency = count
			existingPattern.LastSeen = time.Now()
		} else {
			mms.analyticsEngine.patterns[pattern] = &MessagePattern{
				ID:        uuid.New().String(),
				Name:      pattern,
				Pattern:   pattern,
				Frequency: count,
				LastSeen:  time.Now(),
				CreatedAt: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
		}
	}

	mms.logger.WithField("pattern_count", len(patternCounts)).Debug("Message patterns analyzed")
}

// updateMessageTrends updates message trends
func (mms *MessageMonitoringSystem) updateMessageTrends() {
	mms.analyticsEngine.mu.Lock()
	defer mms.analyticsEngine.mu.Unlock()

	// Calculate trends for different metrics
	trends := map[string]float64{
		"message_volume":  float64(len(mms.getRecentMessages(1 * time.Minute))),
		"average_latency": mms.calculateAverageLatency(1 * time.Minute).Seconds(),
		"error_rate":      mms.calculateErrorRate(1 * time.Minute),
		"throughput":      mms.calculateThroughput(1 * time.Minute),
	}

	// Update trends
	for metric, value := range trends {
		if existingTrend, exists := mms.analyticsEngine.trends[metric]; exists {
			change := value - existingTrend.Value
			existingTrend.Value = value
			existingTrend.Change = change
			existingTrend.LastUpdated = time.Now()
		} else {
			mms.analyticsEngine.trends[metric] = &MessageTrend{
				ID:          uuid.New().String(),
				Name:        metric,
				Metric:      metric,
				Value:       value,
				Change:      0,
				Period:      1 * time.Minute,
				LastUpdated: time.Now(),
				Metadata:    make(map[string]interface{}),
			}
		}
	}

	mms.logger.WithField("trend_count", len(trends)).Debug("Message trends updated")
}

// detectAnomalies detects message anomalies
func (mms *MessageMonitoringSystem) detectAnomalies() {
	mms.analyticsEngine.mu.Lock()
	defer mms.analyticsEngine.mu.Unlock()

	// Get recent messages
	recentMessages := mms.getRecentMessages(5 * time.Minute)

	// Check for volume anomalies
	if len(recentMessages) > 1000 { // High volume threshold
		anomaly := &MessageAnomaly{
			ID:          uuid.New().String(),
			Type:        AnomalyTypeVolume,
			Severity:    AnomalySeverityHigh,
			Description: fmt.Sprintf("High message volume detected: %d messages in 5 minutes", len(recentMessages)),
			DetectedAt:  time.Now(),
			Metadata:    make(map[string]interface{}),
		}
		mms.analyticsEngine.anomalies = append(mms.analyticsEngine.anomalies, anomaly)
	}

	// Check for latency anomalies
	avgLatency := mms.calculateAverageLatency(5 * time.Minute)
	if avgLatency > 5*time.Second { // High latency threshold
		anomaly := &MessageAnomaly{
			ID:          uuid.New().String(),
			Type:        AnomalyTypeLatency,
			Severity:    AnomalySeverityMedium,
			Description: fmt.Sprintf("High latency detected: %v average", avgLatency),
			DetectedAt:  time.Now(),
			Metadata:    make(map[string]interface{}),
		}
		mms.analyticsEngine.anomalies = append(mms.analyticsEngine.anomalies, anomaly)
	}

	// Check for error anomalies
	errorRate := mms.calculateErrorRate(5 * time.Minute)
	if errorRate > 0.1 { // 10% error rate threshold
		anomaly := &MessageAnomaly{
			ID:          uuid.New().String(),
			Type:        AnomalyTypeError,
			Severity:    AnomalySeverityHigh,
			Description: fmt.Sprintf("High error rate detected: %.2f%%", errorRate*100),
			DetectedAt:  time.Now(),
			Metadata:    make(map[string]interface{}),
		}
		mms.analyticsEngine.anomalies = append(mms.analyticsEngine.anomalies, anomaly)
	}

	mms.logger.Debug("Anomaly detection completed")
}

// =============================================================================
// ALERT PROCESSING
// =============================================================================

// processAlerts processes alerts
func (mms *MessageMonitoringSystem) processAlerts() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mms.processActiveAlerts()
		case <-mms.ctx.Done():
			return
		}
	}
}

// processActiveAlerts processes active alerts
func (mms *MessageMonitoringSystem) processActiveAlerts() {
	mms.alertManager.mu.Lock()
	defer mms.alertManager.mu.Unlock()

	// Process active alerts
	for _, alert := range mms.alertManager.alerts {
		if alert.Status == AlertStatusActive {
			// Send notifications
			mms.sendAlertNotifications(alert)
		}
	}
}

// checkAlertRules checks alert rules
func (mms *MessageMonitoringSystem) checkAlertRules(entry *MessageLogEntry) {
	mms.alertManager.mu.Lock()
	defer mms.alertManager.mu.Unlock()

	// Check each alert rule
	for _, rule := range mms.alertManager.alertRules {
		if !rule.Active {
			continue
		}

		// Evaluate rule condition
		if mms.evaluateAlertRule(rule, entry) {
			// Create alert
			alert := &Alert{
				ID:        uuid.New().String(),
				RuleID:    rule.ID,
				Type:      AlertTypeThreshold,
				Severity:  rule.Severity,
				Message:   fmt.Sprintf("Alert rule '%s' triggered", rule.Name),
				Status:    AlertStatusActive,
				CreatedAt: time.Now(),
				Metadata:  make(map[string]interface{}),
			}

			mms.alertManager.alerts[alert.ID] = alert

			mms.logger.WithFields(map[string]interface{}{
				"alert_id":  alert.ID,
				"rule_id":   rule.ID,
				"rule_name": rule.Name,
				"severity":  rule.Severity,
			}).Warn("Alert created")
		}
	}
}

// evaluateAlertRule evaluates an alert rule
func (mms *MessageMonitoringSystem) evaluateAlertRule(rule *AlertRule, entry *MessageLogEntry) bool {
	// This is a simplified implementation
	// In a real system, this would evaluate the rule condition against the entry

	switch rule.Condition {
	case "high_latency":
		return entry.ProcessingTime > time.Duration(rule.Threshold)*time.Second
	case "high_error_rate":
		return entry.Status == MessageStatusFailed
	case "high_volume":
		return len(mms.getRecentMessages(1*time.Minute)) > int(rule.Threshold)
	default:
		return false
	}
}

// sendAlertNotifications sends alert notifications
func (mms *MessageMonitoringSystem) sendAlertNotifications(alert *Alert) {
	// Create notification
	notification := &Notification{
		ID:        uuid.New().String(),
		AlertID:   alert.ID,
		Type:      NotificationTypeEmail,
		Recipient: "admin@example.com",
		Message:   alert.Message,
		Status:    NotificationStatusPending,
		SentAt:    time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Add to notifications
	mms.alertManager.notifications = append(mms.alertManager.notifications, notification)

	// Update notification status
	notification.Status = NotificationStatusSent
	now := time.Now()
	notification.DeliveredAt = &now

	mms.logger.WithFields(map[string]interface{}{
		"notification_id": notification.ID,
		"alert_id":        alert.ID,
		"recipient":       notification.Recipient,
	}).Info("Alert notification sent")
}

// =============================================================================
// PERFORMANCE TRACKING
// =============================================================================

// trackPerformance tracks performance metrics
func (mms *MessageMonitoringSystem) trackPerformance() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mms.aggregatePerformanceMetrics()
		case <-mms.ctx.Done():
			return
		}
	}
}

// updatePerformanceMetrics updates performance metrics
func (mms *MessageMonitoringSystem) updatePerformanceMetrics(entry *MessageLogEntry) {
	mms.performanceTracker.mu.Lock()
	defer mms.performanceTracker.mu.Unlock()

	// Update latency metric
	latencyMetric := &PerformanceMetric{
		ID:        uuid.New().String(),
		Name:      "message_latency",
		Value:     float64(entry.ProcessingTime.Nanoseconds()) / 1e6, // Convert to milliseconds
		Unit:      "ms",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"source":      entry.Source,
			"destination": entry.Destination,
			"type":        entry.MessageType,
		},
		Metadata: make(map[string]interface{}),
	}

	mms.performanceTracker.metrics[latencyMetric.ID] = latencyMetric

	// Update throughput metric
	throughputMetric := &PerformanceMetric{
		ID:        uuid.New().String(),
		Name:      "message_throughput",
		Value:     1, // One message
		Unit:      "messages/sec",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"source":      entry.Source,
			"destination": entry.Destination,
		},
		Metadata: make(map[string]interface{}),
	}

	mms.performanceTracker.metrics[throughputMetric.ID] = throughputMetric
}

// aggregatePerformanceMetrics aggregates performance metrics
func (mms *MessageMonitoringSystem) aggregatePerformanceMetrics() {
	mms.performanceTracker.mu.Lock()
	defer mms.performanceTracker.mu.Unlock()

	// Aggregate latency metrics
	latencyMetrics := make([]*PerformanceMetric, 0)
	for _, metric := range mms.performanceTracker.metrics {
		if metric.Name == "message_latency" {
			latencyMetrics = append(latencyMetrics, metric)
		}
	}

	if len(latencyMetrics) > 0 {
		// Calculate average latency
		var totalLatency float64
		for _, metric := range latencyMetrics {
			totalLatency += metric.Value
		}
		avgLatency := totalLatency / float64(len(latencyMetrics))

		// Update aggregation
		mms.performanceTracker.aggregations["avg_latency"] = &MetricAggregation{
			ID:          uuid.New().String(),
			MetricName:  "message_latency",
			Aggregation: AggregationTypeAverage,
			Value:       avgLatency,
			Period:      1 * time.Minute,
			LastUpdated: time.Now(),
			Metadata:    make(map[string]interface{}),
		}
	}

	mms.logger.Debug("Performance metrics aggregated")
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// getRecentMessages gets recent messages
func (mms *MessageMonitoringSystem) getRecentMessages(duration time.Duration) []*MessageLogEntry {
	mms.messageLogs.mu.RLock()
	defer mms.messageLogs.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recentMessages []*MessageLogEntry

	for _, entry := range mms.messageLogs.entries {
		if entry.Timestamp.After(cutoff) {
			recentMessages = append(recentMessages, entry)
		}
	}

	return recentMessages
}

// calculateAverageLatency calculates average latency
func (mms *MessageMonitoringSystem) calculateAverageLatency(duration time.Duration) time.Duration {
	recentMessages := mms.getRecentMessages(duration)
	if len(recentMessages) == 0 {
		return 0
	}

	var totalLatency time.Duration
	for _, entry := range recentMessages {
		totalLatency += entry.ProcessingTime
	}

	return totalLatency / time.Duration(len(recentMessages))
}

// calculateErrorRate calculates error rate
func (mms *MessageMonitoringSystem) calculateErrorRate(duration time.Duration) float64 {
	recentMessages := mms.getRecentMessages(duration)
	if len(recentMessages) == 0 {
		return 0
	}

	errorCount := 0
	for _, entry := range recentMessages {
		if entry.Status == MessageStatusFailed {
			errorCount++
		}
	}

	return float64(errorCount) / float64(len(recentMessages))
}

// calculateThroughput calculates throughput
func (mms *MessageMonitoringSystem) calculateThroughput(duration time.Duration) float64 {
	recentMessages := mms.getRecentMessages(duration)
	if len(recentMessages) == 0 {
		return 0
	}

	return float64(len(recentMessages)) / duration.Seconds()
}

// checkForAnomalies checks for anomalies in a message entry
func (mms *MessageMonitoringSystem) checkForAnomalies(entry *MessageLogEntry) {
	// Check for high latency
	if entry.ProcessingTime > 10*time.Second {
		anomaly := &MessageAnomaly{
			ID:          uuid.New().String(),
			Type:        AnomalyTypeLatency,
			Severity:    AnomalySeverityMedium,
			Description: fmt.Sprintf("High latency detected: %v", entry.ProcessingTime),
			DetectedAt:  time.Now(),
			Metadata:    make(map[string]interface{}),
		}
		mms.analyticsEngine.anomalies = append(mms.analyticsEngine.anomalies, anomaly)
	}

	// Check for failed messages
	if entry.Status == MessageStatusFailed {
		anomaly := &MessageAnomaly{
			ID:          uuid.New().String(),
			Type:        AnomalyTypeError,
			Severity:    AnomalySeverityHigh,
			Description: "Message processing failed",
			DetectedAt:  time.Now(),
			Metadata:    make(map[string]interface{}),
		}
		mms.analyticsEngine.anomalies = append(mms.analyticsEngine.anomalies, anomaly)
	}
}

// =============================================================================
// STATUS AND MONITORING
// =============================================================================

// GetStatus returns the current status of the monitoring system
func (mms *MessageMonitoringSystem) GetStatus() map[string]interface{} {
	mms.mu.RLock()
	defer mms.mu.RUnlock()

	return map[string]interface{}{
		"started":            mms.started,
		"message_count":      len(mms.messageLogs.entries),
		"pattern_count":      len(mms.analyticsEngine.patterns),
		"trend_count":        len(mms.analyticsEngine.trends),
		"anomaly_count":      len(mms.analyticsEngine.anomalies),
		"alert_count":        len(mms.alertManager.alerts),
		"notification_count": len(mms.alertManager.notifications),
		"metric_count":       len(mms.performanceTracker.metrics),
		"aggregation_count":  len(mms.performanceTracker.aggregations),
		"timestamp":          time.Now().Format(time.RFC3339),
	}
}

// GetAnalytics returns analytics data
func (mms *MessageMonitoringSystem) GetAnalytics() map[string]interface{} {
	mms.analyticsEngine.mu.RLock()
	defer mms.analyticsEngine.mu.RUnlock()

	return map[string]interface{}{
		"patterns":  mms.analyticsEngine.patterns,
		"trends":    mms.analyticsEngine.trends,
		"anomalies": mms.analyticsEngine.anomalies,
	}
}

// GetAlerts returns alerts data
func (mms *MessageMonitoringSystem) GetAlerts() map[string]interface{} {
	mms.alertManager.mu.RLock()
	defer mms.alertManager.mu.RUnlock()

	return map[string]interface{}{
		"alerts":        mms.alertManager.alerts,
		"alert_rules":   mms.alertManager.alertRules,
		"notifications": mms.alertManager.notifications,
	}
}

// GetPerformanceMetrics returns performance metrics
func (mms *MessageMonitoringSystem) GetPerformanceMetrics() map[string]interface{} {
	mms.performanceTracker.mu.RLock()
	defer mms.performanceTracker.mu.RUnlock()

	return map[string]interface{}{
		"metrics":      mms.performanceTracker.metrics,
		"aggregations": mms.performanceTracker.aggregations,
	}
}
