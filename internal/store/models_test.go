package store

import (
	"testing"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

func TestModelStore_CreateAndGet(t *testing.T) {
	// Create in-memory SQLite database
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	// Run migrations to create tables
	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create model store
	store := NewModelStore(db)

	// Test model with JSON defaultParams
	model := &models.Model{
		ID:              "test-model-1",
		Name:            "Test GPT Model",
		Provider:        "openai",
		ModelIdentifier: "gpt-4",
		EndpointURL:     "https://api.openai.com/v1/chat/completions",
		DefaultParams:   `{"temperature": 0.7, "max_tokens": 1000}`,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Test Create
	if err := store.Create(model); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test GetByID
	retrieved, err := store.GetByID("test-model-1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Verify model data integrity
	if retrieved == nil {
		t.Fatal("Expected retrieved model to not be nil")
	}

	if retrieved.ID != model.ID {
		t.Errorf("Expected ID %s, got %s", model.ID, retrieved.ID)
	}

	if retrieved.Name != model.Name {
		t.Errorf("Expected Name %s, got %s", model.Name, retrieved.Name)
	}

	if retrieved.Provider != model.Provider {
		t.Errorf("Expected Provider %s, got %s", model.Provider, retrieved.Provider)
	}

	if retrieved.ModelIdentifier != model.ModelIdentifier {
		t.Errorf("Expected ModelIdentifier %s, got %s", model.ModelIdentifier, retrieved.ModelIdentifier)
	}

	if retrieved.EndpointURL != model.EndpointURL {
		t.Errorf("Expected EndpointURL %s, got %s", model.EndpointURL, retrieved.EndpointURL)
	}

	if retrieved.DefaultParams != model.DefaultParams {
		t.Errorf("Expected DefaultParams %s, got %s", model.DefaultParams, retrieved.DefaultParams)
	}

	// Test List
	models, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != model.ID {
		t.Errorf("Expected first model ID %s, got %s", model.ID, models[0].ID)
	}

	// Test Update
	model.Name = "Updated GPT Model"
	model.DefaultParams = `{"temperature": 0.8, "max_tokens": 1500}`
	model.UpdatedAt = time.Now()

	if err := store.Update(model); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := store.GetByID("test-model-1")
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}

	if updated.Name != "Updated GPT Model" {
		t.Errorf("Expected updated name 'Updated GPT Model', got %s", updated.Name)
	}

	if updated.DefaultParams != `{"temperature": 0.8, "max_tokens": 1500}` {
		t.Errorf("Expected updated DefaultParams, got %s", updated.DefaultParams)
	}

	// Test Delete
	if err := store.Delete("test-model-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, err := store.GetByID("test-model-1")
	if err != nil {
		t.Fatalf("GetByID after delete should not error: %v", err)
	}

	if deleted != nil {
		t.Error("Expected deleted model to be nil")
	}

	// Verify list is empty after delete
	emptyList, err := store.List()
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}

	if len(emptyList) != 0 {
		t.Errorf("Expected empty list after delete, got %d models", len(emptyList))
	}
}

func TestModelStore_NonexistentModel(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	store := NewModelStore(db)

	// Test getting nonexistent model
	model, err := store.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("GetByID should not error on not found: %v", err)
	}

	if model != nil {
		t.Error("Expected nil for nonexistent model")
	}
}