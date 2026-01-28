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

	query += `
		GROUP BY COALESCE(action, endpoint_path, ''), COALESCE(endpoint_path, ''), status_code
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
		var action, endpointPath string
		var statusCode, count int
		if err := rows.Scan(&action, &endpointPath, &statusCode, &count); err != nil {
			return nil, err
		}
		totalCount += count
		results = append(results, FailedEndpoint{
			Action:       action,
			EndpointPath: endpointPath,
			StatusCode:   statusCode,
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

