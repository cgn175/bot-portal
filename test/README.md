# Integration Testing

## Overview
Integration tests for Bot Portal with mock agents.

## Running Tests

### Unit Tests
```bash
go test ./test/...
```

### Integration Tests with Mock Agents
```bash
# Start mock agents
cd test
docker-compose -f docker-compose.test.yml up -d

# Run integration tests
go test -v ./test/integration_test.go

# Stop mock agents
docker-compose -f docker-compose.test.yml down
```

## Mock Agent
Simple HTTP server that implements A2A protocol:
- `GET /.well-known/agent.json` - Returns agent card
- `POST /tasks` - Accepts task requests
- `GET /health` - Health check

## Test Coverage
- Agent registration
- Channel creation (direct and broadcast)
- Message logging
- A2A message routing
- API endpoints
