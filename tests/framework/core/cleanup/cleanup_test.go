/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cleanup

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const configuredMaxCleanupAttempts = 3

func runnerCtx() context.Context {
	return tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("b")),
		tcontext.NewLocal("r"),
	)
}

// recorder captures deletion order and the actor each delete ran as.
type recorder struct {
	mu      sync.Mutex
	deleted []Resource
	fail    map[string]error
}

func newRecorder() *recorder { return &recorder{fail: map[string]error{}} }

func (rec *recorder) deleter(ctx context.Context, res Resource) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if err, bad := rec.fail[res.ID]; bad {
		return err
	}
	rec.deleted = append(rec.deleted, res)
	return nil
}

func (rec *recorder) order() []string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	out := make([]string, 0, len(rec.deleted))
	for _, r := range rec.deleted {
		out = append(out, r.Kind.Name+":"+r.ID)
	}
	return out
}

func registryWith(rec *recorder, kinds ...Kind) *Registry {
	r := NewRegistry(nil)
	for _, k := range kinds {
		r.RegisterDeleter(k, rec.deleter)
	}
	return r
}

func resourceKeys(resources []Resource) []string {
	out := make([]string, 0, len(resources))
	for _, resource := range resources {
		out = append(out, resource.Kind.Name+":"+resource.ID)
	}
	return out
}

func TestRegistrationRequiresAnActor(t *testing.T) {
	// Teardown deletes as the creator, so an unattributed resource cannot be cleaned up
	// correctly. Refusing at registration means the problem surfaces where the information
	// still exists, not at teardown when it is gone.
	r := NewRegistry(nil)

	err := r.Register(Resource{Kind: KindAPI, ID: "api-1"})
	require.ErrorContains(t, err, "no actor")
	require.ErrorContains(t, err, "deletes each resource as its creator")

	require.ErrorContains(t, r.Register(Resource{Kind: KindAPI, Actor: "admin"}), "no id")
	require.ErrorContains(t, r.Register(Resource{ID: "x", Actor: "admin"}), "no kind")

	require.Zero(t, r.Count(), "nothing invalid should have been recorded")
}

func TestRegistrationIsIdempotent(t *testing.T) {
	rec := newRecorder()
	r := registryWith(rec, KindAPI)
	resource := Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}

	require.NoError(t, r.Register(resource))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "different-actor", Description: "duplicate"}))
	require.Equal(t, 1, r.Count())
	require.NoError(t, r.Sweep(context.Background()))
	require.Equal(t, []string{"api:api-1"}, rec.order())
	require.Equal(t, "admin", rec.deleted[0].Actor)
}

func TestRegistryZeroValueIsUsable(t *testing.T) {
	var r Registry
	rec := newRecorder()
	require.NoError(t, r.RegisterDeleter(KindAPI, rec.deleter))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	require.NoError(t, r.Sweep(context.Background()))
	require.Equal(t, []string{"api:api-1"}, rec.order())
}

func TestRegisterDeleterRejectsInvalidValues(t *testing.T) {
	r := NewRegistry(nil)
	require.ErrorContains(t, r.RegisterDeleter(Kind{}, func(context.Context, Resource) error {
		return nil
	}), "no kind")
	require.ErrorContains(t, r.RegisterDeleter(KindAPI, nil), "nil deleter")
}

func TestPendingReturnsAnIndependentOrderedCopy(t *testing.T) {
	r := NewRegistry(nil)
	resources := []Resource{
		{Kind: KindAPI, ID: "api-1", Actor: "admin"},
		{Kind: KindSubscription, ID: "sub-1", Actor: "admin"},
		{Kind: KindAPI, ID: "api-2", Actor: "admin"},
	}
	for _, resource := range resources {
		require.NoError(t, r.Register(resource))
	}

	pending := r.Pending()
	require.Equal(t, []string{"subscription:sub-1", "api:api-2", "api:api-1"}, resourceKeys(pending))
	pending[0].ID = "changed"
	require.Equal(t, "sub-1", r.Pending()[0].ID)
}

func TestAllDefinedKindsFollowTheirConfiguredOrder(t *testing.T) {
	r := NewRegistry(nil)
	kinds := []Kind{
		KindEnvironment, KindCertificate, KindSharedScope, KindPolicy, KindMCPServer,
		KindAPI, KindAPIProduct, KindAPIKey, KindApplication, KindSubscription,
		KindSecret, KindLLMProxy, KindLLMProvider,
	}
	for i, kind := range kinds {
		require.NoError(t, r.Register(Resource{Kind: kind, ID: string(rune('a' + i)), Actor: "admin"}))
	}

	got := r.Pending()
	for i := 1; i < len(got); i++ {
		require.LessOrEqual(t, got[i-1].Kind.Order, got[i].Kind.Order)
	}
}

func TestSweepPassesContextToDeleter(t *testing.T) {
	r := NewRegistry(nil)
	key := struct{}{}
	value := "value"
	var got context.Context
	require.NoError(t, r.RegisterDeleter(KindAPI, func(ctx context.Context, _ Resource) error {
		got = ctx
		return nil
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	ctx := context.WithValue(context.Background(), key, value)
	require.NoError(t, r.Sweep(ctx))
	require.Equal(t, value, got.Value(key))
}

func TestSweepPropagatesCanceledContext(t *testing.T) {
	r := NewRegistry(nil)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(ctx context.Context, _ Resource) error {
		return ctx.Err()
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, r.Sweep(ctx), context.Canceled)
}

func TestSweepJoinsMultipleDeleteErrors(t *testing.T) {
	errA := errors.New("delete A")
	errB := errors.New("delete B")
	r := NewRegistry(nil)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(_ context.Context, res Resource) error {
		if res.ID == "api-a" {
			return errA
		}
		return errB
	}))
	for _, id := range []string{"api-a", "api-b"} {
		require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: id, Actor: "admin"}))
	}
	err := r.Sweep(context.Background())
	require.ErrorIs(t, err, errA)
	require.ErrorIs(t, err, errB)
}

func TestFailedDeleteCanBeRetried(t *testing.T) {
	var attempts int
	r := NewRegistry(nil, configuredMaxCleanupAttempts)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary failure")
		}
		return nil
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	require.Error(t, r.Sweep(context.Background()))
	require.Equal(t, 1, r.Count(), "failed resources must remain available for retry")
	require.NoError(t, r.Sweep(context.Background()))
	require.Equal(t, 2, attempts)
}

func TestFailedDeleteStopsAfterMaximumAttempts(t *testing.T) {
	var attempts int
	r := NewRegistry(nil, configuredMaxCleanupAttempts)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		attempts++
		return errors.New("permanent failure")
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	for attempt := 1; attempt <= configuredMaxCleanupAttempts; attempt++ {
		require.Error(t, r.Sweep(context.Background()))
		if attempt < configuredMaxCleanupAttempts {
			require.Equal(t, 1, r.Count())
		}
	}
	require.Equal(t, configuredMaxCleanupAttempts, attempts)
	require.Zero(t, r.Count(), "permanently failing resources must not retry forever")
	require.NoError(t, r.Sweep(context.Background()))
}

func TestFailedResourceRequeueCollisionIsLogged(t *testing.T) {
	var logs bytes.Buffer
	started := make(chan struct{})
	release := make(chan struct{})
	r := NewRegistry(slog.New(slog.NewTextHandler(&logs, nil)), configuredMaxCleanupAttempts)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		close(started)
		<-release
		return errors.New("delete failed")
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	done := make(chan error, 1)
	go func() { done <- r.Sweep(context.Background()) }()
	<-started
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	close(release)
	require.Error(t, <-done)
	require.Equal(t, 1, r.Count())
	require.Contains(t, logs.String(), "failed cleanup resource already registered; retry retained")
}

func TestCleanupWarningsAreLogged(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	r := NewRegistry(log)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		return errors.New("delete failed")
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	require.NoError(t, r.Register(Resource{Kind: KindApplication, ID: "app-1", Actor: "admin"}))

	require.Error(t, r.Sweep(context.Background()))
	require.Contains(t, logs.String(), "resource cleanup failed: delete error")
	require.Contains(t, logs.String(), "resource cleanup failed: no deleter")
	require.Contains(t, logs.String(), "time=")
}

func TestDeleteFailureDoesNotRetryByDefault(t *testing.T) {
	attempts := 0
	r := NewRegistry(nil)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		attempts++
		return errors.New("delete failed")
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	require.Error(t, r.Sweep(context.Background()))
	require.NoError(t, r.Sweep(context.Background()))
	require.Equal(t, 1, attempts)
	require.Zero(t, r.Count())
}

func TestDeletesAsTheCreatingActor(t *testing.T) {
	// Deleting as the wrong principal yields a cross-tenant 404 that looks like "already
	// gone", so the resource leaks while the run stays green.
	rec := newRecorder()
	r := registryWith(rec, KindAPI, KindApplication)

	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin@tenant1.com"}))
	require.NoError(t, r.Register(Resource{Kind: KindApplication, ID: "app-1", Actor: "subscriberUser"}))

	require.NoError(t, r.Sweep(context.Background()))

	byID := map[string]string{}
	for _, d := range rec.deleted {
		byID[d.ID] = d.Actor
	}
	require.Equal(t, "admin@tenant1.com", byID["api-1"])
	require.Equal(t, "subscriberUser", byID["app-1"],
		"each resource must be deleted as ITS creator, not a single acting actor")
}

func TestFKSafeDeletionOrder(t *testing.T) {
	t.Run("referencing resources go before what they reference", func(t *testing.T) {
		// Registered in the WRONG order deliberately: if order came from registration, the
		// API would be deleted while an application still referenced it.
		rec := newRecorder()
		r := registryWith(rec, KindAPI, KindApplication, KindSubscription)

		require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
		require.NoError(t, r.Register(Resource{Kind: KindApplication, ID: "app-1", Actor: "admin"}))
		require.NoError(t, r.Register(Resource{Kind: KindSubscription, ID: "sub-1", Actor: "admin"}))

		require.NoError(t, r.Sweep(context.Background()))
		require.Equal(t,
			[]string{"subscription:sub-1", "application:app-1", "api:api-1"},
			rec.order(),
			"subscription -> application -> api, regardless of registration order")
	})

	t.Run("within one kind, later registrations are deleted first", func(t *testing.T) {
		// A resource created later may reference one created earlier — an API version, say.
		rec := newRecorder()
		r := registryWith(rec, KindAPI)

		for _, id := range []string{"first", "second", "third"} {
			require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: id, Actor: "admin"}))
		}

		require.NoError(t, r.Sweep(context.Background()))
		require.Equal(t, []string{"api:third", "api:second", "api:first"}, rec.order())
	})

	t.Run("different kinds with the same order are deterministic", func(t *testing.T) {
		rec := newRecorder()
		r := registryWith(rec, KindSecret, KindLLMProxy)
		require.NoError(t, r.Register(Resource{Kind: KindSecret, ID: "secret-1", Actor: "admin"}))
		require.NoError(t, r.Register(Resource{Kind: KindLLMProxy, ID: "proxy-1", Actor: "admin"}))
		require.NoError(t, r.Sweep(context.Background()))
		require.Equal(t, []string{"llm-proxy:proxy-1", "secret:secret-1"}, rec.order())
	})
}

func TestMissingDeleterIsReportedNotSkipped(t *testing.T) {
	// The worst possible outcome is silence: a kind registered with no way to delete it
	// leaks with no error and no warning. So the absence is an error.
	rec := newRecorder()
	r := registryWith(rec, KindAPI) // no deleter for applications

	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	require.NoError(t, r.Register(Resource{
		Kind: KindApplication, ID: "app-1", Actor: "admin", Description: "orphan",
	}))

	err := r.Sweep(context.Background())
	require.ErrorContains(t, err, "no deleter registered for kind \"application\"")
	require.ErrorContains(t, err, "leaked")

	// And the resources that COULD be deleted still were.
	require.Equal(t, []string{"api:api-1"}, rec.order())
}

func TestSweepIsBestEffortButLoud(t *testing.T) {
	// One stubborn resource must not strand everything registered after it.
	rec := newRecorder()
	rec.fail["api-2"] = errors.New("409 conflict")
	r := registryWith(rec, KindAPI)

	for _, id := range []string{"api-1", "api-2", "api-3"} {
		require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: id, Actor: "admin"}))
	}

	err := r.Sweep(context.Background())
	require.ErrorContains(t, err, "api-2")
	require.ErrorContains(t, err, "409 conflict")

	// The other two were still deleted.
	require.ElementsMatch(t, []string{"api:api-3", "api:api-1"}, rec.order())
}

func TestSweepIsIdempotent(t *testing.T) {
	// Called from both a per-scenario hook and a per-runner hook, and safe twice.
	rec := newRecorder()
	r := registryWith(rec, KindAPI)
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	require.NoError(t, r.Sweep(context.Background()))
	require.NoError(t, r.Sweep(context.Background()))
	require.NoError(t, r.Sweep(context.Background()))

	require.Len(t, rec.deleted, 1, "a resource must be deleted exactly once")
	require.Zero(t, r.Count())
}

func TestRegistrationDuringSweepIsRetained(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	r := NewRegistry(nil)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		once.Do(func() {
			close(started)
			<-release
		})
		return nil
	}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))

	sweepDone := make(chan error, 1)
	go func() { sweepDone <- r.Sweep(context.Background()) }()
	<-started
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-2", Actor: "admin"}))
	close(release)
	require.NoError(t, <-sweepDone)

	require.Equal(t, 1, r.Count(), "resources registered during a sweep must not be lost")
	require.NoError(t, r.Sweep(context.Background()))
	require.Zero(t, r.Count())
}

func TestDeregister(t *testing.T) {
	// For the case where a test deleted a resource itself and asserted on the deletion;
	// without this, teardown would retry and log a spurious leak.
	rec := newRecorder()
	r := registryWith(rec, KindAPI)

	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
	require.NoError(t, r.Register(Resource{Kind: KindAPI, ID: "api-2", Actor: "admin"}))

	r.Deregister(KindAPI, "api-1")
	r.Deregister(KindAPI, "unknown")
	r.Deregister(KindAPI, "api-1")
	require.Equal(t, 1, r.Count())

	require.NoError(t, r.Sweep(context.Background()))
	require.Equal(t, []string{"api:api-2"}, rec.order())
}

func TestContextIntegration(t *testing.T) {
	t.Run("register and sweep through the context", func(t *testing.T) {
		ctx := runnerCtx()
		rec := newRecorder()
		require.NoError(t, Install(ctx, registryWith(rec, KindAPI)))

		require.NoError(t, Register(ctx, Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"}))
		require.NoError(t, Sweep(ctx))
		require.Equal(t, []string{"api:api-1"}, rec.order())
	})

	t.Run("registering with no registry installed is an error", func(t *testing.T) {
		// A resource registered into a registry nobody sweeps leaks silently, so a missing
		// installation must be visible rather than lazily papered over.
		err := Register(runnerCtx(), Resource{Kind: KindAPI, ID: "api-1", Actor: "admin"})
		require.ErrorContains(t, err, "no registry in context")
		require.ErrorContains(t, err, "would never be swept")
	})

	t.Run("sweeping with no registry reports the wiring error", func(t *testing.T) {
		require.ErrorContains(t, Sweep(runnerCtx()), "no registry in context")
	})

	t.Run("a wrongly-typed registry value is reported", func(t *testing.T) {
		ctx := runnerCtx()
		require.NoError(t, tcontext.Set(ctx, registryKey, 42))
		_, err := Of(ctx)
		require.ErrorContains(t, err, "does not contain a valid registry")
	})

	t.Run("a typed-nil registry is reported", func(t *testing.T) {
		ctx := runnerCtx()
		var registry *Registry
		require.NoError(t, tcontext.Set(ctx, registryKey, registry))
		_, err := Of(ctx)
		require.ErrorContains(t, err, "does not contain a valid registry")
	})

	t.Run("nil registry installation is rejected", func(t *testing.T) {
		require.ErrorContains(t, Install(runnerCtx(), nil), "nil")
	})

	t.Run("registries remain isolated between contexts", func(t *testing.T) {
		first, second := runnerCtx(), runnerCtx()
		firstRecorder, secondRecorder := newRecorder(), newRecorder()
		require.NoError(t, Install(first, registryWith(firstRecorder, KindAPI)))
		require.NoError(t, Install(second, registryWith(secondRecorder, KindAPI)))
		require.NoError(t, Register(first, Resource{Kind: KindAPI, ID: "first", Actor: "admin"}))
		require.NoError(t, Register(second, Resource{Kind: KindAPI, ID: "second", Actor: "admin"}))
		require.NoError(t, Sweep(first))
		require.Empty(t, secondRecorder.order())
		require.NoError(t, Sweep(second))
		require.Equal(t, []string{"api:first"}, firstRecorder.order())
		require.Equal(t, []string{"api:second"}, secondRecorder.order())
	})
}

func TestConcurrentRegistration(t *testing.T) {
	// Scenarios within a runner may run concurrently and all register into one registry.
	// Run with -race.
	rec := newRecorder()
	r := registryWith(rec, KindAPI, KindApplication)

	const workers = 24
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kind := KindAPI
			if i%2 == 0 {
				kind = KindApplication
			}
			if err := r.Register(Resource{
				Kind:  kind,
				ID:    kind.Name + "-" + string(rune('a'+i%26)),
				Actor: "admin",
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	require.Equal(t, workers, r.Count())
	require.NoError(t, r.Sweep(context.Background()))
	require.Len(t, rec.deleted, workers, "every registered resource must be swept exactly once")

	// Applications before APIs, even under concurrent registration.
	order := rec.order()
	lastApp, firstAPI := -1, -1
	for i, o := range order {
		if len(o) > 11 && o[:11] == "application" {
			lastApp = i
		}
		if firstAPI < 0 && len(o) > 3 && o[:3] == "api" {
			firstAPI = i
		}
	}
	if lastApp >= 0 && firstAPI >= 0 {
		require.Less(t, lastApp, firstAPI, "all applications must precede all APIs")
	}
}

func TestConcurrentRegistryOperations(t *testing.T) {
	r := NewRegistry(nil)
	require.NoError(t, r.RegisterDeleter(KindAPI, func(context.Context, Resource) error {
		return nil
	}))

	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(5)
		go func(i int) {
			defer wg.Done()
			_ = r.Register(Resource{Kind: KindAPI, ID: "api-" + string(rune('a'+i)), Actor: "admin"})
		}(i)
		go func() {
			defer wg.Done()
			_ = r.Pending()
		}()
		go func() {
			defer wg.Done()
			_ = r.Count()
		}()
		go func(i int) {
			defer wg.Done()
			r.Deregister(KindAPI, "missing-"+string(rune('a'+i)))
		}(i)
		go func() {
			defer wg.Done()
			_ = r.RegisterDeleter(KindAPI, func(context.Context, Resource) error { return nil })
		}()
	}
	wg.Wait()
}
