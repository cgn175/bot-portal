package main

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/zeroclaw/bot-portal/internal/api"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func main() {
	// Parse command line flags
	migrateOnly := flag.Bool("migrate-only", false, "Run migrations only and exit")
	flag.Parse()

	// Load .env if present
	_ = godotenv.Load()

	// Initialize SQLite store
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "db/portal.db"
	}
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// If migration only, exit here
	if *migrateOnly {
		log.Println("Migrations completed successfully")
		return
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
