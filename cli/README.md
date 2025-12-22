# WSO2 API Platform CLI (AP)

`ap` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Quick Start

```bash
# Build the CLI
cd cli/src
make build # Build 
make build-all # Build all OS binaries
make build-skip-tests # Build without tests
make test # Run tests
make clean # Clean build artifacts
```

## CLI Project Structure
```
cli/
├── README.md             # This file (single source of truth)
└── src/ 
    ├── cmd/              # Command definitions
    │   ├── root.go       # Root command implementation
    │   └── gateway/      # Gateway subcommands
    ├── internal/         # Internal packages
    ├── utils/            # Shared utilities
    └── main.go           # Entry point
```

In the cmd directory, each subcommand has its own folder, and the root.go file within that folder contains the implementation of the corresponding subcommand.

All command verbs are implemented as separate Go files, each serving as the base implementation for that specific command.

## Supported Short Flags
| Flag             | Short Flag |
|------------------|------------|
| `--display-name` | `-n`       |
| `--server`       | `-s`       |
| `--output`       | `-o`       |
| `--file`         | `-f`       |
| `--version`      | `-v`       |

## Gateway Sub Commands

---

### 1. Add a Gateway

**CLI Command**

```
ap gateway add --display-name <name> --server <server>
```

**Sample Command**

```
ap gateway add --display-name dev --server http://localhost:9090
```

---

### 2. List Gateways

**CLI Command**

```
ap gateway list
```

**Sample Command**

```
ap gateway list
```

---

### 3. Remove a Gateway

**CLI Command**

```
ap gateway remove --display-name <name>
```

**Sample Command**

```
ap gateway remove --display-name dev
```

---

### 4. Change the Gateway

**CLI Command**

```
ap gateway use --display-name <name>
```

**Sample Command**

```
ap gateway use --display-name <name>
```

---

### 5. Check the current Gateway

**CLI Command**

```
ap gateway current
```

**Sample Command**

```
ap gateway current
```

---

### 6. Returns the health status of the Gateway

**CLI Command**

```
ap gateway health
```

**Sample Command**

```
ap gateway health
```

---

### 7. Apply a Resource

**CLI Command**

```
ap gateway apply --file <path>
```

**Sample Command**

```
ap gateway apply--file petstore-api.yaml
```

---

### 8. List all APIs

**CLI Command**

```
ap gateway api list
```

**Sample Command**

```
ap gateway api list
```

---

### 9. Get a specific API by name and version or id

**CLI Command**

```
ap gateway api get --display-name <name> --version <version> --format <json|yaml>
ap gateway api get --id <id> --format <json|yaml>
```

**Sample Command**

```
ap gateway api get --display-name "PetStore API" --version v1.0 --format yaml
ap gateway api get --id sample-1 --format yaml
```

---

### 10. Delete an API 

**CLI Command**

```
ap gateway api delete --id <id> 
```

**Sample Command**

```
ap gateway api delete --id <id>
```

---

### 11. Build a gateway

**CLI Command**

```
ap gateway image build \
  [--name <gateway-name>] \
  [--path <gateway-project-dir>]
  [--repository <image-repository>] \
  [--version <gateway-version>] \
  [--gateway-builder <gateway-builder-image>] \
  [--gateway-controller-base-image <gateway-controller-base-image>] \
  [--router-base-image <router-base-image>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--offline] \
  [--output-dir <output_dir>]
```

**Sample Command**

```
ap gateway build
```

---

### 12. List all MCPs

**CLI Command**

```
ap gateway mcp list
```

**Sample Command**

```
ap gateway mcp list
```

---

### 13. Retrieves a specific MCP 

**CLI Command**

```
ap gateway mcp get --display-name <name> --version <version> --format <json|yaml>
ap gateway mcp get --id <id> --format <json|yaml>
```

**Sample Command**

```
ap gateway mcp get --display-name my-mcp --version 1.0.0 --format json
ap gateway mcp get --id sample-id --format json
```

---

### 14. Permanently deletes a MCP

**CLI Command**

```
ap gateway mcp delete --id <id> 
```

**Sample Command**

```
ap gateway mcp delete --id sample-id
```

---

### 15. Generate MCP 

**CLI Command**

```
ap gateway mcp generate --server <server> --output <path>
```

**Sample Command**

```
ap gateway mcp generate --server http://localhost:3001/mcp --output target
```
