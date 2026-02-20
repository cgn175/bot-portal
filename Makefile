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
