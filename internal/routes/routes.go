package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	auditlogService "github.com/motiso/sparksai-audit-service/internal/auditlog/service"
	"github.com/motiso/sparksai-audit-service/internal/db"
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "sparksai-audit-service",
	})
}

// Kubernetes liveness probe - checks if the application is alive
func livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "alive",
	})
}

// Kubernetes readiness probe - checks if the application is ready to serve traffic
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database connectivity
	dbConn := db.Get()
	if dbConn == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"reason": "database connection unavailable",
		})
		return
	}

	// Ping the database
	if err := dbConn.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_ready",
			"reason": "database ping failed",
			"error":  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

func SetupRoutes(r *mux.Router) {
	auditSvc := auditlogService.Get()
	reportSvc := auditlogService.NewReportService()

	// Health check endpoints
	r.HandleFunc("/health", healthCheckHandler).Methods("GET")
	r.HandleFunc("/health/live", livenessHandler).Methods("GET")
	r.HandleFunc("/health/ready", readinessHandler).Methods("GET")

	// Audit log routes
	r.HandleFunc("/api/audit-logs", auditSvc.CreateAuditLogsHandler).Methods("POST")
	r.HandleFunc("/api/audit-logs", auditSvc.GetAuditLogsHandler).Methods("GET")
	r.HandleFunc("/api/audit-logs/actions", auditSvc.GetActionsHandler).Methods("GET")

	// Report routes
	r.HandleFunc("/api/v1/audit-service/reports/{report_id}", reportSvc.GetReport).Methods("GET")
}

