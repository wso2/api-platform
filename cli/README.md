# apipctl - WSO2 API Platform CLI

`apipctl` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Quick Start

```bash
# Build the CLI
cd src
make build

# Add a gateway
./build/apipctl gateway add -n dev -s http://localhost:9090

# Generate MCP configuration
./build/apipctl gateway mcp generate -s http://localhost:3001/mcp -o target

# Show version
./build/apipctl version
```

**💡 Tip:** All flags support first-letter shortcuts: `--name` → `-n`, `--server` → `-s`, `--token` → `-t`, etc.

## 📖 Full Documentation

For complete usage instructions, examples, and command reference, see **[src/HELP.md](src/HELP.md)**.

Quick command reference:
- `apipctl gateway add` - Add a gateway configuration
- `apipctl gateway mcp generate` - Generate MCP configuration
- `apipctl version` - Show version information

## Development

### Prerequisites

- Go 1.25.x or higher
- Make

### Building from Source

```bash
cd src

# Build for your OS
make build

# Build for all platforms (Linux, MacOS, Windows - amd64 & arm64)
make build-all

# Run tests
make test

# Clean build artifacts
make clean
```

### Project Structure

```
cli/
├── README.md              # This file
└── src/
    ├── HELP.md           # Complete CLI documentation (single source of truth)
    ├── cmd/              # Command definitions
    │   ├── root.go       # Root command
    │   └── gateway/      # Gateway subcommands
    ├── internal/         # Internal packages
    │   ├── config/       # Config management (~/.apipctl/config.yaml)
    │   ├── gateway/      # Gateway client
    │   └── mcp/          # MCP generator
    ├── utils/            # Shared utilities
    └── main.go           # Entry point
```
