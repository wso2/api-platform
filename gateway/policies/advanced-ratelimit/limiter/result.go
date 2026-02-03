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
 
package limiter

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Result contains the outcome of a rate limit check
type Result struct {
	// Allowed indicates whether the request is allowed
	Allowed bool

	// Limit is the maximum number of requests allowed in the time window
	Limit int64

	// Remaining is the number of requests remaining in the current window
	Remaining int64

	// Reset is the time when the rate limit will reset
	Reset time.Time

	// RetryAfter is the duration to wait before retrying (if rate limited)
	RetryAfter time.Duration

	// FullQuotaAt is the time when the full quota (all burst capacity) will be available
	// This is when TAT <= now, meaning all consumed tokens have regenerated
	FullQuotaAt time.Time

	// Duration is the rate limit window duration (common across all algorithms)
	Duration time.Duration

	// Policy that was evaluated (algorithm-specific)
	Policy interface{}
}

// SetHeaders sets all standard rate limit headers on an HTTP response
func (r *Result) SetHeaders(w http.ResponseWriter) {
	r.SetXRateLimitHeaders(w)
	r.SetIETFHeaders(w)

	if !r.Allowed {
		r.SetRetryAfterHeader(w)
	}
}

// SetXRateLimitHeaders sets X-RateLimit-* headers (de facto standard)
// Used by GitHub, Stripe, Twitter, and many other APIs
func (r *Result) SetXRateLimitHeaders(w http.ResponseWriter) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(r.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(r.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(r.Reset.Unix(), 10))
	w.Header().Set("X-RateLimit-Full-Quota-Reset", strconv.FormatInt(r.FullQuotaAt.Unix(), 10))
}

// SetIETFHeaders sets IETF draft standard headers
// Reference: draft-ietf-httpapi-ratelimit-headers-10
func (r *Result) SetIETFHeaders(w http.ResponseWriter) {
	// RateLimit-Limit: <limit>
	w.Header().Set("RateLimit-Limit", strconv.FormatInt(r.Limit, 10))

	// RateLimit-Remaining: <remaining>
	w.Header().Set("RateLimit-Remaining", strconv.FormatInt(r.Remaining, 10))

	// RateLimit-Reset: <seconds until reset>
	resetSeconds := int64(time.Until(r.Reset).Seconds())
	if resetSeconds < 0 {
		resetSeconds = 0
	}
	w.Header().Set("RateLimit-Reset", strconv.FormatInt(resetSeconds, 10))

	// RateLimit-Full-Quota-Reset: <seconds until full quota available>
	fullQuotaSeconds := int64(time.Until(r.FullQuotaAt).Seconds())
	if fullQuotaSeconds < 0 {
		fullQuotaSeconds = 0
	}
	w.Header().Set("RateLimit-Full-Quota-Reset", strconv.FormatInt(fullQuotaSeconds, 10))

	// RateLimit-Policy: <limit>;w=<window_in_seconds>
	policyValue := fmt.Sprintf("%d;w=%d", r.Limit, int64(r.Duration.Seconds()))
	w.Header().Set("RateLimit-Policy", policyValue)
}

// SetRetryAfterHeader sets Retry-After header when rate limited (RFC 7231)
// This should only be set on 429 Too Many Requests responses
func (r *Result) SetRetryAfterHeader(w http.ResponseWriter) {
	if r.RetryAfter > 0 {
		seconds := int64(r.RetryAfter.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	}
}
