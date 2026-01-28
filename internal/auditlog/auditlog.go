package auditlog

// AuditLog represents an audit log entry
type AuditLog struct {
	ID                  int     `json:"id"`
	UserID              *string `json:"user_id,omitempty"`
	EndpointPath        string  `json:"endpoint_path"`
	SessionID           *string `json:"session_id,omitempty"` // String
	Action              *string `json:"action,omitempty"`
	ActionDate          *string `json:"action_date,omitempty"`
	Count               *int    `json:"count,omitempty"`
	HTTPMethod          string  `json:"http_method"`
	StatusCode          int     `json:"status_code"`
	ResponseTimeSeconds float64 `json:"response_time_seconds"`
	CreatedAt           string  `json:"created_at"`
	IPAddress           *string `json:"ip_address,omitempty"`
	UserAgent           *string `json:"user_agent,omitempty"`
	ChatHistoryID       *int    `json:"chat_history_id,omitempty"`
	InsightsID          *int    `json:"insights_id,omitempty"`
	TokensUsed          *int    `json:"tokens_used,omitempty"`
	QueryRaw            *string `json:"query_raw,omitempty"`           // JSONB as string (raw query string)
	BodyRaw             *string `json:"body_raw,omitempty"`            // JSONB as string (raw body)
	ResponseBody        *string `json:"response_body,omitempty"`       // JSONB as string (response body for LLM)
}

// CreateAuditLogsRequest represents the request body for creating audit logs
type CreateAuditLogsRequest struct {
	Logs []AuditLog `json:"logs"`
}

// AuditLogDatastore interface for database operations
type AuditLogDatastore interface {
	// Write operations
	BatchInsertAuditLogs(logs []AuditLog) error
	
	// Read operations
	GetAuditLogs(userID *string, action *string, limit int) ([]AuditLog, error)
	GetDistinctActions() ([]string, error)
}

