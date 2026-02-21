package main

import (
	"log"
	"os"

	"github.com/zeroclaw/bot-portal/internal/api"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func main() {
	// Initialize SQLite store
	db, err := store.NewSQLite("db/portal.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Docker manager
	dockerMgr, err := docker.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize Docker manager: %v", err)
	}
	defer dockerMgr.Close()

	// Create API router
	router := api.NewRouter(db, dockerMgr)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Bot Portal on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
