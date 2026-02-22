# Model & Authentication Central Store Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement centralized configuration store for AI models and authentication credentials in Bot Portal

**Architecture:** Add new database tables for models and auth configurations, extend agent model with foreign keys, create REST APIs for management, and update frontend with dropdowns. Uses AES-256 encryption for credential security and environment variable injection to agent containers.

**Tech Stack:** Go, SQLite, React, crypto/aes encryption

---

## Task 1: Database Schema & Models

**Files:**
- Modify: `internal/store/sqlite.go:25-86`
- Modify: `internal/models/models.go:9-24`
- Create: `internal/store/models.go`
- Create: `internal/store/auth_configs.go`

**Step 1: Add new database tables to migrations**

Modify `internal/store/sqlite.go` migration array:

```go
migrations := []string{
    // ... existing tables ...
    `CREATE TABLE IF NOT EXISTS models (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        provider TEXT NOT NULL,
        model_identifier TEXT NOT NULL,
        endpoint_url TEXT,
        default_params TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`,
    `CREATE TABLE IF NOT EXISTS auth_configs (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        provider TEXT NOT NULL,
        auth_type TEXT NOT NULL,
        credentials TEXT NOT NULL,
        endpoint_url TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`,
}
```

**Step 2: Add foreign keys to agents table**

Add to `alterMigrations` array in `sqlite.go`:

```go
{"agents", "model_id", "TEXT"},
{"agents", "auth_config_id", "TEXT"},
```

**Step 3: Add model structs to models.go**

Add after line 24 in `internal/models/models.go`:

```go
// Model represents an AI model configuration
type Model struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    Provider        string            `json:"provider"`
    ModelIdentifier string            `json:"modelIdentifier"`
    EndpointURL     string            `json:"endpointUrl"`
    DefaultParams   map[string]string `json:"defaultParams"`
    CreatedAt       time.Time         `json:"createdAt"`
    UpdatedAt       time.Time         `json:"updatedAt"`
}

// AuthConfig represents an authentication configuration
type AuthConfig struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Provider    string            `json:"provider"`
    AuthType    string            `json:"authType"`
    Credentials map[string]string `json:"credentials"`
    EndpointURL string            `json:"endpointUrl"`
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
}
```

**Step 4: Add foreign keys to Agent struct**

Modify Agent struct in `models.go` to add:

```go
ModelID      string `json:"modelId"`
AuthConfigID string `json:"authConfigId"`
```

**Step 5: Run migration test**

```bash
cd /Users/hoangta/projects/bot-portal
go run cmd/portal/main.go --migrate-only
```

**Step 6: Commit schema changes**

```bash
git add internal/store/sqlite.go internal/models/models.go
git commit -m "feat: add models and auth_configs database schema"
```

---

## Task 2: Encryption Utility

**Files:**
- Create: `internal/crypto/encryption.go`
- Create: `internal/crypto/encryption_test.go`

**Step 1: Write failing test**

Create `internal/crypto/encryption_test.go`:

```go
package crypto

import (
    "testing"
    "encoding/json"
)

func TestEncryptDecryptCredentials(t *testing.T) {
    creds := map[string]string{"api_key": "sk-test123"}

    encrypted, err := EncryptCredentials(creds)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    decrypted, err := DecryptCredentials(encrypted)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    if decrypted["api_key"] != "sk-test123" {
        t.Errorf("Expected sk-test123, got %s", decrypted["api_key"])
    }
}

func TestEncryptInvalidData(t *testing.T) {
    _, err := DecryptCredentials("invalid-data")
    if err == nil {
        t.Error("Expected error for invalid data")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/crypto/ -v
```
Expected: FAIL with "package not found" or "function not defined"

**Step 3: Write minimal implementation**

Create `internal/crypto/encryption.go`:

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "os"
)

// getKey returns the encryption key from environment or generates one
func getKey() []byte {
    key := os.Getenv("ENCRYPTION_KEY")
    if key == "" {
        // In production, this should come from secure key management
        key = "12345678901234567890123456789012" // 32 bytes for AES-256
    }
    return []byte(key)[:32] // Ensure 32 bytes
}

// EncryptCredentials encrypts a map of credentials
func EncryptCredentials(creds map[string]string) (string, error) {
    data, err := json.Marshal(creds)
    if err != nil {
        return "", err
    }

    key := getKey()
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, data, nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredentials decrypts credentials back to a map
func DecryptCredentials(encrypted string) (map[string]string, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return nil, err
    }

    key := getKey()
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }

    var creds map[string]string
    err = json.Unmarshal(plaintext, &creds)
    return creds, err
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/crypto/ -v
```
Expected: PASS

**Step 5: Commit encryption utilities**

```bash
git add internal/crypto/
git commit -m "feat: add AES-256 credential encryption utilities"
```

---

## Task 3: Model Store Operations

**Files:**
- Create: `internal/store/models.go`
- Create: `internal/store/models_test.go`

**Step 1: Write failing test**

Create `internal/store/models_test.go`:

```go
package store

import (
    "testing"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
)

func TestModelStore_CreateAndGet(t *testing.T) {
    db, err := NewSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    if err := RunMigrations(db); err != nil {
        t.Fatal(err)
    }

    store := NewModelStore(db)

    model := &models.Model{
        ID:              "gpt4",
        Name:            "GPT-4 Turbo",
        Provider:        "openai",
        ModelIdentifier: "gpt-4-turbo",
        EndpointURL:     "https://api.openai.com/v1",
        DefaultParams:   map[string]string{"temperature": "0.7"},
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }

    err = store.Create(model)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    retrieved, err := store.GetByID("gpt4")
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    if retrieved.Name != "GPT-4 Turbo" {
        t.Errorf("Expected GPT-4 Turbo, got %s", retrieved.Name)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/ -run TestModelStore -v
```
Expected: FAIL with "NewModelStore not defined"

**Step 3: Write minimal implementation**

Create `internal/store/models.go`:

```go
package store

import (
    "database/sql"
    "encoding/json"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
)

type ModelStore struct {
    db *sql.DB
}

func NewModelStore(db *sql.DB) *ModelStore {
    return &ModelStore{db: db}
}

func (s *ModelStore) Create(model *models.Model) error {
    paramsJSON, _ := json.Marshal(model.DefaultParams)

    _, err := s.db.Exec(`
        INSERT INTO models (id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        model.ID, model.Name, model.Provider, model.ModelIdentifier,
        model.EndpointURL, string(paramsJSON), model.CreatedAt, model.UpdatedAt)

    return err
}

func (s *ModelStore) GetByID(id string) (*models.Model, error) {
    var model models.Model
    var paramsJSON string

    err := s.db.QueryRow(`
        SELECT id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at
        FROM models WHERE id = ?`, id).Scan(
        &model.ID, &model.Name, &model.Provider, &model.ModelIdentifier,
        &model.EndpointURL, &paramsJSON, &model.CreatedAt, &model.UpdatedAt)

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    json.Unmarshal([]byte(paramsJSON), &model.DefaultParams)
    return &model, nil
}

func (s *ModelStore) List() ([]*models.Model, error) {
    rows, err := s.db.Query(`
        SELECT id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at
        FROM models ORDER BY created_at DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var models []*models.Model
    for rows.Next() {
        var model models.Model
        var paramsJSON string

        err := rows.Scan(&model.ID, &model.Name, &model.Provider, &model.ModelIdentifier,
            &model.EndpointURL, &paramsJSON, &model.CreatedAt, &model.UpdatedAt)
        if err != nil {
            return nil, err
        }

        json.Unmarshal([]byte(paramsJSON), &model.DefaultParams)
        models = append(models, &model)
    }

    return models, nil
}

func (s *ModelStore) Update(model *models.Model) error {
    paramsJSON, _ := json.Marshal(model.DefaultParams)
    model.UpdatedAt = time.Now()

    _, err := s.db.Exec(`
        UPDATE models SET name=?, provider=?, model_identifier=?, endpoint_url=?,
               default_params=?, updated_at=? WHERE id=?`,
        model.Name, model.Provider, model.ModelIdentifier, model.EndpointURL,
        string(paramsJSON), model.UpdatedAt, model.ID)

    return err
}

func (s *ModelStore) Delete(id string) error {
    _, err := s.db.Exec("DELETE FROM models WHERE id=?", id)
    return err
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/ -run TestModelStore -v
```
Expected: PASS

**Step 5: Commit model store**

```bash
git add internal/store/models.go internal/store/models_test.go
git commit -m "feat: implement model store CRUD operations"
```

---

## Task 4: Auth Config Store Operations

**Files:**
- Create: `internal/store/auth_configs.go`
- Create: `internal/store/auth_configs_test.go`

**Step 1: Write failing test**

Create `internal/store/auth_configs_test.go`:

```go
package store

import (
    "testing"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
)

func TestAuthConfigStore_CreateAndGet(t *testing.T) {
    db, err := NewSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    if err := RunMigrations(db); err != nil {
        t.Fatal(err)
    }

    store := NewAuthConfigStore(db)

    config := &models.AuthConfig{
        ID:          "openai-prod",
        Name:        "OpenAI Production",
        Provider:    "openai",
        AuthType:    "api_key",
        Credentials: map[string]string{"api_key": "sk-test123"},
        EndpointURL: "https://api.openai.com/v1",
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    err = store.Create(config)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    retrieved, err := store.GetByID("openai-prod")
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    if retrieved.Name != "OpenAI Production" {
        t.Errorf("Expected OpenAI Production, got %s", retrieved.Name)
    }

    if retrieved.Credentials["api_key"] != "sk-test123" {
        t.Errorf("Expected decrypted credentials")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/ -run TestAuthConfigStore -v
```
Expected: FAIL with "NewAuthConfigStore not defined"

**Step 3: Write minimal implementation**

Create `internal/store/auth_configs.go`:

```go
package store

import (
    "database/sql"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
    "github.com/zeroclaw/bot-portal/internal/crypto"
)

type AuthConfigStore struct {
    db *sql.DB
}

func NewAuthConfigStore(db *sql.DB) *AuthConfigStore {
    return &AuthConfigStore{db: db}
}

func (s *AuthConfigStore) Create(config *models.AuthConfig) error {
    encryptedCreds, err := crypto.EncryptCredentials(config.Credentials)
    if err != nil {
        return err
    }

    _, err = s.db.Exec(`
        INSERT INTO auth_configs (id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        config.ID, config.Name, config.Provider, config.AuthType,
        encryptedCreds, config.EndpointURL, config.CreatedAt, config.UpdatedAt)

    return err
}

func (s *AuthConfigStore) GetByID(id string) (*models.AuthConfig, error) {
    var config models.AuthConfig
    var encryptedCreds string

    err := s.db.QueryRow(`
        SELECT id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at
        FROM auth_configs WHERE id = ?`, id).Scan(
        &config.ID, &config.Name, &config.Provider, &config.AuthType,
        &encryptedCreds, &config.EndpointURL, &config.CreatedAt, &config.UpdatedAt)

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    config.Credentials, err = crypto.DecryptCredentials(encryptedCreds)
    if err != nil {
        return nil, err
    }

    return &config, nil
}

func (s *AuthConfigStore) List() ([]*models.AuthConfig, error) {
    rows, err := s.db.Query(`
        SELECT id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at
        FROM auth_configs ORDER BY created_at DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var configs []*models.AuthConfig
    for rows.Next() {
        var config models.AuthConfig
        var encryptedCreds string

        err := rows.Scan(&config.ID, &config.Name, &config.Provider, &config.AuthType,
            &encryptedCreds, &config.EndpointURL, &config.CreatedAt, &config.UpdatedAt)
        if err != nil {
            return nil, err
        }

        config.Credentials, err = crypto.DecryptCredentials(encryptedCreds)
        if err != nil {
            return nil, err
        }

        configs = append(configs, &config)
    }

    return configs, nil
}

func (s *AuthConfigStore) ListMasked() ([]*models.AuthConfig, error) {
    configs, err := s.List()
    if err != nil {
        return nil, err
    }

    // Mask credentials for API responses
    for _, config := range configs {
        masked := make(map[string]string)
        for key := range config.Credentials {
            masked[key] = "***masked***"
        }
        config.Credentials = masked
    }

    return configs, nil
}

func (s *AuthConfigStore) Update(config *models.AuthConfig) error {
    encryptedCreds, err := crypto.EncryptCredentials(config.Credentials)
    if err != nil {
        return err
    }

    config.UpdatedAt = time.Now()

    _, err = s.db.Exec(`
        UPDATE auth_configs SET name=?, provider=?, auth_type=?, credentials=?,
               endpoint_url=?, updated_at=? WHERE id=?`,
        config.Name, config.Provider, config.AuthType, encryptedCreds,
        config.EndpointURL, config.UpdatedAt, config.ID)

    return err
}

func (s *AuthConfigStore) Delete(id string) error {
    _, err := s.db.Exec("DELETE FROM auth_configs WHERE id=?", id)
    return err
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/ -run TestAuthConfigStore -v
```
Expected: PASS

**Step 5: Commit auth config store**

```bash
git add internal/store/auth_configs.go internal/store/auth_configs_test.go
git commit -m "feat: implement auth config store with encryption"
```

---

## Task 5: Update Agent Store for Foreign Keys

**Files:**
- Modify: `internal/store/agents.go:55-85` (Create method)
- Modify: `internal/store/agents.go:105-125` (Update method)

**Step 1: Write failing test for agent with model/auth IDs**

Add to `internal/store/agents_test.go`:

```go
func TestAgentStore_WithModelAndAuth(t *testing.T) {
    // ... setup db and store ...

    agent := &Agent{
        ID:           "test-agent",
        Name:         "Test Agent",
        Image:        "test:latest",
        ModelID:      "gpt4",
        AuthConfigID: "openai-prod",
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    err := store.Create(agent)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    retrieved, err := store.GetByID("test-agent")
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }

    if retrieved.ModelID != "gpt4" {
        t.Errorf("Expected gpt4, got %s", retrieved.ModelID)
    }

    if retrieved.AuthConfigID != "openai-prod" {
        t.Errorf("Expected openai-prod, got %s", retrieved.AuthConfigID)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/ -run TestAgentStore_WithModelAndAuth -v
```
Expected: FAIL with "ModelID field not found" or similar

**Step 3: Update Agent type in agents.go**

Add ModelID and AuthConfigID fields to Agent struct:

```go
type Agent struct {
    // ... existing fields ...
    ModelID      string `json:"modelId"`
    AuthConfigID string `json:"authConfigId"`
    // ... remaining fields ...
}
```

**Step 4: Update Create method in agents.go**

Modify the INSERT query:

```go
func (s *AgentStore) Create(agent *Agent) error {
    _, err := s.db.Exec(`
        INSERT INTO agents (id, name, description, image, agent_type, endpoint, listen_port,
                           bearer_token, agent_card, config, model_id, auth_config_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        agent.ID, agent.Name, agent.Description, agent.Image, agent.AgentType,
        agent.Endpoint, agent.ListenPort, agent.BearerToken, agentCardJSON, configJSON,
        agent.ModelID, agent.AuthConfigID, agent.CreatedAt, agent.UpdatedAt)
    return err
}
```

**Step 5: Update scanning in GetByID method**

Modify the SELECT and Scan:

```go
func (s *AgentStore) GetByID(id string) (*Agent, error) {
    var agent Agent
    var agentCardJSON, configJSON string

    err := s.db.QueryRow(`
        SELECT id, name, description, image, agent_type, status, container_id, endpoint, listen_port,
               bearer_token, agent_card, config, model_id, auth_config_id, created_at, updated_at
        FROM agents WHERE id = ?`, id).Scan(
        &agent.ID, &agent.Name, &agent.Description, &agent.Image, &agent.AgentType, &agent.Status,
        &agent.ContainerID, &agent.Endpoint, &agent.ListenPort, &agent.BearerToken,
        &agentCardJSON, &configJSON, &agent.ModelID, &agent.AuthConfigID,
        &agent.CreatedAt, &agent.UpdatedAt)

    // ... rest of method unchanged ...
}
```

**Step 6: Run test to verify it passes**

```bash
go test ./internal/store/ -run TestAgentStore_WithModelAndAuth -v
```
Expected: PASS

**Step 7: Commit agent store updates**

```bash
git add internal/store/agents.go internal/store/agents_test.go
git commit -m "feat: add model and auth config foreign keys to agents"
```

---

## Task 6: Models API Endpoints

**Files:**
- Modify: `internal/api/router.go:74-86` (add routes)
- Create: `internal/api/models.go`

**Step 1: Write failing test**

Create `internal/api/models_test.go`:

```go
package api

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/zeroclaw/bot-portal/internal/store"
)

func TestModelsAPI(t *testing.T) {
    db, err := store.NewSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    if err := store.RunMigrations(db); err != nil {
        t.Fatal(err)
    }

    router := NewRouter(db, nil)

    // Test create model
    model := map[string]interface{}{
        "id":              "gpt4",
        "name":            "GPT-4 Turbo",
        "provider":        "openai",
        "modelIdentifier": "gpt-4-turbo",
        "endpointUrl":     "https://api.openai.com/v1",
    }

    body, _ := json.Marshal(model)
    req := httptest.NewRequest("POST", "/api/models", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.handleModels(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("Expected 201, got %d", w.Code)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -run TestModelsAPI -v
```
Expected: FAIL with "handleModels not defined"

**Step 3: Add routes to router.go**

Add after line 86 in `router.go`:

```go
// Model management
mux.HandleFunc("/api/models", r.handleModels)
mux.HandleFunc("/api/models/", r.handleModelDetail)

// Auth config management
mux.HandleFunc("/api/auth-configs", r.handleAuthConfigs)
mux.HandleFunc("/api/auth-configs/", r.handleAuthConfigDetail)
```

**Step 4: Add ModelStore and AuthConfigStore to Router struct**

Modify Router struct around line 22:

```go
type Router struct {
    db              *sql.DB
    dockerMgr       *docker.Manager
    agentStore      *store.AgentStore
    channelStore    *store.ChannelStore
    messageStore    *store.MessageStore
    modelStore      *store.ModelStore
    authConfigStore *store.AuthConfigStore
    a2aRouter       *a2a.Router
}
```

**Step 5: Initialize stores in NewRouter**

Add after line 35 in `router.go`:

```go
modelStore := store.NewModelStore(db)
authConfigStore := store.NewAuthConfigStore(db)
```

And add to router initialization:

```go
router := &Router{
    db:              db,
    dockerMgr:       dockerMgr,
    agentStore:      agentStore,
    channelStore:    channelStore,
    messageStore:    messageStore,
    modelStore:      modelStore,
    authConfigStore: authConfigStore,
}
```

**Step 6: Create models handlers**

Create `internal/api/models.go`:

```go
package api

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
)

func (r *Router) handleModels(w http.ResponseWriter, req *http.Request) {
    switch req.Method {
    case http.MethodGet:
        r.listModels(w, req)
    case http.MethodPost:
        r.createModel(w, req)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (r *Router) handleModelDetail(w http.ResponseWriter, req *http.Request) {
    path := req.URL.Path
    if path == "/api/models/" {
        http.Error(w, "Model ID required", http.StatusBadRequest)
        return
    }

    modelID := path[len("/api/models/"):]

    switch req.Method {
    case http.MethodGet:
        r.getModel(w, req, modelID)
    case http.MethodPut:
        r.updateModel(w, req, modelID)
    case http.MethodDelete:
        r.deleteModel(w, req, modelID)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (r *Router) listModels(w http.ResponseWriter, req *http.Request) {
    models, err := r.modelStore.List()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(models)
}

func (r *Router) createModel(w http.ResponseWriter, req *http.Request) {
    var model models.Model

    if err := json.NewDecoder(req.Body).Decode(&model); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    model.CreatedAt = time.Now()
    model.UpdatedAt = time.Now()

    if err := r.modelStore.Create(&model); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(&model)
}

func (r *Router) getModel(w http.ResponseWriter, req *http.Request, modelID string) {
    model, err := r.modelStore.GetByID(modelID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if model == nil {
        http.Error(w, "Model not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(model)
}

func (r *Router) updateModel(w http.ResponseWriter, req *http.Request, modelID string) {
    model, err := r.modelStore.GetByID(modelID)
    if err != nil || model == nil {
        http.Error(w, "Model not found", http.StatusNotFound)
        return
    }

    var updates map[string]interface{}
    if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Apply updates
    if name, ok := updates["name"].(string); ok {
        model.Name = name
    }
    if provider, ok := updates["provider"].(string); ok {
        model.Provider = provider
    }
    if modelId, ok := updates["modelIdentifier"].(string); ok {
        model.ModelIdentifier = modelId
    }
    if endpoint, ok := updates["endpointUrl"].(string); ok {
        model.EndpointURL = endpoint
    }
    if params, ok := updates["defaultParams"].(map[string]interface{}); ok {
        stringParams := make(map[string]string)
        for k, v := range params {
            if str, ok := v.(string); ok {
                stringParams[k] = str
            }
        }
        model.DefaultParams = stringParams
    }

    if err := r.modelStore.Update(model); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(model)
}

func (r *Router) deleteModel(w http.ResponseWriter, req *http.Request, modelID string) {
    if err := r.modelStore.Delete(modelID); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

**Step 7: Run test to verify it passes**

```bash
go test ./internal/api/ -run TestModelsAPI -v
```
Expected: PASS

**Step 8: Commit models API**

```bash
git add internal/api/router.go internal/api/models.go internal/api/models_test.go
git commit -m "feat: implement models management REST API"
```

---

## Task 7: Auth Configs API Endpoints

**Files:**
- Create: `internal/api/auth_configs.go`
- Create: `internal/api/auth_configs_test.go`

**Step 1: Write failing test**

Create `internal/api/auth_configs_test.go`:

```go
package api

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/zeroclaw/bot-portal/internal/store"
)

func TestAuthConfigsAPI(t *testing.T) {
    db, err := store.NewSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    if err := store.RunMigrations(db); err != nil {
        t.Fatal(err)
    }

    router := NewRouter(db, nil)

    // Test create auth config
    config := map[string]interface{}{
        "id":          "openai-prod",
        "name":        "OpenAI Production",
        "provider":    "openai",
        "authType":    "api_key",
        "credentials": map[string]string{"api_key": "sk-test123"},
        "endpointUrl": "https://api.openai.com/v1",
    }

    body, _ := json.Marshal(config)
    req := httptest.NewRequest("POST", "/api/auth-configs", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.handleAuthConfigs(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("Expected 201, got %d", w.Code)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -run TestAuthConfigsAPI -v
```
Expected: FAIL with "handleAuthConfigs not defined"

**Step 3: Create auth configs handlers**

Create `internal/api/auth_configs.go`:

```go
package api

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/zeroclaw/bot-portal/internal/models"
)

func (r *Router) handleAuthConfigs(w http.ResponseWriter, req *http.Request) {
    switch req.Method {
    case http.MethodGet:
        r.listAuthConfigs(w, req)
    case http.MethodPost:
        r.createAuthConfig(w, req)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (r *Router) handleAuthConfigDetail(w http.ResponseWriter, req *http.Request) {
    path := req.URL.Path
    if path == "/api/auth-configs/" {
        http.Error(w, "Auth config ID required", http.StatusBadRequest)
        return
    }

    configID := path[len("/api/auth-configs/"):]

    switch req.Method {
    case http.MethodGet:
        r.getAuthConfig(w, req, configID)
    case http.MethodPut:
        r.updateAuthConfig(w, req, configID)
    case http.MethodDelete:
        r.deleteAuthConfig(w, req, configID)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (r *Router) listAuthConfigs(w http.ResponseWriter, req *http.Request) {
    // Return masked credentials for security
    configs, err := r.authConfigStore.ListMasked()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(configs)
}

func (r *Router) createAuthConfig(w http.ResponseWriter, req *http.Request) {
    var config models.AuthConfig

    if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    config.CreatedAt = time.Now()
    config.UpdatedAt = time.Now()

    if err := r.authConfigStore.Create(&config); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Mask credentials in response
    masked := make(map[string]string)
    for key := range config.Credentials {
        masked[key] = "***masked***"
    }
    config.Credentials = masked

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(&config)
}

func (r *Router) getAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
    config, err := r.authConfigStore.GetByID(configID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if config == nil {
        http.Error(w, "Auth config not found", http.StatusNotFound)
        return
    }

    // Mask credentials for security
    masked := make(map[string]string)
    for key := range config.Credentials {
        masked[key] = "***masked***"
    }
    config.Credentials = masked

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(config)
}

func (r *Router) updateAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
    config, err := r.authConfigStore.GetByID(configID)
    if err != nil || config == nil {
        http.Error(w, "Auth config not found", http.StatusNotFound)
        return
    }

    var updates map[string]interface{}
    if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Apply updates
    if name, ok := updates["name"].(string); ok {
        config.Name = name
    }
    if provider, ok := updates["provider"].(string); ok {
        config.Provider = provider
    }
    if authType, ok := updates["authType"].(string); ok {
        config.AuthType = authType
    }
    if endpoint, ok := updates["endpointUrl"].(string); ok {
        config.EndpointURL = endpoint
    }
    if creds, ok := updates["credentials"].(map[string]interface{}); ok {
        stringCreds := make(map[string]string)
        for k, v := range creds {
            if str, ok := v.(string); ok {
                stringCreds[k] = str
            }
        }
        config.Credentials = stringCreds
    }

    if err := r.authConfigStore.Update(config); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Mask credentials in response
    masked := make(map[string]string)
    for key := range config.Credentials {
        masked[key] = "***masked***"
    }
    config.Credentials = masked

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(config)
}

func (r *Router) deleteAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
    if err := r.authConfigStore.Delete(configID); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/api/ -run TestAuthConfigsAPI -v
```
Expected: PASS

**Step 5: Commit auth configs API**

```bash
git add internal/api/auth_configs.go internal/api/auth_configs_test.go
git commit -m "feat: implement auth configs management REST API"
```

---

## Task 8: Environment Variable Injection

**Files:**
- Modify: `internal/docker/manager.go:45-75` (CreateContainer method)
- Modify: `internal/api/router.go:450-465` (startAgent method)

**Step 1: Write failing test for environment injection**

Add to existing Docker manager test:

```go
func TestContainerConfig_WithModelAndAuth(t *testing.T) {
    config := ContainerConfig{
        AgentID:     "test-agent",
        AgentImage:  "test:latest",
        PortalURL:   "http://portal:8080",
        PortalToken: "token123",
        ListenPort:  9000,
        ModelConfig: &ModelConfig{
            Provider:        "openai",
            Name:           "gpt-4-turbo",
            Endpoint:       "https://api.openai.com/v1",
            Temperature:    "0.7",
            MaxTokens:      "4096",
        },
        AuthConfig: &AuthConfig{
            Type:     "api_key",
            ApiKey:   "sk-test123",
            Endpoint: "https://api.openai.com/v1",
        },
    }

    envVars := buildEnvironmentVars(config)

    expectedVars := []string{
        "AGENT_ID=test-agent",
        "PORTAL_URL=http://portal:8080",
        "PORTAL_BEARER_TOKEN=token123",
        "MODEL_PROVIDER=openai",
        "MODEL_NAME=gpt-4-turbo",
        "MODEL_ENDPOINT=https://api.openai.com/v1",
        "MODEL_TEMPERATURE=0.7",
        "MODEL_MAX_TOKENS=4096",
        "AUTH_TYPE=api_key",
        "API_KEY=sk-test123",
        "AUTH_ENDPOINT=https://api.openai.com/v1",
    }

    for _, expected := range expectedVars {
        found := false
        for _, actual := range envVars {
            if actual == expected {
                found = true
                break
            }
        }
        if !found {
            t.Errorf("Expected environment variable %s not found", expected)
        }
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/docker/ -run TestContainerConfig_WithModelAndAuth -v
```
Expected: FAIL with missing types/functions

**Step 3: Add ModelConfig and AuthConfig types**

Add to `internal/docker/manager.go`:

```go
type ModelConfig struct {
    Provider    string
    Name        string
    Endpoint    string
    Temperature string
    MaxTokens   string
}

type AuthConfig struct {
    Type     string
    ApiKey   string
    Endpoint string
}

type ContainerConfig struct {
    AgentID     string
    AgentImage  string
    PortalURL   string
    PortalToken string
    ListenPort  int
    ModelConfig *ModelConfig
    AuthConfig  *AuthConfig
}

func buildEnvironmentVars(config ContainerConfig) []string {
    env := []string{
        fmt.Sprintf("AGENT_ID=%s", config.AgentID),
        fmt.Sprintf("PORTAL_URL=%s", config.PortalURL),
        fmt.Sprintf("PORTAL_BEARER_TOKEN=%s", config.PortalToken),
    }

    if config.ModelConfig != nil {
        env = append(env,
            fmt.Sprintf("MODEL_PROVIDER=%s", config.ModelConfig.Provider),
            fmt.Sprintf("MODEL_NAME=%s", config.ModelConfig.Name),
            fmt.Sprintf("MODEL_ENDPOINT=%s", config.ModelConfig.Endpoint),
        )

        if config.ModelConfig.Temperature != "" {
            env = append(env, fmt.Sprintf("MODEL_TEMPERATURE=%s", config.ModelConfig.Temperature))
        }
        if config.ModelConfig.MaxTokens != "" {
            env = append(env, fmt.Sprintf("MODEL_MAX_TOKENS=%s", config.ModelConfig.MaxTokens))
        }
    }

    if config.AuthConfig != nil {
        env = append(env,
            fmt.Sprintf("AUTH_TYPE=%s", config.AuthConfig.Type),
            fmt.Sprintf("AUTH_ENDPOINT=%s", config.AuthConfig.Endpoint),
        )

        if config.AuthConfig.ApiKey != "" {
            env = append(env, fmt.Sprintf("API_KEY=%s", config.AuthConfig.ApiKey))
        }
    }

    return env
}
```

**Step 4: Update CreateContainer to use environment variables**

Modify CreateContainer method to use `buildEnvironmentVars`:

```go
func (m *Manager) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
    // ... existing code for image check ...

    env := buildEnvironmentVars(config)

    containerConfig := &container.Config{
        Image: config.AgentImage,
        Env:   env,
        ExposedPorts: nat.PortSet{
            nat.Port(fmt.Sprintf("%d/tcp", config.ListenPort)): {},
        },
    }

    // ... rest of method unchanged ...
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/docker/ -run TestContainerConfig_WithModelAndAuth -v
```
Expected: PASS

**Step 6: Update startAgent to fetch model and auth configs**

Modify `startAgent` method in `router.go`:

```go
func (r *Router) startAgent(w http.ResponseWriter, req *http.Request, agentID string) {
    agent, err := r.agentStore.GetByID(agentID)
    if err != nil || agent == nil {
        http.Error(w, "Agent not found", http.StatusNotFound)
        return
    }

    // ... existing native agent check ...

    ctx := req.Context()

    // Fetch model and auth configurations
    var modelConfig *docker.ModelConfig
    var authConfig *docker.AuthConfig

    if agent.ModelID != "" {
        model, err := r.modelStore.GetByID(agent.ModelID)
        if err == nil && model != nil {
            modelConfig = &docker.ModelConfig{
                Provider: model.Provider,
                Name:     model.ModelIdentifier,
                Endpoint: model.EndpointURL,
            }
            if temp, ok := model.DefaultParams["temperature"]; ok {
                modelConfig.Temperature = temp
            }
            if maxTokens, ok := model.DefaultParams["max_tokens"]; ok {
                modelConfig.MaxTokens = maxTokens
            }
        }
    }

    if agent.AuthConfigID != "" {
        auth, err := r.authConfigStore.GetByID(agent.AuthConfigID)
        if err == nil && auth != nil {
            authConfig = &docker.AuthConfig{
                Type:     auth.AuthType,
                Endpoint: auth.EndpointURL,
            }
            if apiKey, ok := auth.Credentials["api_key"]; ok {
                authConfig.ApiKey = apiKey
            }
        }
    }

    // ... existing listen port resolution ...

    // Create container if it doesn't exist
    if agent.ContainerID == "" {
        containerID, err := r.dockerMgr.CreateContainer(ctx, docker.ContainerConfig{
            AgentID:     agent.ID,
            AgentImage:  agent.Image,
            PortalURL:   fmt.Sprintf("http://localhost:%d", 8080),
            PortalToken: agent.BearerToken,
            ListenPort:  listenPort,
            ModelConfig: modelConfig,
            AuthConfig:  authConfig,
        })
        // ... rest unchanged ...
    }

    // ... rest of method unchanged ...
}
```

**Step 7: Commit environment injection**

```bash
git add internal/docker/manager.go internal/api/router.go
git commit -m "feat: inject model and auth config as environment variables"
```

---

Plan complete and saved to `docs/plans/2026-02-22-model-auth-central-store.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach would you prefer?