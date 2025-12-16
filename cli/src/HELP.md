# apipctl - WSO2 API Platform CLI

`apipctl` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Installation

```bash
# Build from source
cd cli/src
make build

# The binary will be available at
./build/apipctl
```

## Usage

```bash
apipctl [command] [subcommand] [flags]
```

**Note:** All flags support shorthand notation using their first letter. For example:
- `--name` can be used as `-n`
- `--server` can be used as `-s`
- `--token` can be used as `-t`
- `--output` can be used as `-o`
- `--insecure` can be used as `-i`

## Commands

### Gateway Management

#### Add a Gateway

Add a gateway configuration to connect to your API Platform Gateway instance.

```bash
apipctl gateway add --name <name> --server <url> [--token <token>] [--insecure]
```

**Examples:**

```bash
# Add a local development gateway
apipctl gateway add --name dev --server http://localhost:9090

# Add a production gateway with authentication
apipctl gateway add --name prod --server https://api.example.com --token <TOKEN>

# Add a gateway with self-signed certificate (skip TLS verification)
apipctl gateway add --name local --server https://localhost:9090 --insecure

# Using shorthand flags
apipctl gateway add -n dev -s http://localhost:9090 -t SECRET_TOKEN -i
```

**Flags:**
- `-n, --name` (required): Name to identify the gateway
- `-s, --server` (required): Server URL of the gateway
- `-t, --token`: Authentication token for API requests
- `-i, --insecure`: Skip TLS certificate verification

**Output:**
```
Gateway in <url> added as <name>
Configuration saved to: ~/.apipctl/config.yaml
```

---

#### Generate MCP Configuration

Generate Model Context Protocol (MCP) configuration from an MCP server.

```bash
apipctl gateway mcp generate --server <url> --output <path>
```

**Examples:**

```bash
# Generate MCP configuration
apipctl gateway mcp generate --server http://localhost:3001/mcp --output target

# Generate in current directory
apipctl gateway mcp generate --server http://localhost:3001/mcp --output .

# Using shorthand flags
apipctl gateway mcp generate -s http://localhost:3001/mcp -o target
```

**Flags:**
- `-s, --server` (required): MCP server URL
- `-o, --output`: Output directory for generated configuration (default: current directory)

**Output:**
```
Generating MCP configuration for server: http://localhost:3001/mcp
→ Sending initialize...
→ Captured Session ID: <session-id>
---------------------------------------------------
→ Sending notifications/initialized...
---------------------------------------------------
→ Sending tools/list...
→ Available Tools: X
---------------------------------------------------
→ Sending prompts/list...
→ Available Prompts: X
---------------------------------------------------
→ Sending resources/list...
→ Available Resources: X
---------------------------------------------------
→ Generated MCP configuration YAML file: <output-path>/generated-mcp.yaml
MCP generated successfully.
```

---

### Version

Display the CLI version and build information.

```bash
apipctl version
```

**Output:**
```
apipctl version v0.0.1 (built at 2025-12-16T05:35:30Z)
```

---

## Configuration

Gateway configurations are stored in `~/.apipctl/config.yaml`.

**Example configuration:**

```yaml
gateways:
  - name: dev
    server: http://localhost:9090
    insecure: true
  - name: prod
    server: https://api.example.com
    token: SECRET_TOKEN
  - name: local
    server: https://localhost:9090
    insecure: true
activeGateway: dev
configVersion: 1.0.0
```

**Security Notes:**
- **Tokens are automatically encrypted** using AES-256-GCM encryption when stored in the config file
- The encryption key is derived from machine-specific information (hostname + home directory)
- Tokens are automatically decrypted when the CLI uses them
- This provides security while maintaining cross-platform compatibility

**Note:** Typically, you would use either `token` for authenticated connections or `insecure` for development environments with self-signed certificates, not both together.

---

## Getting Help

Get help for any command:

```bash
apipctl --help
apipctl gateway --help
apipctl gateway add --help
apipctl gateway mcp --help
apipctl gateway mcp generate --help
```
