/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package starter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnsureRunsThePrerequisitesInOrder(t *testing.T) {
	var order []string
	prerequisite := func(name string) Prerequisite {
		return Prerequisite{Name: name, Ensure: func(context.Context) error {
			order = append(order, name)
			return nil
		}}
	}
	require.NoError(t, New(nil).Ensure(context.Background(), "org", []Prerequisite{prerequisite("first"), prerequisite("second")}))
	require.Equal(t, []string{"first", "second"}, order)
}

func TestEnsureStopsAtTheFirstFailureAndNamesIt(t *testing.T) {
	cause := errors.New("no gateway")
	var ranAfter bool
	err := New(nil).Ensure(context.Background(), "org", []Prerequisite{
		{Name: "gateway", Ensure: func(context.Context) error { return cause }},
		{Name: "later", Ensure: func(context.Context) error { ranAfter = true; return nil }},
	})
	require.ErrorIs(t, err, cause)
	require.ErrorContains(t, err, `prerequisite "gateway"`)
	require.False(t, ranAfter)
}

func TestEnsureRunsOncePerKeyWhateverTheNumberOfCallers(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	prerequisites := []Prerequisite{{Name: "slow", Ensure: func(context.Context) error {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil
	}}}
	starter := New(nil)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	call := func(i int) {
		defer wg.Done()
		errs[i] = starter.Ensure(context.Background(), "org", prerequisites)
	}
	wg.Add(1)
	go call(0)
	<-started
	for i := 1; i < len(errs); i++ {
		wg.Add(1)
		go call(i)
	}
	close(release)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, runs.Load())
	require.NoError(t, starter.Ensure(context.Background(), "org", prerequisites), "a later caller shares the finished run")
	require.EqualValues(t, 1, runs.Load())
}

func TestEnsureSharesAFailureWithEveryCaller(t *testing.T) {
	var runs atomic.Int32
	cause := errors.New("refused")
	prerequisites := []Prerequisite{{Name: "gateway", Ensure: func(context.Context) error {
		runs.Add(1)
		return cause
	}}}
	starter := New(nil)
	for range 3 {
		require.ErrorIs(t, starter.Ensure(context.Background(), "org", prerequisites), cause)
	}
	require.EqualValues(t, 1, runs.Load())
}

func TestEnsurePreparesEachKeySeparately(t *testing.T) {
	var runs atomic.Int32
	prerequisites := []Prerequisite{{Name: "gateway", Ensure: func(context.Context) error { runs.Add(1); return nil }}}
	starter := New(nil)
	require.NoError(t, starter.Ensure(context.Background(), "one", prerequisites))
	require.NoError(t, starter.Ensure(context.Background(), "two", prerequisites))
	require.EqualValues(t, 2, runs.Load())
}

func TestEnsureStopsWaitingWhenTheContextEnds(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	prerequisites := []Prerequisite{{Name: "slow", Ensure: func(context.Context) error {
		close(started)
		<-release
		return nil
	}}}
	starter := New(nil)
	go func() { _ = starter.Ensure(context.Background(), "org", prerequisites) }()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, starter.Ensure(ctx, "org", prerequisites), context.DeadlineExceeded)
}

func TestEnsureRejectsInvalidInput(t *testing.T) {
	starter := New(nil)
	require.Error(t, starter.Ensure(context.Background(), "", nil), "a blank key")
	require.Error(t, starter.Ensure(context.Background(), "org", []Prerequisite{{Name: "empty"}}), "a prerequisite with no ensure function")
	require.NoError(t, starter.Ensure(context.Background(), "org", nil), "no prerequisites")
}
