# API Platform CLI

A command-line interface tool for managing API Platform operations, designed for developers and CI/CD automation workflows with support for gateway management, API deployment, and API key management.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Architecture](#architecture)
  - [Core Components](#core-components)
  - [Command Structure](#command-structure)
- [Command Categories](#command-categories)
  - [Gateway Commands](#gateway-commands)
  - [API Commands](#api-commands)
  - [Key Management](#key-management)
  - [Configuration](#configuration)
- [Use Cases](#use-cases)
  - [Local Development](#local-development)
  - [CI/CD Pipeline](#cicd-pipeline)
- [Design Principles](#design-principles)

---

## Overview

The API Platform CLI provides command-line tools for developers and automation workflows to interact with the API Platform. It enables seamless gateway management, API deployment, and API key operations through a consistent, developer-friendly interface.

## Key Features

- **Gateway Management**: List, configure, and manage gateway instances
- **API Deployment**: Push API definitions to gateways with validation
- **API Key Operations**: Generate, list, and revoke API keys
- **Configuration Management**: Manage profiles, contexts, and authentication
- **CI/CD Integration**: Designed for automation workflows and pipelines
- **Multiple Output Formats**: Support for JSON, YAML, and table formats
- **Developer-Friendly**: Clear error messages and helpful command suggestions

---

## Architecture

### Core Components

The CLI consists of the following components:

#### Command Structure
- **Hierarchical Organization**: Logical grouping of commands by domain
- **Consistent Parameters**: Standard naming conventions across commands
- **Output Formatting**: Flexible output formats (JSON, YAML, table)

#### Integration Points
- **Platform API**: Communicates with Management Portal APIs
- **Authentication**: Supports API keys and OAuth
- **Local Operations**: File-based API definition management and validation

### Command Structure

```
api-platform
├── gateway
│   ├── list
│   ├── push
│   ├── status
│   └── api-key
│       ├── generate
│       ├── list
│       └── revoke
├── api
│   ├── validate
│   ├── import
│   └── export
├── login
└── config
    ├── set
    └── get
```

---

## Command Categories

### Gateway Commands

Manage gateway instances and operations:

```bash
# List all gateways
api-platform gateway list

# Push API to gateway
api-platform gateway push --file api.yaml --gateway dev-gateway

# Get gateway status
api-platform gateway status --name dev-gateway
```

### API Commands

Import, export, and validate API definitions:

```bash
# Validate API specification
api-platform api validate --file api.yaml

# Import API definition
api-platform api import --file api.yaml

# Export API definition
api-platform api export --api-name 'MyAPI' --output api.yaml
```

### Key Management

Generate and manage API keys:

```bash
# Generate API key
api-platform gateway api-key generate \
  --api-name 'Weather API' \
  --key-name 'my-app-key'

# List API keys
api-platform gateway api-key list

# Revoke API key
api-platform gateway api-key revoke --key-id <key-id>
```

### Configuration

Manage CLI profiles and authentication:

```bash
# Login with API key
api-platform login --api-key $API_KEY

# Set default context
api-platform config set --context production

# Get configuration
api-platform config get
```

---

## Use Cases

### Local Development

Developer workflow for testing APIs locally:

```bash
# Push API definition to local gateway
api-platform gateway push --file api.yaml

# Generate API key for testing
api-platform gateway api-key generate \
  --api-name 'My API' \
  --key-name 'dev-key'

# Test API with generated key
curl http://localhost:8081/myapi -H 'api-key: <generated-key>'
```

**Best For**: Rapid development, local testing, and debugging

### CI/CD Pipeline

Automated API deployment workflow:

```bash
# Authenticate
api-platform login --api-key $CI_API_KEY

# Validate API spec
api-platform api validate --file api.yaml

# Deploy to gateway
api-platform gateway push \
  --file api.yaml \
  --gateway production-gateway

# Verify deployment
api-platform gateway status --name production-gateway
```

**Best For**: Continuous deployment, automated testing, production deployments

---

## Design Principles

### Consistency
- Predictable command patterns across all operations
- Standard flag naming conventions
- Uniform output formats

### Developer Experience
- Clear, actionable error messages
- Helpful command suggestions for common mistakes
- Interactive prompts when additional input is needed
- Progress indicators for long-running operations

### Automation-Friendly
- Scriptable commands with consistent exit codes
- Machine-readable output formats (JSON, YAML)
- Environment variable support for configuration
- Non-interactive mode for CI/CD pipelines
