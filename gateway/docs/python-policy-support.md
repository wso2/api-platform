# Python Policy Support in the Gateway

## How It Works (End-to-End)

```
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Runtime Container                │
│                                                             │
│  ┌─────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │  Envoy  │───▶│ Policy Engine│───▶│ Python Executor  │   │
│  │ (Router)│    │   (Go)       │    │   (Python 3)     │   │
│  └─────────┘    └──────┬───────┘    └────────┬─────────┘   │
│                        │   gRPC bidi stream   │             │
│                        │   over Unix Socket   │             │
│                        └──────────────────────┘             │
│                                                             │
│  Process management: tini → Python Executor → Policy Engine │
│                      → Envoy (sequential startup)           │
└─────────────────────────────────────────────────────────────┘
```

### Policy Lifecycle

1. **Chain Building** (when an API is deployed/updated):
   - Go Policy Engine calls `GetPolicy()` on the `BridgeFactory` for each Python policy in the chain
   - `BridgeFactory.GetPolicy()` sends `InitPolicy` RPC to Python Executor
   - Python Executor: `PolicyLoader` finds the factory → calls `get_policy(metadata, params)` → stores instance in `PolicyInstanceStore` → returns `instance_id`
   - `PythonBridge` stores the `instance_id` and is ready for request processing

2. **Request Processing** (per HTTP request):
   - Envoy receives request, sends to Policy Engine via ext_proc
   - Policy Engine runs policy chain — for Python policies, `PythonBridge.OnRequest/OnResponse()` is called
   - `PythonBridge` sends `ExecuteStream` RPC with `instance_id` → Python Executor dispatches to the correct instance
   - Instance's `on_request()`/`on_response()` method is called → action returned

3. **Cleanup** (when route is removed/replaced, per discussion #734):
   - Go Kernel calls `Close()` on the `PythonBridge`
   - `PythonBridge.Close()` sends `DestroyPolicy` RPC with `instance_id`
   - Python Executor: calls `instance.close()` → removes from `PolicyInstanceStore`

### Build Flow

1. `gateway-builder` reads `build.yaml`, discovers policies (Go or Python based on `runtime: python` in `policy-definition.yaml`)
2. For Python policies: copies the executor source + policy sources into the build output, generates `python_policy_registry.py` (maps `"name:version"` → module path), merges `requirements.txt`
3. Generates `plugin_registry.go` which registers Go policies directly and Python policies via `BridgeFactory`
4. Dockerfile installs Python deps using the runtime's own `python3` and copies everything into the final image

## Is It Modular? (Can you run without Python?)

**Yes, fully modular.** If no Python policies are in `build.yaml`:

- `generatePythonExecutorBase()` creates an empty scaffold (so Docker `COPY` doesn't fail)
- No `BridgeFactory` registrations in `plugin_registry.go`
- The entrypoint checks `if [ -f /app/python-executor/main.py ]` — if no Python policies, the registry file won't exist, so the Python Executor **never starts**
- Gateway runs with only Go policies as before

The only overhead for a Go-only build is `python3` being installed in the runtime image (~30MB). This can be made conditional in a future optimization.

## Files Created

### Proto Definition
| File | Purpose |
|------|---------|
| `gateway-runtime/proto/python_executor.proto` | gRPC contract: `ExecuteStream` (bidi), `HealthCheck`, `InitPolicy`, `DestroyPolicy` |

### Go — Python Bridge (`policy-engine/internal/pythonbridge/`)
| File | Purpose |
|------|---------|
| `bridge.go` | `PythonBridge` struct implementing `policy.Policy` — stores `instance_id`, delegates to Python via gRPC, has `Close()` method |
| `client.go` | `StreamManager` — singleton gRPC client, persistent bidi stream, request-ID correlation, `InitPolicy()`/`DestroyPolicy()` methods |
| `factory.go` | `BridgeFactory` — creates `PythonBridge` instances, calls `InitPolicy` RPC, registered as `PolicyFactory` |
| `translator.go` | Converts proto responses → Go SDK action types |
| `proto/python_executor.pb.go` | Generated Go protobuf types |
| `proto/python_executor_grpc.pb.go` | Generated Go gRPC client stubs |

### Python Executor (`gateway-runtime/python-executor/`)
| File | Purpose |
|------|---------|
| `main.py` | Entry point — asyncio event loop, signal handling, logging setup |
| `executor/server.py` | gRPC servicer — handles `ExecuteStream`, `InitPolicy`, `DestroyPolicy`; dispatches to policy instances by `instance_id` |
| `executor/policy_loader.py` | Loads `get_policy` factory functions from generated `python_policy_registry.py` using `importlib` |
| `executor/instance_store.py` | **NEW** — `PolicyInstanceStore` maps `instance_id → Policy` instance, thread-safe |
| `executor/translator.py` | Converts proto messages ↔ Python SDK types |
| `sdk/policy.py` | Python Policy SDK — `Policy` ABC with `on_request`, `on_response`, `close` methods |
| `requirements.txt` | Base deps: `grpcio`, `protobuf` |
| `proto/python_executor_pb2.py` | Generated Python protobuf types |
| `proto/python_executor_pb2_grpc.py` | Generated Python gRPC stubs |

### Sample Python Policy (`sample-policies/add-python-header/`)
| File | Purpose |
|------|---------|
| `policy-definition.yaml` | Policy metadata: name, version, `runtime: python`, `processingMode`, params schema |
| `policy.py` | Implementation: class extending `Policy` with `get_policy()` factory |
| `requirements.txt` | Policy-specific deps (empty for this simple policy) |

### Controller Policy Definition
| File | Purpose |
|------|---------|
| `gateway-controller/default-policies/add-python-header.yaml` | So the controller knows the policy exists and can validate API configs |

## Files Modified

### Gateway Builder
| File | Change |
|------|--------|
| `pkg/types/manifest.go` | Added `Pythonmodule` field to `BuildEntry` |
| `pkg/types/policy.go` | Added `Runtime`, `PythonSourceDir`, `ProcessingMode` to `DiscoveredPolicy` |
| `internal/discovery/manifest.go` | Added `discoverPythonPolicy()`, `parseProcessingMode()`, `filePath` runtime detection |
| `internal/discovery/policy.go` | Added `ValidatePythonDirectoryStructure()`, `CollectPythonSourceFiles()` |
| `internal/discovery/python_module.go` | **New file** — fetches remote Python policies via GitHub tarball |
| `internal/policyengine/generator.go` | Added `generatePythonExecutorBase()`, `GeneratePythonArtifacts()`, `generatePythonRegistry()`, `mergeRequirements()` |
| `internal/policyengine/registry.go` | Added `Runtime`/`ProcessingMode` to `PolicyImport`, template data for Python |
| `templates/plugin_registry.go.tmpl` | Added Python policy registration block using `BridgeFactory` |

### SDK
| File | Change |
|------|--------|
| `sdk/gateway/policy/v1alpha/definition.go` | Added `Runtime`, `ProcessingModeConfig` to `PolicyDefinition` |

### Gateway Runtime
| File | Change |
|------|--------|
| `Dockerfile` | Added `python3`/`pip3` install, copies executor, pip-installs deps in runtime stage |
| `docker-entrypoint.sh` | Added Python Executor startup, UDS socket wait, `--py.*` arg forwarding, process monitoring |

### Build Config
| File | Change |
|------|--------|
| `build.yaml` | Added `add-python-header` entry with `filePath` |

## Writing a Python Policy

Python policies are **class-based** with a factory function. Each policy module exports a `get_policy()` factory that returns a `Policy` instance:

```python
from typing import Any, Dict
from sdk.policy import (
    Policy, PolicyMetadata,
    RequestContext, ResponseContext,
    RequestAction, ResponseAction,
    UpstreamRequestModifications,
)

class MyPolicy(Policy):
    """Example Python policy with lifecycle."""
    
    def __init__(self, header_name: str, header_value: str):
        # Initialize with config extracted from params
        self._header_name = header_name
        self._header_value = header_value
    
    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        """Process request phase.
        
        Args:
            ctx: Request context with headers, body, path, method, shared metadata.
            params: Policy configuration parameters from policy-definition.yaml.
        
        Returns:
            None for pass-through, UpstreamRequestModifications for changes,
            or ImmediateResponse to short-circuit.
        """
        return UpstreamRequestModifications(
            set_headers={self._header_name: self._header_value}
        )
    
    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        """Process response phase.
        
        Args:
            ctx: Response context with request data, response headers/body/status, shared metadata.
            params: Policy configuration parameters.
        
        Returns:
            None for pass-through or UpstreamResponseModifications for changes.
        """
        return None  # Pass-through
    
    def close(self) -> None:
        """Release resources when the policy instance is destroyed.
        
        Called when the route is removed or replaced. Override to close
        connections, stop background threads, flush caches, etc.
        Must be idempotent — may be called multiple times.
        """
        pass  # Default: no-op


def get_policy(metadata: PolicyMetadata, params: Dict[str, Any]) -> Policy:
    """Factory function — mirrors Go's GetPolicy.
    
    Args:
        metadata: Route/API metadata for this policy instance.
        params: Merged system + user parameters (already resolved by Go side).
    
    Returns:
        A Policy instance. The factory controls instancing — can return a
        fresh instance, a cached singleton, or a shared instance keyed by
        config hash, exactly like Go's GetPolicy pattern.
    """
    header_name = params.get("headerName", "X-Custom-Header")
    header_value = params.get("headerValue", "default-value")
    return MyPolicy(header_name, header_value)
```

### Key features of the class-based approach:
- **`get_policy(metadata, params)` factory** — controls instancing (singleton, per-route, cached, etc.)
- **`Policy` class with `on_request`, `on_response`** — implements policy logic
- **`close()` method** — cleanup hook called when route is removed
- **Thread-safe** — each route gets its own instance; instances are not shared between routes unless the factory returns a shared one

See `sample-policies/add-python-header/policy.py` for a complete example.

## Instancing Patterns

The factory controls instancing strategy:

### Fresh instance per route (default)
```python
def get_policy(metadata, params):
    return MyPolicy(params)  # New instance every time
```

### Singleton (shared across all routes)
```python
_instance = None

def get_policy(metadata, params):
    global _instance
    if _instance is None:
        _instance = MyPolicy(params)
    return _instance
```

### Cached by config hash
```python
import hashlib
_cache = {}

def get_policy(metadata, params):
    config_hash = hashlib.sha256(str(params).encode()).hexdigest()
    if config_hash not in _cache:
        _cache[config_hash] = MyPolicy(params)
    return _cache[config_hash]
```

## Connection to Discussion #734

When discussion #734 is implemented, the Go `Policy` interface will gain a `Close() error` method. The `PythonBridge` already implements this method — it calls `DestroyPolicy` RPC which triggers `instance.close()` on the Python side.

Flow when route is removed:
```
Kernel.ApplyWholeRoutes(newRoutes)
  → identifies removed routes
  → go func() {  // async cleanup
        for _, policy := range removedChain.Policies {
            policy.Close()  // PythonBridge.Close()
              → DestroyPolicy RPC
                → Python: instance.close()
        }
    }()
```

## Bug Fixes Applied During Bring-up

| Bug | Fix |
|-----|-----|
| `grpcio` C extension mismatch — `python:3.11` build stage vs `python3.10` runtime | Removed separate `python-deps` Docker stage; pip-install in runtime stage using its own `python3` |
| `import python_executor_pb2` failing — absolute import in generated gRPC stub | Changed to `from proto import python_executor_pb2` |
| Missing `proto/__init__.py` | Created it so `proto/` is a proper Python package |
| `proto.Struct` type annotation — references wrong module | Removed incorrect type annotation on `_dict_to_struct()` |
| `struct_to_dict()` returning protobuf `Value` objects instead of native Python types | Replaced with `_proto_value_to_python()` that uses `WhichOneof('kind')` to unwrap properly |
| `filePath:` Python policies routed to Go discovery | Added runtime detection: reads `policy-definition.yaml` to check `runtime` before dispatching |
| Docker build failing without Python policies | Added `generatePythonExecutorBase()` that always creates the output directory |

## Policy Lifecycle Changes (2025-03)

The Python Executor was refactored to support **policy lifecycle** with `InitPolicy` / `DestroyPolicy` RPCs:

| Before | After |
|--------|-------|
| Stateless `on_request()`/`on_response()` functions | Class-based `Policy` with `on_request`, `on_response`, `close` |
| No factory, functions called directly | `get_policy(metadata, params)` factory controls instancing |
| No instance identity | Opaque `instance_id` generated by Python, echoed by Go |
| No cleanup | `DestroyPolicy` RPC calls `instance.close()` |
| `Policy` ABC deprecated | `Policy` ABC is the primary pattern |

The gRPC contract expanded with:
- `InitPolicy` / `InitPolicyResponse` — create instance
- `DestroyPolicy` / `DestroyPolicyResponse` — destroy instance  
- `ExecutionRequest.instance_id` — dispatch to correct instance

## Verified Working

Tested with a **Go → Python → Go** policy chain:
- `log-message` (Go) → `add-python-header` (Python) → `add-headers` (Go)
- Both `X-Python-Policy` and `X-Go-Policy` headers appeared in the upstream request
- All three processes (Python Executor, Policy Engine, Envoy) running stable in the container
- `InitPolicy` called during chain building, `instance_id` stored in `PythonBridge`
- `ExecuteStream` requests include `instance_id`, correctly dispatched to instance
