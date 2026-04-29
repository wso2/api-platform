# Policy Languages and Runtimes

## Overview

The API Platform Gateway uses a **policy-based architecture** to intercept and transform API traffic flowing through the data plane. Policies are self-contained units of logic that run inside the Gateway Runtime, with the ability to inspect and modify requests, responses, headers, and body content at each stage of the processing pipeline.

The gateway supports **two languages** for authoring policies:

| Language | Runtime | Best For |
|----------|---------|----------|
| **Go** (default) | Compiled into the Policy Engine binary | Standard API policies — authentication, rate limiting, header manipulation, guardrails |
| **Python** (beta) | Executed by the Python Executor | AI/ML workloads, prompt engineering, complex data transformations, and scenarios that benefit from Python's rich ecosystem |

Go is the **primary and recommended language** for policy development. It provides maximum execution performance, strict type safety, and minimal per-request latency. Python is available as a **specialized runtime** for use cases where access to Python-native libraries (NLP toolkits, compression engines, ML inference clients, etc.) outweighs the overhead of cross-process communication.

## How Policies Execute

Understanding where each language fits requires a brief look at the Gateway Runtime architecture:

```
                        ┌──────────────────────────────────────┐
                        │          Gateway Runtime             │
  Incoming              │                                      │
  Request  ────────►    │  ┌────────┐      ┌──────────────┐    │     Upstream
                        │  │ Router │─────►│ Policy Engine│────┼────► Backend
  Response ◄────────    │  │(Envoy) │◄─────│   (Go)       │    │
                        │  └────────┘      └──────┬───────┘    │
                        │                         │ gRPC/UDS   │
                        │                  ┌──────▼───────┐    │
                        │                  │   Python     │    │
                        │                  │   Executor   │    │
                        │                  └──────────────┘    │
                        └──────────────────────────────────────┘
```

- **Go policies** are compiled directly into the **Policy Engine** binary at image build time. When the Router hands off a request to the Policy Engine via the `ext_proc` filter, Go policies execute in-process with zero serialization overhead.

- **Python policies** run in a dedicated **Python Executor** process. The Go Policy Engine delegates execution to the Python Executor over a local gRPC connection using a Unix Domain Socket. The executor manages policy lifecycle — loading, initialization, execution, and teardown in an isolated Python runtime.

> **Note:** Both Go and Python policies share the same policy evaluation pipeline. From the perspective of API configuration and deployment, a policy's language is transparent — you attach Go and Python policies to APIs in exactly the same way.

## Policy Anatomy

Regardless of language, every policy consists of two parts:

## 1. Policy Definition (`policy-definition.yaml`)

A declarative YAML file that describes the policy's identity, version, and configuration schema. This file is the same for both Go and Python policies.

```yaml
name: my-policy
version: v1.0.0
displayName: My Policy
description: |
  A short description of what this policy does.

parameters:
  type: object
  properties:
    myParam:
      type: string
      description: "An example parameter."
      default: "hello"
  required:
    - myParam

systemParameters:
  type: object
  additionalProperties: false
  properties: {}
```

| Field | Purpose |
|-------|---------|
| `name` | Unique policy identifier, used in API definitions to reference the policy |
| `version` | Semantic version (e.g., `v1.0.0`). The major version is used as the policy version qualifier |
| `parameters` | JSON Schema describing the user-configurable parameters for the policy |
| `systemParameters` | JSON Schema for operator-level configuration (set via gateway config, not per-API) |

## 2. Policy Implementation

The implementation is where the two languages diverge.

### 2.1. Go Policies

Go is the **default and recommended** language for policy development. Every built-in policy that ships with the gateway — authentication, rate limiting, CORS, guardrails, header manipulation — is written in Go.

### Why Go?

- **Performance:** Compiled into the Policy Engine binary. No serialization, no IPC, no interpreter overhead.
- **Type safety:** Compile-time guarantees reduce runtime errors in production.
- **Ecosystem alignment:** The Policy Engine, Gateway Builder, and Gateway Controller are all Go codebases.
- **Broad applicability:** Ideal for the vast majority of API management use cases.

### Go Policy Structure

A Go policy is a standard **Go module** containing the policy definition and the implementation:

```
my-go-policy/
├── policy-definition.yaml
├── go.mod
├── go.sum
├── policy.go
└── policy_test.go
```

| File | Purpose |
|------|---------|
| `policy-definition.yaml` | Declares name, version, and parameter schema |
| `go.mod` / `go.sum` | Go module definition and dependency lockfiles |
| `policy.go` | Policy implementation |
| `policy_test.go` | Unit tests for the policy logic |

Go policies implement interfaces from the gateway's Policy SDK. The Policy Engine loads them at build time via the `build.yaml` manifest.

### Build Integration

Go policies are referenced in `build.yaml` using the `gomodule` field, which points to the Go module path:

```yaml
policies:
  - name: my-go-policy
    gomodule: github.com/wso2/gateway-controllers/policies/my-go-policy@v1
```

The **Gateway Builder** resolves these modules, compiles them into the Policy Engine binary, and produces a custom gateway image containing all declared policies.

### 2.2. Python Policies (Beta)

Python policy support extends the gateway's capabilities into domains where Python's ecosystem is unmatched — particularly **AI/ML, natural language processing, and complex data transformations**.

### Why Python?

- **AI/ML ecosystem:** Direct access to libraries like `transformers`, `tiktoken`, `scikit-learn`, and custom compression engines.
- **Rapid prototyping:** Faster iteration for experimental or research-oriented policies.
- **Specialized use cases:** Prompt compression, semantic analysis, content classification, and other tasks where Python libraries provide capabilities that would be impractical to reimplement in Go.

### Python Policy Structure

Python policies follow the standard `src` layout and are packaged as installable Python packages:

```
my-python-policy/
├── policy-definition.yaml
├── pyproject.toml
├── requirements.txt
├── src/
│   └── my_python_policy_v1/
│       ├── __init__.py
│       └── policy.py
└── tests/
    └── test_policy.py
```

| File | Purpose |
|------|---------|
| `policy-definition.yaml` | Same format as Go — declares name, version, and parameter schema |
| `pyproject.toml` | Standard Python packaging configuration. Uses `hatchling` as the build backend |
| `requirements.txt` | Runtime dependencies |
| `src/<package>/policy.py` | Policy implementation |
| `tests/` | Unit tests for the policy logic |

### Build Integration

Python policies are referenced in `build.yaml` using the `pipPackage` field instead of `gomodule`:

```yaml
policies:
  - name: my-python-policy
    pipPackage: github.com/wso2/gateway-controllers/policies/my-python-policy@v1
```

The Gateway Builder resolves the Python package, installs its dependencies, generates the policy registry, and bundles everything into the gateway image alongside the Python Executor.

## Choosing a Language

Use this decision guide when planning a new policy:

| Consideration | Choose Go | Choose Python |
|---------------|-----------|---------------|
| **Performance-critical path** | ✅ In-process, zero overhead | ❌ Cross-process gRPC call |
| **Standard API management** (auth, rate limiting, headers) | ✅ Existing patterns and SDK | Possible, but unnecessary |
| **AI/ML or NLP processing** | Requires reimplementation of libraries | ✅ Direct access to Python ecosystem |
| **Complex data transformations** | Good for structured transforms | ✅ Better for text/NLP transforms |
| **Third-party library dependency** | Go library must exist | ✅ Vast PyPI ecosystem |
| **Production stability** | ✅ Compiled, type-safe | Interpreted, requires thorough testing |
| **Team expertise** | Go-proficient team | Python-proficient team |

Start with Go unless your policy specifically requires Python libraries or Python-native capabilities. The majority of gateway policies are written in Go.
