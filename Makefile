.PHONY: help build run test clean setup docker-up docker-down install lint

# Default target
help:
	@echo "TeleHook - Available Commands:"
	@echo ""
	@echo "  make build       - Build the application"
	@echo "  make run         - Run the application"
	@echo "  make test        - Run API tests"
	@echo "  make setup       - Setup database"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make install     - Install dependencies"
	@echo "  make docker-up   - Start with Docker Compose"
	@echo "  make docker-down - Stop Docker containers"
	@echo "  make lint        - Run go fmt and vet"
	@echo ""

# Build the application
build:
	@echo "Building application..."
	@go build -o telehook cmd/server/main.go
	@echo "Build complete: telehook"

# Run the application
run:
	@echo "Starting server..."
	@go run cmd/server/main.go

# Run API tests
test:
	@echo "Running API tests..."
	@./test_api.sh

# Setup database
setup:
	@echo "Setting up database..."
	@./setup_db.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f main telehook
	@echo "Clean complete"

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed"

# Docker Compose up
docker-up:
	@echo "Starting with Docker Compose..."
	@docker-compose up -d
	@echo "Services started. Check with: docker-compose ps"

# Docker Compose down
docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down
	@echo "Services stopped"

# Lint code
lint:
	@echo "Running go fmt..."
	@go fmt ./...
	@echo "Running go vet..."
	@go vet ./...
	@echo "Linting complete"

# Build for production
prod:
	@echo "Building for production..."
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o telehook cmd/server/main.go
	@echo "Production build complete: telehook"

# Run with live reload (requires air: go install github.com/air-verse/air@latest)
dev:
	@echo "Starting development server with live reload..."
	@air || echo "air not installed. Install with: go install github.com/air-verse/air@latest"
