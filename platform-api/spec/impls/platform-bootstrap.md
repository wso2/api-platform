# Platform Bootstrap Implementation

## Entry Points

- `platform-api/cmd/main.go` – loads configuration and starts the HTTPS server.
- `platform-api/internal/server/server.go` – wires repositories, services, and Gin router before serving TLS.
- `platform-api/config/config.go` – parses environment variables and sets SQLite defaults.
- `platform-api/internal/database/connection.go` – opens the database connection and enforces schema initialization.

## Behaviour

1. `main()` obtains a singleton `config.Server` instance.
2. `StartPlatformAPIServer` initializes the SQLite schema, constructs repositories/services/handlers, and returns a server wrapper.
3. `Server.Start` loads or generates TLS certificates and runs `ListenAndServeTLS`.

## Verification

- Build and run `go run ./cmd/main.go` within `platform-api`; confirm log output shows schema load and HTTPS startup.
