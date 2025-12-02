# Envoy Policy Engine

A flexible, extensible HTTP request and response processing system that integrates with Envoy Proxy via the ext_proc filter.

## Overview

The Envoy Policy Engine is an external processor service consisting of three major components:

1. **Policy Engine Runtime** - Core framework (kernel + worker + interfaces) with ZERO built-in policies
2. **Policy Engine Builder** - Build-time tooling that discovers, validates, and compiles custom policies
3. **Sample Policy Implementations** - Optional reference examples (SetHeader, JWT, Rate Limiting, etc.)

### Critical Architecture Note

**The Policy Engine runtime ships with NO policies by default.** ALL policies (including sample/reference policies) must be compiled via the Builder. This ensures minimal attack surface and allows you to include only the policies you need.

## Quick Start

For detailed setup instructions, see [Quickstart Guide](specs/001-policy-engine/quickstart.md).

### Prerequisites

- Docker and Docker Compose
- Go 1.23+ (for custom policy development)

### Build Policy Engine with Sample Policies

First build the Gateway Builder image:
```bash
make build-builder
```

Then, use the following commands to build the Policy Engine runtime with sample policies and Dockerfiles of other components:
```bash
# Build using Gateway Builder
docker run --rm \
    -v $(pwd)/sample-policies:/workspace/sample-policies \
    -v $(pwd)/policy-manifest.yaml:/workspace/policy-manifest.yaml \
    -v $(pwd)/output:/workspace/output \
    wso2/api-platform-gateway-builder:latest
```

# Build gateway images

```bash
cd output
cd policy-engine
docker build -t myregistry/policy-engine:v1.0.0 .
cd ../gateway-controller
docker build -t myregistry/gateway-controller:v1.0.0 .
cd ../router
docker build -t myregistry/router:v1.0.0 .
```

# Start development environment

docker-compose up -d

# Test

```bash
curl http://localhost:8000/api/v1/public/health
```

## Architecture

### Components

- **Kernel Layer** (`src/kernel/`): Envoy integration via ext_proc and xDS protocols
- **Worker Layer** (`src/worker/`): Policy chain execution engine with CEL evaluation
- **Policies** (`policies/`): Optional sample policy implementations (NOT bundled with runtime)
- **Builder** (`build/`): Go-based build tooling for policy compilation

### Key Features

- ✅ Route-based policy chains
- ✅ Dynamic configuration via xDS (zero-downtime updates)
- ✅ Policy versioning (multiple versions can coexist)
- ✅ Conditional execution (CEL expressions)
- ✅ Short-circuit logic (authentication failures stop processing)
- ✅ Inter-policy communication (shared metadata)
- ✅ Dynamic body processing optimization (SKIP vs BUFFERED modes)
- ✅ Custom policy development framework

## Documentation

- **Full Specification**: [Spec.md](Spec.md)
- **Implementation Plan**: [specs/001-policy-engine/plan.md](specs/001-policy-engine/plan.md)
- **Builder Design**: [BUILDER_DESIGN.md](BUILDER_DESIGN.md)
- **Data Model**: [specs/001-policy-engine/data-model.md](specs/001-policy-engine/data-model.md)
- **Quickstart Guide**: [specs/001-policy-engine/quickstart.md](specs/001-policy-engine/quickstart.md)
- **Policy API Contracts**: [specs/001-policy-engine/contracts/policy-api.md](specs/001-policy-engine/contracts/policy-api.md)

## Development

### Project Structure

```
src/                    # Policy Engine runtime framework (NO built-in policies)
policies/               # Sample policy implementations (OPTIONAL)
build/                  # Builder Go application
templates/              # Code generation templates
configs/                # Configuration files
tests/                  # Unit, integration, and contract tests
```

### Creating Custom Policies

See [Quickstart Guide](specs/001-policy-engine/quickstart.md) for detailed instructions on creating custom policies.

## Testing

```bash
# Run unit tests
cd src
go test ./...

# Run integration tests
cd tests/integration
docker-compose up --abort-on-container-exit

# Run contract tests
cd tests/contract
go test ./...
```

## License

[Add License Information]

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.
