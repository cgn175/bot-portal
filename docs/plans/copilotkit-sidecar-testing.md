# CopilotKit Sidecar Integration Testing

## Prerequisites

1. Go backend running (port 8080) with at least one model configured
2. Node.js sidecar running (port 3001)
3. React frontend running (port 5173)

## Test Procedure

### 1. Open the App

Navigate to http://localhost:5173

**Expected:**
- App loads without errors
- CopilotKit popup appears in bottom-right corner (or chat icon)
- No errors in browser console

### 2. Open CopilotKit Chat

Click the chat icon/popup

**Expected:**
- Chat window opens
- Shows initial message: "Hi! I can help you manage agents, models, and auth configs. What would you like to do?"

### 3. Test Context Exposure

Type: "What agents do I have?"

**Expected:**
- Request sent to http://localhost:3001/copilot (check Network tab)
- Response streams back (SSE or GraphQL subscription)
- Assistant responds with current agents list
- If no agents: "You don't have any agents configured yet."

### 4. Test Actions

Type: "Create a test agent named demo-agent"

**Expected:**
- CopilotKit calls the `createAgent` action
- Agent appears in the agents list on the main page
- Success message in chat

### 5. Test Error Handling

Stop the Go backend, then type a message

**Expected:**
- Sidecar returns error to frontend
- Chat shows user-friendly error message
- Error appears in sidecar console logs

### 6. Test Streaming

Ask a complex question: "Explain how agents work in Bot Portal"

**Expected:**
- Response streams word-by-word
- No long delays before text appears
- Complete response renders properly

## Debugging

### Check Sidecar Logs

```bash
cd sidecar
npm run dev
# Watch for GraphQL requests and backend calls
```

### Check Go Backend Logs

```bash
make run
# Watch for /api/copilot/chat/completions requests
```

### Browser DevTools

- Network tab: Look for requests to localhost:3001/copilot
- Console: Check for CopilotKit errors
- React DevTools: Verify CopilotProvider is rendering

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| 404 on /api/copilot | Frontend pointing to wrong URL | Check `VITE_COPILOT_RUNTIME_URL` |
| CORS errors | Sidecar CORS misconfigured | Set `CORS_ORIGIN=http://localhost:5173` |
| No streaming | Backend not streaming | Verify Go `/api/copilot/chat/completions` returns SSE |
| Actions not working | Actions not registered | Check `CopilotActions` component is rendered |
| Connection refused | Sidecar not running | Start sidecar with `npm run dev` |
| Empty responses | No model selected | Ensure at least one model is configured in backend |

## Environment Variables

Ensure all three services have correct environment variables:

### Go Backend (.env)
```
PORT=8080
DB_PATH=./bot-portal.db
ENCRYPTION_KEY=<your-32-byte-key>
```

### Sidecar (.env)
```
PORT=3001
BACKEND_URL=http://localhost:8080
CORS_ORIGIN=http://localhost:5173
```

### Frontend (.env)
```
VITE_API_URL=http://localhost:8080
VITE_COPILOT_RUNTIME_URL=http://localhost:3001/copilot
```

## Manual Testing Checklist

- [ ] All three services start without errors
- [ ] Frontend loads and CopilotKit UI appears
- [ ] Chat opens and shows initial message
- [ ] Context queries work (agents, models, auth configs)
- [ ] Actions work (create/update/delete operations)
- [ ] Error handling works when backend is stopped
- [ ] Streaming responses work properly
- [ ] Browser console shows no errors
- [ ] Network tab shows proper request flow
- [ ] Data changes reflect immediately in UI

## Architecture Validation

The integration test validates this flow:

```
User Input (Frontend)
    ↓
CopilotKit Provider (React)
    ↓
GraphQL Request → http://localhost:3001/copilot
    ↓
Node.js Sidecar (CopilotRuntime)
    ↓
HTTP POST → http://localhost:8080/api/copilot/chat/completions
    ↓
Go Backend (LLM Proxy)
    ↓
LLM Provider (OpenAI/Anthropic/etc.)
    ↓
Response Stream ← SSE
    ↓
Sidecar forwards stream
    ↓
Frontend renders response
```

## Success Criteria

✅ All 6 test scenarios pass
✅ No errors in any service logs
✅ Streaming works smoothly
✅ Actions trigger backend API calls
✅ UI updates reflect backend changes
✅ Error handling is graceful

## Next Steps After Testing

If all tests pass:
1. Update main README.md with CopilotKit integration instructions
2. Consider adding automated E2E tests with Playwright
3. Document any discovered edge cases
4. Create GitHub issues for any bugs found

If tests fail:
1. Document exact failure scenario
2. Check service logs for errors
3. Verify environment variables
4. Review network requests in DevTools
5. Report findings to development team
