package store

import (
	"database/sql"
	"testing"

	"github.com/zeroclaw/bot-portal/internal/models"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create agents table first (for foreign key)
	_, err = db.Exec(`CREATE TABLE agents (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		image TEXT NOT NULL,
		agent_type TEXT DEFAULT 'docker',
		status TEXT DEFAULT 'stopped',
		container_id TEXT,
		endpoint TEXT,
		listen_port INTEGER DEFAULT 17000,
		bearer_token TEXT,
		agent_card TEXT,
		config TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("Failed to create agents table: %v", err)
	}

	// Create identity files table
	_, err = db.Exec(`CREATE TABLE agent_identity_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		filename TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		char_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(agent_id, filename),
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
	)`)
	if err != nil {
		t.Fatalf("Failed to create identity_files table: %v", err)
	}

	// Insert a test agent
	_, err = db.Exec("INSERT INTO agents (id, name, image) VALUES (?, ?, ?)", "agent-1", "Test Agent", "test:latest")
	if err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	return db
}

func TestIdentityFileStore_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	file := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "IDENTITY.md",
		Content:  "# Identity\n\nI am a helpful assistant.",
	}

	err := store.Create(file)
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	if file.ID == 0 {
		t.Error("Expected ID to be set after create")
	}

	if file.CharCount != len(file.Content) {
		t.Errorf("Expected CharCount to be %d, got %d", len(file.Content), file.CharCount)
	}
}

func TestIdentityFileStore_GetByAgentAndFilename(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create a file
	file := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "SOUL.md",
		Content:  "Soul content here",
	}
	if err := store.Create(file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve it
	retrieved, err := store.GetByAgentAndFilename("agent-1", "SOUL.md")
	if err != nil {
		t.Errorf("GetByAgentAndFilename failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to retrieve file, got nil")
	}

	if retrieved.Content != "Soul content here" {
		t.Errorf("Expected content 'Soul content here', got %q", retrieved.Content)
	}

	// Test non-existent file
	notFound, err := store.GetByAgentAndFilename("agent-1", "NONEXISTENT.md")
	if err != nil {
		t.Errorf("GetByAgentAndFilename for non-existent failed: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for non-existent file")
	}
}

func TestIdentityFileStore_ListByAgent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create multiple files
	files := []*models.AgentIdentityFile{
		{AgentID: "agent-1", Filename: "IDENTITY.md", Content: "Identity"},
		{AgentID: "agent-1", Filename: "SOUL.md", Content: "Soul"},
		{AgentID: "agent-1", Filename: "TOOLS.md", Content: "Tools"},
	}

	for _, f := range files {
		if err := store.Create(f); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List files
	retrieved, err := store.ListByAgent("agent-1")
	if err != nil {
		t.Errorf("ListByAgent failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 files, got %d", len(retrieved))
	}
}

func TestIdentityFileStore_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create a file
	file := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "USER.md",
		Content:  "Original content",
	}
	if err := store.Create(file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update it
	file.Content = "Updated content"
	if err := store.Update(file); err != nil {
		t.Errorf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := store.GetByAgentAndFilename("agent-1", "USER.md")
	if err != nil {
		t.Fatalf("GetByAgentAndFilename failed: %v", err)
	}

	if retrieved.Content != "Updated content" {
		t.Errorf("Expected content 'Updated content', got %q", retrieved.Content)
	}

	if retrieved.CharCount != len("Updated content") {
		t.Errorf("Expected CharCount %d, got %d", len("Updated content"), retrieved.CharCount)
	}
}

func TestIdentityFileStore_CreateOrUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create new file via CreateOrUpdate
	file := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "AGENTS.md",
		Content:  "Initial",
	}
	if err := store.CreateOrUpdate(file); err != nil {
		t.Fatalf("CreateOrUpdate (create) failed: %v", err)
	}

	if file.ID == 0 {
		t.Error("Expected ID to be set after CreateOrUpdate (create)")
	}

	// Update existing file via CreateOrUpdate
	file.Content = "Updated via CreateOrUpdate"
	originalID := file.ID
	if err := store.CreateOrUpdate(file); err != nil {
		t.Fatalf("CreateOrUpdate (update) failed: %v", err)
	}

	if file.ID != originalID {
		t.Errorf("Expected ID to remain %d, got %d", originalID, file.ID)
	}

	// Verify
	retrieved, err := store.GetByAgentAndFilename("agent-1", "AGENTS.md")
	if err != nil {
		t.Fatalf("GetByAgentAndFilename failed: %v", err)
	}

	if retrieved.Content != "Updated via CreateOrUpdate" {
		t.Errorf("Expected updated content, got %q", retrieved.Content)
	}
}

func TestIdentityFileStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create a file
	file := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "TOOLS.md",
		Content:  "Tools content",
	}
	if err := store.Create(file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete it
	if err := store.Delete("agent-1", "TOOLS.md"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify deletion
	retrieved, err := store.GetByAgentAndFilename("agent-1", "TOOLS.md")
	if err != nil {
		t.Fatalf("GetByAgentAndFilename failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected file to be deleted")
	}
}

func TestIdentityFileStore_DeleteAllForAgent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create multiple files
	files := []*models.AgentIdentityFile{
		{AgentID: "agent-1", Filename: "IDENTITY.md", Content: "Identity"},
		{AgentID: "agent-1", Filename: "SOUL.md", Content: "Soul"},
	}

	for _, f := range files {
		if err := store.Create(f); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Delete all for agent
	if err := store.DeleteAllForAgent("agent-1"); err != nil {
		t.Errorf("DeleteAllForAgent failed: %v", err)
	}

	// Verify
	retrieved, err := store.ListByAgent("agent-1")
	if err != nil {
		t.Fatalf("ListByAgent failed: %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 files after delete all, got %d", len(retrieved))
	}
}

func TestIdentityFileStore_UniqueConstraint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewIdentityFileStore(db)

	// Create first file
	file1 := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "IDENTITY.md",
		Content:  "First",
	}
	if err := store.Create(file1); err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Try to create duplicate (should fail at DB level)
	file2 := &models.AgentIdentityFile{
		AgentID:  "agent-1",
		Filename: "IDENTITY.md",
		Content:  "Second",
	}
	err := store.Create(file2)
	if err == nil {
		t.Error("Expected error for duplicate agent_id/filename combination")
	}
}
