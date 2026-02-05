package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

// Database application name for PostgreSQL connection identification
const DB_APPLICATION_NAME = "SparksAI-Audit"

var db *sql.DB

func initDB() {
	db = connectDB()
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(4)
}

func Get() *sql.DB {
	if db == nil {
		initDB()
	}
	return db
}

func getDBConfig() (username string, password string,
	databasename string, databaseHost string, databasePort string) {

	databasename = strings.ToLower(viper.GetString("POSTGRES_DB")) // Standardize to lowercase
	databaseHost = viper.GetString("POSTGRES_HOST")
	username = viper.GetString("POSTGRES_USER")
	password = viper.GetString("POSTGRES_PASSWORD")
	databasePort = viper.GetString("POSTGRES_PORT")
	return
}

func checkDBExists(db *sql.DB, dbName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE lower(datname) = lower($1));`
	err := db.QueryRow(query, dbName).Scan(&exists)
	return exists, err
}

func createAndOpen(name string, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn+" dbname=postgres")
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer db.Close()

	// Create a new database
	exists, err := checkDBExists(db, name)
	if err != nil {
		log.Fatal("Error checking database existence:", err)
	}

	if exists {
		log.Println("Database already exists:", name)
	} else {
		// Create the database if it does not exist, with quoting
		_, err = db.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, name))
		if err != nil {
			log.Fatal("Error creating database:", err)
		}
		log.Println("Database created successfully:", name)
	}

	newConnStr := fmt.Sprintf(dsn+" dbname=%s", name)
	newDB, err := sql.Open("postgres", newConnStr)
	if err != nil {
		log.Fatal("Error connecting to new database:", err)
	}

	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255),
			severity VARCHAR(20) DEFAULT 'NONE' NOT NULL,
			endpoint_path VARCHAR(500) NOT NULL,
			session_id VARCHAR(255),
			action VARCHAR(255),
			action_date TIMESTAMP WITH TIME ZONE,
			count INTEGER,
			http_method VARCHAR(10) NOT NULL,
			status_code INTEGER NOT NULL,
			response_time_seconds NUMERIC(10, 3) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			ip_address INET,
			user_agent TEXT,
			chat_history_id INTEGER,
			insights_id INTEGER,
			tokens_used INTEGER,
			query_raw JSONB,
			body_raw JSONB,
			response_body JSONB
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_severity ON audit_logs(severity);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_body_raw ON audit_logs USING GIN (body_raw);`,
	}

	// Execute table creation statements
	for _, stmt := range statements {
		if _, err := newDB.Exec(stmt); err != nil {
			return nil, err
		}
	}

	return newDB, nil
}

func connectDB() *sql.DB {
	username, password, databasename, databaseHost, databasePort := getDBConfig()

	// Debug: Print database config (without password)
	log.Printf("Connecting to database: host=%s port=%s user=%s db=%s", databaseHost, databasePort, username, databasename)

	dbURI := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=disable application_name=%s ", databaseHost, databasePort, username, password, DB_APPLICATION_NAME)

	//connect to db URI
	db, err := createAndOpen(databasename, dbURI)
	if err != nil {
		fmt.Println("error", err)
		log.Fatalf(err.Error())
	}

	return db
}

