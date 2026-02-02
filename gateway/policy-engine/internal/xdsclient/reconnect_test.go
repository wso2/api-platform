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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewReconnectManager tests the constructor
func TestNewReconnectManager(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)

	assert.NotNil(t, rm)
	assert.Equal(t, config, rm.config)
	assert.Equal(t, config.InitialReconnectDelay, rm.currentDelay)
	assert.Equal(t, 0, rm.reconnectCount)
}

// TestNextDelay_FirstCall tests that first call returns initial delay
func TestNextDelay_FirstCall(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)
	delay := rm.NextDelay()

	// First call: 1s * 2^0 = 1s
	assert.Equal(t, 1*time.Second, delay)
	assert.Equal(t, 1, rm.reconnectCount)
	assert.Equal(t, 1*time.Second, rm.currentDelay)
}

// TestNextDelay_ExponentialIncrease tests exponential backoff
func TestNextDelay_ExponentialIncrease(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)

	// First call: 1s * 2^0 = 1s
	delay1 := rm.NextDelay()
	assert.Equal(t, 1*time.Second, delay1)
	assert.Equal(t, 1, rm.reconnectCount)

	// Second call: 1s * 2^1 = 2s
	delay2 := rm.NextDelay()
	assert.Equal(t, 2*time.Second, delay2)
	assert.Equal(t, 2, rm.reconnectCount)

	// Third call: 1s * 2^2 = 4s
	delay3 := rm.NextDelay()
	assert.Equal(t, 4*time.Second, delay3)
	assert.Equal(t, 3, rm.reconnectCount)

	// Fourth call: 1s * 2^3 = 8s
	delay4 := rm.NextDelay()
	assert.Equal(t, 8*time.Second, delay4)
	assert.Equal(t, 4, rm.reconnectCount)
}

// TestNextDelay_MaxCapReached tests that delay is capped at max
func TestNextDelay_MaxCapReached(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     5 * time.Second, // Low max for testing
	}

	rm := NewReconnectManager(config)

	// Call NextDelay multiple times to exceed max
	delays := []time.Duration{}
	for i := 0; i < 10; i++ {
		delays = append(delays, rm.NextDelay())
	}

	// Expected: 1s, 2s, 4s, 5s (capped), 5s (capped), ...
	assert.Equal(t, 1*time.Second, delays[0])
	assert.Equal(t, 2*time.Second, delays[1])
	assert.Equal(t, 4*time.Second, delays[2])
	assert.Equal(t, 5*time.Second, delays[3]) // Capped at max
	assert.Equal(t, 5*time.Second, delays[4]) // Still capped
	assert.Equal(t, 5*time.Second, delays[9]) // Still capped
}

// TestReset tests that Reset clears state
func TestReset(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)

	// Advance state
	rm.NextDelay()
	rm.NextDelay()
	rm.NextDelay()

	assert.Equal(t, 3, rm.reconnectCount)
	assert.Equal(t, 4*time.Second, rm.currentDelay)

	// Reset
	rm.Reset()

	assert.Equal(t, 0, rm.reconnectCount)
	assert.Equal(t, config.InitialReconnectDelay, rm.currentDelay)
}

// TestWaitWithContext_Success tests successful wait
func TestWaitWithContext_Success(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 10 * time.Millisecond, // Short delay for test
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)
	ctx := context.Background()

	start := time.Now()
	err := rm.WaitWithContext(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	// Should wait approximately 10ms (first delay)
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)
	assert.LessOrEqual(t, elapsed, 100*time.Millisecond) // Some tolerance
}

// TestWaitWithContext_ContextCancellation tests context cancellation
func TestWaitWithContext_ContextCancellation(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 5 * time.Second, // Long delay
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := rm.WaitWithContext(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Should return quickly after cancellation
	assert.Less(t, elapsed, 200*time.Millisecond)
}

// TestWaitWithContext_AlreadyCancelled tests with already cancelled context
func TestWaitWithContext_AlreadyCancelled(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	start := time.Now()
	err := rm.WaitWithContext(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Should return immediately
	assert.Less(t, elapsed, 50*time.Millisecond)
}

// TestGetReconnectCount tests getter
func TestGetReconnectCount(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)

	assert.Equal(t, 0, rm.GetReconnectCount())

	rm.NextDelay()
	assert.Equal(t, 1, rm.GetReconnectCount())

	rm.NextDelay()
	assert.Equal(t, 2, rm.GetReconnectCount())

	rm.Reset()
	assert.Equal(t, 0, rm.GetReconnectCount())
}

// TestGetCurrentDelay tests getter
func TestGetCurrentDelay(t *testing.T) {
	config := &Config{
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	rm := NewReconnectManager(config)

	assert.Equal(t, 1*time.Second, rm.GetCurrentDelay())

	rm.NextDelay()
	assert.Equal(t, 1*time.Second, rm.GetCurrentDelay())

	rm.NextDelay()
	assert.Equal(t, 2*time.Second, rm.GetCurrentDelay())

	rm.NextDelay()
	assert.Equal(t, 4*time.Second, rm.GetCurrentDelay())
}
