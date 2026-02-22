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

func TestAgentStore_WithModelAndAuth(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	store := NewAgentStore(db)

	// Create agent with model and auth config IDs
	agent := &Agent{
		ID:           "agent-with-refs",
		Name:         "Agent with Foreign Keys",
		Description:  "Test agent with model and auth config references",
		Image:        "test:latest",
		Status:       "stopped",
		ModelID:      "model-123",
		AuthConfigID: "auth-456",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := store.Create(agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify foreign key fields
	retrieved, err := store.GetByID("agent-with-refs")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ModelID != "model-123" {
		t.Errorf("Expected ModelID 'model-123', got '%s'", retrieved.ModelID)
	}
	if retrieved.AuthConfigID != "auth-456" {
		t.Errorf("Expected AuthConfigID 'auth-456', got '%s'", retrieved.AuthConfigID)
	}

	// Test updating foreign key fields
	retrieved.ModelID = "model-789"
	retrieved.AuthConfigID = "auth-999"
	if err := store.Update(retrieved); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := store.GetByID("agent-with-refs")
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}

	if updated.ModelID != "model-789" {
		t.Errorf("Expected updated ModelID 'model-789', got '%s'", updated.ModelID)
	}
	if updated.AuthConfigID != "auth-999" {
		t.Errorf("Expected updated AuthConfigID 'auth-999', got '%s'", updated.AuthConfigID)
	}

	// Test List method includes foreign key fields
	agents, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, a := range agents {
		if a.ID == "agent-with-refs" {
			found = true
			if a.ModelID != "model-789" {
				t.Errorf("List: Expected ModelID 'model-789', got '%s'", a.ModelID)
			}
			if a.AuthConfigID != "auth-999" {
				t.Errorf("List: Expected AuthConfigID 'auth-999', got '%s'", a.AuthConfigID)
			}
			break
		}
	}
	if !found {
		t.Error("Agent not found in List results")
	}
}
