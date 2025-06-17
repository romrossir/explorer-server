package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"explorer-server/db"
	"explorer-server/handlers"
)

func main() {
	// 2a. Retrieve PostgreSQL connection string
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "user=postgres password=password dbname=explorerdb sslmode=disable host=localhost port=5432"
		log.Println("DATABASE_URL not set, using default:", databaseURL)
	}

	// 2b. Initialize the database connection
	database, err := db.NewDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Successfully connected to the database.")

	// 2c. Create an instance of ComponentHandler
	compHandler := handlers.NewComponentHandler(database)

	// 2d. Set up an http.ServeMux as the router
	mux := http.NewServeMux()

	// 2e. Register handlers
	// Handler for /components (POST for create, GET for list all)
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			compHandler.CreateComponentHandler(w, r)
		case http.MethodGet:
			compHandler.GetAllComponentsHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handler for /components/{id} (GET by ID, PUT, DELETE)
	// The trailing slash is important for ServeMux to catch all paths starting with /components/
	mux.HandleFunc("/components/", func(w http.ResponseWriter, r *http.Request) {
		// Basic check to ensure there's an ID part
		pathParts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 || pathParts[2] == "" { // expecting /components/{id}
			http.Error(w, "Component ID is missing in URL path", http.StatusBadRequest)
			return
		}
		// The actual ID extraction is done within each specific handler as designed previously

		switch r.Method {
		case http.MethodGet:
			compHandler.GetComponentByIDHandler(w, r)
		case http.MethodPut:
			compHandler.UpdateComponentHandler(w, r)
		case http.MethodDelete:
			compHandler.DeleteComponentHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 2f. Start the HTTP server
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Server starting on port %s", port)

	// 2g. Graceful shutdown
	// Run server in a goroutine so that it doesn't block.
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v\n", server.Addr, err)
		}
	}()

	// Listen for termination signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	<-stopChan // Wait for signal

	log.Println("Shutting down server...")

	// Set a deadline for graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}

	log.Println("Server exited properly")
}
