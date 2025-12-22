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

package xdsclient

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// ReconnectManager handles reconnection logic with exponential backoff
type ReconnectManager struct {
	config         *Config
	currentDelay   time.Duration
	reconnectCount int
}

// NewReconnectManager creates a new ReconnectManager
func NewReconnectManager(config *Config) *ReconnectManager {
	return &ReconnectManager{
		config:       config,
		currentDelay: config.InitialReconnectDelay,
	}
}

// NextDelay calculates the next reconnection delay using exponential backoff
// Formula: min(initialDelay * 2^attempt, maxDelay)
func (rm *ReconnectManager) NextDelay() time.Duration {
	// Calculate exponential backoff
	delay := time.Duration(float64(rm.config.InitialReconnectDelay) * math.Pow(2, float64(rm.reconnectCount)))

	// Cap at maximum delay
	if delay > rm.config.MaxReconnectDelay {
		delay = rm.config.MaxReconnectDelay
	}

	rm.currentDelay = delay
	rm.reconnectCount++

	return delay
}

// Reset resets the reconnection state (called on successful connection)
func (rm *ReconnectManager) Reset() {
	rm.currentDelay = rm.config.InitialReconnectDelay
	rm.reconnectCount = 0
}

// WaitWithContext waits for the next reconnection attempt, respecting context cancellation
func (rm *ReconnectManager) WaitWithContext(ctx context.Context) error {
	delay := rm.NextDelay()

	slog.InfoContext(ctx, "Waiting before reconnection attempt",
		"delay", delay,
		"attempt", rm.reconnectCount)

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetReconnectCount returns the current reconnection attempt count
func (rm *ReconnectManager) GetReconnectCount() int {
	return rm.reconnectCount
}

// GetCurrentDelay returns the current delay value
func (rm *ReconnectManager) GetCurrentDelay() time.Duration {
	return rm.currentDelay
}
