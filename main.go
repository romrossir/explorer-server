package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"component-service/db"     // Assuming db is in component-service/db
	"component-service/routes" // Assuming routes is in component-service/routes
)

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Environment variable %s not set, using default: %s", key, fallback)
	return fallback
}

func main() {
	// Database Configuration
	// For a real application, avoid hardcoding default credentials directly in code.
	// These are illustrative defaults.
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "user")      // Placeholder default user
	dbPassword := getEnv("DB_PASSWORD", "secret") // Placeholder default password
	dbName := getEnv("DB_NAME", "servicedb")   // Placeholder default db name
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	// Initialize Database
	// The InitDB function will log.Fatalf on connection errors.
	db.InitDB(dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	// Ensure the components table exists
	// CreateTable is idempotent due to "IF NOT EXISTS" in its SQL.
	// It will log.Fatalf on error.
	db.CreateTable()

	// Register Routes
	routes.RegisterComponentRoutes()

	// Server Configuration
	serverPort := getEnv("SERVER_PORT", "8080")
	addr := fmt.Sprintf(":%s", serverPort)

	log.Printf("Server starting on http://localhost%s", addr)
	// Using http.DefaultServeMux by passing nil as the handler.
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server on address %s: %v", addr, err)
	}
}
