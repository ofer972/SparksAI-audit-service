package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	database "github.com/motiso/sparksai-audit-service/internal/db"
	"github.com/motiso/sparksai-audit-service/internal/auditlog"
)

type AuditLogDB struct {
	*sql.DB
}

func GetAuditLogDataStore() auditlog.AuditLogDatastore {
	return &AuditLogDB{database.Get()}
}

// parseQueryRaw parses raw query string into JSONB object
func parseQueryRaw(queryRaw string) interface{} {
	if queryRaw == "" {
		return nil
	}
	
	// Parse query string
	values, err := url.ParseQuery(queryRaw)
	if err != nil {
		// If parsing fails, return as string
		return queryRaw
	}
	
	// Convert to map (take first value for each key)
	queryMap := make(map[string]interface{})
	for k, v := range values {
		if len(v) > 0 {
			if len(v) == 1 {
				queryMap[k] = v[0]
			} else {
				queryMap[k] = v // Multiple values as array
			}
		}
	}
	
	if len(queryMap) == 0 {
		return nil
	}
	
	return queryMap
}

// parseBodyRaw parses raw body string into JSONB object (if JSON) or keeps as string
func parseBodyRaw(bodyRaw string) interface{} {
	if bodyRaw == "" {
		return nil
	}
	
	// Try to parse as JSON
	var bodyJSON interface{}
	if err := json.Unmarshal([]byte(bodyRaw), &bodyJSON); err == nil {
		// Valid JSON - return as object
		return bodyJSON
	}
	
	// Not JSON - return as string
	return bodyRaw
}

// marshalToJSONBString marshals a parsed object to JSON string for JSONB storage
// If marshaling fails, returns the original raw string as fallback
func marshalToJSONBString(parsed interface{}, rawString string) interface{} {
	if parsed == nil {
		return nil
	}
	
	// Marshal to JSON string for JSONB column
	if jsonBytes, err := json.Marshal(parsed); err == nil {
		return string(jsonBytes)
	}
	
	// If marshaling fails, store as original string
	return rawString
}

// BatchInsertAuditLogs inserts multiple audit logs in a single transaction
func (db *AuditLogDB) BatchInsertAuditLogs(logs []auditlog.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement
	stmt, err := tx.Prepare(`
		INSERT INTO audit_logs (
			user_id, endpoint_path, session_id, action, action_date, count, http_method, status_code,
			response_time_seconds, ip_address, user_agent,
			chat_history_id, insights_id, tokens_used,
			query_raw, body_raw, response_body
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert each log
	for _, logEntry := range logs {
		var chatHistoryIDVal, insightsIDVal, tokensUsedVal interface{}
		if logEntry.ChatHistoryID != nil {
			chatHistoryIDVal = *logEntry.ChatHistoryID
		}
		if logEntry.InsightsID != nil {
			insightsIDVal = *logEntry.InsightsID
		}
		if logEntry.TokensUsed != nil {
			tokensUsedVal = *logEntry.TokensUsed
		}

		var sessionIDVal interface{}
		if logEntry.SessionID != nil {
			sessionIDVal = *logEntry.SessionID
		}

		var actionDateVal interface{}
		if logEntry.ActionDate != nil {
			actionDateVal = *logEntry.ActionDate
		}

		var countVal interface{}
		if logEntry.Count != nil {
			countVal = *logEntry.Count
		}

		// Parse query_raw and body_raw before storing, then marshal to JSON string for JSONB
		var queryRawVal interface{}
		if logEntry.QueryRaw != nil {
			parsed := parseQueryRaw(*logEntry.QueryRaw)
			queryRawVal = marshalToJSONBString(parsed, *logEntry.QueryRaw)
		}

		var bodyRawVal interface{}
		if logEntry.BodyRaw != nil {
			parsed := parseBodyRaw(*logEntry.BodyRaw)
			bodyRawVal = marshalToJSONBString(parsed, *logEntry.BodyRaw)
		}

		var responseBodyVal interface{}
		if logEntry.ResponseBody != nil {
			parsed := parseBodyRaw(*logEntry.ResponseBody)
			responseBodyVal = marshalToJSONBString(parsed, *logEntry.ResponseBody)
		}

		_, err := stmt.Exec(
			logEntry.UserID,
			logEntry.EndpointPath,
			sessionIDVal,
			logEntry.Action,
			actionDateVal,
			countVal,
			logEntry.HTTPMethod,
			logEntry.StatusCode,
			logEntry.ResponseTimeSeconds,
			logEntry.IPAddress,
			logEntry.UserAgent,
			chatHistoryIDVal,
			insightsIDVal,
			tokensUsedVal,
			queryRawVal,
			bodyRawVal,
			responseBodyVal,
		)
		if err != nil {
			return fmt.Errorf("failed to insert audit log: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully inserted %d audit log entries", len(logs))
	return nil
}

// GetAuditLogs retrieves audit logs with optional filters
// userID and action are optional filters, limit defaults to 500 (max 500)
func (db *AuditLogDB) GetAuditLogs(userID *string, action *string, limit int) ([]auditlog.AuditLog, error) {
	// Validate and set limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 500 {
		limit = 500
	}

	// Build query with optional filters
	query := `
		SELECT 
			id, user_id, endpoint_path, session_id, action, action_date, count, http_method, status_code,
			response_time_seconds, created_at, ip_address, user_agent,
			chat_history_id, insights_id, tokens_used,
			query_raw, body_raw
		FROM audit_logs
		WHERE 1=1
	`
	
	args := []interface{}{}
	argPos := 1

	if userID != nil && *userID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, *userID)
		argPos++
	}

	if action != nil && *action != "" {
		query += fmt.Sprintf(" AND action = $%d", argPos)
		args = append(args, *action)
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

		var logs []auditlog.AuditLog
		for rows.Next() {
			var logEntry auditlog.AuditLog
			var userIDVal, sessionIDVal, actionVal, ipAddressVal, userAgentVal sql.NullString
			var chatHistoryIDVal, insightsIDVal, tokensUsedVal, countVal sql.NullInt64
			var queryRawVal, bodyRawVal sql.NullString
			var createdAt, actionDateVal sql.NullTime

			err := rows.Scan(
				&logEntry.ID,
				&userIDVal,
				&logEntry.EndpointPath,
				&sessionIDVal,
				&actionVal,
				&actionDateVal,
				&countVal,
				&logEntry.HTTPMethod,
				&logEntry.StatusCode,
				&logEntry.ResponseTimeSeconds,
				&createdAt,
				&ipAddressVal,
				&userAgentVal,
				&chatHistoryIDVal,
				&insightsIDVal,
				&tokensUsedVal,
				&queryRawVal,
				&bodyRawVal,
			)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		// Convert nullable fields using helper functions
		logEntry.UserID = nullStringToPtr(userIDVal)
		logEntry.SessionID = nullStringToPtr(sessionIDVal)
		logEntry.Action = nullStringToPtr(actionVal)
		logEntry.IPAddress = nullStringToPtr(ipAddressVal)
		logEntry.UserAgent = nullStringToPtr(userAgentVal)
		logEntry.QueryRaw = nullStringToPtr(queryRawVal)
		logEntry.BodyRaw = nullStringToPtr(bodyRawVal)
		
		logEntry.ActionDate = nullTimeToRFC3339Ptr(actionDateVal)
		logEntry.CreatedAt = nullTimeToRFC3339(createdAt)
		
		logEntry.Count = nullInt64ToIntPtr(countVal)
		logEntry.ChatHistoryID = nullInt64ToIntPtr(chatHistoryIDVal)
		logEntry.InsightsID = nullInt64ToIntPtr(insightsIDVal)
		logEntry.TokensUsed = nullInt64ToIntPtr(tokensUsedVal)

		logs = append(logs, logEntry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// GetDistinctActions retrieves all distinct action values from audit_logs
func (db *AuditLogDB) GetDistinctActions() ([]string, error) {
	query := `
		SELECT DISTINCT action
		FROM audit_logs
		WHERE action IS NOT NULL
		ORDER BY action ASC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct actions: %w", err)
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}
		actions = append(actions, action)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating actions: %w", err)
	}

	return actions, nil
}

// Helper functions for nullable field conversions

// nullStringToPtr converts sql.NullString to *string
func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// nullInt64ToIntPtr converts sql.NullInt64 to *int
func nullInt64ToIntPtr(ni sql.NullInt64) *int {
	if ni.Valid {
		val := int(ni.Int64)
		return &val
	}
	return nil
}

// nullTimeToRFC3339Ptr converts sql.NullTime to *string (RFC3339 format)
func nullTimeToRFC3339Ptr(nt sql.NullTime) *string {
	if nt.Valid {
		formatted := nt.Time.Format("2006-01-02T15:04:05Z07:00")
		return &formatted
	}
	return nil
}

// nullTimeToRFC3339 converts sql.NullTime to string (RFC3339 format)
func nullTimeToRFC3339(nt sql.NullTime) string {
	if nt.Valid {
		return nt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}

