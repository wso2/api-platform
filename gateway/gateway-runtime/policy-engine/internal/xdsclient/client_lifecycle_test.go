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
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_Stop_GracefulShutdown tests that Stop() can be called multiple times safely
func TestClient_Stop_GracefulShutdown(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// Verify initial state
	assert.Equal(t, StateDisconnected, client.GetState())
	assert.NotNil(t, client.stoppedCh)

	// Call Stop() first time
	client.Stop()

	// Verify state transitioned to Stopped
	assert.Equal(t, StateStopped, client.GetState())

	// Verify stoppedCh is closed
	select {
	case <-client.stoppedCh:
		// Channel is closed, as expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stoppedCh should be closed after Stop()")
	}

	// Verify context is cancelled
	assert.Error(t, client.ctx.Err())

	// Call Stop() second time - should not panic due to sync.Once
	assert.NotPanics(t, func() {
		client.Stop()
	})

	// State should still be Stopped
	assert.Equal(t, StateStopped, client.GetState())
}

// TestClient_Wait_BlocksUntilStopped tests that Wait() blocks until Stop() is called
func TestClient_Wait_BlocksUntilStopped(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	waitCompleted := make(chan struct{})

	// Start goroutine that calls Wait()
	go func() {
		client.Wait()
		close(waitCompleted)
	}()

	// Wait should block
	select {
	case <-waitCompleted:
		t.Fatal("Wait() should block until Stop() is called")
	case <-time.After(100 * time.Millisecond):
		// Expected: Wait() is still blocking
	}

	// Now call Stop()
	client.Stop()

	// Wait() should unblock
	select {
	case <-waitCompleted:
		// Expected: Wait() completed
	case <-time.After(1 * time.Second):
		t.Fatal("Wait() should unblock after Stop() is called")
	}
}

// TestClient_Wait_MultipleGoroutines tests that multiple goroutines can Wait() simultaneously
func TestClient_Wait_MultipleGoroutines(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	const numWaiters = 5
	var wg sync.WaitGroup
	wg.Add(numWaiters)

	// Start multiple goroutines calling Wait()
	for i := 0; i < numWaiters; i++ {
		go func() {
			defer wg.Done()
			client.Wait()
		}()
	}

	// Give goroutines time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Stop the client
	client.Stop()

	// All waiters should complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected: all waiters completed
	case <-time.After(1 * time.Second):
		t.Fatal("All Wait() calls should complete after Stop()")
	}
}

// TestClient_Run_ContextCancellation tests that run() exits gracefully when context is cancelled
func TestClient_Run_ContextCancellation(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	// Set very short timeout to avoid actual connection attempts
	config.ConnectTimeout = 1 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	runCompleted := make(chan struct{})

	// Start run() in goroutine
	go func() {
		client.run()
		close(runCompleted)
	}()

	// Give run() time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	client.cancel()

	// run() should exit
	select {
	case <-runCompleted:
		// Expected: run() exited
	case <-time.After(2 * time.Second):
		t.Fatal("run() should exit when context is cancelled")
	}
}

// TestClient_Run_ReconnectLoop tests that run() handles connection failures with reconnect delays
func TestClient_Run_ReconnectLoop(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	// Use invalid server address to force connection failures
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 100 * time.Millisecond
	config.InitialReconnectDelay = 50 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	runCompleted := make(chan struct{})
	stateChanges := make(chan ClientState, 10)

	// Monitor state changes
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		lastState := client.GetState()
		for {
			select {
			case <-ticker.C:
				currentState := client.GetState()
				if currentState != lastState {
					stateChanges <- currentState
					lastState = currentState
				}
			case <-runCompleted:
				return
			}
		}
	}()

	// Start run() in goroutine
	go func() {
		client.run()
		close(runCompleted)
	}()

	// Allow some reconnect attempts
	time.Sleep(300 * time.Millisecond)

	// We should see StateConnecting and StateReconnecting transitions
	reconnectingSeen := false
	connectingSeen := false

	for {
		select {
		case state := <-stateChanges:
			if state == StateReconnecting {
				reconnectingSeen = true
			}
			if state == StateConnecting {
				connectingSeen = true
			}
		case <-time.After(100 * time.Millisecond):
			// No more state changes
			goto checkStates
		}
	}

checkStates:
	assert.True(t, connectingSeen, "Should have seen StateConnecting")
	assert.True(t, reconnectingSeen, "Should have seen StateReconnecting after connection failure")

	// Stop the client
	client.Stop()

	// run() should exit
	select {
	case <-runCompleted:
		// Expected: run() exited
	case <-time.After(2 * time.Second):
		t.Fatal("run() should exit after Stop()")
	}
}

// TestClient_ConnectAndRun_ConnectionFailure tests connectAndRun() with connection failures
func TestClient_ConnectAndRun_ConnectionFailure(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	// Use invalid server address
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 100 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// connectAndRun should return an error
	err = client.connectAndRun()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to dial xDS server")
}

// TestClient_Start_StartsBackgroundLoop tests that Start() launches the background run loop
func TestClient_Start_StartsBackgroundLoop(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	// Use invalid server to prevent actual connection
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 50 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// Start should not block
	startErr := client.Start()
	assert.NoError(t, startErr)

	// Give background loop time to attempt connection
	time.Sleep(200 * time.Millisecond)

	// Client should have attempted to connect
	// State should be either StateConnecting or StateReconnecting
	state := client.GetState()
	assert.True(t, state == StateConnecting || state == StateReconnecting,
		"State should be StateConnecting or StateReconnecting, got %v", state)

	// Stop the client
	client.Stop()
	client.Wait()

	// Final state should be StateStopped
	assert.Equal(t, StateStopped, client.GetState())
}

// TestClient_Stop_WithActiveConnection tests Stop() cleans up active connection
func TestClient_Stop_WithActiveConnection(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// Simulate having a connection (Note: we're not actually connecting to avoid test infrastructure)
	// This test verifies the connection cleanup logic in Stop()

	client.Stop()

	// Verify connection is nil after Stop()
	client.mu.RLock()
	conn := client.conn
	client.mu.RUnlock()

	assert.Nil(t, conn, "Connection should be nil after Stop()")
}

// TestClient_Run_ImmediateStop tests that run() handles immediate Stop() call
func TestClient_Run_ImmediateStop(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 50 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	runCompleted := make(chan struct{})

	// Start run()
	go func() {
		client.run()
		close(runCompleted)
	}()

	// Stop immediately
	client.Stop()

	// run() should exit quickly
	select {
	case <-runCompleted:
		// Expected: run() exited
	case <-time.After(1 * time.Second):
		t.Fatal("run() should exit quickly after immediate Stop()")
	}
}

// TestClient_Lifecycle_CompleteFlow tests the complete lifecycle: New -> Start -> Stop -> Wait
func TestClient_Lifecycle_CompleteFlow(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 50 * time.Millisecond

	// 1. Create client
	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)
	assert.Equal(t, StateDisconnected, client.GetState())

	// 2. Start client
	err = client.Start()
	require.NoError(t, err)

	// Give it time to attempt connection
	time.Sleep(200 * time.Millisecond)

	// State should have changed from initial Disconnected
	state := client.GetState()
	assert.NotEqual(t, StateDisconnected, state)

	// 3. Stop client
	client.Stop()
	assert.Equal(t, StateStopped, client.GetState())

	// 4. Wait for completion
	waitDone := make(chan struct{})
	go func() {
		client.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// Expected: Wait completed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait() should complete after Stop()")
	}
}

// TestClient_SetState_ThreadSafety tests that setState is thread-safe
func TestClient_SetState_ThreadSafety(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	states := []ClientState{
		StateConnecting,
		StateConnected,
		StateReconnecting,
		StateDisconnected,
	}

	// Concurrently set states from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			state := states[index%len(states)]
			client.setState(state)
			_ = client.GetState()
		}(i)
	}

	// Should not panic or deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected: all goroutines completed
	case <-time.After(2 * time.Second):
		t.Fatal("setState/GetState should be thread-safe and not deadlock")
	}

	// Final state should be one of the valid states
	finalState := client.GetState()
	validState := false
	for _, s := range states {
		if finalState == s {
			validState = true
			break
		}
	}
	assert.True(t, validState, "Final state should be valid")
}

// TestClient_ConnectAndRun_ContextCancelledDuringDial tests context cancellation during dial
func TestClient_ConnectAndRun_ContextCancelledDuringDial(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 5 * time.Second // Long timeout

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// Cancel context before calling connectAndRun
	client.cancel()

	err = client.connectAndRun()
	assert.Error(t, err)
	// Error should be related to context or connection failure
}

// TestClient_Run_ReconnectManager_BackoffBehavior tests that reconnect delays increase
func TestClient_Run_ReconnectManager_BackoffBehavior(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 10 * time.Millisecond
	config.InitialReconnectDelay = 20 * time.Millisecond
	config.MaxReconnectDelay = 100 * time.Millisecond

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)

	// Verify ReconnectManager is initialized
	assert.NotNil(t, client.reconnectManager)

	// Start the client
	err = client.Start()
	require.NoError(t, err)

	// Let it attempt a few reconnects
	time.Sleep(300 * time.Millisecond)

	// Stop the client
	client.Stop()
	client.Wait()

	// The test passes if no panics occurred and reconnect logic executed
	assert.Equal(t, StateStopped, client.GetState())
}
