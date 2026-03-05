# A2A Send Tool — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a native `a2a_send` LLM-callable tool to ZeroClaw that reuses the existing `A2AChannel` from the live channel registry, so agents can send messages to peers without using `http_request` or `curl` workarounds.

**Architecture:** One new file `src/tools/a2a_send.rs` implementing the `Tool` trait. It calls `get_live_channel("a2a")` to get the already-registered A2A channel, then calls `.send()`. Zero changes to the channel layer, config, or protocol crate. Registration via 3 small additions in `src/tools/mod.rs`.

**Tech Stack:** Rust, async_trait, serde_json, existing `Tool` trait + `get_live_channel`

**Repo:** `~/projects/zeroclaw`

---

## Task 1: Create `src/tools/a2a_send.rs`

**Files:**
- Create: `src/tools/a2a_send.rs`

**Step 1: Write the tool implementation**

```rust
//! A2A Send Tool — send messages to peer agents via the A2A protocol.
//!
//! This tool is a thin wrapper over the live A2A channel registered by
//! `start_channels()`. It calls `get_live_channel("a2a")` and invokes
//! `.send()` on the existing `A2AChannel`, which handles bearer auth,
//! SSE subscription, and response routing automatically.

use super::traits::{Tool, ToolResult};
use crate::channels::{get_live_channel, SendMessage};
use async_trait::async_trait;
use serde_json::json;

/// LLM-callable tool for sending messages to peer agents via A2A.
pub struct A2aSendTool;

impl A2aSendTool {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl Tool for A2aSendTool {
    fn name(&self) -> &str {
        "a2a_send"
    }

    fn description(&self) -> &str {
        "Send a message to a peer agent via the A2A (Agent-to-Agent) protocol. \
         The recipient must be a configured peer in your A2A channel. \
         Messages are delivered asynchronously — responses arrive through your A2A channel."
    }

    fn parameters_schema(&self) -> serde_json::Value {
        json!({
            "type": "object",
            "properties": {
                "to": {
                    "type": "string",
                    "description": "Peer agent ID to send the message to (must be a configured A2A peer)"
                },
                "message": {
                    "type": "string",
                    "description": "Message content to send to the peer agent"
                }
            },
            "required": ["to", "message"]
        })
    }

    async fn execute(&self, args: serde_json::Value) -> anyhow::Result<ToolResult> {
        let to = args
            .get("to")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let message = args
            .get("message")
            .and_then(|v| v.as_str())
            .unwrap_or_default();

        if to.is_empty() {
            return Ok(ToolResult {
                success: false,
                output: String::new(),
                error: Some("Missing required parameter: 'to'".to_string()),
            });
        }
        if message.is_empty() {
            return Ok(ToolResult {
                success: false,
                output: String::new(),
                error: Some("Missing required parameter: 'message'".to_string()),
            });
        }

        let channel = match get_live_channel("a2a") {
            Some(ch) => ch,
            None => {
                return Ok(ToolResult {
                    success: false,
                    output: String::new(),
                    error: Some(
                        "A2A channel is not active. Ensure A2A is enabled and peers are configured."
                            .to_string(),
                    ),
                });
            }
        };

        let send_msg = SendMessage::new(message, to);

        match channel.send(&send_msg).await {
            Ok(()) => Ok(ToolResult {
                success: true,
                output: format!(
                    "Message sent to peer '{}'. Response will arrive asynchronously via A2A channel.",
                    to
                ),
                error: None,
            }),
            Err(e) => Ok(ToolResult {
                success: false,
                output: String::new(),
                error: Some(format!("Failed to send A2A message: {e}")),
            }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tool_metadata() {
        let tool = A2aSendTool::new();
        assert_eq!(tool.name(), "a2a_send");
        assert!(!tool.description().is_empty());
        let schema = tool.parameters_schema();
        assert!(schema["properties"]["to"].is_object());
        assert!(schema["properties"]["message"].is_object());
        assert_eq!(schema["required"], json!(["to", "message"]));
    }

    #[tokio::test]
    async fn missing_to_returns_error() {
        let tool = A2aSendTool::new();
        let result = tool.execute(json!({"message": "hello"})).await.unwrap();
        assert!(!result.success);
        assert!(result.error.as_deref().unwrap().contains("to"));
    }

    #[tokio::test]
    async fn missing_message_returns_error() {
        let tool = A2aSendTool::new();
        let result = tool.execute(json!({"to": "peer-1"})).await.unwrap();
        assert!(!result.success);
        assert!(result.error.as_deref().unwrap().contains("message"));
    }

    #[tokio::test]
    async fn no_active_channel_returns_error() {
        let tool = A2aSendTool::new();
        let result = tool
            .execute(json!({"to": "peer-1", "message": "hello"}))
            .await
            .unwrap();
        assert!(!result.success);
        assert!(result.error.as_deref().unwrap().contains("not active"));
    }
}
```

**Step 2: Verify file created**

Run: `cat src/tools/a2a_send.rs | head -5`
Expected: Shows the module doc comment

---

## Task 2: Register the tool in `src/tools/mod.rs`

**Files:**
- Modify: `src/tools/mod.rs`

Three small additions:

**Step 1: Add module declaration** (after line 17 `pub mod agents_ipc;`)

```rust
pub mod a2a_send;
```

**Step 2: Add re-export** (after line 75 `pub use apply_patch::ApplyPatchTool;`)

```rust
pub use a2a_send::A2aSendTool;
```

**Step 3: Register in `all_tools_with_runtime`**

After the `agents_ipc` block (around line 536, after the closing `}` of the `if root_config.agents_ipc.enabled` block), add:

```rust
// A2A send tool — available when A2A channel is configured
if root_config.channels_config.a2a.as_ref().is_some_and(|a| a.enabled) {
    tool_arcs.push(Arc::new(A2aSendTool::new()));
}
```

**Step 4: Verify compilation**

Run: `cargo check 2>&1 | tail -5`
Expected: No errors

**Step 5: Run tests**

Run: `cargo test --lib tools::a2a_send 2>&1 | tail -10`
Expected: 4 tests pass (tool_metadata, missing_to, missing_message, no_active_channel)

---

## Task 3: Verify full build and existing tests

**Step 1: Full build**

Run: `cargo build 2>&1 | tail -5`
Expected: SUCCESS

**Step 2: Run existing tool tests**

Run: `cargo test --lib tools:: 2>&1 | tail -20`
Expected: All existing tests pass + 4 new tests

**Step 3: Run zeroclaw-a2a crate tests**

Run: `cargo test -p zeroclaw-a2a 2>&1 | tail -10`
Expected: All 36 tests pass (unchanged)

---

## Summary of changes

| File | Change | Lines |
|------|--------|-------|
| `src/tools/a2a_send.rs` | **New file** — `A2aSendTool` implementing `Tool` trait | ~140 |
| `src/tools/mod.rs` | Add `pub mod a2a_send;` | +1 |
| `src/tools/mod.rs` | Add `pub use a2a_send::A2aSendTool;` | +1 |
| `src/tools/mod.rs` | Register tool in `all_tools_with_runtime` | +3 |

**Total: 1 new file, 5 lines added to existing file. Zero changes to channels, config, or protocol.**

Merging main → this branch will have zero conflicts since `mod.rs` additions are append-only at well-separated locations.
