# Platform Bootstrap Implementation

## Entry Points

- `platform-api/cmd/main.go` – loads configuration and starts the HTTPS server.
- `platform-api/internal/server/server.go` – wires repositories, services, and Gin router before serving TLS.
- `platform-api/config/config.go` – merges the layered `-config` file(s) over built-in defaults and resolves `{{ env }}` / `{{ file }}` interpolation tokens (there is no environment-variable provider; env values enter only through those tokens).
- `platform-api/internal/database/connection.go` – opens the database connection and enforces schema initialization.

## Behaviour

1. `main()` obtains a singleton `config.Server` instance.
2. `StartPlatformAPIServer` initializes the SQLite schema, constructs repositories/services/handlers, and returns a server wrapper.
3. `Server.Start` loads or generates TLS certificates and runs `ListenAndServeTLS`.

## Verification

- Build and run `go run ./cmd/main.go -config config/config.toml` within `platform-api`; confirm log output shows schema load and HTTPS startup. The `-config` flag is now **required** and repeatable (`-config base.toml -config overlay.toml`, merged last-wins) — the binary exits non-zero if no `-config` is given rather than booting on built-in defaults.
