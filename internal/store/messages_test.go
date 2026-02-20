package store

import (
	"testing"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
)

func TestMessageCRUD(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	store := NewMessageStore(db)

	// Create
	log := &TaskLog{
		ID:          "task-123",
		ChannelID:   "agent1::agent2",
		SenderID:    "agent1",
		RecipientID: "agent2",
		Status:      "pending",
		Direction:   "outbound",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Create(log); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read
	retrieved, err := store.GetByID("task-123")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.SenderID != "agent1" {
		t.Errorf("Expected sender 'agent1', got '%s'", retrieved.SenderID)
	}

	// Update
	log.Status = "completed"
	if err := store.Update(log); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := store.GetByID("task-123")
	if updated.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", updated.Status)
	}

	// List by channel
	logs, err := store.ListByChannel("agent1::agent2", 10)
	if err != nil {
		t.Fatalf("ListByChannel failed: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}

	// Delete
	if err := store.Delete("task-123"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, _ := store.GetByID("task-123")
	if deleted != nil {
		t.Error("Log should be deleted")
	}
}

func TestAppendMessage(t *testing.T) {
	db, _ := NewSQLite(":memory:")
	defer db.Close()
	RunMigrations(db)

	store := NewMessageStore(db)

	log := &TaskLog{
		ID:        "task-123",
		ChannelID: "agent1::agent2",
		SenderID:  "agent1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Create(log)

	// Append first message
	msg1 := a2a.TaskMessage{
		Role:    "user",
		Content: "Hello",
	}
	if err := store.AppendMessage("task-123", msg1); err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}

	// Append second message
	msg2 := a2a.TaskMessage{
		Role:    "assistant",
		Content: "Hi there",
	}
	if err := store.AppendMessage("task-123", msg2); err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}

	// Verify messages were appended
	retrieved, _ := store.GetByID("task-123")
	if len(retrieved.Messages) == 0 {
		t.Error("Expected messages to be stored")
	}
}
