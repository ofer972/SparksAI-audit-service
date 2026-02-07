package service

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	database "github.com/motiso/sparksai-audit-service/internal/db"
	"github.com/motiso/sparksai-audit-service/internal/auditlog"
)

type ReportService struct {
	DB *sql.DB
}

func NewReportService() *ReportService {
	return &ReportService{
		DB: database.Get(),
	}
}

// GetReport handles report requests and routes to appropriate data function
func (s *ReportService) GetReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["report_id"]

	// Parse filters from query parameters
	months := r.URL.Query().Get("months")
	month := r.URL.Query().Get("month") // For specific month filter (e.g., "2026-01")
	userID := r.URL.Query().Get("user_id")
	httpMethod := r.URL.Query().Get("http_method")
	action := r.URL.Query().Get("action")
	minTokens := r.URL.Query().Get("min_tokens")
	minResponseTime := r.URL.Query().Get("min_response_time")
	statusCode := r.URL.Query().Get("status_code")
	statusCodeMin := r.URL.Query().Get("status_code_min")
	statusCodeMax := r.URL.Query().Get("status_code_max")
	searchQuery := r.URL.Query().Get("search_query")
	severity := r.URL.Query().Get("severity")

	filters := map[string]interface{}{
		"months":            months,
		"month":             month,
		"user_id":           userID,
		"http_method":       httpMethod,
		"action":            action,
		"min_tokens":        minTokens,
		"min_response_time": minResponseTime,
		"status_code":       statusCode,
		"status_code_min":   statusCodeMin,
		"status_code_max":   statusCodeMax,
		"search_query":      searchQuery,
		"severity":          severity,
	}

	// Route to appropriate data function
	var result interface{}
	var err error

	switch reportID {
	case "audit-frequently-used-actions":
		result, err = s.getFrequentlyUsedActions(filters)
	case "audit-issues-synced-trend":
		result, err = s.getIssuesSyncedTrend(filters)
	case "audit-token-usage":
		result, err = s.getTokenUsage(filters)
	case "audit-slow-actions":
		result, err = s.getSlowActions(filters)
	case "audit-failed-endpoints":
		result, err = s.getFailedEndpoints(filters)
	case "audit-user-questions":
		result, err = s.getUserQuestions(filters)
	case "audit-most-active-users":
		result, err = s.getMostActiveUsers(filters)
	case "audit-daily-active-users":
		result, err = s.getDailyActiveUsers(filters)
	case "audit-logs":
		result, err = s.getAuditLogs(filters)
	default:
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("Error getting report data for %s: %v", reportID, err)
		http.Error(w, "Failed to get report data", http.StatusInternalServerError)
		return
	}

	// Return only the data - backend will add definition and format response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// calculateDateRange calculates date_from and date_to from months parameter
func calculateDateRange(monthsStr string) (time.Time, time.Time, error) {
	months, err := strconv.Atoi(monthsStr)
	if err != nil || months <= 0 {
		months = 1
	}

	// Use UTC to match database timezone
	now := time.Now().UTC()
	dateTo := now
	dateFrom := now.AddDate(0, -months, 0)

	return dateFrom, dateTo, nil
}

type FrequentlyUsedAction struct {
	Action           string  `json:"action"`
	EndpointPath     string  `json:"endpoint_path"`
	Count            int     `json:"count"`
	Percentage       float64 `json:"percentage"`
	AvgResponseTime  float64 `json:"avg_response_time"`
}

func (s *ReportService) getFrequentlyUsedActions(filters map[string]interface{}) ([]FrequentlyUsedAction, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			COALESCE(action, endpoint_path, '') as action,
			COALESCE(endpoint_path, '') as endpoint_path,
			COUNT(*) as count,
			AVG(response_time_seconds) as avg_response_time
		FROM audit_logs
		WHERE created_at >= $1
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if userID := getString(filters, "user_id", ""); userID != "" {
		query += " AND user_id = $" + strconv.Itoa(argIndex)
		args = append(args, userID)
		argIndex++
	}

	if httpMethod := getString(filters, "http_method", ""); httpMethod != "" {
		query += " AND http_method = $" + strconv.Itoa(argIndex)
		args = append(args, httpMethod)
		argIndex++
	}

	query += `
		GROUP BY COALESCE(action, endpoint_path, ''), COALESCE(endpoint_path, '')
		ORDER BY count DESC
		LIMIT 400
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FrequentlyUsedAction
	var totalCount int

	for rows.Next() {
		var action, endpointPath string
		var count int
		var avgResponseTime sql.NullFloat64
		if err := rows.Scan(&action, &endpointPath, &count, &avgResponseTime); err != nil {
			return nil, err
		}
		totalCount += count
		avg := 0.0
		if avgResponseTime.Valid {
			avg = avgResponseTime.Float64
		}
		results = append(results, FrequentlyUsedAction{
			Action:          action,
			EndpointPath:    endpointPath,
			Count:           count,
			AvgResponseTime: avg,
		})
	}

	// Calculate percentages
	for i := range results {
		if totalCount > 0 {
			results[i].Percentage = float64(results[i].Count) / float64(totalCount) * 100
		}
	}

	return results, nil
}

type IssuesSyncedTrend struct {
	Date            string  `json:"date"`
	AvgIssuesSynced float64 `json:"avg_issues_synced"`
	TotalRequests   int     `json:"total_requests"`
}

func (s *ReportService) getIssuesSyncedTrend(filters map[string]interface{}) ([]IssuesSyncedTrend, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			DATE(created_at) as date,
			AVG(count) as avg_issues_synced,
			COUNT(*) as total_requests
		FROM audit_logs
		WHERE created_at >= $1
			AND count IS NOT NULL
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if action := getString(filters, "action", ""); action != "" {
		query += " AND action = $" + strconv.Itoa(argIndex)
		args = append(args, action)
		argIndex++
	}

	query += `
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []IssuesSyncedTrend
	for rows.Next() {
		var date time.Time
		var avgIssuesSynced sql.NullFloat64
		var totalRequests int
		if err := rows.Scan(&date, &avgIssuesSynced, &totalRequests); err != nil {
			return nil, err
		}
		avg := 0.0
		if avgIssuesSynced.Valid {
			avg = avgIssuesSynced.Float64
		}
		results = append(results, IssuesSyncedTrend{
			Date:            date.Format("2006-01-02"),
			AvgIssuesSynced: avg,
			TotalRequests:   totalRequests,
		})
	}

	return results, nil
}

type TokenUsage struct {
	Action       string  `json:"action"`
	TotalTokens  int     `json:"total_tokens"`
	AvgTokens    float64 `json:"avg_tokens"`
	RequestCount int     `json:"request_count"`
}

func (s *ReportService) getTokenUsage(filters map[string]interface{}) ([]TokenUsage, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			COALESCE(action, endpoint_path) as action,
			SUM(tokens_used) as total_tokens,
			AVG(tokens_used) as avg_tokens,
			COUNT(*) as request_count
		FROM audit_logs
		WHERE created_at >= $1
			AND tokens_used IS NOT NULL
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if action := getString(filters, "action", ""); action != "" {
		query += " AND action = $" + strconv.Itoa(argIndex)
		args = append(args, action)
		argIndex++
	}

	if minTokens := getString(filters, "min_tokens", ""); minTokens != "" {
		if min, err := strconv.Atoi(minTokens); err == nil {
			query += " AND tokens_used >= $" + strconv.Itoa(argIndex)
			args = append(args, min)
			argIndex++
		}
	}

	query += `
		GROUP BY COALESCE(action, endpoint_path)
		ORDER BY total_tokens DESC
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TokenUsage
	for rows.Next() {
		var action string
		var totalTokens sql.NullInt64
		var avgTokens sql.NullFloat64
		var requestCount int
		if err := rows.Scan(&action, &totalTokens, &avgTokens, &requestCount); err != nil {
			return nil, err
		}
		total := 0
		if totalTokens.Valid {
			total = int(totalTokens.Int64)
		}
		avg := 0.0
		if avgTokens.Valid {
			avg = avgTokens.Float64
		}
		results = append(results, TokenUsage{
			Action:       action,
			TotalTokens:  total,
			AvgTokens:    avg,
			RequestCount: requestCount,
		})
	}

	return results, nil
}

type SlowAction struct {
	EndpointPath      string  `json:"endpoint_path"`
	Action           string  `json:"action"`
	AvgResponseTime  float64 `json:"avg_response_time"`
	MaxResponseTime  float64 `json:"max_response_time"`
	RequestCount     int     `json:"request_count"`
}

func (s *ReportService) getSlowActions(filters map[string]interface{}) ([]SlowAction, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			COALESCE(endpoint_path, '') as endpoint_path,
			COALESCE(action, '') as action,
			AVG(response_time_seconds) as avg_response_time,
			MAX(response_time_seconds) as max_response_time,
			COUNT(*) as request_count
		FROM audit_logs
		WHERE created_at >= $1
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if minResponseTime := getString(filters, "min_response_time", ""); minResponseTime != "" {
		if min, err := strconv.ParseFloat(minResponseTime, 64); err == nil {
			query += " AND response_time_seconds >= $" + strconv.Itoa(argIndex)
			args = append(args, min)
			argIndex++
		}
	}

	if statusCode := getString(filters, "status_code", ""); statusCode != "" {
		if code, err := strconv.Atoi(statusCode); err == nil {
			query += " AND status_code = $" + strconv.Itoa(argIndex)
			args = append(args, code)
			argIndex++
		}
	}

	query += `
		GROUP BY COALESCE(endpoint_path, ''), COALESCE(action, '')
		ORDER BY avg_response_time DESC
		LIMIT 400
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SlowAction
	for rows.Next() {
		var endpointPath, action string
		var avgResponseTime, maxResponseTime sql.NullFloat64
		var requestCount int
		if err := rows.Scan(&endpointPath, &action, &avgResponseTime, &maxResponseTime, &requestCount); err != nil {
			return nil, err
		}
		avg := 0.0
		if avgResponseTime.Valid {
			avg = avgResponseTime.Float64
		}
		max := 0.0
		if maxResponseTime.Valid {
			max = maxResponseTime.Float64
		}
		results = append(results, SlowAction{
			EndpointPath:     endpointPath,
			Action:           action,
			AvgResponseTime:  avg,
			MaxResponseTime:  max,
			RequestCount:     requestCount,
		})
	}

	return results, nil
}

type FailedEndpoint struct {
	Action       string  `json:"action"`
	EndpointPath string  `json:"endpoint_path"`
	StatusCode   int     `json:"status_code"`
	Severity     string  `json:"severity"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
}

func (s *ReportService) getFailedEndpoints(filters map[string]interface{}) ([]FailedEndpoint, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			COALESCE(action, endpoint_path, '') as action,
			COALESCE(endpoint_path, '') as endpoint_path,
			status_code,
			severity,
			COUNT(*) as count
		FROM audit_logs
		WHERE created_at >= $1
			AND status_code::integer >= 400
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if httpMethod := getString(filters, "http_method", ""); httpMethod != "" {
		query += " AND http_method = $" + strconv.Itoa(argIndex)
		args = append(args, httpMethod)
		argIndex++
	}

	if severity := getString(filters, "severity", ""); severity != "" {
		query += " AND severity = $" + strconv.Itoa(argIndex)
		args = append(args, severity)
		argIndex++
	}

	query += `
		GROUP BY COALESCE(action, endpoint_path, ''), COALESCE(endpoint_path, ''), status_code, severity
		ORDER BY count DESC
		LIMIT 400
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FailedEndpoint
	var totalCount int

	for rows.Next() {
		var action, endpointPath, severity string
		var statusCode, count int
		if err := rows.Scan(&action, &endpointPath, &statusCode, &severity, &count); err != nil {
			return nil, err
		}
		totalCount += count
		results = append(results, FailedEndpoint{
			Action:       action,
			EndpointPath: endpointPath,
			StatusCode:   statusCode,
			Severity:     severity,
			Count:        count,
		})
	}

	// Calculate percentages
	for i := range results {
		if totalCount > 0 {
			results[i].Percentage = float64(results[i].Count) / float64(totalCount) * 100
		}
	}

	return results, nil
}

type UserQuestion struct {
	CreatedAt           string  `json:"created_at"`
	UserID              string  `json:"user_id"`
	Question            string  `json:"question"`
	Answer              string  `json:"answer"`
	TokensUsed          int     `json:"tokens_used"`
	ResponseTimeSeconds float64 `json:"response_time_seconds"`
	StatusCode          int     `json:"status_code"`
	InsightsID          *int    `json:"insights_id"`
}

func (s *ReportService) getUserQuestions(filters map[string]interface{}) ([]UserQuestion, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			created_at,
			COALESCE(user_id, '') as user_id,
			body_raw->>'question' as question,
			response_body->'data'->>'response' as answer,
			COALESCE(tokens_used, 0) as tokens_used,
			response_time_seconds,
			status_code,
			insights_id
		FROM audit_logs
		WHERE created_at >= $1
			AND body_raw->>'question' IS NOT NULL
			AND body_raw->>'question' != ''
	`
	args := []interface{}{dateFrom}
	argIndex := 2

	if userID := getString(filters, "user_id", ""); userID != "" {
		query += " AND user_id = $" + strconv.Itoa(argIndex)
		args = append(args, userID)
		argIndex++
	}

	if searchQuery := getString(filters, "search_query", ""); searchQuery != "" {
		query += " AND body_raw->>'question' ILIKE $" + strconv.Itoa(argIndex)
		args = append(args, "%"+searchQuery+"%")
		argIndex++
	}

	query += `
		ORDER BY created_at DESC
		LIMIT 400
	`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UserQuestion
	for rows.Next() {
		var createdAt time.Time
		var userID, question, answer sql.NullString
		var tokensUsed, insightsID sql.NullInt64
		var responseTimeSeconds float64
		var statusCode int
		if err := rows.Scan(&createdAt, &userID, &question, &answer, &tokensUsed, &responseTimeSeconds, &statusCode, &insightsID); err != nil {
			return nil, err
		}
		userIDStr := ""
		if userID.Valid {
			userIDStr = userID.String
		}
		questionStr := ""
		if question.Valid {
			questionStr = question.String
		}
		answerStr := ""
		if answer.Valid {
			answerStr = answer.String
		}
		tokens := 0
		if tokensUsed.Valid {
			tokens = int(tokensUsed.Int64)
		}
		var insightsIDPtr *int
		if insightsID.Valid {
			val := int(insightsID.Int64)
			insightsIDPtr = &val
		}
		results = append(results, UserQuestion{
			CreatedAt:           createdAt.Format(time.RFC3339),
			UserID:              userIDStr,
			Question:            questionStr,
			Answer:              answerStr,
			TokensUsed:          tokens,
			ResponseTimeSeconds: responseTimeSeconds,
			StatusCode:          statusCode,
			InsightsID:          insightsIDPtr,
		})
	}

	return results, nil
}

type MostActiveUser struct {
	UserID        string  `json:"user_id"`
	RequestCount  int     `json:"request_count"`
	Percentage    float64 `json:"percentage"`
}

func (s *ReportService) getMostActiveUsers(filters map[string]interface{}) ([]MostActiveUser, error) {
	dateFrom, _, _ := calculateDateRange(getString(filters, "months", "1"))

	query := `
		SELECT 
			COALESCE(user_id, 'Unknown') as user_id,
			COUNT(*) as request_count
		FROM audit_logs
		WHERE created_at >= $1
			AND user_id IS NOT NULL
		GROUP BY user_id
		ORDER BY request_count DESC
		LIMIT 400
	`

	rows, err := s.DB.Query(query, dateFrom)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MostActiveUser
	var totalCount int

	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, err
		}
		totalCount += count
		results = append(results, MostActiveUser{
			UserID:       userID,
			RequestCount: count,
		})
	}

	// Calculate percentages
	for i := range results {
		if totalCount > 0 {
			results[i].Percentage = float64(results[i].RequestCount) / float64(totalCount) * 100
		}
	}

	return results, nil
}

type DailyActiveUsers struct {
	Date        string `json:"date"`
	Day         int    `json:"day"`
	UniqueUsers int    `json:"unique_users"`
}

func (s *ReportService) getDailyActiveUsers(filters map[string]interface{}) ([]DailyActiveUsers, error) {
	// Get month filter (e.g., "2026-01")
	month := getString(filters, "month", "")
	
	// Default to current month if not provided
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}

	// Parse month to get start and end dates
	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		monthStart = time.Now().UTC()
		monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	monthEnd := monthStart.AddDate(0, 1, 0)

	query := `
		SELECT 
			DATE(created_at) as date,
			EXTRACT(DAY FROM created_at)::integer as day,
			COUNT(DISTINCT user_id) as unique_users
		FROM audit_logs
		WHERE created_at >= $1
			AND created_at < $2
			AND user_id IS NOT NULL
		GROUP BY DATE(created_at), EXTRACT(DAY FROM created_at)
		ORDER BY date ASC
	`

	rows, err := s.DB.Query(query, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DailyActiveUsers
	for rows.Next() {
		var date time.Time
		var day int
		var uniqueUsers int
		if err := rows.Scan(&date, &day, &uniqueUsers); err != nil {
			return nil, err
		}
		results = append(results, DailyActiveUsers{
			Date:        date.Format("2006-01-02"),
			Day:         day,
			UniqueUsers: uniqueUsers,
		})
	}

	return results, nil
}

// Helper function to get string from filters map
func getString(filters map[string]interface{}, key, defaultValue string) string {
	if val, ok := filters[key]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return defaultValue
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetAuditLogsFilterValues retrieves all distinct values for filter dropdowns
func (s *ReportService) GetAuditLogsFilterValues() (map[string]interface{}, error) {
	query := `
		WITH http_methods AS (
			SELECT DISTINCT http_method::text as value, 'http_method' as type
			FROM audit_logs
			WHERE http_method IS NOT NULL
		),
		status_codes AS (
			SELECT DISTINCT status_code::text as value, 'status_code' as type
			FROM audit_logs
		),
		severities AS (
			SELECT DISTINCT severity as value, 'severity' as type
			FROM audit_logs
			WHERE severity IS NOT NULL
		),
		user_ids AS (
			SELECT DISTINCT user_id as value, 'user_id' as type
			FROM audit_logs
			WHERE user_id IS NOT NULL
		),
		actions AS (
			SELECT DISTINCT action as value, 'action' as type
			FROM audit_logs
			WHERE action IS NOT NULL
		)
		SELECT value, type FROM http_methods
		UNION ALL
		SELECT value, type FROM status_codes
		UNION ALL
		SELECT value, type FROM severities
		UNION ALL
		SELECT value, type FROM user_ids
		UNION ALL
		SELECT value, type FROM actions
		ORDER BY type, value
	`

	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]interface{}{
		"http_methods": []string{},
		"status_codes": []int{},
		"severities":  []string{},
		"user_ids":     []string{},
		"actions":     []string{},
	}

	httpMethods := []string{}
	statusCodes := []int{}
	severities := []string{}
	userIDs := []string{}
	actions := []string{}

	for rows.Next() {
		var value, typeStr string
		if err := rows.Scan(&value, &typeStr); err != nil {
			return nil, err
		}

		switch typeStr {
		case "http_method":
			httpMethods = append(httpMethods, value)
		case "status_code":
			if code, err := strconv.Atoi(value); err == nil {
				statusCodes = append(statusCodes, code)
			}
		case "severity":
			severities = append(severities, value)
		case "user_id":
			userIDs = append(userIDs, value)
		case "action":
			actions = append(actions, value)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	result["http_methods"] = httpMethods
	result["status_codes"] = statusCodes
	result["severities"] = severities
	result["user_ids"] = userIDs
	result["actions"] = actions

	return result, nil
}

// getAuditLogs retrieves audit logs with filters
func (s *ReportService) getAuditLogs(filters map[string]interface{}) ([]auditlog.AuditLog, error) {
	// Parse filters
	userID := getString(filters, "user_id", "")
	severity := getString(filters, "severity", "")
	action := getString(filters, "action", "")
	httpMethod := getString(filters, "http_method", "")
	statusCodeStr := getString(filters, "status_code", "")
	dateFromStr := getString(filters, "date_from", "")
	minTokensStr := getString(filters, "min_tokens", "")
	limitStr := getString(filters, "limit", "100")

	// Parse limit (default 100, max 500)
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			if parsed > 0 && parsed <= 500 {
				limit = parsed
			} else if parsed > 500 {
				limit = 500
			}
		}
	}

	// Build query
	query := `
		SELECT 
			id, user_id, severity, endpoint_path, session_id, action, action_date, count, http_method, status_code,
			response_time_seconds, created_at, ip_address, user_agent,
			chat_history_id, insights_id, tokens_used,
			query_raw, body_raw, response_body
		FROM audit_logs
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	// Default date_from to today if not provided
	var dateFrom time.Time
	if dateFromStr != "" {
		parsed, err := time.Parse("2006-01-02", dateFromStr)
		if err == nil {
			dateFrom = parsed.UTC()
		} else {
			dateFrom = time.Now().UTC().Truncate(24 * time.Hour)
		}
	} else {
		dateFrom = time.Now().UTC().Truncate(24 * time.Hour)
	}
	query += " AND created_at >= $" + strconv.Itoa(argIndex)
	args = append(args, dateFrom)
	argIndex++

	if userID != "" {
		query += " AND user_id = $" + strconv.Itoa(argIndex)
		args = append(args, userID)
		argIndex++
	}

	if severity != "" {
		query += " AND severity = $" + strconv.Itoa(argIndex)
		args = append(args, severity)
		argIndex++
	}

	if action != "" {
		query += " AND action = $" + strconv.Itoa(argIndex)
		args = append(args, action)
		argIndex++
	}

	if httpMethod != "" {
		query += " AND http_method = $" + strconv.Itoa(argIndex)
		args = append(args, httpMethod)
		argIndex++
	}

	if statusCodeStr != "" {
		if statusCode, err := strconv.Atoi(statusCodeStr); err == nil {
			query += " AND status_code = $" + strconv.Itoa(argIndex)
			args = append(args, statusCode)
			argIndex++
		}
	}

	if minTokensStr != "" {
		if minTokens, err := strconv.Atoi(minTokensStr); err == nil {
			query += " AND tokens_used >= $" + strconv.Itoa(argIndex)
			args = append(args, minTokens)
			argIndex++
		}
	}

	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argIndex)
	args = append(args, limit)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []auditlog.AuditLog
	for rows.Next() {
		var logEntry auditlog.AuditLog
		var userIDVal, sessionIDVal, actionVal, ipAddressVal, userAgentVal sql.NullString
		var chatHistoryIDVal, insightsIDVal, tokensUsedVal, countVal sql.NullInt64
		var queryRawVal, bodyRawVal, responseBodyVal sql.NullString
		var createdAt, actionDateVal sql.NullTime

		err := rows.Scan(
			&logEntry.ID,
			&userIDVal,
			&logEntry.Severity,
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
			&responseBodyVal,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		logEntry.UserID = nullStringToPtr(userIDVal)
		logEntry.SessionID = nullStringToPtr(sessionIDVal)
		logEntry.Action = nullStringToPtr(actionVal)
		logEntry.IPAddress = nullStringToPtr(ipAddressVal)
		logEntry.UserAgent = nullStringToPtr(userAgentVal)
		logEntry.QueryRaw = nullStringToPtr(queryRawVal)
		logEntry.BodyRaw = nullStringToPtr(bodyRawVal)
		logEntry.ResponseBody = nullStringToPtr(responseBodyVal)

		logEntry.ActionDate = nullTimeToRFC3339Ptr(actionDateVal)
		logEntry.CreatedAt = nullTimeToRFC3339(createdAt)

		logEntry.Count = nullInt64ToIntPtr(countVal)
		logEntry.ChatHistoryID = nullInt64ToIntPtr(chatHistoryIDVal)
		logEntry.InsightsID = nullInt64ToIntPtr(insightsIDVal)
		logEntry.TokensUsed = nullInt64ToIntPtr(tokensUsedVal)

		logs = append(logs, logEntry)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

