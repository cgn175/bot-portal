# A2A Round-Trip Communication Completion Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete end-to-end agent-to-agent communication by adding `allowed_domains` to config.toml, auto-injecting AGENTS.md/TOOLS.md with A2A instructions on agent creation, and adding a portal-mediated `/api/agents/{id}/send` endpoint.

**Architecture:** Three layers of changes:
1. **Config layer** — Add `[tools.http_request]` with `allowed_domains` to the config.toml template so agents can reach the portal and each other via `http_request` tool.
2. **Identity injection layer** — Auto-populate AGENTS.md and TOOLS.md with A2A communication instructions when an agent is created/started, so agents know how to talk to each other without manual setup.
3. **Portal API layer** — Add `POST /api/agents/{id}/send` endpoint to let the frontend (or any client) send a message to a specific agent via the A2A protocol, completing the UX loop.

**Tech Stack:** Go, Docker SDK, config.toml (TOML template), identity files (Markdown), SSE

---

## Task 1: Add `allowed_domains` to config.toml template

**Files:**
- Modify: `internal/docker/manager.go:261-288` (agentConfigTmpl)

**Step 1: Update the config.toml template**

Add a `[tools]` section after `[channels_config.a2a]` that includes `allowed_domains` with `host.docker.internal` and all peer agent hostnames. This allows the agent's `http_request` tool to reach the portal and peers.

```go
var agentConfigTmpl = template.Must(template.New("config").Parse(`workspace_dir = "{{ .ZeroClawWorkDir }}/workspace"
config_path = "{{ .ZeroClawWorkDir }}/.zeroclaw/config.toml"
{{ if .DefaultProvider }}default_provider = "{{ .DefaultProvider }}"
{{ end }}{{ if .DefaultModel }}default_model = "{{ .DefaultModel }}"
{{ end }}{{ if .ApiURL }}api_url = "{{ .ApiURL }}"
{{ end }}default_temperature = {{ .DefaultTemperature }}

[gateway]
port = {{ .GatewayPort }}
host = "[::]"
allow_public_bind = true

[channels_config]
cli = false

[channels_config.a2a]
enabled = true
listen_port = {{ .GatewayPort }}
discovery_mode = "static"
allowed_peer_ids = ["*"]
{{ range .Peers }}
[[channels_config.a2a.peers]]
id = "{{ .ID }}"
endpoint = "{{ .Endpoint }}"
bearer_token = "{{ .BearerToken }}"
enabled = true
{{ end }}
[tools.http_request]
allowed_domains = [{{ range $i, $d := .AllowedDomains }}{{ if $i }}, {{ end }}"{{ $d }}"{{ end }}]
`))
```

**Step 2: Update `generateAgentConfig` to populate `AllowedDomains`**

In `generateAgentConfig()` (manager.go:290-380), add the `AllowedDomains` field to the template data struct. Include:
- `host.docker.internal` (always — the portal)
- Each peer's hostname extracted from their endpoint

```go
// Build allowed domains for http_request tool
allowedDomains := []string{"host.docker.internal"}
for _, peer := range peers {
    if peer.Endpoint != "" {
        if u, err := url.Parse(peer.Endpoint); err == nil {
            allowedDomains = append(allowedDomains, u.Hostname())
        }
    }
}
```

Add `AllowedDomains []string` to the template data struct and pass it.

**Step 3: Add `net/url` import if not present**

Check if `net/url` is already imported in manager.go. If not, add it.

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/docker/manager.go
git commit -m "feat: add allowed_domains to agent config.toml template"
```

---

## Task 2: Auto-inject AGENTS.md with A2A instructions on agent start

**Files:**
- Modify: `internal/api/agents.go` (in `doStartAgent`, after container creation and start)
- Modify: `internal/docker/workspace_files.go` (no changes needed — already supports AGENTS.md)

**Step 1: Create a default AGENTS.md template function**

Add a function in `internal/api/agents.go` (or a new file `internal/api/defaults.go`) that generates the default AGENTS.md content with A2A instructions. The content should include:

```go
func defaultAgentsMarkdown(agentID, portalURL, bearerToken string, peers []peerInfo) string {
    var sb strings.Builder
    sb.WriteString("# Agent Communication Guide\n\n")
    sb.WriteString("## Your Identity\n\n")
    sb.WriteString(fmt.Sprintf("- **Agent ID:** `%s`\n", agentID))
    sb.WriteString(fmt.Sprintf("- **Portal URL:** `%s`\n\n", portalURL))
    sb.WriteString("## How to Send Messages to Other Agents\n\n")
    sb.WriteString("Use the `http_request` tool to POST to the portal's `/tasks` endpoint:\n\n")
    sb.WriteString("```\n")
    sb.WriteString(fmt.Sprintf("POST %s/tasks\n", portalURL))
    sb.WriteString("Headers:\n")
    sb.WriteString(fmt.Sprintf("  Authorization: Bearer %s\n", bearerToken))
    sb.WriteString("  Content-Type: application/json\n")
    sb.WriteString(fmt.Sprintf("  X-Agent-ID: %s\n", agentID))
    sb.WriteString("  X-Channel-ID: <your-id>::<recipient-id>\n\n")
    sb.WriteString("Body:\n")
    sb.WriteString("{\n")
    sb.WriteString("  \"message\": {\n")
    sb.WriteString("    \"role\": \"user\",\n")
    sb.WriteString("    \"content\": \"Your message here\"\n")
    sb.WriteString("  }\n")
    sb.WriteString("}\n")
    sb.WriteString("```\n\n")
    if len(peers) > 0 {
        sb.WriteString("## Available Peers\n\n")
        for _, p := range peers {
            sb.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", p.Name, p.ID))
        }
        sb.WriteString("\n")
    }
    sb.WriteString("## Important Notes\n\n")
    sb.WriteString("- Messages are delivered asynchronously. Responses arrive via your A2A channel.\n")
    sb.WriteString("- The portal routes messages between agents — you don't need to know peer endpoints.\n")
    sb.WriteString("- Your memory system preserves conversation context across async turns.\n")
    return sb.String()
}

type peerInfo struct {
    ID   string
    Name string
}
```

**Step 2: Inject AGENTS.md after container start**

In `doStartAgent()` (agents.go:372-530), after `r.dockerMgr.StartContainer(ctx, agent.ContainerID)` succeeds, check if the agent already has an AGENTS.md in the database. If not, generate one and write it:

```go
// After StartContainer succeeds and before the final return:

// Auto-inject AGENTS.md with A2A instructions if not already set
identityFileStore := store.NewIdentityFileStore(r.db)
existingFile, _ := identityFileStore.GetByAgentAndFilename(agentID, "AGENTS.md")
if existingFile == nil || existingFile.Content == "" {
    portalURL := os.Getenv("PORTAL_INTERNAL_URL")
    if portalURL == "" {
        portalURL = "http://host.docker.internal:8080"
    }

    var peerIDs []string
    if len(agent.PeerAgentIDs) > 0 {
        json.Unmarshal(agent.PeerAgentIDs, &peerIDs)
    }
    var peers []peerInfo
    for _, pid := range peerIDs {
        if pa, err := r.agentStore.GetByID(pid); err == nil && pa != nil {
            peers = append(peers, peerInfo{ID: pid, Name: pa.Name})
        }
    }

    content := defaultAgentsMarkdown(agentID, portalURL, agent.BearerToken, peers)
    file := &models.AgentIdentityFile{
        AgentID:  agentID,
        Filename: "AGENTS.md",
        Content:  content,
    }
    identityFileStore.CreateOrUpdate(file)

    // Sync to running container
    r.dockerMgr.WriteWorkspaceFile(ctx, agentID, "AGENTS.md", []byte(content))
}
```

**Step 3: Verify compilation**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/api/agents.go
git commit -m "feat: auto-inject AGENTS.md with A2A instructions on agent start"
```

---

## Task 3: Add portal-mediated send endpoint `POST /api/agents/{id}/send`

**Files:**
- Modify: `internal/api/agents.go` (add handler + route case)
- Modify: `internal/api/router.go` (add route registration)

**Step 1: Check existing router registration**

Read `internal/api/router.go` to understand how routes are registered. The `handleAgentDetail` function uses `req.URL.Query().Get("action")` for sub-routes.

**Step 2: Add "send" action case to `handleAgentDetail`**

Add a new case in `handleAgentDetail()` (agents.go:72-100):

```go
case "send":
    r.sendToAgent(w, req, agentID)
```

**Step 3: Implement `sendToAgent` handler**

This handler accepts a message, creates a task, forwards it to the agent via A2A, and returns the task ID. It reuses the existing `handleAgentChat` pattern but is simpler:

```go
// sendToAgent handles POST /api/agents/{id}?action=send
// Sends a message to a specific agent via the A2A protocol.
func (r *Router) sendToAgent(w http.ResponseWriter, req *http.Request, agentID string) {
    if req.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    agent, err := r.agentStore.GetByID(agentID)
    if err != nil || agent == nil {
        http.Error(w, "Agent not found", http.StatusNotFound)
        return
    }

    var sendReq struct {
        Message   string `json:"message"`
        ChannelID string `json:"channelId,omitempty"`
    }
    if err := json.NewDecoder(req.Body).Decode(&sendReq); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    if sendReq.Message == "" {
        http.Error(w, "Message cannot be empty", http.StatusBadRequest)
        return
    }

    channelID := sendReq.ChannelID
    if channelID == "" {
        channelID = a2a.ChannelID("portal", agentID)
    }

    // Create the task via the A2A protocol
    taskURL := fmt.Sprintf("%s/tasks", agent.Endpoint)
    createReq := a2a.CreateTaskRequest{
        Message: a2a.TaskMessage{
            Role:      "user",
            Content:   sendReq.Message,
            Timestamp: time.Now(),
        },
    }
    body, _ := json.Marshal(createReq)

    httpReq, _ := http.NewRequest("POST", taskURL, bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+agent.BearerToken)
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-Agent-ID", "portal")
    httpReq.Header.Set("X-Channel-ID", channelID)

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(httpReq)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to reach agent: %v", err), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        respBody, _ := io.ReadAll(resp.Body)
        http.Error(w, fmt.Sprintf("Agent error: %s", string(respBody)), resp.StatusCode)
        return
    }

    var agentResp struct {
        Task struct {
            ID string `json:"id"`
        } `json:"task"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil || agentResp.Task.ID == "" {
        http.Error(w, "Failed to parse agent response", http.StatusInternalServerError)
        return
    }

    taskID := agentResp.Task.ID

    // Save locally and subscribe to SSE
    r.createTaskWithID(taskID, channelID, "portal", agentID, createReq.Message)
    go r.subscribeToAgentSSE(agent, taskID)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "taskId":  taskID,
        "status":  "sent",
        "agentId": agentID,
    })
}
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/api/agents.go
git commit -m "feat: add POST /api/agents/{id}?action=send endpoint for portal-mediated A2A"
```

---

## Task 4: Update `regenerateAgentConfig` to include `AllowedDomains`

**Files:**
- Modify: `internal/api/agents.go:768-826` (`regenerateAgentConfig`)

**Step 1: Add AllowedDomains to regenerateAgentConfig**

When `regenerateAgentConfig` rebuilds config.toml for a running agent (e.g., after peer changes), it must also include the `AllowedDomains` field:

```go
func (r *Router) regenerateAgentConfig(agent *store.Agent) error {
    // ... existing peer resolution code ...

    // Build allowed domains
    allowedDomains := []string{"host.docker.internal"}
    for _, peer := range a2aPeers {
        if peer.Endpoint != "" {
            if u, err := url.Parse(peer.Endpoint); err == nil {
                allowedDomains = append(allowedDomains, u.Hostname())
            }
        }
    }

    // ... rest of function, adding AllowedDomains to containerConfig ...
}
```

Note: `ContainerConfig` already doesn't have an `AllowedDomains` field — we need to add it in Task 1's template data struct. Actually, looking more carefully, the template data struct is local to `generateAgentConfig`, not `ContainerConfig`. We need to add `AllowedDomains` to `ContainerConfig` and thread it through.

**Step 2: Add `AllowedDomains` field to `ContainerConfig`**

In `internal/docker/manager.go`:

```go
type ContainerConfig struct {
    // ... existing fields ...
    AllowedDomains []string
}
```

**Step 3: Pass AllowedDomains from both `doStartAgent` and `regenerateAgentConfig`**

In `doStartAgent` (agents.go:442-448), add:
```go
containerConfig.AllowedDomains = []string{"host.docker.internal"}
// Add peer hostnames
for _, peer := range a2aPeers {
    // peers don't have endpoints at this point (they're derived in generateAgentConfig)
    // so just add their IDs as Docker network aliases
    containerConfig.AllowedDomains = append(containerConfig.AllowedDomains, peer.ID)
}
```

In `regenerateAgentConfig` (agents.go:770-826), add the same logic.

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/docker/manager.go internal/api/agents.go
git commit -m "feat: thread AllowedDomains through ContainerConfig and regeneration"
```

---

## Task 5: Integration verification

**Step 1: Verify full build**

Run: `go build ./...`
Expected: SUCCESS

**Step 2: Run existing tests**

Run: `go test ./...`
Expected: All existing tests pass

**Step 3: Manual verification checklist**

- [ ] Config.toml template includes `[tools.http_request]` with `allowed_domains`
- [ ] `allowed_domains` includes `host.docker.internal` and peer hostnames
- [ ] AGENTS.md auto-injected on first agent start with correct portal URL and bearer token
- [ ] `POST /api/agents/{id}?action=send` accepts message and returns taskId
- [ ] `regenerateAgentConfig` includes AllowedDomains when rebuilding

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete A2A round-trip - allowed_domains, AGENTS.md injection, send endpoint"
```

---

## Execution Order

Tasks 1 and 4 are tightly coupled (both modify the template and ContainerConfig). They should be done together.

**Recommended order:**
1. Task 1 + Task 4 (config.toml template + ContainerConfig + regeneration)
2. Task 2 (AGENTS.md auto-injection)
3. Task 3 (send endpoint)
4. Task 5 (integration verification)

Tasks 2 and 3 are independent and can be parallelized.
