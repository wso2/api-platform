# [Proposal] Add Policy Lifecycle Close Method for Resource Cleanup

**Discussion #734** · [wso2/api-platform](https://github.com/wso2/api-platform/discussions/734)  
**Author:** [@renuka-fernando](https://github.com/renuka-fernando) · **Date:** Jan 21, 2026  
**Category:** 💡 Ideas · **Label:** `Area/PolicyEngine`

---

## Summary

Add a `Close()` method to the `Policy` interface to enable proper cleanup of resources (database connections, goroutines, HTTP clients) when policies are removed or replaced. This prevents resource leaks that occur when APIs are deleted or updated, as the current implementation simply replaces the policy map without any cleanup mechanism.

---

## Motivation

### Problem Statement

When APIs are deleted or updated via xDS, the `ApplyWholeRoutes` method replaces the entire routes map without calling any cleanup on existing policy instances:

```go
// internal/kernel/mapper.go:83-89
func (k *Kernel) ApplyWholeRoutes(newRoutes map[string]*registry.PolicyChain) {
    k.mu.Lock()
    defer k.mu.Unlock()
    k.Routes = newRoutes  // Old routes discarded without cleanup
}
```

Policies like `SemanticCachePolicy` create external resources during initialization:

- Redis/Milvus database connections via `VectorDBProvider`
- HTTP clients for embedding providers (OpenAI, Azure, Mistral)
- These providers have `Close()` methods that are never called

Over time, this causes:

- Connection pool exhaustion
- File descriptor leaks
- Memory leaks from orphaned goroutines
- Potential denial of service in long-running deployments

### Who Benefits

- **Platform Operators**: Stable, long-running gateway deployments without resource exhaustion
- **Policy Developers**: Clear contract for resource lifecycle management
- **API Developers**: Reliable API updates without gateway degradation

### Why Now

- The `SemanticCachePolicy` and rate limiting policies already create connections that leak
- Future policies are planned to use goroutines for background tasks (caching, health checks)
- The problem compounds with API churn — each update leaks more resources
- Fix is simpler now before more policies depend on the current behavior

---

## Detailed Design

### Overview

Add `Close() error` as a required method on the `Policy` interface. The Kernel will call `Close()` on all policies in a chain when that route is removed or replaced. Policies without resources simply return `nil`. This is a breaking change that forces all policy developers to explicitly consider resource cleanup, preventing leaks by design.

### Changes Required

| Component | File/Area | Change Description |
|---|---|---|
| SDK | `sdk/gateway/policy/v1alpha/interface.go` | Add `Close() error` to Policy interface |
| Policy Engine | `internal/kernel/mapper.go` | Update `ApplyWholeRoutes` to call `Close()` on removed policies |
| Policy Engine | `internal/kernel/mapper.go` | Update `UnregisterRoute` to call `Close()` |
| Policies | All policies in `policies/*/` | Implement `Close()` — return `nil` if no resources to cleanup |
| Policies | `policies/semantic-cache/` | Implement `Close()` to release vector store and embedding provider |
| Policies | `policies/advanced-ratelimit/` | Implement `Close()` for Redis-backed limiters |

### API Changes

```go
// sdk/gateway/policy/v1alpha/interface.go

// Policy is the base interface that all policies must implement
type Policy interface {
    // Mode returns the policy's processing mode for each phase
    Mode() ProcessingMode

    // OnRequest executes the policy during request phase
    OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction

    // OnResponse executes the policy during response phase
    OnResponse(ctx *ResponseContext, params map[string]interface{}) ResponseAction

    // Close releases any resources held by the policy (connections, goroutines, etc.)
    // Called when the policy is removed from active routes (API deletion or update)
    // Implementations must be idempotent and safe to call multiple times
    // Return nil if no cleanup is needed
    Close() error
}
```

> **Breaking Change**: All existing policies must implement `Close() error`. Policies without resources simply return `nil`.

### Configuration Changes

```yaml
# policy-engine config.yaml
policy:
  # Timeout for policy Close() calls during cleanup
  # Policies that exceed this timeout are logged and skipped
  # Default: 30s
  closeTimeout: 30s

  # Number of concurrent workers for policy cleanup
  # Limits goroutine creation during rapid API churn
  # Default: 10
  closeWorkers: 10
```

### Examples

**Before:**

```go
// Policies have no way to cleanup
type SemanticCachePolicy struct {
    vectorStoreProvider vectordbproviders.VectorDBProvider
    embeddingProvider   embeddingproviders.EmbeddingProvider
}

// When API is deleted, these connections leak forever
```

**After:**

```go
type SemanticCachePolicy struct {
    vectorStoreProvider vectordbproviders.VectorDBProvider
    embeddingProvider   embeddingproviders.EmbeddingProvider
}

// Close implements policy.Closer interface
func (p *SemanticCachePolicy) Close() error {
    var errs []error
    if p.vectorStoreProvider != nil {
        if err := p.vectorStoreProvider.Close(); err != nil {
            errs = append(errs, fmt.Errorf("vector store close: %w", err))
        }
    }
    // embeddingProvider cleanup if needed
    return errors.Join(errs...)
}
```

**Kernel cleanup (async with timeout):**

```go
const DefaultCloseTimeout = 30 * time.Second

type Kernel struct {
    mu           sync.RWMutex
    Routes       map[string]*registry.PolicyChain
    closeWg      sync.WaitGroup  // Track in-flight Close() calls for graceful shutdown
    closeTimeout time.Duration
}

func (k *Kernel) ApplyWholeRoutes(newRoutes map[string]*registry.PolicyChain) {
    k.mu.Lock()
    oldRoutes := k.Routes
    k.Routes = newRoutes  // New routes active immediately
    k.mu.Unlock()

    // Collect chains to close (routes that no longer exist)
    var chainsToClose []*registry.PolicyChain
    for routeKey, chain := range oldRoutes {
        if _, exists := newRoutes[routeKey]; !exists {
            chainsToClose = append(chainsToClose, chain)
        }
    }

    // Async cleanup - old policies already unreachable
    if len(chainsToClose) > 0 {
        k.closeWg.Add(1)
        go func() {
            defer k.closeWg.Done()
            for _, chain := range chainsToClose {
                k.closeChainWithTimeout(chain)
            }
        }()
    }
}

func (k *Kernel) closeChainWithTimeout(chain *registry.PolicyChain) {
    for _, p := range chain.Policies {
        // Run Close() with timeout to prevent hanging
        done := make(chan error, 1)
        go func(policy policy.Policy) {
            done <- policy.Close()
        }(p)

        select {
        case err := <-done:
            if err != nil {
                slog.Warn("Policy close failed", "error", err)
            }
        case <-time.After(k.closeTimeout):
            slog.Error("Policy close timed out", "timeout", k.closeTimeout)
        }
    }
}

// Shutdown waits for all pending Close() calls to complete
// Call this during graceful shutdown
func (k *Kernel) Shutdown(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        k.closeWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("shutdown timed out: %w", ctx.Err())
    }
}
```

---

## Drawbacks

- **Breaking change**: All existing policies must be updated to compile
- **Boilerplate**: Policies without resources must implement `Close() error { return nil }`
- **Error handling**: Close errors during shutdown need careful handling (log vs propagate)
- **Ordering**: If policies have dependencies, close order may matter (not addressed in this proposal)
- **Performance**: Small overhead during route updates to iterate and close policies

---

## Alternatives Considered

### Alternative 1: Separate Optional Closer Interface

- **Description**: Add a separate `Closer` interface; Kernel checks if policy implements it via type assertion
- **Pros**: Backward compatible; existing policies work unchanged
- **Cons**: Runtime type assertions; easy for developers to forget; no compile-time enforcement
- **Rejection Reason**: Opt-in model doesn't force developers to consider cleanup; leaks by default

### Alternative 2: Finalizers / Runtime Finalizers

- **Description**: Use Go's `runtime.SetFinalizer` to cleanup when policies are garbage collected
- **Pros**: No interface changes needed; automatic cleanup
- **Cons**:
  - Non-deterministic timing; GC runs based on memory pressure, not when policies are removed
  - Goroutines prevent GC; policies with background workers hold references, running forever
  - Discouraged pattern; Go team explicitly discourages finalizers for resource management
- **Rejection Reason**: Unreliable timing and goroutine issues make this unsuitable for systematic resource cleanup

### Alternative 3: Global Resource Pool with Reference Counting

- **Description**: Policies share connections from a global pool; connections closed when reference count hits zero
- **Pros**: Efficient connection reuse; automatic cleanup
- **Cons**: Significant complexity; requires refactoring all policies; pooling semantics vary by resource type
- **Rejection Reason**: Over-engineered for current needs; can be added later if beneficial

### Alternative 4: Context Cancellation

- **Description**: Pass a context to policy factory; cancel context when policy is removed

```go
// Factory would receive a context
func GetPolicy(ctx context.Context, metadata PolicyMetadata, params map[string]interface{}) (Policy, error) {
    p := &MyPolicy{redisClient: redis.NewClient(...)}

    // Goroutine respects context cancellation
    go func() {
        for {
            select {
            case <-ctx.Done():
                return  // Goroutine exits, but redisClient is still open!
            case <-time.After(time.Minute):
                p.doBackgroundWork()
            }
        }
    }()
    return p, nil
}
```

- **Pros**: Standard Go pattern; goroutines can listen for cancellation signal
- **Cons**:
  - Context is a **signal**, not an **action** — it tells goroutines to stop, but doesn't close connections
  - `redisClient.Close()` must still be called explicitly; context cancellation doesn't trigger it
  - Requires all policies to implement context-aware goroutines correctly
  - No compile-time enforcement; easy to forget to check `ctx.Done()`
  - Adds complexity to factory signature for policies that don't use goroutines
- **Rejection Reason**: Context cancellation handles goroutine lifecycle but not resource cleanup. A policy with a Redis connection and a goroutine needs **both**: context to signal the goroutine, and `Close()` to release the connection. Since `Close()` is required anyway, it's simpler to handle goroutine shutdown there too:

```go
func (p *MyPolicy) Close() error {
    close(p.stopChan)      // Signal goroutines to stop
    p.wg.Wait()            // Wait for goroutines to exit
    return p.redisClient.Close()  // Then close connections
}
```

---

## Compatibility

| Question | Answer | Details |
|---|---|---|
| Backwards compatible? | No | Policy interface gains new required method |
| Requires migration? | Yes | All policies must implement `Close() error` |
| Breaking changes? | Yes | Compile-time break for all policy implementations |

### Migration Steps

1. Update SDK with `Close()` method in Policy interface
2. Update all existing policies to implement `Close() error`
3. Update Kernel to call `Close()` on policy replacement
4. Update policy documentation with Close() requirements

---

## Design Decisions

| Question | Decision |
|---|---|
| Sync vs async Close()? | **Async** — Close() runs in background goroutine; route updates are non-blocking |
| Timeout for Close()? | **Yes** — 30 second default, configurable via `policy.closeTimeout` |
| Graceful shutdown? | **Yes** — `Kernel.Shutdown(ctx)` waits for pending Close() calls |
| Metrics? | **Yes** — Expose Prometheus metrics for close operations |
| Worker pool? | **Yes** — Bounded worker pool to prevent goroutine explosion during rapid churn |

### Metrics to Expose

| Metric | Type | Labels | Description |
|---|---|---|---|
| `policy_close_total` | Counter | `status=[success\|error\|timeout]` | Total Close() calls by outcome |
| `policy_close_duration_seconds` | Histogram | | Duration of Close() calls |
| `policy_close_queue_size` | Gauge | | Current number of policies waiting to be closed |

### Worker Pool Design

- Fixed pool of N workers (default: 10, configurable via `policy.closeWorkers`)
- Policies queued for cleanup are processed by available workers
- Prevents unbounded goroutine creation during rapid API updates
- Queue size exposed as metric for monitoring backpressure

```yaml
# policy-engine config.yaml
policy:
  closeTimeout: 30s
  closeWorkers: 10  # Number of concurrent Close() workers
```

---

## Unresolved Questions

None — proposal is fully specified.

---

## Definition of Done

**SDK Changes:**
- `Close() error` method added to `Policy` interface

**Kernel Changes:**
- `ApplyWholeRoutes` calls `Close()` asynchronously on removed policies
- `UnregisterRoute` calls `Close()` on removed policy chain
- Close timeout implemented (30s default, configurable via `policy.closeTimeout`)
- Bounded worker pool (10 default, configurable via `policy.closeWorkers`)
- `Kernel.Shutdown(ctx)` method for graceful shutdown
- WaitGroup tracking for in-flight Close() calls
- Prometheus metrics: `policy_close_total`, `policy_close_duration_seconds`, `policy_close_queue_size`

**Policy Updates:**
- All existing policies implement `Close() error`
- `SemanticCachePolicy` closes vector store and embedding provider
- Rate limiting policies close Redis connections

**Testing:**
- Unit tests for async Close() behavior
- Unit tests for Close() timeout
- Unit tests for graceful shutdown
- Integration test verifying connection cleanup on API deletion

**Documentation:**
- Policy developer guide updated with Close() requirements
- Example policy with proper Close() implementation

---

## References

- [Go io.Closer interface](https://pkg.go.dev/io#Closer) — Standard Go pattern for resource cleanup
- [Effective Go - Cleanup](https://go.dev/doc/effective_go#defer) — Defer and cleanup patterns
- [VectorDBProvider interface](sdk/utils/vectordbproviders/provider.go) — Already has Close() method