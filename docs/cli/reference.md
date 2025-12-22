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
- Either:
  - Export the environment variables `WSO2AP_GW_USERNAME` and `WSO2AP_GW_PASSWORD`, or
  - Use an alternative mechanism to ensure these credentials are available in the environment.
- The Gateway is secured, and the CLI uses Basic Authentication to communicate with it.

---

### 1. Add a Gateway

#### CLI Command

```
ap gateway add --display-name <name> --server <server>
```

#### Sample Command

```
ap gateway add --display-name dev --server http://localhost:9090
```

---

### 2. List Gateways

#### CLI Command

```
ap gateway list
```

#### Sample Command

```
ap gateway list
```

---

### 3. Remove a Gateway

#### CLI Command

```
ap gateway remove --display-name <name>
```

#### Sample Command

```
ap gateway remove --display-name dev
```

---

### 4. Change the Gateway

#### CLI Command

```
ap gateway use --display-name <name>
```

#### Sample Command

```
ap gateway use --display-name <name>
```

---

### 5. Check the current Gateway

#### CLI Command

```
ap gateway current
```

#### Sample Command

```
ap gateway current
```

---

### 6. Returns the health status of the Gateway

#### CLI Command

```
ap gateway health
```

#### Sample Command

```
ap gateway health
```

---

### 7. Apply a Resource

#### CLI Command

```
ap gateway apply --file <path>
```

#### Sample Command

```
ap gateway apply --file petstore-api.yaml
```

---

### 8. List all APIs

#### CLI Command

```
ap gateway api list
```

#### Sample Command

```
ap gateway api list
```

---

### 9. Get a specific API by name and version or id

#### CLI Command

```
ap gateway api get --display-name <name> --version <version> --format <json|yaml>
ap gateway api get --id <id> --format <json|yaml>
```

#### Sample Command

```
ap gateway api get --display-name "PetStore API" --version v1.0 --format yaml
ap gateway api get --id sample-1 --format yaml
```

---

### 10. Delete an API 

#### CLI Command

```
ap gateway api delete --id <id> 
```

#### Sample Command

```
ap gateway api delete --id <id>
```

---

### 11. Build a gateway

#### CLI Command

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

#### Sample Command

```
ap gateway build
```

#### Additional Note for Users

Use `ap gateway build --help` to view detailed usage information for this command.

Docker is a prerequisite for executing this command. If Docker is not available, the command will validate this at the beginning and fail immediately.

A `policy_manifest.yaml` file is mandatory. By default, the CLI expects this file to be present in the current working directory unless a specific path is provided.

**Sample `policy_manifest.yaml`:**

```yaml
version: v1
versionResolution: minor
policies:
  - name: basic-auth
    version: v1.0.0
    versionResolution: exact
  - name: custom-auth
    version: v2.0.0
    filePath: ./sample.zip
```

Policies defined with a `filePath` are treated as **local policies**:

- These policies are not cached or managed through PolicyHub.

All other policies are treated as **PolicyHub policies**:

- They are cached locally and validated using PolicyHub checksums.
- If a policy is missing or the checksum does not match, it will be downloaded or replaced automatically.

Even an output directory is not explicitly specified, the built images are cached by default. On subsequent executions, the cache is cleared and replaced with newly built images. The cache location is displayed in the command output.

The `policy_manifest_lock.yaml` file will be automatically generated or updated with the resolved policies in the path. This file can be used later for offline mode if needed.

When running in **offline mode**, a `policy_manifest_lock.yaml` file is mandatory in the same path. Offline mode means the CLI will not communicate with PolicyHub:

- PolicyHub policies must already be cached locally. If they are not available, the CLI will prompt you to run the command in online mode at least once.
- Local policies continue to work as expected, as they are not dependent on PolicyHub.

**Non-self-explanatory flags:**

- `--name` (`-n`): Sets the gateway name. If not provided, the current directory name is used by default.
- `--path`: Specifies the directory containing `policy_manifest.yaml`. By default, this is the current working directory.
- `push`: Pushes the built images to Docker.
- `no-cache`: Disables Docker build cache usage.
- `--platform`: Specifies the target platform. By default, the host platform is used.

---

### 12. List all MCPs

#### CLI Command

```
ap gateway mcp list
```

#### Sample Command

```
ap gateway mcp list
```

---

### 13. Retrieves a specific MCP 

#### CLI Command

```
ap gateway mcp get --display-name <name> --version <version> --format <json|yaml>
ap gateway mcp get --id <id> --format <json|yaml>
```

#### Sample Command

```
ap gateway mcp get --display-name my-mcp --version 1.0.0 --format json
ap gateway mcp get --id sample-id --format json
```

---

### 14. Permanently deletes a MCP

#### CLI Command

```
ap gateway mcp delete --id <id> 
```

#### Sample Command

```
ap gateway mcp delete --id sample-id
```

---

### 15. Generate MCP 

#### CLI Command

```
ap gateway mcp generate --server <server> --output <path>
```

#### Sample Command

```
ap gateway mcp generate --server http://localhost:3001/mcp --output target
```
