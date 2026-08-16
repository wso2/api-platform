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
	"fmt"
	"log/slog"
	"time"
)

// PropagationCeiling is the deadline floor for every wait here. A shorter Timeout is raised
// to it, so no call site waits less than the shared minimum.
const PropagationCeiling = 180 * time.Second

// BaseInterval is the poll cadence floor. A shorter Interval is raised to it, so a wait
// cannot hammer a component that is already behind.
const BaseInterval = 2 * time.Second

// pacing widens the interval as the wait goes on: base for the first minute, then 5s, then 10s.
func pacing(elapsed, base time.Duration) time.Duration {
	switch {
	case elapsed < time.Minute:
		return base
	case elapsed < 2*time.Minute:
		return max(base, 5*time.Second)
	default:
		return max(base, 10*time.Second)
	}
}

// Until: the result is the assertion target.

// Attempt produces a result. Its error means the call could not be made, not that it returned
// something unwanted.
type Attempt[T any] func(ctx context.Context) (T, error)

// Accept reports whether a result is the one being waited for.
type Accept[T any] func(T) bool

// Options tune a wait.
type Options struct {
	// Timeout is a MINIMUM, floored at PropagationCeiling.
	Timeout time.Duration
	// Interval is the base polling cadence.
	Interval time.Duration
	// Retryable classifies an attempt error as transient. Nil means only errors matching
	// IsTransient are retried.
	Retryable func(error) bool
	// Logger receives self-heal and diagnostic lines.
	Logger *slog.Logger

	// subBaseIntervalForTests permits an Interval below BaseInterval. Unexported so only this
	// package's tests can set it.
	subBaseIntervalForTests bool

	// subCeilingTimeoutForTests permits a Timeout below PropagationCeiling, so a test can reach
	// the deadline-expiry exit — where Until returns a nil error having never satisfied accept —
	// without waiting out the real ceiling. Unexported for the same reason.
	subCeilingTimeoutForTests bool
}

func (o Options) deadline(now time.Time) time.Time {
	timeout := o.Timeout
	if timeout < PropagationCeiling && !o.subCeilingTimeoutForTests {
		timeout = PropagationCeiling
	}
	return now.Add(timeout)
}

func (o Options) interval() time.Duration {
	if o.Interval <= 0 {
		return BaseInterval
	}
	// Floored, so a step cannot poll faster than the suite's agreed cadence. The only way
	// below it is subBaseIntervalForTests, which is unexported.
	if o.Interval < BaseInterval && !o.subBaseIntervalForTests {
		return BaseInterval
	}
	return o.Interval
}

func (o Options) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.Default()
}

// Until polls until accept is satisfied and returns the last result either way, so the caller
// asserts on the exact value. Side-effect free. Only transient attempt errors are retried.
func Until[T any](ctx context.Context, opts Options, attempt Attempt[T], accept Accept[T]) (T, error) {
	var last T
	var lastErr error

	start := time.Now()
	deadline := opts.deadline(start)
	attempts := 0

	for {
		attempts++
		result, err := attempt(ctx)
		switch {
		case err == nil:
			last = result
			lastErr = nil
			if accept(result) {
				return result, nil
			}
		case isRetryable(err, opts.Retryable):
			lastErr = err
		default:
			return last, fmt.Errorf("retry: attempt failed with a non-retryable error after %s: %w",
				time.Since(start).Round(time.Millisecond), err)
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return last, fmt.Errorf("retry: cancelled after %d attempt(s): %w", attempts, ctxErr)
		}
		if !time.Now().Before(deadline) {
			break
		}

		wait := pacing(time.Since(start), opts.interval())
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return last, fmt.Errorf("retry: cancelled while waiting: %w", ctx.Err())
			case <-time.After(wait):
			}
		}
	}

	// No error when attempts succeeded but never satisfied accept: the caller asserts on the
	// returned result.
	if lastErr != nil {
		return last, fmt.Errorf("retry: never succeeded within %s (%d attempt(s)); last error: %w",
			time.Since(start).Round(time.Second), attempts, lastErr)
	}
	return last, nil
}

// Await polls until accept holds, and fails when it never did.
//
// Prefer this over Until. Until returns a NIL ERROR when every attempt succeeded at the transport
// level but none ever satisfied accept, so a caller that checks only the error passes after
// burning its whole budget. Await folds that check in, so it cannot be forgotten. Use Until only
// where the failure message needs the last result.
func Await[T any](
	ctx context.Context, opts Options, attempt Attempt[T], accept Accept[T], what string,
) error {
	last, err := Until(ctx, opts, attempt, accept)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if accept(last) {
		return nil
	}
	if d, ok := any(last).(interface{ Describe() string }); ok {
		return fmt.Errorf("%s: the condition never held; the last response was %s", what, d.Describe())
	}
	return fmt.Errorf("%s: the condition never held; the last result was %v", what, last)
}

// SettledCount: a monotonic counter's final value.

// Counter reports a monotonically non-decreasing value.
type Counter func(ctx context.Context) (int, error)

// Settled is the outcome of waiting for a counter to go quiet.
type Settled struct {
	// Value is the last observed value.
	Value int
	// Quiet reports whether the value stopped changing for the required quiet period. When
	// false, the value was still moving when the deadline passed.
	Quiet bool
	// Samples is how many observations were taken.
	Samples int
}

// SettledCount waits until a counter stops changing for quiet, then reports its final value.
// Use it when the assertion target is HOW MANY arrived: Until would return the moment the
// counter touches N and miss a later arrival. Choose quiet longer than the gap between
// arrivals.
func SettledCount(ctx context.Context, opts Options, quiet time.Duration, count Counter) (Settled, error) {
	if quiet <= 0 {
		quiet = 2 * time.Second
	}

	start := time.Now()
	deadline := opts.deadline(start)

	var (
		out         Settled
		lastValue   = -1
		lastChanged = start
	)

	for {
		value, err := count(ctx)
		if err != nil {
			if !isRetryable(err, opts.Retryable) {
				return out, fmt.Errorf("retry: counting failed with a non-retryable error: %w", err)
			}
		} else {
			out.Samples++
			if value != lastValue {
				if lastValue > value {
					return out, fmt.Errorf("retry: counter decreased from %d to %d, which is not monotonic",
						lastValue, value)
				}
				lastValue = value
				lastChanged = time.Now()
			}
			out.Value = value

			if time.Since(lastChanged) >= quiet {
				out.Quiet = true
				return out, nil
			}
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return out, fmt.Errorf("retry: counting cancelled after %d sample(s): %w", out.Samples, ctxErr)
		}
		if !time.Now().Before(deadline) {
			return out, nil
		}

		wait := pacing(time.Since(start), opts.interval())
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return out, fmt.Errorf("retry: counting cancelled: %w", ctx.Err())
			case <-time.After(wait):
			}
		}
	}
}

// Transient classification.

// transientError marks an error as worth retrying.
type transientError struct{ err error }

func (t transientError) Error() string { return t.err.Error() }
func (t transientError) Unwrap() error { return t.err }

// Transient marks an error as retryable — a connection refused while a component warms up,
// for instance. Anything not marked is treated as a programming error and fails fast.
func Transient(err error) error {
	if err == nil {
		return nil
	}
	return transientError{err: err}
}

// IsTransient reports whether an error was marked retryable.
func IsTransient(err error) bool {
	var t transientError
	return errors.As(err, &t)
}

func isRetryable(err error, custom func(error) bool) bool {
	if err == nil {
		return false
	}
	if custom != nil && custom(err) {
		return true
	}
	return IsTransient(err)
}

// LogAuthRejection logs an authentication rejection seen inside a wait and returns the same
// text for the caller's assertion message. A rejected credential will not start working, so
// the wait stops here rather than timing out.
func LogAuthRejection(log *slog.Logger, what, credentialKey string, status int, body string, elapsed time.Duration) string {
	if log == nil {
		log = slog.Default()
	}
	const maxBody = 256
	if len(body) > maxBody {
		body = body[:maxBody] + "…"
	}
	msg := fmt.Sprintf("auth-reject: %s was rejected with %d after %s (credential from context key %q): %s",
		what, status, elapsed.Round(time.Millisecond), credentialKey, body)
	log.Warn("auth-reject", "what", what, "status", status, "credentialKey", credentialKey,
		"elapsed", elapsed.Round(time.Millisecond), "body", body)
	return msg
}

// Never: an invariant that must hold for a whole window.

// Forbidden reports whether an attempt's result violates the invariant.
type Forbidden[T any] func(T) bool

// Never polls for the whole window and fails the moment forbidden holds, proving absence across
// time rather than at one instant.
//
// The window must exceed the longest latency the positive path allows, or the check proves
// nothing. It is NOT floored at PropagationCeiling: for a negative the window is a
// cost-versus-confidence choice. Zero means PropagationCeiling.
//
// Returns the last observed result either way.
func Never[T any](
	ctx context.Context, opts Options, window time.Duration, attempt Attempt[T], forbidden Forbidden[T],
) (T, error) {
	var last T
	if window <= 0 {
		window = PropagationCeiling
	}
	start := time.Now()
	deadline := start.Add(window)
	attempts := 0
	for {
		result, err := attempt(ctx)
		switch {
		case err == nil:
			last = result
			attempts++
			if forbidden(result) {
				return last, fmt.Errorf(
					"retry: the invariant was violated after %s (%d attempt(s)); it was expected to hold for %s",
					time.Since(start).Round(time.Millisecond), attempts, window)
			}
		case isRetryable(err, opts.Retryable):
			// A transient error is not a clean sample, so it must not count toward the window.
		default:
			return last, fmt.Errorf("retry: attempt failed with a non-retryable error after %s: %w",
				time.Since(start).Round(time.Millisecond), err)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return last, fmt.Errorf("retry: cancelled after %d attempt(s): %w", attempts, ctxErr)
		}
		if !time.Now().Before(deadline) {
			if attempts == 0 {
				// No clean sample, so the window proved nothing.
				return last, fmt.Errorf(
					"retry: the invariant could not be checked — no attempt succeeded within %s", window)
			}
			return last, nil
		}
		wait := pacing(time.Since(start), opts.interval())
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return last, fmt.Errorf("retry: cancelled while waiting: %w", ctx.Err())
			case <-time.After(wait):
			}
		}
	}
}
