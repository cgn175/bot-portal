.PHONY: build run test clean deps docker-build docker-run

# Build variables
BINARY_NAME=bot-portal
GO=go
GOFLAGS=-v

# Default target
all: deps build

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Build the application
build:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/portal

# Run the application
run: build
	./bin/$(BINARY_NAME)

# Run tests
test:
	$(GO) test $(GOFLAGS) -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f $(BINARY_NAME)
	rm -f *.db

# Run with docker-compose
docker-build:
	docker-compose build

docker-run:
	docker-compose up -d

docker-down:
	docker-compose down

# Development helpers
dev: docker-run
	@echo "Bot Portal running at http://localhost:8080"

# Database
db-reset:
	rm -f bot-portal.db

# Lint
lint:
	golangci-lint run

# Format code
fmt:
	$(GO) fmt ./...
	$(GO) vet ./...

# CopilotKit Sidecar targets
.PHONY: sidecar-install sidecar-dev sidecar-build dev-all

sidecar-install:
	cd sidecar && npm install

sidecar-dev:
	cd sidecar && npm run dev

sidecar-build:
	cd sidecar && npm run build

# Run all services in development
dev-all:
	@echo "Starting Go backend..."
	@make run &
	@sleep 2
	@echo "Starting Node.js sidecar..."
	@cd sidecar && npm run dev &
	@sleep 2
	@echo "Starting React frontend..."
	@cd web && npx vite --host
