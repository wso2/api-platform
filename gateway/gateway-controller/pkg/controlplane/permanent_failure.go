/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package controlplane

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
)

// errPermanentControlPlaneFailure marks a control-plane response that cannot be
// resolved by retrying (misconfiguration: bad token, wrong gateway ID, version
// mismatch, etc.). Callers must abort and exit the process so the operator
// notices via CrashLoopBackOff instead of silent infinite reconnects.
var errPermanentControlPlaneFailure = errors.New("permanent control plane failure")

// permanentControlPlaneStatuses are HTTP status codes from the control plane
// that indicate a configuration error the controller cannot self-heal from.
var permanentControlPlaneStatuses = map[int]struct{}{
	http.StatusUnauthorized:        {}, // 401
	http.StatusForbidden:           {}, // 403
	http.StatusNotFound:            {}, // 404
	http.StatusConflict:            {}, // 409
	http.StatusUnprocessableEntity: {}, // 422
}

// isPermanentControlPlaneStatus reports whether the HTTP status code from the
// control plane represents a permanent (non-retryable) failure.
func isPermanentControlPlaneStatus(code int) bool {
	_, ok := permanentControlPlaneStatuses[code]
	return ok
}

// exitOnPermanentFailure logs a fatal error and terminates the process so the
// container runtime (Kubernetes) surfaces the misconfiguration via restart
// status instead of looping silently.
func (c *Client) exitOnPermanentFailure(reason string, err error) {
	c.logger.Error("Fatal control-plane error, exiting controller",
		slog.String("reason", reason),
		slog.Any("error", err),
	)
	os.Exit(1)
}
