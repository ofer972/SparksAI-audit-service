package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/motiso/sparksai-audit-service/internal/db"
	"github.com/motiso/sparksai-audit-service/internal/routes"
	"github.com/rs/cors"
	"github.com/spf13/viper"
)

func main() {
	// Initialize database
	dbConn := db.Get()
	if dbConn == nil {
		log.Fatal("Failed to initialize database")
	}
	defer dbConn.Close()

	// Setup router
	r := mux.NewRouter()

	// Set up routes
	routes.SetupRoutes(r)

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Content-Type"},
	})

	handler := c.Handler(r)
	port := viper.GetString("SERVER_PORT")
	if port == "" {
		port = "8083"
	}
	log.Printf("Audit Service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func init() {
	dir, _ := os.Getwd()

	if path := os.Getenv("AGILEAGENT_SERVER_HOMEDIR"); path != "" {
		dir = path + "/configs"
	} else {
		dir += "/../configs"
	}

	viper.SetConfigName("app")
	viper.AddConfigPath(dir)
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	viper.AutomaticEnv()
}
