# A2A Send Tool — Gateway Mode Implementation

**Date:** 2026-03-05  
**Status:** Completed  
**Feature:** Self-contained `a2a_send` tool for ZeroClaw agents in gateway mode

## Problem Statement

The initial `a2a_send` tool implementation (from thread T-019cbb28-2722-723c-bd06-aef65761ab28) relied on `get_live_channel("a2a")` from the live channel registry. This worked for daemon mode but **failed in gateway mode** because:

1. **Gateway mode** (`run_gateway` in `src/gateway/mod.rs`) runs a single HTTP endpoint for portal communication
2. **Channels/daemon mode** (`start_channels` in `src/channels/mod.rs`) runs multi-channel messaging and populates the live registry
3. **Bot-portal agents run in gateway mode only** (started via Docker containers)
4. The A2A gateway endpoints (`POST /tasks`, `GET /tasks/{id}/stream`) ARE active in gateway mode
5. But the live channel registry is empty, so `get_live_channel("a2a")` returns `None`

**Result:** Agents said "A2A channel isn't active" even though portal → agent A2A communication worked perfectly.

## Solution: Option A (Self-Contained Implementation)

Redesigned `a2a_send` to work **without** the live channel registry, following the pattern of `agents_ipc` tools:

### Architecture

```
a2a_send tool
├── Reads A2A config directly from Arc<Config>
├── Finds peer in config.channels_config.a2a.peers
├── Creates HTTP client on-demand
├── POST to peer's /tasks endpoint (Google A2A protocol)
└── Returns success with task_id
```

No dependency on:
- ❌ `get_live_channel("a2a")`
- ❌ Live channel registry
- ❌ `start_channels()` daemon

Works in:
- ✅ Gateway mode (bot-portal agents)
- ✅ Daemon mode (standalone ZeroClaw)

## Implementation

### File: `src/tools/a2a_send.rs` (new)

**Key differences from original:**

| Original (registry-based) | New (self-contained) |
|--------------------------|---------------------|
| `get_live_channel("a2a")` | `self.config.channels_config.a2a` |
| `channel.send(&SendMessage::new(...))` | Direct HTTP POST to `peer.endpoint + "/tasks"` |
| Requires `start_channels()` | Works in gateway mode |
| ~100 lines | ~180 lines (includes HTTP client setup) |

**Implementation details:**

```rust
pub struct A2aSendTool {
    config: Arc<Config>,  // NOT Arc<Mutex<Config>> - Config is already Clone
}

async fn execute(&self, args: Value) -> Result<ToolResult> {
    // 1. Read A2A config from self.config.channels_config.a2a
    let a2a_config = match &self.config.channels_config.a2a {
        Some(cfg) if cfg.enabled => cfg,
        // ... error handling
    };

    // 2. Find peer in config
    let peer = a2a_config.peers.iter()
        .find(|p| p.id == to && p.enabled)
        .ok_or(...)?;

    // 3. Build HTTP client on-demand
    let http_client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .build()?;

    // 4. POST to peer's /tasks endpoint (Google A2A protocol)
    let response = http_client
        .post(format!("{}/tasks", peer.endpoint))
        .bearer_auth(&peer.bearer_token)
        .json(&json!({"message": message}))
        .send()
        .await?;

    // 5. Parse response and return task_id
    let task_id = response.json::<Value>()
        .get("task")
        .get("id")
        .as_str()?;

    Ok(ToolResult {
        success: true,
        output: format!("Message sent to peer '{}' (task_id: {})", to, task_id),
        error: None,
    })
}
```

### File: `src/tools/mod.rs` (modified)

**3 changes:**

1. **Line 18:** `pub mod a2a_send;` (module declaration)
2. **Line 76:** `pub use a2a_send::A2aSendTool;` (re-export)
3. **Line 540-543:** Register tool in `all_tools_with_runtime`:

```rust
// A2A send tool — available when A2A channel is configured
if root_config.channels_config.a2a.as_ref().is_some_and(|a| a.enabled) {
    tool_arcs.push(Arc::new(A2aSendTool::new(config.clone())));
}
```

## Testing

### Unit Tests (6 total, all passing)

```bash
$ cargo test --lib tools::a2a_send
```

| Test | Validates |
|------|-----------|
| `tool_metadata` | Tool name, description, schema |
| `missing_to_returns_error` | Parameter validation |
| `missing_message_returns_error` | Parameter validation |
| `no_a2a_config_returns_error` | Config validation |
| `disabled_a2a_returns_error` | Enabled check |
| `unknown_peer_returns_error` | Peer lookup |

### Integration Test (bot-portal)

**Before fix:**
```
Agent: "A2A channel isn't active"
```

**After fix:**
```
Agent uses a2a_send tool successfully
Portal receives task on /tasks endpoint
Agent receives response via A2A gateway
```

## Zero Conflicts

Changes are append-only in well-separated locations:

| File | Lines Changed | Conflict Risk |
|------|---------------|---------------|
| `src/tools/a2a_send.rs` | +273 (new) | None |
| `src/tools/mod.rs` | +5 | None (different locations) |

## Bot-Portal Integration

### Config Injection (already implemented)

**File:** `internal/docker/manager.go`

```go
[[channels.a2a.peers]]
id = "{{.ID}}"
endpoint = "{{.Endpoint}}"
bearer_token = "{{.BearerToken}}"
enabled = true
```

### Auto-approval (already implemented)

**File:** `internal/docker/manager.go` (line 296)

```toml
[autonomy]
auto_approve = ["file_read", "memory_recall", "a2a_send"]
```

### AGENTS.md Injection (already implemented)

**File:** `internal/api/agents.go`

Generates peer list in `AGENTS.md` so agents know how to use `a2a_send`:

```markdown
# Available Agents

## Agent: researcher (ID: agent-123)
- **Endpoint:** http://agent-123:9000
- **How to contact:** Use `a2a_send` tool with `to: "agent-123"`
```

## Benefits Over Original Implementation

1. **Works in gateway mode** — agents created by bot-portal can use it
2. **Self-contained** — no dependency on channel daemon
3. **Same API** — tool parameters unchanged from original design
4. **Follows ZeroClaw patterns** — matches `agents_ipc` tools (self-contained, config-based)
5. **Zero breaking changes** — original tool was never released, this is the first working version

## Related Files

| File | Purpose |
|------|---------|
| `src/gateway/a2a.rs` | Wires A2A gateway endpoints in gateway mode |
| `crates/zeroclaw-a2a/src/channel.rs` | A2AChannel.send() implementation (for daemon mode) |
| `crates/zeroclaw-a2a/src/protocol.rs` | A2A protocol types (CreateTaskRequest, etc.) |
| `internal/docker/manager.go` | Portal's config template with A2A peers |
| `internal/api/agents.go` | Auto-generates AGENTS.md on agent start |

## Verification

```bash
# In zeroclaw repo
cd ~/projects/zeroclaw
cargo test --lib tools::a2a_send  # All 6 tests pass
cargo build                        # Compiles successfully
```

## Summary

- **Problem:** Original `a2a_send` tool failed in gateway mode (bot-portal agents)
- **Root cause:** Dependency on live channel registry (only populated in daemon mode)
- **Solution:** Self-contained implementation reading config directly (like `agents_ipc` tools)
- **Result:** Tool works in both gateway and daemon modes
- **Files changed:** 1 new file + 5 lines in existing file
- **Tests:** 6 unit tests, all passing
- **Integration:** Works with bot-portal's existing A2A gateway infrastructure
