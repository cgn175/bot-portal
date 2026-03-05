package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentsHandler "github.com/zeroclaw/bot-portal/internal/api/agents"
	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
	_ "modernc.org/sqlite"
)

func setupIdentityFilesTestDB(t *testing.T) (*sql.DB, *store.AgentStore) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db, store.NewAgentStore(db)
}

func TestHandleAgentIdentityFiles_List(t *testing.T) {
	db, agentStore := setupIdentityFilesTestDB(t)
	defer db.Close()

	// Insert a test agent using store
	agent := &store.Agent{
		ID:     "test-agent",
		Name:   "Test Agent",
		Image:  "test:latest",
		Status: "stopped",
	}
	if err := agentStore.Create(agent); err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	// Insert some identity files
	identityStore := store.NewIdentityFileStore(db)
	files := []*models.AgentIdentityFile{
		{AgentID: "test-agent", Filename: "IDENTITY.md", Content: "Identity content"},
		{AgentID: "test-agent", Filename: "SOUL.md", Content: "Soul content"},
	}
	for _, f := range files {
		if err := identityStore.Create(f); err != nil {
			t.Fatalf("Failed to create identity file: %v", err)
		}
	}

	router := NewRouter(db, nil)

	// Test list endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/agents/test-agent/identity-files", nil)
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFiles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response agentsHandler.AgentIdentityFilesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(response.Files))
	}
}

func TestHandleAgentIdentityFiles_AgentNotFound(t *testing.T) {
	db, _ := setupIdentityFilesTestDB(t)
	defer db.Close()

	router := NewRouter(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/nonexistent/identity-files", nil)
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFiles(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleAgentIdentityFileDetail_Get(t *testing.T) {
	db, agentStore := setupIdentityFilesTestDB(t)
	defer db.Close()

	// Insert a test agent using store
	agent := &store.Agent{
		ID:     "test-agent",
		Name:   "Test Agent",
		Image:  "test:latest",
		Status: "stopped",
	}
	if err := agentStore.Create(agent); err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	// Insert an identity file
	identityStore := store.NewIdentityFileStore(db)
	file := &models.AgentIdentityFile{
		AgentID:  "test-agent",
		Filename: "IDENTITY.md",
		Content:  "This is the identity content",
	}
	if err := identityStore.Create(file); err != nil {
		t.Fatalf("Failed to create identity file: %v", err)
	}

	router := NewRouter(db, nil)

	// Test get endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/agents/test-agent/identity-files/IDENTITY.md", nil)
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFileDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["filename"] != "IDENTITY.md" {
		t.Errorf("Expected filename IDENTITY.md, got %s", response["filename"])
	}

	if response["content"] != "This is the identity content" {
		t.Errorf("Expected content 'This is the identity content', got %s", response["content"])
	}
}

func TestHandleAgentIdentityFileDetail_Update(t *testing.T) {
	db, agentStore := setupIdentityFilesTestDB(t)
	defer db.Close()

	// Insert a test agent using store
	agent := &store.Agent{
		ID:     "test-agent",
		Name:   "Test Agent",
		Image:  "test:latest",
		Status: "stopped",
	}
	if err := agentStore.Create(agent); err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	router := NewRouter(db, nil)

	// Test update endpoint
	request := agentsHandler.UpdateIdentityFileRequest{Content: "New content here"}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/test-agent/identity-files/SOUL.md", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFileDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	if response["filename"] != "SOUL.md" {
		t.Errorf("Expected filename SOUL.md, got %s", response["filename"])
	}

	// Verify the file was stored in database
	identityStore := store.NewIdentityFileStore(db)
	stored, err := identityStore.GetByAgentAndFilename("test-agent", "SOUL.md")
	if err != nil {
		t.Fatalf("Failed to get stored file: %v", err)
	}
	if stored == nil {
		t.Fatal("Expected file to be stored")
	}
	if stored.Content != "New content here" {
		t.Errorf("Expected content 'New content here', got %s", stored.Content)
	}
}

func TestHandleAgentIdentityFileDetail_UpdateSizeLimit(t *testing.T) {
	db, agentStore := setupIdentityFilesTestDB(t)
	defer db.Close()

	// Insert a test agent using store
	agent := &store.Agent{
		ID:     "test-agent",
		Name:   "Test Agent",
		Image:  "test:latest",
		Status: "stopped",
	}
	if err := agentStore.Create(agent); err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	router := NewRouter(db, nil)

	// Test with content exceeding 16KB
	largeContent := strings.Repeat("x", 16*1024+1)
	request := agentsHandler.UpdateIdentityFileRequest{Content: largeContent}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/test-agent/identity-files/SOUL.md", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFileDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for oversized content, got %d", w.Code)
	}
}

func TestHandleAgentIdentityFileDetail_MethodNotAllowed(t *testing.T) {
	db, agentStore := setupIdentityFilesTestDB(t)
	defer db.Close()

	// Insert a test agent using store
	agent := &store.Agent{
		ID:     "test-agent",
		Name:   "Test Agent",
		Image:  "test:latest",
		Status: "stopped",
	}
	if err := agentStore.Create(agent); err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}

	router := NewRouter(db, nil)

	// Test DELETE (not implemented)
	req := httptest.NewRequest(http.MethodDelete, "/api/agents/test-agent/identity-files/IDENTITY.md", nil)
	w := httptest.NewRecorder()

	router.agentHandler.HandleAgentIdentityFileDetail(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAgentIdentityFileDetail_InvalidPath(t *testing.T) {
	db, _ := setupIdentityFilesTestDB(t)
	defer db.Close()

	router := NewRouter(db, nil)

	tests := []struct {
		name string
		path string
	}{
		{"missing agent ID", "/api/agents//identity-files"},
		{"missing filename", "/api/agents/test-agent/identity-files/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			router.agentHandler.HandleAgentIdentityFileDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}
