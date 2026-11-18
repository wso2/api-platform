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
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// answer evaluates c and renders the result as the gateway health routes do.
func answer(t *testing.T, c *Checker) (int, string) {
	t.Helper()
	res := c.Evaluate()
	body, err := RenderBody(res)
	require.NoError(t, err)
	return StatusCode(res), string(body)
}

func failing(err error) CheckFunc {
	return func(context.Context) error { return err }
}

func TestAnswerHealthy(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", passing)
	c.Register("cache", passing)

	code, body := answer(t, c)

	assert.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `{
		"status": "ready",
		"timestamp": "2026-10-13T09:00:00Z",
		"checks": {
			"cache": {"status": "up"},
			"database": {"status": "up"}
		}
	}`, body)
}

func TestAnswerUnhealthy(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", failing(Reason("database unreachable")))
	c.Register("cache", passing)

	code, body := answer(t, c)

	assert.Equal(t, http.StatusServiceUnavailable, code)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, "unhealthy", got["status"])
	assert.NotContains(t, got, "component")

	checks := got["checks"].(map[string]any)
	down := checks["database"].(map[string]any)
	assert.Equal(t, "down", down["status"])
	assert.Equal(t, "database unreachable", down["error"])
	assert.Equal(t, "2026-10-13T09:00:00Z", down["since"])

	up := checks["cache"].(map[string]any)
	assert.Equal(t, "up", up["status"])
	assert.NotContains(t, up, "error")
	assert.NotContains(t, up, "since")
}

func TestAnswerNoChecksRendersEmptyObject(t *testing.T) {
	c, _ := newTestChecker(testOptions())

	code, body := answer(t, c)

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `"checks":{}`)
}

func TestAnswerTimestampIsUTCWithSecondPrecision(t *testing.T) {
	c, clk := newTestChecker(testOptions())
	zone := time.FixedZone("UTC+5:30", 5*3600+30*60)
	clk.mu.Lock()
	clk.t = time.Date(2026, 10, 13, 14, 30, 7, 987654321, zone)
	clk.mu.Unlock()
	c.Register("database", failing(errors.New("boom")))

	_, body := answer(t, c)

	var got struct {
		Timestamp string `json:"timestamp"`
		Checks    map[string]struct {
			Since string `json:"since"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, "2026-10-13T09:00:07Z", got.Timestamp)
	assert.Equal(t, "2026-10-13T09:00:07Z", got.Checks["database"].Since)
}

func TestAnswerNeverRendersRawCause(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", failing(errors.New("dial postgres://u:p@host/db: connection refused")))

	code, body := answer(t, c)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.NotContains(t, body, "postgres://u:p@host")
	assert.NotContains(t, body, "connection refused")
	assert.Contains(t, body, genericFailure)
}

func TestStatusCode(t *testing.T) {
	assert.Equal(t, http.StatusOK, StatusCode(Result{Healthy: true}))
	assert.Equal(t, http.StatusServiceUnavailable, StatusCode(Result{Healthy: false}))
}

func TestRenderBody(t *testing.T) {
	since := time.Date(2026, 10, 13, 8, 59, 30, 0, time.UTC)
	res := Result{
		Healthy:     false,
		Status:      UnhealthyStatus,
		EvaluatedAt: time.Date(2026, 10, 13, 9, 0, 0, 5, time.UTC),
		Checks: map[string]CheckResult{
			"database": {Status: StatusDown, Error: "database unreachable", Since: since, cause: errors.New("secret")},
			"cache":    {Status: StatusUp},
		},
	}

	got, err := RenderBody(res)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"status": "unhealthy",
		"timestamp": "2026-10-13T09:00:00Z",
		"checks": {
			"cache": {"status": "up"},
			"database": {"status": "down", "error": "database unreachable", "since": "2026-10-13T08:59:30Z"}
		}
	}`, string(got))
	assert.Regexp(t, `^\{"status":"unhealthy","timestamp":`, string(got))
}

func TestAnswerHealthyStatusPerChecker(t *testing.T) {
	opts := testOptions()
	opts.HealthyStatus = "healthy"
	c, _ := newTestChecker(opts)
	c.Register("database", passing)

	code, body := answer(t, c)

	assert.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `{
		"status": "healthy",
		"timestamp": "2026-10-13T09:00:00Z",
		"checks": {
			"database": {"status": "up"}
		}
	}`, body)
}
