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

package finalizer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnedByMatchesOnlyTheTrackedSuffixSegments(t *testing.T) {
	suffixes := []string{"abcde234"}
	for name, names := range map[string][]string{
		"inside a name":              {"syn-gw-abcde234-3"},
		"ends the name":              {"development-syn-gw-abcde234"},
		"underscores read as dashes": {"Syn_Env_ABCDE234_1"},
		"one of several names":       {"development-syn-gw-ab", "syn-gw-abcde234-12"},
	} {
		require.True(t, OwnedBy(names, suffixes), name)
	}
	for name, names := range map[string][]string{
		"another runner":       {"syn-gw-zzzzz222-3"},
		"inside a longer word": {"syn-gwabcde234x-3"},
		"a default resource":   {"development-default", "development"},
		"no names":             nil,
		"blank name":           {" "},
	} {
		require.False(t, OwnedBy(names, suffixes), name)
	}
	require.False(t, OwnedBy([]string{"syn-gw-abcde234-3"}, []string{"", " "}), "a blank suffix matches nothing")
}

// leftovers is a fake system under test: named collections that record deletions in order.
type leftovers struct {
	mu      sync.Mutex
	items   map[string][]Leftover
	fail    map[string]error
	deleted []string
}

func (l *leftovers) collection(name string) Collection {
	return Collection{
		Name: name,
		List: func(context.Context) ([]Leftover, error) {
			l.mu.Lock()
			defer l.mu.Unlock()
			if err := l.fail["list "+name]; err != nil {
				return nil, err
			}
			return l.items[name], nil
		},
		Delete: func(_ context.Context, id string) error {
			l.mu.Lock()
			defer l.mu.Unlock()
			if err := l.fail[name+"/"+id]; err != nil {
				return err
			}
			l.deleted = append(l.deleted, name+"/"+id)
			return nil
		},
	}
}

func newLeftovers() *leftovers {
	return &leftovers{fail: map[string]error{}, items: map[string][]Leftover{
		"/gateways": {
			{ID: "gw-1", Names: []string{"gw-abcde234-1"}},
			{ID: "default", Names: []string{"default"}},
			{ID: "gw-2", Names: []string{"gw-zzzzz222-1"}},
			{ID: "gw-cut", Names: []string{"gw-cut", "gw-abcde234-12"}},
		},
		"/projects": {{ID: "proj-1", Names: []string{"proj-abcde234-2"}}},
	}}
}

func TestSweepDeletesOnlyTheTrackedRunnersEntriesInOrder(t *testing.T) {
	system := newLeftovers()
	tracker := NewTracker(nil)
	tracker.Track("abcde234")

	removed, err := tracker.Sweep(context.Background(), []Collection{system.collection("/gateways"), system.collection("/projects")})
	require.NoError(t, err)
	require.Equal(t, []string{"/gateways/gw-1", "/gateways/gw-cut", "/projects/proj-1"}, removed)
	require.Equal(t, removed, system.deleted)
}

func TestSweepDoesNothingBeforeARunnerIsTracked(t *testing.T) {
	system := newLeftovers()
	tracker := NewTracker(nil)
	tracker.Track("")
	require.False(t, tracker.Tracked())

	removed, err := tracker.Sweep(context.Background(), []Collection{system.collection("/gateways")})
	require.NoError(t, err)
	require.Empty(t, removed)
	require.Empty(t, system.deleted)
}

func TestSweepCarriesOnPastFailures(t *testing.T) {
	system := newLeftovers()
	system.fail["list /gateways"] = errors.New("listing refused")
	system.fail["/projects/proj-1"] = errors.New("delete refused")
	system.items["/pipelines"] = []Leftover{{ID: "pipe-1", Names: []string{"pipe-abcde234-1"}}}
	tracker := NewTracker(nil)
	tracker.Track("abcde234")

	removed, err := tracker.Sweep(context.Background(), []Collection{
		system.collection("/gateways"), system.collection("/projects"), system.collection("/pipelines"),
	})
	require.ErrorContains(t, err, "listing /gateways: listing refused")
	require.ErrorContains(t, err, "delete refused")
	require.Equal(t, []string{"/pipelines/pipe-1"}, removed)
}

func TestSweepRejectsAnIncompleteCollectionAndAnEntryWithoutID(t *testing.T) {
	system := newLeftovers()
	system.items["/gateways"] = []Leftover{{Names: []string{"gw-abcde234-1"}}}
	tracker := NewTracker(nil)
	tracker.Track("abcde234")

	removed, err := tracker.Sweep(context.Background(), []Collection{{Name: "/broken"}, system.collection("/gateways")})
	require.ErrorContains(t, err, `collection "/broken" needs both a list and a delete function`)
	require.ErrorContains(t, err, "/gateways holds a leftover with no id")
	require.Empty(t, removed)
}

func TestTrackIsSafeForConcurrentRunners(t *testing.T) {
	tracker := NewTracker(nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Track("abcde234")
			_ = tracker.Tracked()
		}()
	}
	wg.Wait()
	require.True(t, tracker.Tracked())
}
