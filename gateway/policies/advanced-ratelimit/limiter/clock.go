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
	"sync"
	"time"
)

// Clock provides an abstraction for time operations (useful for testing)
type Clock interface {
	Now() time.Time
}

// SystemClock uses the system time
type SystemClock struct{}

// Now returns the current system time
func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// FixedClock returns a fixed time (for testing)
// Thread-safe for concurrent reads and writes
type FixedClock struct {
	mu   sync.RWMutex
	time time.Time
}

// NewFixedClock creates a new FixedClock with the given time
func NewFixedClock(t time.Time) *FixedClock {
	return &FixedClock{time: t}
}

// Now returns the fixed time (thread-safe)
func (c *FixedClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.time
}

// Set updates the fixed time (thread-safe)
func (c *FixedClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time = t
}
