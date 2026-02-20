package store

import (
	"testing"
	"time"
)

func TestAgentCRUD(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	store := NewAgentStore(db)

	// Create
	agent := &Agent{
		ID:          "test-agent",
		Name:        "Test Agent",
		Image:       "test:latest",
		Endpoint:    "http://localhost:8080",
		Status:      "stopped",
		BearerToken: "test-token",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Create(agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read
	retrieved, err := store.GetByID("test-agent")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.Name != "Test Agent" {
		t.Errorf("Expected name 'Test Agent', got '%s'", retrieved.Name)
	}

	// Update
	agent.Status = "running"
	if err := store.Update(agent); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := store.GetByID("test-agent")
	if updated.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}

	// List
	agents, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}

	// Delete
	if err := store.Delete("test-agent"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, _ := store.GetByID("test-agent")
	if deleted != nil {
		t.Error("Agent should be deleted")
	}
}

func TestGenerateBearerToken(t *testing.T) {
	db, _ := NewSQLite(":memory:")
	defer db.Close()
	RunMigrations(db)

	store := NewAgentStore(db)

	agent := &Agent{
		ID:        "test",
		Name:      "Test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Create(agent)

	token, err := store.GenerateBearerToken("test")
	if err != nil {
		t.Fatalf("GenerateBearerToken failed: %v", err)
	}

	if len(token) != 64 { // 32 bytes hex = 64 chars
		t.Errorf("Expected token length 64, got %d", len(token))
	}

	// Verify token is stored
	retrieved, _ := store.GetByID("test")
	if retrieved.BearerToken != token {
		t.Error("Token not stored correctly")
	}
}

func TestAgentNotFound(t *testing.T) {
	db, _ := NewSQLite(":memory:")
	defer db.Close()
	RunMigrations(db)

	store := NewAgentStore(db)

	agent, err := store.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("GetByID should not error on not found: %v", err)
	}
	if agent != nil {
		t.Error("Expected nil for nonexistent agent")
	}
}
