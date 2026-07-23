# Rule: Go Network Service Hardening Standards (Timeouts, Resource Limits, Resilience)

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that stands up an HTTP or gRPC server (management API, admin API, metrics endpoints), a background polling loop against an external control plane (EventHub, Platform-API), or a bounded-concurrency worker pool (Python/gRPC executors, `ext_proc` request processing). The goal is to prevent resource-exhaustion denial-of-service — from malicious/slow clients, from scale-out synchronization against a shared upstream, and from a compromised or merely malfunctioning control plane — and to ensure a single misbehaving caller or component cannot degrade the service for everyone else.

This is the service-hardening counterpart to `authentication_authorization.md` (which governs *who* may call a service) and `ssrf-prevention.md`/`xxe-xml-processing.md` (which govern *what* a service may be tricked into doing) — this rule governs how a service survives volume, slowness, and hostile timing regardless of who the caller is.

---

## Directives

### 1. Every HTTP Server Must Set Explicit Timeouts and Size Ceilings

* **No Bare `http.ListenAndServe`:** Construct an explicit `http.Server{}` with `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` set from configuration (with safe non-zero defaults) — the zero-value server has no timeouts at all, so a slow-loris-style client can hold a connection (and a goroutine) open indefinitely.
* **Cap Header and Body Size:** Set `MaxHeaderBytes` on the server, and wrap every request body a handler reads from in `http.MaxBytesReader(w, r.Body, maxBytes)` with the ceiling sourced from configuration (see `file-access.md` directive 5 for the same principle applied to file uploads specifically).
* **Default to TLS in Non-Development Builds:** Use `ListenAndServeTLS` by default; a plaintext listener must be an explicit, narrowly-scoped development-mode opt-out, not the shipped default for a management or admin surface.

### 2. Bound Every gRPC Server's Message Size and Concurrency

* **Set Limits on Every `grpc.NewServer(...)` Call:** `grpc.MaxRecvMsgSize`, `grpc.MaxSendMsgSize`, and `grpc.MaxConcurrentStreams` must be set explicitly — including internal control-plane/xDS servers (see `go-control-plane-xds-security.md` for xDS-specific requirements). The library defaults either have no limit or a limit sized for general-purpose RPC, not for a specific service's threat model.

### 3. A Bounded Queue Must Sit in Front of a Bounded Worker Pool — and Be Bounded Itself

* **Reject, Don't Queue Unboundedly:** If work is queued ahead of a semaphore/worker-pool concurrency limit (e.g. N execution slots), the queue admitting callers into that wait must have its own explicit ceiling. Once the ceiling is hit, reject new work immediately with a typed resource-exhaustion error (e.g. gRPC `RESOURCE_EXHAUSTED`, HTTP `503`) rather than growing the queue further — an unbounded queue in front of a bounded pool is not bounded concurrency, it is delayed unbounded memory growth.
* **Cap Resource Use Per Worker, and Per Invocation When Genuinely Required:** Where workers execute arbitrary or semi-trusted logic (a Python executor, a policy-engine script), apply a memory/CPU ceiling on each worker process (e.g. `resource.setrlimit` called once in the worker's init) in addition to bounding the number of concurrent workers — concurrency limits alone don't stop one request from exhausting resources by itself. Note that `resource.setrlimit` is a per-process limit: it bounds a worker for its entire lifetime, not a single request, so a long-lived worker handling many requests sequentially is capped in aggregate, not per invocation. Where a hard per-request budget is genuinely required, isolate each invocation in its own short-lived sandboxed process or container with its own enforced resource limit, rather than relying on a shared worker's process-wide `rlimit`.

### 4. Add Jitter to Any Fixed-Interval Poll Against a Shared Upstream

* **Desynchronize Replica Polling:** A poll loop run identically across many replicas of the same service (an EventHub poll, a control-plane heartbeat) must desynchronize with random jitter — e.g. `time.Sleep(interval + rand.N(interval/2))` (Go's `math/rand/v2`) — computed and waited *before* each fetch, not after, and never a bare fixed-interval sleep. Without jitter, scale-out produces a synchronized thundering herd against the shared upstream every time all replicas restart or roll together, including the very first fetch after a simultaneous restart — which is exactly why the jittered wait must precede the fetch rather than follow it. Validate that `interval` is positive (and large enough that `interval/2` isn't zero) before computing jitter from it.

### 5. Tune Chained Timeouts So the Outer Bound Is Tighter Than What It Wraps, Not Looser

* **Outer Timeout Slightly Above the Inner, Never Many Multiples Larger:** When one component's operation is itself bounded by a downstream call that has its own shorter timeout (e.g. an `ext_proc` per-message timeout wrapping a policy-engine chain that itself times out in a few seconds), set the outer timeout modestly above the inner one — not at a generic default many multiples larger. A stuck downstream call should be caught by the inner timeout, not left to hold the outer resource (a worker thread, a connection slot, a stream) for far longer than the chain it wraps could ever legitimately take.

### 6. Do Not Let a Remote Control Plane's Instructions Terminate Your Process

* **Terminal Statuses From a Remote Peer Trigger Degradation, Not `os.Exit`:** Treat a "permanent failure" or similarly terminal status reported by a remote control plane as a signal to enter a degraded/backoff state — an internal circuit breaker, surfaced via a local health/readiness endpoint — never as a direct trigger for process termination. A compromised, buggy, or simply momentarily-wrong control plane must not be able to use its own protocol responses to take down every connected service instance simultaneously. Reserve `os.Exit` for conditions the process itself has verified locally (e.g. a locally-detected fatal misconfiguration at startup, per `authentication_authorization.md` GO-AUTH-011), not for a remote party's assertion about this process's own fate.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: bare ListenAndServe — no timeouts, no size caps.
func StartManagementAPI(handler http.Handler) error {
    return http.ListenAndServe(":8080", handler)
}

// BAD: gRPC server with no message-size or concurrency limits.
func NewXDSServer() *grpc.Server {
    return grpc.NewServer() // unbounded defaults
}

// BAD: unbounded queue in front of a bounded semaphore.
var executionSlots = make(chan struct{}, 100)

func Execute(req *ExecRequest) error {
    executionSlots <- struct{}{} // callers queue without limit
    defer func() { <-executionSlots }()
    return run(req)
}

// BAD: fixed-interval poll — no jitter across replicas.
func PollEventHub(interval time.Duration) {
    for {
        fetchEvents()
        time.Sleep(interval)
    }
}

// BAD: outer timeout many multiples larger than the inner chain's own timeout.
const MessageTimeout = 60 * time.Second // Policy-engine chain timeout is 5s

// BAD: a remote control plane's response directly kills the process.
func handlePlatformAPIStatus(status string) {
    if status == "permanent_failure" {
        os.Exit(1) // remote-controlled process termination
    }
}
```

### Best Practice (What to Generate)

```go
// GOOD: explicit timeouts, header/body caps, TLS by default.
func StartManagementAPI(cfg ServerConfig, handler http.Handler) error {
    wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes) // Configurable ceiling
        handler.ServeHTTP(w, r)
    })

    srv := &http.Server{
        Addr:           cfg.Addr,
        Handler:        wrapped,
        ReadTimeout:    60 * time.Second,
        WriteTimeout:   60 * time.Second,
        IdleTimeout:    120 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }

    if cfg.DevelopmentMode {
        log.Warn("DEVELOPMENT MODE: serving management API over plaintext HTTP")
        return srv.ListenAndServe()
    }
    return srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile) // TLS is the production default
}

// GOOD: explicit gRPC resource limits.
func NewXDSServer() *grpc.Server {
    return grpc.NewServer(
        grpc.MaxRecvMsgSize(50<<20),
        grpc.MaxSendMsgSize(50<<20),
        grpc.MaxConcurrentStreams(1000),
    )
}

// GOOD: the admission queue ahead of the concurrency semaphore is itself
// bounded — new work is rejected immediately once the ceiling is hit.
type BoundedExecutor struct {
    slots   chan struct{}
    pending int64
    maxPending int64
}

func (e *BoundedExecutor) Execute(req *ExecRequest) error {
    if atomic.AddInt64(&e.pending, 1) > e.maxPending {
        atomic.AddInt64(&e.pending, -1)
        return status.Error(codes.ResourceExhausted, "too many pending executions")
    }
    defer atomic.AddInt64(&e.pending, -1)

    e.slots <- struct{}{}
    defer func() { <-e.slots }()
    return run(req)
}

// GOOD: jittered polling — desynchronizes replicas instead of a thundering
// herd, including on simultaneous restart. The jittered wait happens BEFORE
// each fetch, not after, so replicas that all boot at once don't immediately
// stampede the shared upstream on their very first call.
func PollEventHub(interval time.Duration) error {
    if interval <= 0 {
        return fmt.Errorf("poll interval must be positive, got %s", interval)
    }
    for {
        delay := interval
        if half := interval / 2; half > 0 { // Guards against a too-small interval where half rounds to zero
            delay += rand.N(half) // math/rand/v2 — real, supported random-duration construction
        }
        time.Sleep(delay)
        fetchEvents()
    }
}

// GOOD: outer timeout set modestly above the inner chain's own timeout —
// a stuck downstream call is caught by the chain's 5s timeout, not by holding
// the ext_proc worker for 12x longer than necessary.
const ChainTimeout = 5 * time.Second
const MessageTimeout = 10 * time.Second // Generous vs ChainTimeout, well below worker-pool exhaustion threshold

// GOOD: a remote control plane's terminal status degrades this instance
// locally — it never triggers process termination on the control plane's say-so.
type ControlPlaneHealth struct {
    degraded atomic.Bool
}

func (h *ControlPlaneHealth) handlePlatformAPIStatus(status string) {
    if status == "permanent_failure" {
        h.degraded.Store(true) // Surfaced via /health; a circuit breaker backs off reconnect attempts
        logger.Error("control plane reported a permanent failure status — entering degraded mode")
        return
    }
    h.degraded.Store(false)
}
```

---

> **Verification Checklist before outputting code:**
> * Does an `http.Server` omit `ReadTimeout`/`WriteTimeout`/`IdleTimeout`, or use bare `http.ListenAndServe`? (If yes, construct an explicit `http.Server` with all three set from configuration.)
> * Is every request body read without first wrapping it in `http.MaxBytesReader`? (If yes, add a configurable ceiling.)
> * Does a `grpc.NewServer(...)` call omit `MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams`? (If yes, add explicit limits — including on internal/control-plane gRPC servers.)
> * Is there a queue or unbounded channel send in front of a concurrency-limiting semaphore, with no ceiling on how many callers may be waiting? (If yes, add an explicit pending-count limit that rejects new work once exceeded.)
> * Does a poll loop running across multiple replicas sleep a bare fixed interval with no randomized jitter? (If yes, add jitter sized relative to the interval.)
> * Is an outer/wrapping timeout set to a generic default that is many multiples larger than the specific downstream timeout it wraps? (If yes, tune it to modestly exceed the inner timeout instead.)
> * Does a terminal/permanent status reported by a remote control plane directly call `os.Exit`/terminate the process? (If yes, replace with a local degraded-state/circuit-breaker pattern surfaced via a health endpoint — never let a remote peer's response be the direct trigger for this process's termination.)
