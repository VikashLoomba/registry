# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the MCP Registry - a community-driven registry service for Model Context Protocol (MCP) servers. It's a Go-based REST API service that provides centralized repository for MCP server entries with search, discovery, and management capabilities.

## Essential Commands

### Development Environment Setup
```bash
# Start the service with MongoDB using Docker Compose (recommended)
docker compose up

# Build the Docker image
docker build -t registry .

# Run the service directly (requires MongoDB running)
go run cmd/registry/main.go

# Build the binary
go build ./cmd/registry
```

### Testing
```bash
# Run all unit tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run integration tests
./integrationtests/run_tests.sh

# Test API endpoints (requires running server)
./scripts/test_endpoints.sh

# Test specific endpoint
./scripts/test_endpoints.sh --endpoint health
```

### Environment Configuration
The service uses environment variables with the `MCP_REGISTRY_` prefix. Key variables:
- `MCP_REGISTRY_DATABASE_URL` - MongoDB connection string (default: mongodb://localhost:27017)
- `MCP_REGISTRY_DATABASE_TYPE` - Database type: mongodb or memory (default: mongodb)
- `MCP_REGISTRY_GITHUB_CLIENT_ID` - GitHub OAuth client ID for authentication
- `MCP_REGISTRY_GITHUB_CLIENT_SECRET` - GitHub OAuth client secret
- `MCP_REGISTRY_SEED_IMPORT` - Import seed data on startup (default: true)

## Architecture Overview

### Layered Architecture
The project follows a clean layered architecture:

1. **HTTP Layer** (`internal/api/`)
   - `server.go` - HTTP server setup with graceful shutdown
   - `router.go` - Route definitions and middleware
   - `handlers/v0/` - Versioned endpoint handlers

2. **Service Layer** (`internal/service/`)
   - `registry_service.go` - Business logic implementation
   - `service.go` - Service interface definitions
   - Context-aware operations with proper timeout handling

3. **Database Layer** (`internal/database/`)
   - `database.go` - Database interface
   - `mongodb.go` - MongoDB implementation
   - `memory.go` - In-memory implementation for development
   - Supports pagination, search, and filtering

4. **Authentication** (`internal/auth/`)
   - GitHub OAuth implementation
   - Ephemeral token generation (1-hour validity, HMAC-SHA256 signed)
   - Repository ownership validation
   - Extensible design for additional auth methods

5. **Models** (`internal/model/`)
   - Core domain entities (Server, Package, Repository, etc.)
   - Input validation structures
   - Authentication method definitions

### Key API Endpoints
- `GET /v0/health` - Health check
- `GET /v0/servers` - List servers with pagination
- `GET /v0/servers/{id}` - Get server details
- `GET /v0/search` - Search servers (text search, registry filtering)
- `POST /v0/publish` - Publish new server (requires auth)
- `POST /v0/publish-oss` - Publish OSS server (requires auth)
- `POST /v0/authorize` - Generate ephemeral auth token
- `GET /v0/swagger/index.html` - Swagger UI documentation

### Authentication Flow
1. User authenticates via GitHub OAuth
2. System generates ephemeral token (1-hour validity)
3. Token contains GitHub user ID, username, timestamps, and nonce
4. Token validated against repository ownership for publish operations

### Database Operations Pattern
All database operations follow this pattern:
- Interface-based design for easy testing
- Context with timeout (5 seconds default)
- Consistent error types (NotFound, AlreadyExists, InvalidInput)
- Cursor-based pagination for list operations

## Development Guidelines

### Adding New Endpoints
1. Define handler in `internal/api/handlers/v0/`
2. Add route in `internal/api/router.go`
3. Update OpenAPI spec in `docs/openapi.yaml`
4. Add integration tests if needed

### Working with Database
- Always use the `Database` interface, not concrete implementations
- Handle context cancellation properly
- Use structured errors from `internal/database/errors.go`

### Testing Approach
- Unit tests alongside implementation files (*_test.go)
- Integration tests in `integrationtests/` directory
- Use fake implementations for testing (see `internal/service/fake_service.go`)
- Test both MongoDB and memory database implementations

### Common Patterns
- Dependency injection via constructor functions
- Context propagation throughout the stack
- Interface-based design for testability
- Factory functions for creating services
- Environment-based configuration