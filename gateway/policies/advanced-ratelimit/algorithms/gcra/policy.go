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
 
package gcra

import "time"

// Policy defines a rate limit policy (e.g., 10 req/sec, 1000 req/day)
type Policy struct {
	// Limit is the maximum number of requests allowed in the duration
	Limit int64

	// Duration is the time window for the limit
	Duration time.Duration

	// Burst is the maximum burst size (tokens that can accumulate)
	// For GCRA: burst capacity in number of requests
	Burst int64
}

// NewPolicy creates a new rate limit policy
// limit: maximum number of requests allowed in the duration
// duration: time window for the limit
// burst: maximum burst size (number of requests that can accumulate)
func NewPolicy(limit int64, duration time.Duration, burst int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: duration,
		Burst:    burst,
	}
}

// EmissionInterval calculates the time between each request
// EI = Duration / Limit
func (p *Policy) EmissionInterval() time.Duration {
	return time.Duration(int64(p.Duration) / p.Limit)
}

// BurstAllowance calculates the maximum TAT-now difference allowed
// Burst Allowance = (Burst - 1) × Emission Interval
// For a burst of N, we allow N-1 intervals of tolerance
func (p *Policy) BurstAllowance() time.Duration {
	if p.Burst <= 1 {
		return 0
	}
	return time.Duration(int64(p.EmissionInterval()) * (p.Burst - 1))
}

// WithBurst creates a new policy with custom burst size
func (p *Policy) WithBurst(burst int64) *Policy {
	return &Policy{
		Limit:    p.Limit,
		Duration: p.Duration,
		Burst:    burst,
	}
}

// PerSecond creates a rate limit policy for requests per second
func PerSecond(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Second,
		Burst:    limit,
	}
}

// PerMinute creates a rate limit policy for requests per minute
func PerMinute(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Minute,
		Burst:    limit,
	}
}

// PerHour creates a rate limit policy for requests per hour
func PerHour(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Hour,
		Burst:    limit,
	}
}

// PerDay creates a rate limit policy for requests per day
func PerDay(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: 24 * time.Hour,
		Burst:    limit,
	}
}
