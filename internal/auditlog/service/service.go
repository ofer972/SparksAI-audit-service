package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/motiso/sparksai-audit-service/internal/auditlog"
	"github.com/motiso/sparksai-audit-service/internal/buffer"
)

var numericEndingRegex = regexp.MustCompile(`/\d+$`)
var purelyNumericRegex = regexp.MustCompile(`^\d+$`)

// normalizeEndpoint replaces trailing numeric IDs with /*
// Example: /api/v1/goals/135 -> /api/v1/goals/*
func normalizeEndpoint(path string) string {
	return numericEndingRegex.ReplaceAllString(path, "/*")
}

// normalizeAction returns a short action name if the action is purely numeric
// Example: action="135", endpoint="/api/v1/goals/135" -> "goal-by-id"
func normalizeAction(action string, endpointPath string) string {
	if action == "" || !purelyNumericRegex.MatchString(action) {
		return action
	}
	// Extract resource from endpoint: /api/v1/goals/135 -> "goals"
	parts := strings.Split(strings.Trim(endpointPath, "/"), "/")
	if len(parts) >= 3 {
		resource := parts[2] // "goals", "issues", "sprints", etc.
		// Singularize by removing trailing 's'
		singular := strings.TrimSuffix(resource, "s")
		return singular + "-by-id"
	}
	return action
}

var auditService *AuditService

func Get() *AuditService {
	if auditService == nil {
		datastore := GetAuditLogDataStore()
		auditService = &AuditService{
			DB:     datastore,
			Buffer: buffer.Get(datastore),
		}
		return auditService
	}
	return auditService
}

type AuditService struct {
	DB     auditlog.AuditLogDatastore
	Buffer *buffer.Buffer
}

// CreateAuditLogsHandler handles POST /api/audit-logs
// Accepts array of audit logs, adds them to buffer, returns immediately (non-blocking)
func (as *AuditService) CreateAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	var req auditlog.CreateAuditLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Logs) == 0 {
		http.Error(w, "Logs array cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate required fields and normalize
	for i := range req.Logs {
		if req.Logs[i].EndpointPath == "" {
			http.Error(w, fmt.Sprintf("endpoint_path is required for log entry %d", i), http.StatusBadRequest)
			return
		}
		if req.Logs[i].HTTPMethod == "" {
			http.Error(w, fmt.Sprintf("http_method is required for log entry %d", i), http.StatusBadRequest)
			return
		}
		if req.Logs[i].StatusCode == 0 {
			http.Error(w, fmt.Sprintf("status_code is required for log entry %d", i), http.StatusBadRequest)
			return
		}

		// Set default severity if empty (database will also default to "NONE", but set it here for consistency)
		if req.Logs[i].Severity == "" {
			req.Logs[i].Severity = "NONE"
		}

		// Normalize action and endpoint
		if req.Logs[i].Action != nil {
			normalized := normalizeAction(*req.Logs[i].Action, req.Logs[i].EndpointPath)
			req.Logs[i].Action = &normalized
		}
		req.Logs[i].EndpointPath = normalizeEndpoint(req.Logs[i].EndpointPath)
	}

	// Add logs to buffer (non-blocking)
	queuedCount := as.Buffer.AddLogs(req.Logs)

	// Return immediately
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "accepted",
		"message":      "Audit logs queued for processing",
		"queued_count": queuedCount,
	})
}

// GetAuditLogsHandler handles GET /api/audit-logs
// Query parameters: user_id (optional), action (optional), limit (optional, default: 500, max: 500)
// Results are ordered by created_at DESC (latest first)
func (as *AuditService) GetAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	userIDParam := r.URL.Query().Get("user_id")
	actionParam := r.URL.Query().Get("action")
	limitParam := r.URL.Query().Get("limit")

	// Parse user_id (optional)
	var userID *string
	if userIDParam != "" {
		userID = &userIDParam
	}

	// Parse action (optional)
	var action *string
	if actionParam != "" {
		action = &actionParam
	}

	// Parse limit (optional, default 500, max 500)
	limit := 500
	if limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		if parsedLimit > 0 && parsedLimit <= 500 {
			limit = parsedLimit
		} else if parsedLimit > 500 {
			limit = 500
		}
	}

	// Get audit logs from database
	logs, err := as.DB.GetAuditLogs(userID, action, limit)
	if err != nil {
		log.Printf("error occurred during GetAuditLogs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// GetActionsHandler handles GET /api/audit-logs/actions
// Returns a list of all distinct action values from audit logs
func (as *AuditService) GetActionsHandler(w http.ResponseWriter, r *http.Request) {
	// Get distinct actions from database
	actions, err := as.DB.GetDistinctActions()
	if err != nil {
		log.Printf("error occurred during GetDistinctActions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(actions)
}

// GetAuditLogsFilterValuesHandler handles GET /api/audit-logs/filter-values
// Returns all distinct values for filter dropdowns in a single response
func (as *AuditService) GetAuditLogsFilterValuesHandler(w http.ResponseWriter, r *http.Request) {
	reportSvc := NewReportService()
	values, err := reportSvc.GetAuditLogsFilterValues()
	if err != nil {
		log.Printf("error occurred during GetAuditLogsFilterValues: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}

