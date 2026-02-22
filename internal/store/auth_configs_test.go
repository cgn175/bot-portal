package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zeroclaw/bot-portal/internal/crypto"
	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/google/uuid"
)

func setupAuthConfigTestDB(t *testing.T) *sql.DB {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory database: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func TestAuthConfigStore_CreateAndGet(t *testing.T) {
	db := setupAuthConfigTestDB(t)
	defer db.Close()

	store := NewAuthConfigStore(db)

	// Test creating an auth config
	authConfig := &models.AuthConfig{
		ID:          uuid.New().String(),
		Name:        "Test OpenAI Config",
		Provider:    "openai",
		AuthType:    "bearer_token",
		Credentials: `{"api_key": "sk-test-key-123", "organization": "org-456"}`,
		EndpointURL: "https://api.openai.com",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create the auth config
	err := store.Create(authConfig)
	if err != nil {
		t.Fatalf("failed to create auth config: %v", err)
	}

	// Get the auth config by ID
	retrieved, err := store.GetByID(authConfig.ID)
	if err != nil {
		t.Fatalf("failed to get auth config: %v", err)
	}

	// Verify the retrieved data matches original (except credentials should be decrypted)
	if retrieved.ID != authConfig.ID {
		t.Errorf("expected ID %s, got %s", authConfig.ID, retrieved.ID)
	}
	if retrieved.Name != authConfig.Name {
		t.Errorf("expected name %s, got %s", authConfig.Name, retrieved.Name)
	}
	if retrieved.Provider != authConfig.Provider {
		t.Errorf("expected provider %s, got %s", authConfig.Provider, retrieved.Provider)
	}
	if retrieved.AuthType != authConfig.AuthType {
		t.Errorf("expected auth type %s, got %s", authConfig.AuthType, retrieved.AuthType)
	}
	if retrieved.EndpointURL != authConfig.EndpointURL {
		t.Errorf("expected endpoint URL %s, got %s", authConfig.EndpointURL, retrieved.EndpointURL)
	}

	// Verify credentials are properly decrypted by comparing the parsed JSON content
	var originalCreds, retrievedCreds map[string]string

	err = json.Unmarshal([]byte(authConfig.Credentials), &originalCreds)
	if err != nil {
		t.Fatalf("original credentials are not valid JSON: %v", err)
	}

	err = json.Unmarshal([]byte(retrieved.Credentials), &retrievedCreds)
	if err != nil {
		t.Fatalf("retrieved credentials are not valid JSON: %v", err)
	}

	// Compare credential contents
	for key, value := range originalCreds {
		if retrievedCreds[key] != value {
			t.Errorf("credential mismatch for key %s: expected %s, got %s", key, value, retrievedCreds[key])
		}
	}


	// Verify that credentials are actually encrypted in the database
	var storedCredentials string
	err = db.QueryRow("SELECT credentials FROM auth_configs WHERE id = ?", authConfig.ID).Scan(&storedCredentials)
	if err != nil {
		t.Fatalf("failed to query stored credentials: %v", err)
	}

	// The stored credentials should be encrypted (different from original JSON format)
	if storedCredentials == authConfig.Credentials {
		t.Error("credentials appear to be stored unencrypted")
	}

	// Verify we can decrypt the stored credentials and they match the original data
	decryptedCreds, err := crypto.DecryptCredentials(storedCredentials)
	if err != nil {
		t.Fatalf("failed to decrypt stored credentials: %v", err)
	}

	for key, value := range originalCreds {
		if decryptedCreds[key] != value {
			t.Errorf("decrypted credential mismatch for key %s: expected %s, got %s", key, value, decryptedCreds[key])
		}
	}
}

func TestAuthConfigStore_List(t *testing.T) {
	db := setupAuthConfigTestDB(t)
	defer db.Close()

	store := NewAuthConfigStore(db)

	// Create multiple auth configs
	configs := []*models.AuthConfig{
		{
			ID:          uuid.New().String(),
			Name:        "OpenAI Config",
			Provider:    "openai",
			AuthType:    "bearer_token",
			Credentials: `{"api_key": "sk-openai-key"}`,
		},
		{
			ID:          uuid.New().String(),
			Name:        "Anthropic Config",
			Provider:    "anthropic",
			AuthType:    "bearer_token",
			Credentials: `{"api_key": "sk-ant-key"}`,
		},
	}

	for _, config := range configs {
		err := store.Create(config)
		if err != nil {
			t.Fatalf("failed to create auth config: %v", err)
		}
	}

	// Test List method
	retrieved, err := store.List()
	if err != nil {
		t.Fatalf("failed to list auth configs: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("expected 2 auth configs, got %d", len(retrieved))
	}

	// Test ListMasked method
	masked, err := store.ListMasked()
	if err != nil {
		t.Fatalf("failed to list masked auth configs: %v", err)
	}

	if len(masked) != 2 {
		t.Errorf("expected 2 masked auth configs, got %d", len(masked))
	}

	// Verify credentials are masked
	for _, config := range masked {
		if !strings.Contains(config.Credentials, "****") {
			t.Error("expected credentials to be masked in ListMasked result")
		}
	}
}

func TestAuthConfigStore_Update(t *testing.T) {
	db := setupAuthConfigTestDB(t)
	defer db.Close()

	store := NewAuthConfigStore(db)

	// Create initial auth config
	authConfig := &models.AuthConfig{
		ID:          uuid.New().String(),
		Name:        "Test Config",
		Provider:    "openai",
		AuthType:    "bearer_token",
		Credentials: `{"api_key": "old-key"}`,
		EndpointURL: "https://api.openai.com",
	}

	err := store.Create(authConfig)
	if err != nil {
		t.Fatalf("failed to create auth config: %v", err)
	}

	// Update the auth config
	authConfig.Name = "Updated Config"
	authConfig.Credentials = `{"api_key": "new-key"}`
	authConfig.UpdatedAt = time.Now()

	err = store.Update(authConfig)
	if err != nil {
		t.Fatalf("failed to update auth config: %v", err)
	}

	// Retrieve and verify the update
	retrieved, err := store.GetByID(authConfig.ID)
	if err != nil {
		t.Fatalf("failed to get updated auth config: %v", err)
	}

	if retrieved.Name != "Updated Config" {
		t.Errorf("expected name 'Updated Config', got %s", retrieved.Name)
	}

	var creds map[string]string
	err = json.Unmarshal([]byte(retrieved.Credentials), &creds)
	if err != nil {
		t.Fatalf("failed to parse credentials: %v", err)
	}

	if creds["api_key"] != "new-key" {
		t.Errorf("expected api_key 'new-key', got %s", creds["api_key"])
	}
}

func TestAuthConfigStore_Delete(t *testing.T) {
	db := setupAuthConfigTestDB(t)
	defer db.Close()

	store := NewAuthConfigStore(db)

	// Create an auth config
	authConfig := &models.AuthConfig{
		ID:          uuid.New().String(),
		Name:        "Test Config",
		Provider:    "openai",
		AuthType:    "bearer_token",
		Credentials: `{"api_key": "test-key"}`,
	}

	err := store.Create(authConfig)
	if err != nil {
		t.Fatalf("failed to create auth config: %v", err)
	}

	// Delete the auth config
	err = store.Delete(authConfig.ID)
	if err != nil {
		t.Fatalf("failed to delete auth config: %v", err)
	}

	// Verify it's deleted
	_, err = store.GetByID(authConfig.ID)
	if err == nil {
		t.Error("expected error when getting deleted auth config")
	}
}