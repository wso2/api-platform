# Rule: Go Network Service Hardening Standards (Timeouts, Resource Limits, Resilience)

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that stands up an HTTP or gRPC server (management API, admin API, metrics endpoints), a background polling loop against an external control plane (EventHub, Platform-API), or a bounded-concurrency worker pool (Python/gRPC executors, `ext_proc` request processing). The goal is to prevent resource-exhaustion denial-of-service — from slow/malicious clients, scale-out synchronization against a shared upstream, or a compromised/malfunctioning control plane — so a single misbehaving caller or component can't degrade the service for everyone else. This is the hardening counterpart to `authentication_authorization.md`/`ssrf-prevention.md`/`xxe-xml-processing.md`: those govern *who* may call and *what* a service may be tricked into doing; this one governs how it survives volume, slowness, and hostile timing.

## Directives

1. **Explicit HTTP timeouts and size ceilings.** Never use bare `http.ListenAndServe` — construct an `http.Server{ReadTimeout, WriteTimeout, IdleTimeout}` from configuration with safe non-zero defaults (the zero-value server has none, so a slow-loris client holds a goroutine open indefinitely). Set `MaxHeaderBytes` and wrap every request body in `http.MaxBytesReader(w, r.Body, maxBytes)`. Default to `ListenAndServeTLS`; plaintext must be an explicit, narrowly-scoped dev-mode opt-out.
2. **Bound every gRPC server's message size and concurrency.** Set `grpc.MaxRecvMsgSize`, `grpc.MaxSendMsgSize`, and `grpc.MaxConcurrentStreams` explicitly on every `grpc.NewServer(...)` call, including internal control-plane/xDS servers (see `go-control-plane-xds-security.md`) — library defaults are unbounded or sized for general-purpose RPC, not this service's threat model.
3. **A bounded queue in front of a bounded worker pool must itself be bounded.** If callers queue ahead of a concurrency semaphore, cap the queue with an explicit pending-count and reject new work immediately (gRPC `RESOURCE_EXHAUSTED`, HTTP `503`) once hit — an unbounded queue in front of a bounded pool is just delayed unbounded memory growth. Where workers run semi-trusted logic, also cap memory/CPU per worker process (`resource.setrlimit`); remember that's a per-process, not per-request, ceiling — use a short-lived sandboxed process per invocation when a hard per-request budget is required.
4. **Jitter any fixed-interval poll against a shared upstream.** A poll loop replicated across instances (EventHub poll, control-plane heartbeat) must desynchronize with random jitter computed and waited *before* each fetch, not after — otherwise simultaneous restarts stampede the upstream on the very first call. Validate `interval > 0` (and large enough that `interval/2` isn't zero) before computing jitter.
5. **Tune chained timeouts so the outer bound is tighter, never many multiples looser.** When an operation wraps a downstream call with its own shorter timeout (an `ext_proc` message timeout wrapping a policy-engine chain), set the outer timeout modestly above the inner one, not at a generic multiples-larger default — a stuck downstream call should be caught by the inner timeout, not hold the outer resource far longer than the chain could legitimately take.
6. **Never let a remote control plane's instructions terminate your process.** Treat a "permanent failure" or similar terminal status from a remote peer as a signal to enter a degraded/backoff state (an internal circuit breaker surfaced via a local health endpoint) — never a direct `os.Exit` trigger. A compromised or buggy control plane must not be able to use its own protocol responses to take down every connected instance at once; reserve `os.Exit` for conditions verified locally at startup (per `authentication_authorization.md` GO-AUTH-011).
7. **No deferring a violation behind a code comment.** Never resolve a missing timeout, size ceiling, queue bound, or jitter gap by adding a `// TODO`/`FIXME`-style comment and shipping the code anyway — a comment does not bound a request or desynchronize a poll loop. Fix it before merging, or raise the gap explicitly for an approved exception rather than leaving it annotated in the source.

## Example

```go
// BAD: no timeouts/caps, and a bare fixed-interval poll with no jitter.
func StartManagementAPI(handler http.Handler) error {
    return http.ListenAndServe(":8080", handler) // slow-loris holds goroutines forever
}
func PollEventHub(interval time.Duration) {
    for {
        fetchEvents()
        time.Sleep(interval) // every replica stampedes the upstream in lockstep
    }
}

// GOOD: explicit server timeouts + body cap, TLS by default.
func StartManagementAPI(cfg ServerConfig, handler http.Handler) error {
    wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes)
        handler.ServeHTTP(w, r)
    })
    srv := &http.Server{
        Addr: cfg.Addr, Handler: wrapped,
        ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }
    if cfg.DevelopmentMode {
        log.Warn("DEVELOPMENT MODE: serving management API over plaintext HTTP")
        return srv.ListenAndServe()
    }
    return srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
}

// GOOD: jittered wait happens BEFORE each fetch, desynchronizing replicas
// even on a simultaneous restart.
func PollEventHub(interval time.Duration) error {
    if interval <= 0 {
        return fmt.Errorf("poll interval must be positive, got %s", interval)
    }
    for {
        delay := interval
        if half := interval / 2; half > 0 {
            delay += rand.N(half) // math/rand/v2
        }
        time.Sleep(delay)
        fetchEvents()
    }
}
```

> **Verification Checklist before outputting code:**
> * Does an `http.Server` omit `ReadTimeout`/`WriteTimeout`/`IdleTimeout`, or use bare `ListenAndServe`, or read a body without `http.MaxBytesReader`?
> * Does any `grpc.NewServer(...)` — including internal/control-plane servers — omit `MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams`?
> * Is there an unbounded queue/channel send in front of a concurrency semaphore, with no pending-count ceiling that rejects new work?
> * Does a multi-replica poll loop sleep a bare fixed interval with no jitter computed before the fetch?
> * Is an outer/wrapping timeout a generic default many multiples larger than the specific inner timeout it wraps?
> * Does a terminal/permanent status from a remote control plane directly call `os.Exit` instead of entering a local degraded state?
> * Is a gap in this rule "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or an explicitly raised, approved exception?
