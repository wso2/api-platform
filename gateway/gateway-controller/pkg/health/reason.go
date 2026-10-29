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

package health

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// genericFailure is the reason shown for any check error that does not carry a Reason.
const genericFailure = "check failed"

// Reason is an error whose text is safe to show in a health response and in health log
// lines. A check returns one (directly or wrapped) to explain a failure in its own words;
// the text must be fixed and written by the check author, never built from a driver or
// network error, because health responses are served without authentication.
type Reason string

// Error returns the reason text.
func (r Reason) Error() string { return string(r) }

// safeReason maps a check error to text that may leave the process: the check's own
// Reason, a timeout message, or a generic failure. The raw error text is never returned,
// since it can carry connection strings, hostnames or credentials.
func safeReason(err error, timeout time.Duration) string {
	var r Reason
	if errors.As(err, &r) {
		return string(r)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("timed out after %s", timeout)
	}
	return genericFailure
}
