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
ap gateway add --display-name <name> --server <server> [--platform <platform>] [--auth <none|basic|bearer>]
```

#### Sample Commands

```shell
# Add a gateway with no authentication (default)
ap gateway add --display-name dev --platform eu --server http://localhost:9090

# Add a gateway with basic authentication
ap gateway add --display-name dev --platform eu --server http://localhost:9090 --auth basic

# Add a gateway with bearer token authentication
ap gateway add --display-name prod --platform eu --server https://api.example.com --auth bearer
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
ap gateway list --platform <platform>
```

#### Sample Command

```shell
ap gateway list --platform eu
```

---

### 3. Remove a Gateway

#### CLI Command

```shell
ap gateway remove --display-name <name> --platform <platform>
```

#### Sample Command

```shell
ap gateway remove --display-name dev --platform eu
```

---

### 4. Change the Gateway

#### CLI Command

```shell
ap gateway use --display-name <name> --platform <platform>
```

#### Sample Command

```shell
ap gateway use --display-name dev --platform eu
```

---

### 5. Check the current Gateway

#### CLI Command

```shell
ap gateway current --platform <platform>
```

#### Sample Command

```shell
ap gateway current --platform eu
```

---

### 6. Returns the health status of the Gateway

#### CLI Command

```shell
ap gateway health --platform <platform>
```

#### Sample Command

```shell
ap gateway health --platform eu
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
ap gateway rest-api list
```

#### Sample Command

```shell
ap gateway rest-api list
```

---

### 9. Get a specific API by name and version or id

#### CLI Command

```shell
ap gateway rest-api get --display-name <name> --version <version> --format <json|yaml>
ap gateway rest-api get --id <id> --format <json|yaml>
```

#### Sample Command

```shell
ap gateway rest-api get --display-name "PetStore API" --version v1.0 --format yaml
ap gateway rest-api get --id sample-1 --format yaml
```

---

### 10. Delete an API 

#### CLI Command

```shell
ap gateway rest-api delete --id <id> 
```

#### Sample Command

```shell
ap gateway rest-api delete --id <id>
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

---

## DevPortal Sub Commands

### 1. Add a DevPortal

#### CLI Command

```shell
ap devportal add --display-name <portal-name> --server <url> --platform <platform> --auth <basic|oauth|api-key> [--username <username>] [--password <password>] [--token <token>] [--api-key <api-key>] [--no-interactive]
```

#### Sample Commands

```shell
# Add a DevPortal with basic auth
ap devportal add --display-name my-portal --platform eu --server https://devportal.example.com --auth basic

# Add a DevPortal with OAuth auth
ap devportal add --display-name my-portal --platform eu --server https://devportal.example.com --auth oauth

# Add a DevPortal without interactive prompts
ap devportal add --display-name my-portal --platform eu --server https://devportal.example.com --auth api-key --no-interactive --api-key <api-key>
```

#### Authentication Setup

For DevPortal authentication, export the environment variables for the configured auth type:

```shell
export WSO2AP_DEVPORTAL_USERNAME=<username>
export WSO2AP_DEVPORTAL_PASSWORD=<password>
export WSO2AP_DEVPORTAL_TOKEN=<token>
export WSO2AP_DEVPORTAL_API_KEY=<api-key>
```

**Note:** The environment variable can be used instead of storing the API key in the CLI configuration.

---

### 2. List DevPortals

#### CLI Command

```shell
ap devportal list --platform <platform>
```

### 3. Remove a DevPortal

#### CLI Command

```shell
ap devportal remove --display-name <portal-name> --platform <platform>
```

### 4. Set the Active DevPortal

#### CLI Command

```shell
ap devportal use --display-name <portal-name> --platform <platform>
```

### 5. Show the Current DevPortal

#### CLI Command

```shell
ap devportal current --platform <platform>
```

### 6. Check DevPortal Health

#### CLI Command

```shell
ap devportal health --platform <platform>
```
