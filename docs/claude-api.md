# Claude API Support

Bot Portal natively supports both OpenAI and Claude API formats, allowing you to use Claude models with tools like Claude Code CLI while storing models from any provider.

## Endpoints

### 1. `/api/claude` - Native Claude API Format

This endpoint accepts Claude API format requests directly and is the recommended endpoint for Claude Code CLI.

**Request Format:**
```json
POST /api/claude
Content-Type: application/json

{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "max_tokens": 1024,
  "stream": false
}
```

**Response Format:**
```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 20
  }
}
```

**Streaming:** Set `"stream": true` in the request. Response will be SSE (Server-Sent Events) with `event: message_start`, `event: content_block_delta`, etc.

### 2. `/api/chat/completions` - Adaptive Format

This endpoint auto-detects the model type and uses the appropriate format.

**Claude Model Detection:**
- Any model name starting with `claude-` is automatically detected
- Request format: OpenAI (chat completions)
- Bot Portal transforms to Claude API format internally
- Response transformed back to OpenAI format

**Request Format (OpenAI):**
```json
POST /api/chat/completions
Content-Type: application/json

{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "max_tokens": 1024,
  "stream": false
}
```

**Response Format (OpenAI):**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "claude-3-5-sonnet-20241022",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

## Claude Code CLI Usage

Claude Code CLI (claude.ai/code) can be configured to use Bot Portal as its backend.

### Setup

1. **Set the base URL environment variable:**
   ```bash
   export ANTHROPIC_BASE_URL="http://localhost:8080/api"
   ```

2. **Run Claude Code CLI:**
   ```bash
   claude
   ```

### How It Works

1. Claude Code CLI sends requests to `${ANTHROPIC_BASE_URL}/claude` (i.e., `http://localhost:8080/api/claude`)
2. Bot Portal receives the native Claude API format request
3. Bot Portal looks up the model in its database (by name)
4. Bot Portal proxies the request to the configured provider with proper authentication
5. Response flows back through Bot Portal to Claude Code CLI

### Benefits

- **Centralized Model Management:** Store all your models (OpenAI, Anthropic, custom) in one place
- **Credential Security:** Bot Portal handles authentication, credentials encrypted at rest
- **Provider Flexibility:** Use any OpenAI-compatible provider with Claude models
- **Logging:** All requests logged in Bot Portal for debugging

## Format Transformation Details

### OpenAI → Claude Transformation

When a Claude model is detected in `/api/chat/completions`:

**Messages:**
- System messages extracted and combined into `system` parameter
- User/assistant messages preserved in `messages` array
- Tool calls converted to Claude's tool use format

**Parameters:**
- `max_tokens` → `max_tokens` (required by Claude)
- `temperature` → `temperature`
- `top_p` → `top_p`
- `stop` → `stop_sequences`
- `stream` → `stream`

**Tools:**
- OpenAI function format converted to Claude tool format
- Tool choice mapping: `auto` → `auto`, `required` → `any`, `none` → omitted

### Claude → OpenAI Transformation

When transforming Claude responses back to OpenAI format:

**Response Structure:**
- `id` → `id`
- `model` → `model`
- `content[0].text` → `choices[0].message.content`
- `stop_reason` → `choices[0].finish_reason`
  - `end_turn` → `stop`
  - `max_tokens` → `length`
  - `stop_sequence` → `stop`
  - `tool_use` → `tool_calls`

**Usage:**
- `input_tokens` → `prompt_tokens`
- `output_tokens` → `completion_tokens`
- `total_tokens` calculated as sum

**Streaming:**
- Claude SSE events transformed to OpenAI streaming format
- `content_block_delta` → `choices[0].delta.content`
- `message_stop` → `choices[0].finish_reason`

## Logging

Bot Portal logs all transformations for debugging.

### Claude API Request (Native)
```
2024-03-02T10:30:00Z INFO Claude API request model=claude-3-5-sonnet-20241022 endpoint=https://api.anthropic.com/v1/messages
```

### OpenAI → Claude Transformation
```
2024-03-02T10:30:00Z INFO Detected Claude model, transforming OpenAI request to Claude format model=claude-3-5-sonnet-20241022
2024-03-02T10:30:00Z DEBUG Extracted system message from OpenAI messages system="You are a helpful assistant"
2024-03-02T10:30:00Z DEBUG Transformed OpenAI request to Claude format messages=2 system_length=28
```

### Claude → OpenAI Response
```
2024-03-02T10:30:05Z INFO Transforming Claude response to OpenAI format stop_reason=end_turn input_tokens=10 output_tokens=20
2024-03-02T10:30:05Z DEBUG OpenAI response created choices=1 usage.total_tokens=30
```

## Error Handling

Both endpoints return consistent error formats:

**OpenAI Format Error:**
```json
{
  "error": {
    "message": "Model not found: claude-unknown",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

**Claude Format Error:**
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Model not found: claude-unknown"
  }
}
```

## Model Configuration

To use Claude models with Bot Portal:

1. **Create an Auth Config** with provider `anthropic` and your API key
2. **Discover Models** - Bot Portal fetches available models from Anthropic
3. **Reference by Name** - Use the model name (e.g., `claude-3-5-sonnet-20241022`) in requests

Bot Portal automatically:
- Routes to the correct provider endpoint
- Applies authentication headers
- Handles format transformations
- Returns responses in the expected format

## Testing

Test the endpoints with curl:

### Test Native Claude API
```bash
curl -X POST http://localhost:8080/api/claude \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 1024
  }'
```

### Test Adaptive OpenAI Format
```bash
curl -X POST http://localhost:8080/api/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 1024
  }'
```

Both should return equivalent responses in their respective formats.
