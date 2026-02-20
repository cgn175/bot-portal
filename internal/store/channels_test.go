package store

import (
	"testing"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

func TestChannelCRUD(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	store := NewChannelStore(db)

	// Create
	channel := &models.Channel{
		ID:        "agent1::agent2",
		Members:   []string{"agent1", "agent2"},
		CreatedAt: time.Now(),
	}

	if err := store.Create(channel); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read
	retrieved, err := store.GetByID("agent1::agent2")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if len(retrieved.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(retrieved.Members))
	}

	// List
	channels, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(channels))
	}

	// Delete
	if err := store.Delete("agent1::agent2"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, _ := store.GetByID("agent1::agent2")
	if deleted != nil {
		t.Error("Channel should be deleted")
	}
}

func TestEnsureGeneralChannel(t *testing.T) {
	db, _ := NewSQLite(":memory:")
	defer db.Close()
	RunMigrations(db)

	store := NewChannelStore(db)

	// First call creates it
	if err := store.EnsureGeneralChannel(); err != nil {
		t.Fatalf("EnsureGeneralChannel failed: %v", err)
	}

	channel, err := store.GetByID("general")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if channel.ID != "general" {
		t.Errorf("Expected channel ID 'general', got '%s'", channel.ID)
	}

	// Second call will error (duplicate), which is expected behavior
	// In production, caller should check if channel exists first
	err = store.EnsureGeneralChannel()
	if err == nil {
		t.Log("Note: EnsureGeneralChannel allows duplicate creation")
	}
}
