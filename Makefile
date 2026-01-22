# ================================================
# SPSC loanEasy v1.0 - Makefile
# ================================================

.PHONY: help run-dev run-prod build swagger clean tidy test

# Default target
help:
	@echo "================================================"
	@echo "  SPSC loanEasy v1.0 - Available Commands"
	@echo "================================================"
	@echo ""
	@echo "  make run-dev     - Run in development mode"
	@echo "  make run-prod    - Run in production mode"
	@echo "  make build       - Build binary"
	@echo "  make swagger     - Generate Swagger docs"
	@echo "  make tidy        - Tidy go modules"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make migrate     - Run database migrations"
	@echo ""

# Run in development mode
run-dev:
	@echo "🚀 Starting server in DEVELOPMENT mode..."
	APP_MODE=dev go run cmd/server/main.go

# Run in production mode
run-prod:
	@echo "🚀 Starting server in PRODUCTION mode..."
	APP_MODE=prod go run cmd/server/main.go

# Build binary
build:
	@echo "📦 Building binary..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server cmd/server/main.go
	@echo "✅ Binary built: bin/server"

# Generate Swagger documentation
swagger:
	@echo "📝 Generating Swagger documentation..."
	swag init -g cmd/server/main.go -o docs
	@echo "✅ Swagger docs generated in ./docs"

# Tidy go modules
tidy:
	@echo "🧹 Tidying go modules..."
	go mod tidy
	@echo "✅ Go modules tidied"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "🗑️  Cleaning build artifacts..."
	rm -rf bin/
	@echo "✅ Cleaned"

# Install dependencies
deps:
	@echo "📥 Installing dependencies..."
	go mod download
	@echo "✅ Dependencies installed"

# Install swag CLI
install-swag:
	@echo "📥 Installing swag CLI..."
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✅ swag installed"

# Run database migrations (Phase 2+)
migrate:
	@echo "🗄️  Running database migrations..."
	go run cmd/migrate/main.go
	@echo "✅ Migrations completed"

# Full setup (install deps + generate swagger)
setup: deps install-swag swagger
	@echo "✅ Setup complete!"
