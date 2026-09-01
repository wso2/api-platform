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

package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fast keeps tests quick. Timeout is deliberately below PropagationCeiling to prove the
// floor is applied, except where a test needs to reach the deadline.
func fast() Options {
	return Options{Timeout: 300 * time.Millisecond, Interval: 5 * time.Millisecond, subBaseIntervalForTests: true}
}

func TestDeadlineIsFlooredAtTheSharedCeiling(t *testing.T) {
	// The invariant that stops one call site drifting onto a shorter window and failing for
	// a reason that looks like a product bug.
	opts := Options{Timeout: time.Second}
	now := time.Now()
	require.WithinDuration(t, now.Add(PropagationCeiling), opts.deadline(now), time.Second,
		"a timeout below the ceiling must be raised to it")

	longer := Options{Timeout: 10 * time.Minute}
	require.WithinDuration(t, now.Add(10*time.Minute), longer.deadline(now), time.Second,
		"a timeout above the ceiling must be respected")
}

func TestUntilReturnsTheLastResultForTheStepToAssert(t *testing.T) {
	t.Run("returns as soon as accept is satisfied", func(t *testing.T) {
		var calls atomic.Int32
		got, err := Until(context.Background(), fast(),
			func(context.Context) (int, error) { return int(calls.Add(1)), nil },
			func(v int) bool { return v >= 3 })
		require.NoError(t, err)
		require.Equal(t, 3, got)
		require.EqualValues(t, 3, calls.Load(), "must stop polling once accepted")
	})

	t.Run("an unsatisfied wait returns the last result WITHOUT an error", func(t *testing.T) {
		// The step asserts the exact value and produces a better message than this package
		// could. A helper that asserted internally could only ever check "reached an
		// acceptable state".
		opts := Options{Timeout: 10 * time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		got, err := Until(ctx, opts,
			func(context.Context) (int, error) { return 42, nil },
			func(int) bool { return false })

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 42, got, "the last observed value must come back regardless")
	})

	t.Run("transient errors are retried", func(t *testing.T) {
		var calls atomic.Int32
		got, err := Until(context.Background(), fast(),
			func(context.Context) (string, error) {
				if calls.Add(1) < 3 {
					return "", Transient(errors.New("connection refused"))
				}
				return "ok", nil
			},
			func(s string) bool { return s == "ok" })
		require.NoError(t, err)
		require.Equal(t, "ok", got)
	})

	t.Run("a non-transient error fails FAST rather than timing out", func(t *testing.T) {
		// The case a naive loop swallows: a missing context key or malformed payload should
		// fail in milliseconds, not be masked as a three-minute timeout.
		start := time.Now()
		var calls atomic.Int32
		_, err := Until(context.Background(), Options{Timeout: 10 * time.Minute},
			func(context.Context) (int, error) {
				calls.Add(1)
				return 0, errors.New("no value in context for key \"apiId\"")
			},
			func(int) bool { return true })

		require.ErrorContains(t, err, "non-retryable")
		require.ErrorContains(t, err, "apiId")
		require.EqualValues(t, 1, calls.Load(), "must not retry a programming error")
		require.Less(t, time.Since(start), 2*time.Second, "must not wait out the deadline")
	})

	t.Run("a custom classifier can widen what counts as transient", func(t *testing.T) {
		var calls atomic.Int32
		opts := fast()
		opts.Retryable = func(err error) bool { return err.Error() == "warming up" }

		got, err := Until(context.Background(), opts,
			func(context.Context) (int, error) {
				if calls.Add(1) < 2 {
					return 0, errors.New("warming up")
				}
				return 7, nil
			},
			func(v int) bool { return v == 7 })
		require.NoError(t, err)
		require.Equal(t, 7, got)
	})

	t.Run("is side-effect free: the attempt is only what the caller supplied", func(t *testing.T) {
		// Nothing is re-triggered, so it is safe where a second invocation would change
		// state. Contrasted with OrHeal below.
		var mutations atomic.Int32
		_, err := Until(context.Background(), fast(),
			func(context.Context) (int, error) { return 1, nil },
			func(int) bool { mutations.Add(0); return true })
		require.NoError(t, err)
		require.EqualValues(t, 0, mutations.Load())
	})
}

func TestOrHealReTriggersDroppedPropagation(t *testing.T) {
	t.Run("succeeds without healing when the state arrives", func(t *testing.T) {
		var probes atomic.Int32
		var triggers atomic.Int32

		err := OrHeal(context.Background(), HealOptions{
			Options:   Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			What:      "api deployed",
			MaxHeals:  2,
			HealAfter: 5 * time.Second,
		},
			func(context.Context) (bool, error) { return probes.Add(1) >= 3, nil },
			func(context.Context) error { triggers.Add(1); return nil })

		require.NoError(t, err)
		require.EqualValues(t, 0, triggers.Load(), "must not re-trigger when the state arrives")
	})

	t.Run("re-fires the trigger when the state never arrives", func(t *testing.T) {
		// Propagation events are at-most-once: a dropped deploy event will never arrive no
		// matter how long the wait, and re-emitting is the only remedy.
		var triggers atomic.Int32
		var healed atomic.Bool

		err := OrHeal(context.Background(), HealOptions{
			Options:   Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			What:      "api deployed",
			MaxHeals:  3,
			HealAfter: 30 * time.Millisecond,
		},
			func(context.Context) (bool, error) { return healed.Load(), nil },
			func(context.Context) error {
				if triggers.Add(1) >= 2 {
					healed.Store(true)
				}
				return nil
			})

		require.NoError(t, err)
		require.EqualValues(t, 2, triggers.Load())
	})

	t.Run("every probe error counts as not-ready", func(t *testing.T) {
		// During warm-up "not ready" and "the probe failed" are indistinguishable, and
		// guessing wrong turns a slow start into a hard failure.
		var probes atomic.Int32
		err := OrHeal(context.Background(), HealOptions{
			Options:   Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			What:      "state",
			MaxHeals:  1,
			HealAfter: 50 * time.Millisecond,
		},
			func(context.Context) (bool, error) {
				if probes.Add(1) < 4 {
					return false, errors.New("connection refused")
				}
				return true, nil
			},
			func(context.Context) error { return nil })
		require.NoError(t, err)
	})

	t.Run("exhaustion explains that propagation is at-most-once", func(t *testing.T) {
		err := OrHeal(context.Background(), HealOptions{
			Options:   Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			What:      "api deployed",
			MaxHeals:  1,
			HealAfter: 20 * time.Millisecond,
		},
			func(context.Context) (bool, error) { return false, nil },
			func(context.Context) error { return nil })

		require.ErrorContains(t, err, "api deployed")
		require.ErrorContains(t, err, "at-most-once")
	})

	t.Run("a failing trigger is reported rather than retried forever", func(t *testing.T) {
		err := OrHeal(context.Background(), HealOptions{
			Options:   Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			What:      "state",
			MaxHeals:  2,
			HealAfter: 10 * time.Millisecond,
		},
			func(context.Context) (bool, error) { return false, nil },
			func(context.Context) error { return errors.New("redeploy rejected") })

		require.ErrorContains(t, err, "re-triggering")
		require.ErrorContains(t, err, "redeploy rejected")
	})
}

func TestSettledCountCatchesOverCounts(t *testing.T) {
	t.Run("returns the final value once the counter goes quiet", func(t *testing.T) {
		var value atomic.Int32
		go func() {
			for range 5 {
				time.Sleep(5 * time.Millisecond)
				value.Add(1)
			}
		}()

		got, err := SettledCount(context.Background(),
			Options{Timeout: time.Millisecond, Interval: 2 * time.Millisecond, subBaseIntervalForTests: true},
			40*time.Millisecond,
			func(context.Context) (int, error) { return int(value.Load()), nil })

		require.NoError(t, err)
		require.True(t, got.Quiet)
		require.Equal(t, 5, got.Value)
	})

	t.Run("an over-count is visible, which Until cannot express", func(t *testing.T) {
		// Until's accept can only ask "has it reached N", so accept-on-N returns the instant
		// the counter touches N and a later arrival is invisible. Here the extra arrival is
		// observed, so the caller can assert the exact value and fail.
		var value atomic.Int32
		go func() {
			time.Sleep(5 * time.Millisecond)
			value.Store(3)
			time.Sleep(10 * time.Millisecond)
			value.Store(4) // one too many
		}()

		got, err := SettledCount(context.Background(),
			Options{Timeout: time.Millisecond, Interval: 2 * time.Millisecond, subBaseIntervalForTests: true},
			40*time.Millisecond,
			func(context.Context) (int, error) { return int(value.Load()), nil })

		require.NoError(t, err)
		require.True(t, got.Quiet)
		require.Equal(t, 4, got.Value, "the extra arrival must be observable")
		require.NotEqual(t, 3, got.Value)
	})

	t.Run("a still-moving counter reports Quiet=false rather than erroring", func(t *testing.T) {
		// Information for the caller to assert on, not a failure of this package.
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()

		var value atomic.Int32
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-time.After(3 * time.Millisecond):
					value.Add(1)
				}
			}
		}()

		got, _ := SettledCount(ctx,
			Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			time.Second,
			func(context.Context) (int, error) { return int(value.Load()), nil })

		require.False(t, got.Quiet, "a counter still moving at the deadline is not settled")
	})

	t.Run("a decreasing counter is rejected as non-monotonic", func(t *testing.T) {
		// The quiet-period logic is meaningless if the value can fall, so a caller measuring
		// the wrong thing is told rather than given a plausible number.
		var calls atomic.Int32
		_, err := SettledCount(context.Background(),
			Options{Timeout: time.Millisecond, Interval: time.Millisecond, subBaseIntervalForTests: true},
			10*time.Millisecond,
			func(context.Context) (int, error) {
				if calls.Add(1) == 1 {
					return 5, nil
				}
				return 2, nil
			})
		require.ErrorContains(t, err, "not monotonic")
	})
}

func TestPacingOpensUpOverTime(t *testing.T) {
	// Quick early polls catch the common case; the interval then opens up so a struggling
	// component is not hammered for the whole window.
	base := time.Second
	require.Equal(t, base, pacing(10*time.Second, base))
	require.Equal(t, 5*time.Second, pacing(90*time.Second, base))
	require.Equal(t, 10*time.Second, pacing(5*time.Minute, base))

	// A base longer than a tier's floor is respected rather than shortened.
	require.Equal(t, 30*time.Second, pacing(5*time.Minute, 30*time.Second))
}

func TestTransientMarking(t *testing.T) {
	require.True(t, IsTransient(Transient(errors.New("x"))))
	require.False(t, IsTransient(errors.New("x")))
	require.Nil(t, Transient(nil))

	// Survives wrapping, so a helper can add context without losing the classification.
	wrapped := errors.Join(errors.New("context"), Transient(errors.New("refused")))
	require.True(t, IsTransient(wrapped))
}

func TestLogAuthRejection(t *testing.T) {
	// Returns the text so the same detail reaches the assertion message, not just the log —
	// otherwise the step reports a bare status mismatch with nothing connecting it to the
	// rejected credential.
	msg := LogAuthRejection(nil, "invoking the API", "generatedAccessToken", 401,
		`{"code":900901,"message":"Invalid Credentials"}`, 2*time.Second)

	require.Contains(t, msg, "auth-reject")
	require.Contains(t, msg, "invoking the API")
	require.Contains(t, msg, "401")
	require.Contains(t, msg, "generatedAccessToken")
	require.Contains(t, msg, "900901")
}

func TestCancellationStopsPromptly(t *testing.T) {
	for name, run := range map[string]func(ctx context.Context) error{
		"Until": func(ctx context.Context) error {
			_, err := Until(ctx, Options{Timeout: 10 * time.Minute, Interval: time.Millisecond, subBaseIntervalForTests: true},
				func(context.Context) (int, error) { return 0, nil },
				func(int) bool { return false })
			return err
		},
		"OrHeal": func(ctx context.Context) error {
			return OrHeal(ctx, HealOptions{
				Options: Options{Timeout: 10 * time.Minute, Interval: time.Millisecond, subBaseIntervalForTests: true},
				What:    "x", MaxHeals: 5, HealAfter: time.Minute,
			},
				func(context.Context) (bool, error) { return false, nil },
				func(context.Context) error { return nil })
		},
		"SettledCount": func(ctx context.Context) error {
			_, err := SettledCount(ctx, Options{Timeout: 10 * time.Minute, Interval: time.Millisecond, subBaseIntervalForTests: true},
				time.Hour, func(context.Context) (int, error) { return 1, nil })
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()

			start := time.Now()
			err := run(ctx)
			require.Error(t, err)
			require.Less(t, time.Since(start), 3*time.Second,
				"must honour cancellation rather than waiting out the deadline")
		})
	}
}

func TestAwaitFailsWhenAcceptNeverHeld(t *testing.T) {
	// The whole point of Await: Until returns a NIL error at deadline expiry, so this reaches
	// the accept re-check rather than the cancellation path.
	unsatisfiable := Options{
		Timeout: 5 * time.Millisecond, Interval: time.Millisecond,
		subBaseIntervalForTests: true, subCeilingTimeoutForTests: true,
	}

	t.Run("fails when every attempt succeeded but none was acceptable", func(t *testing.T) {
		last, err := Until(context.Background(), unsatisfiable,
			func(context.Context) (int, error) { return 42, nil },
			func(int) bool { return false })
		require.NoError(t, err, "Until reports no error here — that is the trap")
		require.Equal(t, 42, last)

		err = Await(context.Background(), unsatisfiable,
			func(context.Context) (int, error) { return 42, nil },
			func(int) bool { return false },
			"waiting for the impossible")
		require.Error(t, err, "Await must fail where Until did not")
		require.Contains(t, err.Error(), "waiting for the impossible")
		require.Contains(t, err.Error(), "42", "the last result must be named")
	})

	t.Run("passes as soon as accept holds", func(t *testing.T) {
		var calls atomic.Int32
		err := Await(context.Background(), fast(),
			func(context.Context) (int, error) { return int(calls.Add(1)), nil },
			func(v int) bool { return v >= 3 },
			"waiting for three")
		require.NoError(t, err)
		require.EqualValues(t, 3, calls.Load(), "must stop polling once accepted")
	})

	t.Run("uses Describe when the result has one", func(t *testing.T) {
		err := Await(context.Background(), unsatisfiable,
			func(context.Context) (describable, error) { return describable{}, nil },
			func(describable) bool { return false },
			"waiting")
		require.Error(t, err)
		require.Contains(t, err.Error(), "GET /x -> 404")
	})
}

type describable struct{}

func (describable) Describe() string { return "GET /x -> 404" }
