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
 
package fixedwindow

import "time"

// Policy defines a fixed window rate limit policy
// Fixed window divides time into fixed-duration intervals and counts requests per interval
type Policy struct {
	// Limit is the maximum number of requests allowed per window
	Limit int64

	// Duration is the window duration (e.g., 1 second, 1 minute, 1 hour)
	Duration time.Duration
}

// NewPolicy creates a new fixed window rate limit policy
// limit: maximum number of requests allowed in the window
// duration: time window duration
func NewPolicy(limit int64, duration time.Duration) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: duration,
	}
}

// WindowStart returns the start time of the current window for the given timestamp
// Windows are aligned to Unix epoch (truncated to duration boundary)
func (p *Policy) WindowStart(now time.Time) time.Time {
	return now.Truncate(p.Duration)
}

// WindowEnd returns the end time of the current window for the given timestamp
func (p *Policy) WindowEnd(now time.Time) time.Time {
	return p.WindowStart(now).Add(p.Duration)
}

// PerSecond creates a rate limit policy for requests per second
func PerSecond(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Second,
	}
}

// PerMinute creates a rate limit policy for requests per minute
func PerMinute(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Minute,
	}
}

// PerHour creates a rate limit policy for requests per hour
func PerHour(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: time.Hour,
	}
}

// PerDay creates a rate limit policy for requests per day
func PerDay(limit int64) *Policy {
	return &Policy{
		Limit:    limit,
		Duration: 24 * time.Hour,
	}
}
