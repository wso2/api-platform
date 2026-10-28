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

package steps

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithTimeoutReturnsTheFunctionsResult(t *testing.T) {
	require.NoError(t, withTimeout(time.Second, func() error { return nil }))

	boom := errors.New("boom")
	require.ErrorIs(t, withTimeout(time.Second, func() error { return boom }), boom)
}

func TestWithTimeoutFailsWhenTheFunctionNeverReturns(t *testing.T) {
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) }) // lets the leaked goroutine exit once the test is done

	err := withTimeout(20*time.Millisecond, func() error {
		<-blocked
		return nil
	})
	require.ErrorContains(t, err, "timed out after 20ms")
}

func TestWithTimeoutIsConcurrencySafe(t *testing.T) {
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, withTimeout(time.Second, func() error { return nil }))
		}()
	}
	wg.Wait()
}
