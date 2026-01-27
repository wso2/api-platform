# WSO2 API Platform CLI (AP)

`ap` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Supported Short Flags

| Flag             | Short Flag |
|------------------|------------|
| `--display-name` | `-n`       |
| `--server`       | `-s`       |
| `--output`       | `-o`       |
| `--file`         | `-f`       |
| `--version`      | `-v`       |

## Gateway Sub Commands

> **Note:** Each command supports the `--help` flag for detailed usage information.

## Prerequisites for Gateway Controller Commands

- You must first add and/or select a gateway in the CLI using the appropriate gateway-related commands.
- Credentials for a gateway can come from either the gateway configuration (when you add the gateway) or from environment variables. **Environment variables take precedence** over configuration and will override credentials stored in the config when present.
- Depending on the gateway's authentication type:
  - **none**: No authentication required
  - **basic**: Provide credentials via config or export `WSO2AP_GW_USERNAME=<username>` and `WSO2AP_GW_PASSWORD=<password>` (env vars override config)
  - **bearer**: Provide a token via config or export `WSO2AP_GW_TOKEN=<token>` (env var overrides config)

---

### 1. Add a Gateway

#### CLI Command

```shell
ap gateway add --display-name <name> --server <server> [--auth <none|basic|bearer>]
```

#### Sample Commands

```shell
# Add a gateway with no authentication (default)
ap gateway add --display-name dev --server http://localhost:9090

# Add a gateway with basic authentication
ap gateway add --display-name dev --server http://localhost:9090 --auth basic

# Add a gateway with bearer token authentication
ap gateway add --display-name prod --server https://api.example.com --auth bearer
```

#### Authentication Setup

For **basic** authentication, export these environment variables (replace the placeholders with your values):
```shell
export WSO2AP_GW_USERNAME=<username>
export WSO2AP_GW_PASSWORD=<password>
```

For **bearer** authentication, export this environment variable (replace `<token>` with your token):
```shell
export WSO2AP_GW_TOKEN=<token>
```

**Note:** Environment variables override credentials stored in the gateway configuration.

---

### 2. List Gateways

#### CLI Command

```shell
ap gateway list
```

#### Sample Command

```shell
ap gateway list
```

---

### 3. Remove a Gateway

#### CLI Command

```shell
ap gateway remove --display-name <name>
```

#### Sample Command

```shell
ap gateway remove --display-name dev
```

---

### 4. Change the Gateway

#### CLI Command

```shell
ap gateway use --display-name <name>
```

#### Sample Command

```shell
ap gateway use --display-name dev
```

---

### 5. Check the current Gateway

#### CLI Command

```shell
ap gateway current
```

#### Sample Command

```shell
ap gateway current
```

---

### 6. Returns the health status of the Gateway

#### CLI Command

```shell
ap gateway health
```

#### Sample Command

```shell
ap gateway health
```

---

### 7. Apply a Resource

#### CLI Command

```shell
ap gateway apply --file <path>
```

#### Sample Command

```shell
ap gateway apply --file petstore-api.yaml
```

---

### 8. List all APIs

#### CLI Command

```shell
ap gateway api list
```

#### Sample Command

```shell
ap gateway api list
```

---

### 9. Get a specific API by name and version or id

#### CLI Command

```shell
ap gateway api get --display-name <name> --version <version> --format <json|yaml>
ap gateway api get --id <id> --format <json|yaml>
```

#### Sample Command

```shell
ap gateway api get --display-name "PetStore API" --version v1.0 --format yaml
ap gateway api get --id sample-1 --format yaml
```

---

### 10. Delete an API 

#### CLI Command

```shell
ap gateway api delete --id <id> 
```

#### Sample Command

```shell
ap gateway api delete --id <id>
```

---

### 11. Build a gateway

#### CLI Command

```shell
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

#### Sample Command

```shell
ap gateway image build
```

#### Additional Note for Users

Refer to [this document](customizing-gateway-policies.md) for more information.

### 12. List all MCPs

#### CLI Command

```shell
ap gateway mcp list
```

#### Sample Command

```shell
ap gateway mcp list
```

---

### 13. Retrieves a specific MCP 

#### CLI Command

```shell
ap gateway mcp get --display-name <name> --version <version> --format <json|yaml>
ap gateway mcp get --id <id> --format <json|yaml>
```

#### Sample Command

```shell
ap gateway mcp get --display-name my-mcp --version 1.0.0 --format json
ap gateway mcp get --id sample-id --format json
```

---

### 14. Permanently deletes a MCP

#### CLI Command

```shell
ap gateway mcp delete --id <id> 
```

#### Sample Command

```shell
ap gateway mcp delete --id sample-id
```

---

### 15. Generate MCP 

#### CLI Command

```shell
ap gateway mcp generate --server <server> --output <path>
```

#### Sample Command

```shell
ap gateway mcp generate --server http://localhost:3001/mcp --output target
```
