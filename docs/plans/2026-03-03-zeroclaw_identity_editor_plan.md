# ZeroClaw Identity Editor Implementation Plan

**Date:** 2026-03-03  
**Status:** Draft  
**Feature:** Agent Identity & Behavior File Editor

## Goal

Add UI + API to manage `IDENTITY.md`, `SOUL.md`, and `AGENTS.md` for each ZeroClaw agent, enabling runtime behavior customization without agent restarts.

## Key Decisions

- **Identity format:** OpenClaw markdown only (AIEOS deferred to v2)
- **Files managed:** `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md` (core set)
- **Optional files for v2:** `HEARTBEAT.md`, `BOOTSTRAP.md`, `MEMORY.md`
- **Storage:** Agent workspace volume (`bot-portal-agent-<id>-workspace`)
- **No restart required:** Files are read on every message turn (verified in ZeroClaw source)
- **Reset to defaults:** Included in v1 using exact ZeroClaw scaffold templates
- **File access method:** Docker CP API (more efficient than helper containers)

## How ZeroClaw Reads Identity Files

Based on review of `zeroclaw/src/agent/prompt.rs` and `zeroclaw/src/channels/mod.rs`:

1. **System prompt is built fresh for every message** via `build_system_prompt_with_mode()`
2. **Identity files are injected during prompt building** via `inject_workspace_file()`
3. **Files are read from disk each time:**
   - `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md`, `HEARTBEAT.md`, `BOOTSTRAP.md`
   - If file missing, prompt shows: `### IDENTITY.md\n\n[File not found: IDENTITY.md]\n`
   - **Files are truncated at 20,000 chars** with notice in prompt: `[... truncated at 20000 chars — use 'read' for full file]`

**Implication:** Changes to identity files take effect immediately on the next message - no agent restart needed.

## Backend Implementation

### 1. API Endpoints

Add to `internal/api/agent_identity.go`:

#### `GET /api/agents/{id}/identity`

**Response:**
```json
{
  "identity": "# IDENTITY.md content...",
  "soul": "# SOUL.md content...",
  "agents": "# AGENTS.md content...",
  "user": "# USER.md content...",
  "tools": "# TOOLS.md content...",
  "missing": ["SOUL.md"]
}
```

- Returns empty string for missing files
- Lists missing files in `missing` array
- Non-blocking: agent continues to work with `[File not found]` in prompt
- Warns if file exceeds 20,000 chars (will be truncated in agent's prompt)

#### `PUT /api/agents/{id}/identity`

**Request:**
```json
{
  "identity": "# New content...",
  "soul": "# Updated soul...",
  "agents": "# Updated agents...",
  "user": "# Updated user...",
  "tools": "# Updated tools..."
}
```

- Only updates provided fields (partial updates allowed)
- Validates file size limits (200KB max per file)
- Validates basic markdown structure (optional warning if malformed)
- Creates parent directory if needed
- **Concurrent access handling:** Returns warning if agent is currently running/processing
- Returns success/error status

#### `POST /api/agents/{id}/identity/reset`

**Request:** (empty body)

**Action:**
- Writes exact default templates from ZeroClaw scaffolds (see `zeroclaw/src/onboard/wizard.rs`)
- Creates all 5 files: `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md`
- Uses agent name from portal's agent record
- Uses system timezone or UTC
- Fixed communication style: "Be warm, natural, and clear."
- Returns success with list of files created

#### `GET /api/agents/{id}/identity/preview`

**Query params:** `?file=SOUL.md`

**Response:**
```json
{
  "preview": "### SOUL.md\n\n*You're not a chatbot...\n\n[... truncated at 20000 chars — use 'read' for full file]\n",
  "truncated": true,
  "charCount": 25000
}
```

- Shows exactly how the file will appear in the agent's system prompt
- Applies 20,000 char truncation rule
- Useful for validating files before save

### 2. Docker Volume File Access

Add to `internal/docker/manager.go`:

```go
// ReadWorkspaceFile reads a file from agent workspace volume using Docker CP API
func (m *Manager) ReadWorkspaceFile(ctx context.Context, agentID, filename string) (string, error)

// WriteWorkspaceFile writes a file to agent workspace volume using Docker CP API
func (m *Manager) WriteWorkspaceFile(ctx context.Context, agentID, filename, content string) error
```

**Implementation approach (Docker CP API):**

1. Use Docker's `CopyFromContainer` and `CopyToContainer` APIs
2. **Read operation:**
   - `CopyFromContainer(ctx, volumeName, "/workspace/IDENTITY.md")`
   - Extract content from tar stream
   - More efficient than helper containers (no container lifecycle overhead)
3. **Write operation:**
   - Create tar archive in memory with file content
   - `CopyToContainer(ctx, volumeName, "/workspace/", tarReader, ...)`
4. **Strict filename allowlist:** Only `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md`
5. **Size limit:** Max 200KB per file (reject with 400 Bad Request)
6. **Validation:**
   - Check for valid UTF-8 encoding
   - Basic markdown structure validation (optional warning)
   - Character count validation (warn if > 20,000 chars)

**Reference:** Docker SDK for Go - `client.CopyFromContainer()` and `client.CopyToContainer()`

### 3. Default Templates

**Copy exact templates from** `zeroclaw/src/onboard/wizard.rs` (lines 5680-5870):

```go
// Port these exact templates to Go:
func defaultIdentity(agentName string) string
func defaultSoul(agentName string) string  
func defaultAgents(agentName string) string
func defaultUser(userName, timezone, commStyle string) string
func defaultTools() string
```

**Template contents (verbatim from ZeroClaw):**

- **IDENTITY.md:** Name, creature, vibe, emoji (lines 5682-5690)
- **SOUL.md:** Core truths, communication style, boundaries, continuity (lines 5735-5771)
- **AGENTS.md:** Session requirements, safety rules, external vs internal behavior, group chat rules, tools, sub-tasks (lines 5692-5722)
- **USER.md:** Name, timezone, languages, communication style, preferences, work context (lines 5773-5788)
- **TOOLS.md:** SSH hosts, device nicknames, built-in tool descriptions (lines 5790-5820)

**For v1:**
- Use agent name from portal's `agent.Name` field
- Get timezone from system or default to UTC
- Hard-code communication style to: "Be warm, natural, and clear."
- Get user name from environment or default to "User"

### 4. Security & Guardrails

- **Filename allowlist:** Hard-coded list `[IDENTITY.md, SOUL.md, AGENTS.md, USER.md, TOOLS.md]`, no path traversal
- **File size limit:** 200KB max per file (configurable constant)
- **Character count warning:** Warn if file > 20,000 chars (will be truncated in prompt)
- **Validation:**
  - UTF-8 encoding check
  - Basic markdown validation (optional warnings for malformed syntax)
  - Detect missing `#` headers (files should start with `# FILENAME`)
- **Concurrent access:**
  - Check agent status before write
  - Return warning if agent is actively processing a message
  - Still allow writes (changes apply on next turn)
- **Audit logging:** Log agent ID, filename, operation (read/write/reset), timestamp, file size

## Frontend Implementation

### 1. UI Location

Add to `web/src/pages/AgentDetail.tsx`:

**New section:** "Identity & Behavior"

- Positioned below agent status/controls, above logs/chat
- 5 tabbed editors: `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md`
- Monospace font, markdown syntax highlighting (optional, use CodeMirror or Monaco)
- Collapsible section to save screen space

### 2. Controls

**Buttons:**
- **Load** - Fetches latest from workspace (useful if external edits made)
- **Save** - PUTs only changed files
- **Preview** - Shows how file will appear in agent's system prompt (with truncation)
- **Reset to Defaults** - POSTs reset, prompts for confirmation

**Indicators:**
- Missing file warning: "⚠️ SOUL.md not found - agent will see [File not found] in prompt"
- Truncation warning: "⚠️ File is 25,000 chars - will be truncated to 20,000 in agent's prompt"
- Agent running warning: "⚠️ Agent is processing a message - changes will apply on next turn"
- Success toast: "✓ Identity files saved. Changes will apply to the next message."
- Character count per file: "15,234 / 20,000 chars" (green < 20k, yellow 20k-180k, red > 180k)
- Unsaved changes indicator: "● Unsaved changes" badge on modified tabs

### 3. User Experience

**On page load:**
1. `GET /api/agents/{id}/identity`
2. Populate editors with content
3. Show warnings for missing files
4. Mark editors as "pristine" (no unsaved changes)

**On save:**
1. Track which editors have changes
2. `PUT /api/agents/{id}/identity` with only changed fields
3. Show success message: **"Changes saved. Will take effect on next message - no restart needed."**
4. Mark editors as pristine

**On reset:**
1. Show confirmation dialog: "This will overwrite all 5 identity files with default templates. Continue?"
2. `POST /api/agents/{id}/identity/reset`
3. Reload content
4. Show success: "✓ Identity files reset to defaults."

**On preview:**
1. `GET /api/agents/{id}/identity/preview?file=SOUL.md`
2. Show modal with preview text
3. Display truncation notice if applicable
4. Show char count and truncation point

### 4. API Client

Add to `web/src/api/client.ts`:

```typescript
async getAgentIdentity(id: string): Promise<AgentIdentityResponse>
async updateAgentIdentity(id: string, data: AgentIdentityUpdate): Promise<void>
async resetAgentIdentity(id: string): Promise<void>
async previewAgentIdentityFile(id: string, filename: string): Promise<AgentIdentityPreview>
```

**Types:**
```typescript
interface AgentIdentityResponse {
  identity: string
  soul: string
  agents: string
  user: string
  tools: string
  missing: string[]
  warnings?: {
    truncated: string[]  // Files > 20k chars
    agentRunning: boolean
  }
}

interface AgentIdentityUpdate {
  identity?: string
  soul?: string
  agents?: string
  user?: string
  tools?: string
}

interface AgentIdentityPreview {
  preview: string
  truncated: boolean
  charCount: number
}
```

## Testing Plan

### Backend Unit Tests

- ✓ Filename allowlist enforced (reject `../etc/passwd`)
- ✓ File size limit enforced (reject 300KB file)
- ✓ Character count warning triggered for files > 20,000 chars
- ✓ Missing file returns empty string + missing marker
- ✓ Read/write round-trip preserves content
- ✓ Partial updates work (only update `soul`, leave others unchanged)
- ✓ UTF-8 validation rejects invalid encoding
- ✓ Preview endpoint shows correct truncation
- ✓ Markdown validation warns on malformed files

### Integration Tests

1. Create agent → reset identity → read back → verify all 5 default files applied
2. Create agent → update SOUL.md → read back → verify persisted
3. Update identity → send message to agent → verify behavior reflects changes
4. Write file > 20k chars → verify truncation warning → preview → verify truncated output
5. Update while agent running → verify warning shown → verify changes apply on next message

### Manual Testing

1. Start agent container
2. Update `SOUL.md` via portal UI to add "Always respond in haiku format"
3. Send message to agent
4. Verify agent responds in haiku (proves no restart needed)
5. Test preview mode with file > 20k chars
6. Verify character count warnings display correctly
7. Test concurrent edit warning when agent is processing

## Rollout Steps

1. **Backend:** Implement Docker CP API file access helpers
2. **Backend:** Port exact default templates from ZeroClaw wizard.rs
3. **Backend:** Implement API endpoints (GET, PUT, POST reset, GET preview)
4. **Backend:** Add routes to router
5. **Backend:** Add validation (UTF-8, markdown, size limits, char count)
6. **Frontend:** Build 5-tab editor UI component
7. **Frontend:** Implement preview modal
8. **Frontend:** Add character count indicators and warnings
9. **Frontend:** Integrate API client
10. **Testing:** Run unit + integration tests
11. **Manual QA:** Test behavior change without restart, preview mode, concurrent access warnings
12. **Docs:** Update README.md with identity editor feature

## Documentation Updates

Add to `bot-portal/README.md`:

### Agent Identity Editor

Edit your ZeroClaw agent's personality and behavior without restarting:

- **IDENTITY.md** - Agent name, vibe, emoji
- **SOUL.md** - Core personality, boundaries, communication style  
- **AGENTS.md** - Session requirements, safety rules, tool usage guidelines
- **USER.md** - User preferences, timezone, work context
- **TOOLS.md** - Local notes, SSH hosts, device specifics

**Features:**
- Changes take effect immediately on the next message - no restart required
- Preview mode shows exactly how files appear in agent's system prompt
- Character count warnings (files truncated at 20,000 chars in prompt)
- Reset to defaults using ZeroClaw's official templates

**Limits:**
- Max 200KB per file
- Files > 20,000 chars are truncated in the agent's prompt (full file preserved on disk)

## Future Enhancements (v2+)

- Support for `HEARTBEAT.md`, `BOOTSTRAP.md`, `MEMORY.md`
- AIEOS identity format support
- Diff view showing changes before save
- Version history / rollback functionality
- Collaborative editing with conflict detection
- Template library with pre-made personalities
- Real-time validation with ZeroClaw's prompt builder
- Export/import identity profiles
- AI-assisted identity tuning suggestions
