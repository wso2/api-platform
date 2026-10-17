/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package server

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/wso2/api-platform/platform-api/internal/health"
)

// errDatabaseUnreachable is the reason reported when the database answers a ping with an
// error. Health answers are served without authentication, so the driver error itself,
// which can name hosts, users or DSN fragments, is kept only as the wrapped cause.
var errDatabaseUnreachable = health.Reason("database unreachable")

// pinger is the part of the database pool the health check needs.
type pinger interface {
	PingContext(ctx context.Context) error
}

// newDatabaseCheck returns the health check that pings the database pool. A ping cut
// short by the check deadline is reported as a timeout; any other ping error as the
// database being unreachable.
//
// At most one ping is in flight at a time. A driver that ignores its context could
// otherwise leave one ping per evaluation stuck on the database, so probes against a
// hung database would pile up pooled connections; while an earlier ping is still
// running, the check reports a timeout without starting another.
func newDatabaseCheck(db pinger) health.CheckFunc {
	var running atomic.Bool
	return func(ctx context.Context) error {
		if !running.CompareAndSwap(false, true) {
			return context.DeadlineExceeded
		}
		// Buffered, so the ping goroutine never blocks once the check has given up.
		done := make(chan error, 1)
		go func() {
			err := db.PingContext(ctx)
			// Cleared before the result is handed over, so the next evaluation after
			// this one can ping again.
			running.Store(false)
			done <- err
		}()

		select {
		case err := <-done:
			if err == nil {
				return nil
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("%w: %w", errDatabaseUnreachable, err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
