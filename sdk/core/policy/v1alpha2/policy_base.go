package policyv1alpha2

import (
	"context"
	"sync"
)

// PolicyBase provides concrete default implementations for the optional lifecycle
// hooks (CloseablePolicy, RequestLifecyclePolicy) and goroutine lifecycle
// management utilities.
//
// Embedding PolicyBase gives a policy:
//   - An instance-scoped context (Ctx()) that is cancelled when Close() is called.
//   - A Go() helper that starts background goroutines and tracks them — Close()
//     blocks until every goroutine started via Go() has returned.
//   - A no-op OnRequestComplete that can be overridden for per-request cleanup.
//
// PolicyBase uses lazy initialisation and is safe to use as a zero value — no
// constructor is required. All methods require a pointer receiver; the standard
// usage pattern (policy instances returned as pointers from GetPolicy) satisfies
// this automatically.
//
// PolicyBase intentionally does NOT implement the base Policy interface — it
// does not provide Mode(). Policy authors must still implement Mode() themselves.
//
// # Minimal usage (stateless policy)
//
// For a policy with no resources, embedding PolicyBase is a one-line change:
//
//	type UppercaseBodyPolicy struct {
//	    policyv1alpha2.PolicyBase
//	}
//
// Nothing else changes. Close() and OnRequestComplete() are satisfied by no-ops
// (the goroutine WaitGroup is empty so Close() returns immediately).
//
// # Background goroutine (use Go + Ctx)
//
//	type TokenBucketPolicy struct {
//	    policyv1alpha2.PolicyBase
//	    tokens chan struct{}
//	}
//
//	func GetPolicy(meta policyv1alpha2.PolicyMetadata, _ map[string]interface{}) (policyv1alpha2.Policy, error) {
//	    p := &TokenBucketPolicy{tokens: make(chan struct{}, 100)}
//	    p.Go(func(ctx context.Context) {   // goroutine tracked by PolicyBase
//	        ticker := time.NewTicker(time.Second)
//	        defer ticker.Stop()
//	        for {
//	            select {
//	            case <-ctx.Done():          // exits when kernel calls Close()
//	                return
//	            case <-ticker.C:
//	                p.tokens <- struct{}{}
//	            }
//	        }
//	    })
//	    return p, nil
//	}
//
//	// No Close() override needed — PolicyBase.Close() cancels the context and
//	// waits for the goroutine to exit before returning.
//
// # Additional resources (override Close, call PolicyBase.Close first)
//
//	type RateLimitPolicy struct {
//	    policyv1alpha2.PolicyBase
//	    redisPool *redis.Pool
//	}
//
//	func (p *RateLimitPolicy) Close() error {
//	    _ = p.PolicyBase.Close() // cancel context, drain tracked goroutines
//	    return p.redisPool.Close()
//	}
//
// # Per-request cleanup (override OnRequestComplete)
//
//	func (p *RateLimitPolicy) OnRequestComplete(cause policyv1alpha2.CompletionCause, shared *policyv1alpha2.SharedContext) {
//	    if cause == policyv1alpha2.CompletionCancelled {
//	        _ = p.refundTokens(shared.RequestID) // roll back optimistic decrement
//	    }
//	}
type PolicyBase struct {
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// init lazily initialises the instance context. Safe to call multiple times —
// sync.Once ensures the context and cancel function are created exactly once.
func (b *PolicyBase) init() {
	b.once.Do(func() {
		b.ctx, b.cancel = context.WithCancel(context.Background())
	})
}

// Ctx returns the instance-scoped context. The context is active for the full
// lifetime of the policy instance and is cancelled when Close() is called.
//
// Use it to bind any blocking call or outbound request to the policy instance
// lifetime so it is automatically interrupted on decommission:
//
//	req, _ := http.NewRequestWithContext(p.Ctx(), "GET", url, nil)
func (b *PolicyBase) Ctx() context.Context {
	b.init()
	return b.ctx
}

// Go starts a background goroutine tied to the policy instance lifetime.
// The goroutine receives the instance context as its argument and must return
// when ctx.Done() is closed.
//
// Close() will not return until every goroutine started via Go() has exited,
// ensuring the policy instance is fully shut down before the kernel discards it.
//
// Do not call Go() after Close() has been called — the context will already be
// cancelled and the goroutine will exit immediately. In practice the kernel
// guarantees Close() is only called after all requests on the retiring chain
// have completed, so Go() calls in On* phase methods are safe.
func (b *PolicyBase) Go(f func(ctx context.Context)) {
	b.init()
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		f(b.ctx)
	}()
}

// Close cancels the instance context and blocks until all goroutines started
// via Go() have returned.
//
// The kernel calls Close() after draining all in-flight requests on the retiring
// chain — no On* phase methods will run concurrently with or after Close().
//
// If the policy holds additional resources beyond tracked goroutines, override
// Close() and call p.PolicyBase.Close() first to ensure goroutine cleanup runs
// before the resources they depend on are released:
//
//	func (p *MyPolicy) Close() error {
//	    _ = p.PolicyBase.Close() // must come first: stops goroutines
//	    return p.pool.Close()    // safe to close after goroutines have stopped
//	}
func (b *PolicyBase) Close() error {
	b.init() // ensure cancel != nil even if Ctx/Go were never called
	b.cancel()
	b.wg.Wait()
	return nil
}

// OnRequestComplete is a no-op default. Override it in policies that hold
// per-request resources (accumulated buffers, in-flight HTTP calls, open tracing
// spans, optimistic counter decrements) that need cleanup when a request ends
// abnormally.
//
// The cause parameter tells you why the request ended:
//   - CompletionNormal: full pipeline completed — cleanup is usually unnecessary.
//   - CompletionCancelled: client disconnected mid-flight.
//   - CompletionError: upstream or policy error ended the request early.
func (b *PolicyBase) OnRequestComplete(CompletionCause, *SharedContext) {}
