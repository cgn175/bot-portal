# Bot Portal — Detailed Reference

## Store Layer Function Signatures

### `AgentStore` (`internal/store/agents.go`)
**Note:** Has its OWN `Agent` struct with `json.RawMessage` fields (different from `models.Agent`).
```go
NewAgentStore(db *sql.DB) *AgentStore
Create(agent *Agent) error
GetByID(id string) (*Agent, error)           // returns (nil, nil) if not found
List() ([]*Agent, error)
Update(agent *Agent) error
Delete(id string) error
UpdateStatus(id, status string) error
GenerateBearerToken(id string) (string, error) // 32 random bytes → 64 hex chars
```
Store Agent fields: ID, Name, Description, Image, AgentType, Status, ContainerID, Endpoint, ListenPort, BearerToken(`json:"-"`), AgentCard(RawMessage), Config(RawMessage), ModelID, AuthConfigID, PeerAgentIDs(RawMessage)

### `ModelStore` (`internal/store/models.go`)
```go
NewModelStore(db *sql.DB) *ModelStore
Create(model *models.Model) error
GetByID(id string) (*models.Model, error)     // returns (nil, nil) if not found
List() ([]models.Model, error)                // ORDER BY created_at DESC
Update(model *models.Model) error             // returns ErrNotFound
Delete(id string) error                       // returns ErrNotFound
DeleteByAuthConfigID(authConfigID string) error
DeleteAll() error
GetDefault() (*models.Model, error)           // is_default=1 or first model; nil if none
SetDefault(id string) error                   // transactional: clear all, set one
```

### `AuthConfigStore` (`internal/store/auth_configs.go`)
```go
NewAuthConfigStore(db *sql.DB) *AuthConfigStore
Create(config *models.AuthConfig) error       // auto-encrypts credentials
GetByID(id string) (*models.AuthConfig, error) // auto-decrypts; (nil,nil) if not found
List() ([]*models.AuthConfig, error)          // auto-decrypts
ListMasked() ([]*models.AuthConfig, error)    // returns {"key":"****"} values
Update(config *models.AuthConfig) error       // auto-encrypts; returns ErrNotFound
Delete(id string) error                       // returns ErrNotFound
```
`ErrNotFound` sentinel defined here.

### `ChannelStore` (`internal/store/channels.go`)
```go
NewChannelStore(db *sql.DB) *ChannelStore
Create(channel *models.Channel) error
GetByID(id string) (*models.Channel, error)
List() ([]*models.Channel, error)             // ordered by most recent task_log activity
Delete(id string) error
EnsureGeneralChannel() error                  // creates "general" if missing
```

### `MessageStore` (`internal/store/messages.go`)
```go
NewMessageStore(db *sql.DB) *MessageStore
Create(log *TaskLog) error
GetByID(id string) (*TaskLog, error)
ListByChannel(channelID string, limit int) ([]*TaskLog, error) // default limit 100
ListAll(limit int) ([]*TaskLog, error)
Update(log *TaskLog) error
Delete(id string) error
AppendMessage(id string, message a2a.TaskMessage) error
```

### `IdentityFileStore` (`internal/store/identity_files.go`)
```go
NewIdentityFileStore(db *sql.DB) *IdentityFileStore
GetByAgentAndFilename(agentID, filename string) (*models.AgentIdentityFile, error)
ListByAgent(agentID string) ([]*models.AgentIdentityFile, error)
Create(file *models.AgentIdentityFile) error
Update(file *models.AgentIdentityFile) error
CreateOrUpdate(file *models.AgentIdentityFile) error
Delete(agentID, filename string) error
DeleteAllForAgent(agentID string) error
```

---

## Data Model Structs (`internal/models/models.go`)

```go
type Agent struct {
    ID, Name, Description, Image, AgentType, Status, ContainerID, Endpoint string
    ListenPort int; BearerToken string `json:"-"`
    AgentCard *AgentCard; Config map[string]string
    ModelID, AuthConfigID *string
    CreatedAt, UpdatedAt time.Time
}

type Model struct {
    ID, Name, AuthConfigID, ModelIdentifier, EndpointURL, DefaultParams string
    IsDefault bool; CreatedAt, UpdatedAt time.Time
}

type AuthConfig struct {
    ID, Name, Provider, AuthType, Credentials, EndpointURL string
    CreatedAt, UpdatedAt time.Time
}

type Channel struct { ID string; Members []string; CreatedAt time.Time }
type TaskLog struct {
    ID, ChannelID, SenderID, RecipientID, Status, Direction string
    Messages, Artifacts json.RawMessage; CreatedAt, UpdatedAt time.Time
}
type AgentIdentityFile struct {
    ID int64; AgentID, Filename, Content string
    CharCount int; UpdatedAt time.Time
}
```

---

## Docker Manager Methods (`internal/docker/manager.go`)

```go
NewManager() (*Manager, error)                          // resolves Docker context (Desktop/Podman)
CreateContainer(ctx, ContainerConfig) (string, error)   // full setup with env, mounts, network
StartContainer(ctx, containerID string) error
StopContainer(ctx, containerID string) error            // 10s timeout
RestartContainer(ctx, containerID string) error         // 10s timeout
RemoveContainer(ctx, containerID string) error          // force
ContainerExists(ctx, containerID string) bool
GetContainerStatus(ctx, containerID string) (string, error)
GetContainerIP(ctx, containerID string) (string, error) // in bot-portal network
EnsureNetwork(ctx) error                                // creates "bot-portal" bridge
ContainerLogs(ctx, containerID, tail string) (string, error)
RawContainerLogs(ctx, containerID string, opts) (io.ReadCloser, error)
RegenerateConfig(config, gatewayPort string) error      // updates bind-mounted config.toml
ReadWorkspaceFile(ctx, agentID, filename string) ([]byte, error)  // Docker CP
WriteWorkspaceFile(ctx, agentID, filename string, content []byte) error  // Docker CP + chown
Close() error
```

### Container Config Types:
```go
type ContainerConfig struct {
    AgentID, AgentImage, PortalURL, PortalToken, AgentName, AgentDesc string
    ListenPort int; A2APeersJSON string
    ModelConfig *ModelConfig; AuthConfig *AuthConfig; SecureMode bool
}
type ModelConfig struct { Provider, Name, Endpoint string; Temperature *float64; MaxTokens *int }
type AuthConfig struct { Type, ApiKey, Endpoint string }
type A2APeer struct { ID, Endpoint, BearerToken string }
```

`ZEROCLAW_WORK_DIR = "/zeroclaw-data"`

---

## A2A Types (`internal/a2a/types.go`)

```go
type TaskStatus string  // "pending", "running", "completed", "failed", "cancelled"
type Task struct { ID string; Status TaskStatus; Messages []TaskMessage; Artifacts []Artifact; ... }
type TaskMessage struct { Role, Content string; Timestamp time.Time }
type TaskUpdate struct { TaskID string; Status TaskStatus; Message *TaskMessage; Artifact *Artifact }
type CreateTaskRequest struct { Message TaskMessage; Metadata json.RawMessage }
type CreateTaskResponse struct { Task struct { ID string } }

func ChannelID(senderID, recipientID string) string  // sorted "a::b"
func IsDirectChannel(channelID string) bool
```

---

## Provider Registry (`internal/provider/registry.go`)

```go
func Get(id string) (Provider, bool)                    // case-insensitive
func List() []Provider                                   // deduplicated
func IsValid(id string) bool
func GetAPIKeyEnvVar(providerID string) string
func GetDefaultURL(providerID string) string
func GetHeaders(providerID string) map[string]string
func ResolveProvider(id string) (Provider, bool)         // supports "custom:https://..."
func GetAuthHeader(providerID, apiKey string) (name, value string)

type AuthStyle string  // "bearer", "x-api-key", "custom"
type Provider struct {
    ID, Name, AuthType string; AuthStyle AuthStyle; AuthHeader string
    DefaultURL, APIKeyEnvVar, Description string
    Headers map[string]string; Aliases []string
    IsRegional bool; Region string
}
```

28 built-in providers: openai, anthropic, github_copilot, kimi, kimi-code, deepseek, glm, glm-cn, minimax, minimax-cn, qwen, qwen-intl, qwen-code, mistral, groq, xai, together, fireworks, perplexity, cohere, openrouter, ollama, lmstudio, gemini, cloudflare, qianfan, zai, custom.

---

## Encryption (`internal/crypto/encryption.go`)

```go
func EncryptCredentials(creds map[string]string) (string, error)  // JSON→AES-256-GCM→base64
func DecryptCredentials(encrypted string) (map[string]string, error)
func EncryptString(plaintext string) (string, error)
func DecryptString(encrypted string) (string, error)
```
Key: `ENCRYPTION_KEY` env var, must be exactly 32 bytes.

---

## Chat Proxy Types (`internal/api/chat.go`)

```go
type ChatRequest struct {
    Model string; Messages []ChatMessage; MaxTokens int
    Temperature, TopP *float64; Stream bool
    Tools, ToolChoice json.RawMessage
}
type ChatMessage struct {
    Role string; Content json.RawMessage; ToolCalls json.RawMessage
    ToolCallID, Name string
}
func (m ChatMessage) ContentString() string
func NewChatMessage(role, content string) ChatMessage
```

---

## Agent Handler Key Methods (`internal/api/agents/`)

### handler.go
```go
NewHandler(db, dockerMgr, agentStore, modelStore, authConfigStore, messageStore) *Handler
HandleAgents(w, req)              // GET→list, POST→create
HandleAgentDetail(w, req)         // dispatches by ?action= or HTTP method
StreamAgents(w, req)              // SSE, 3s interval
StreamContainerLogs(w, req)       // SSE container logs with rate limiting
DoStartAgent(agentID string) error
DoStopAgent(agentID string)
```

### lifecycle.go
```go
doStartAgent(agentID) error       // resolve model/auth, create container, start, inject AGENTS.md
doStopAgent(agentID)              // stop container, update status
handleAgentChat(w, req, agentID)  // creates A2A task, subscribes to SSE
```

### peers.go
```go
syncSymmetricPeers(agentID, old, new []string)    // bidirectional peer sync
regenerateAgentConfig(agent *store.Agent) error   // rebuild config.toml
injectDefaultAgentsMD(ctx, agent)                  // auto-generate AGENTS.md
```

---

## Copilot OAuth Flow (`internal/api/copilot_auth.go`)

```go
// Constants
GitHubCopilotClientID = "Iv1.b507a08c87ecfe98"
DefaultCopilotAPIURL  = "https://api.githubcopilot.com"

// Token management
ensureFreshCopilotToken(config *models.AuthConfig) (string, error)  // auto-refresh within 5min
refreshCopilotToken(config, accessToken) (string, error)

// OAuth helpers
requestGitHubDeviceCode() (*DeviceCodeResponse, error)
pollGitHubAccessToken(deviceCode string) (*AccessTokenResponse, error)
exchangeForCopilotAPIKey(accessToken string) (*CopilotAPIKeyResponse, error)
```

---

## Database Schema

### agents
```sql
id TEXT PK, name TEXT, description TEXT, image TEXT, agent_type TEXT DEFAULT 'docker',
status TEXT DEFAULT 'stopped', container_id TEXT, endpoint TEXT,
listen_port INTEGER DEFAULT 17000, bearer_token TEXT, agent_card TEXT, config TEXT,
model_id TEXT, auth_config_id TEXT, peer_agent_ids TEXT DEFAULT '[]',
created_at DATETIME, updated_at DATETIME
```

### models
```sql
id TEXT PK, name TEXT, auth_config_id TEXT, model_identifier TEXT,
endpoint_url TEXT, default_params TEXT, is_default INTEGER DEFAULT 0,
created_at DATETIME, updated_at DATETIME
```

### auth_configs
```sql
id TEXT PK, name TEXT, provider TEXT, auth_type TEXT,
credentials TEXT (AES-256-GCM encrypted), endpoint_url TEXT,
created_at DATETIME, updated_at DATETIME
```

### channels
```sql
id TEXT PK, members TEXT (JSON array), created_at DATETIME
```

### task_logs
```sql
id TEXT PK, channel_id TEXT, sender_id TEXT, recipient_id TEXT,
status TEXT, messages TEXT (JSON), artifacts TEXT (JSON), direction TEXT,
created_at DATETIME, updated_at DATETIME
-- Indexes: channel_id, sender_id, created_at
```

### agent_identity_files
```sql
id INTEGER PK AUTOINCREMENT, agent_id TEXT, filename TEXT,
content TEXT DEFAULT '', char_count INTEGER DEFAULT 0,
updated_at DATETIME, UNIQUE(agent_id, filename),
FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
-- Index: agent_id
```

---

## Frontend Components (`web/src/`)

### Pages
| File | Route | Purpose |
|------|-------|---------|
| `Dashboard.tsx` | `/` | Agent grid + create/start/stop/delete |
| `AgentDetail.tsx` | `/agents/:id` | Agent detail with actions |
| `IdentityFileEditor.tsx` | `/agents/:agentId/identity` | Tabbed editor for identity files |
| `Models.tsx` | `/models` | Model CRUD list |
| `AuthConfigs.tsx` | `/auth-configs` | Auth config list + Copilot OAuth |
| `AuthConfigForm.tsx` | *(modal)* | Auth config creation form |
| `TestChat.tsx` | `/test-chat` | Chat with configured models |
| `MessageViewer.tsx` | `/messages` | Task log list + detail |
| `ChannelView.tsx` | `/channels/:id` | Channel message view |

### Components
| File | Purpose |
|------|---------|
| `AgentCard.tsx` | Agent card in grid |
| `AgentChat.tsx` | Chat UI for agent communication |
| `AgentForm.tsx` | Agent create/edit form |
| `AgentGrid.tsx` | Grid layout for agents |
| `AgentLogs.tsx` | Agent log viewer |
| `Alert.tsx` | Alert/notification component |
| `CopilotOAuthModal.tsx` | GitHub Copilot OAuth device flow modal |
| `EmptyState.tsx` | Empty state placeholder |
| `ErrorBoundary.tsx` | React error boundary |
| `LiveLogViewer.tsx` | Real-time SSE log viewer |
| `LoadingState.tsx` | Loading spinner |
| `Modal.tsx` | Generic modal |
| `ModelForm.tsx` | Model create/edit form |
| `SearchableSelect.tsx` | Searchable dropdown |
| `StatusBadge.tsx` | Agent status badge |

### CopilotKit hooks (`copilot/`)
| File | Purpose |
|------|---------|
| `CopilotProvider.tsx` | Wraps CopilotKit, connects to sidecar at `VITE_COPILOT_RUNTIME_URL` |
| `CopilotActions.tsx` | Aggregates all action hooks |
| `useAgentActions.ts` | Agent CRUD actions |
| `useModelActions.ts` | Model operations |
| `useAuthConfigActions.ts` | Auth config operations |
| `useIdentityFileActions.ts` | Identity file operations |
| `useNavigationActions.ts` | Page navigation |
| `useAppContext.ts` | Readable context (agents, models, configs state) |

### Dependencies (package.json):
- `@copilotkit/react-core`, `@copilotkit/react-ui`, `@copilotkit/runtime` ^1.53.0
- `react` ^18.2.0, `react-router-dom` ^6.20.0
- `ansi-to-html` ^0.7.2
- Build: Vite 5, TypeScript 5.2

---

## Test Files

| Test | Location |
|------|----------|
| Auth config API | `internal/api/auth_configs_test.go` |
| Chat completions | `internal/api/chat_test.go` |
| Claude API | `internal/api/claude_test.go` |
| Copilot OAuth | `internal/api/copilot_auth_test.go` |
| Identity files API | `internal/api/identity_files_test.go` |
| Models API | `internal/api/models_test.go` |
| Agent logs | `internal/api/agents/logs_test.go` |
| Agent logs integration | `internal/api/agents/logs_integration_test.go` |
| A2A routing | `internal/a2a/router_test.go` |
| Agent store | `internal/store/agents_test.go` |
| Auth config store | `internal/store/auth_configs_test.go` |
| Channel store | `internal/store/channels_test.go` |
| Identity file store | `internal/store/identity_files_test.go` |
| Message store | `internal/store/messages_test.go` |
| Model store | `internal/store/models_test.go` |
| Docker manager | `internal/docker/manager_test.go` |
| Workspace files | `internal/docker/workspace_files_test.go` |
| Encryption | `internal/crypto/encryption_test.go` |
| Integration | `test/integration_test.go` |
| Claude integration | `test/claude_integration_test.go` |
| Shell tests | `test/test_smoke.sh`, `test_launch.sh`, `test_perms.sh`, `test_idempotency.sh`, `test_token_rotation.sh` |
