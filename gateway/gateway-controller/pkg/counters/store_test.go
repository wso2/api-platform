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

package counters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

var (
	november = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	december = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
)

// stores runs a test against every Store.
func stores(t *testing.T) map[string]Store {
	t.Helper()
	return map[string]Store{
		"memory": NewMemory(0),
		"redis":  newRedisWithBackend(newMockRedis(), "test:", time.Hour),
	}
}

func TestStoreAppliesEachReportOnceAndReturnsTotals(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			a := counterproto.KeyFor("budget", "alice", november)
			b := counterproto.KeyFor("budget", "bob", november)
			adds := []counterproto.Add{{Key: a, Delta: 0.5}, {Key: b, Delta: 1.25}}

			totals, err := store.Sync(ctx, "engine-1", "r1", adds, []counterproto.Key{a, b})
			require.NoError(t, err)
			assert.Equal(t, []counterproto.Total{{Key: a, Total: 0.5}, {Key: b, Total: 1.25}}, totals)

			// A resent report is not applied again.
			totals, err = store.Sync(ctx, "engine-1", "r1", adds, []counterproto.Key{a})
			require.NoError(t, err)
			assert.Equal(t, 0.5, totals[0].Total)

			// Another engine's report with the same ID is its own.
			totals, err = store.Sync(ctx, "engine-2", "r1", []counterproto.Add{{Key: a, Delta: 0.25}}, []counterproto.Key{a})
			require.NoError(t, err)
			assert.Equal(t, 0.75, totals[0].Total)

			// Reading only.
			totals, err = store.Sync(ctx, "engine-3", "", nil, []counterproto.Key{a, counterproto.KeyFor("budget", "alice", december)})
			require.NoError(t, err)
			assert.Equal(t, 0.75, totals[0].Total)
			assert.Equal(t, 0.0, totals[1].Total, "a new window starts at zero")
		})
	}
}

func TestStoreKeepsKeysApart(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			// IDs that could run together once joined into one string.
			a := counterproto.Key{Namespace: "budget", ID: "x:1"}
			b := counterproto.Key{Namespace: "budget:x", ID: "1"}

			totals, err := store.Sync(ctx, "e", "r1", []counterproto.Add{{Key: a, Delta: 1}}, []counterproto.Key{a, b})

			require.NoError(t, err)
			assert.Equal(t, 1.0, totals[0].Total)
			assert.Equal(t, 0.0, totals[1].Total)
		})
	}
}

func TestRedisKeysExpireAfterRetention(t *testing.T) {
	mock := newMockRedis()
	store := newRedisWithBackend(mock, "test:", time.Hour)
	key := counterproto.KeyFor("budget", "alice", november)

	_, err := store.Sync(context.Background(), "e", "r1", []counterproto.Add{{Key: key, Delta: 1}}, nil)
	require.NoError(t, err)

	assert.Equal(t, time.Hour, mock.ttl(store.key(key)), "the script sets the retention as the counter's expiry")
	mock.advance(time.Hour + time.Second)
	totals, err := store.Sync(context.Background(), "e", "", nil, []counterproto.Key{key})
	require.NoError(t, err)
	assert.Equal(t, 0.0, totals[0].Total)
}

func TestMemoryForgetsIdleCounters(t *testing.T) {
	store := NewMemory(time.Hour)
	now := november
	store.now = func() time.Time { return now }
	key := counterproto.KeyFor("budget", "alice", november)
	_, _ = store.Sync(context.Background(), "e", "r1", []counterproto.Add{{Key: key, Delta: 1}}, nil)

	now = now.Add(2 * time.Hour)
	totals, _ := store.Sync(context.Background(), "e", "", nil, []counterproto.Key{key})

	assert.Equal(t, 0.0, totals[0].Total)
}

// Totals and Delete find a budget's counters by their ID prefix, in every window, and
// leave other budgets and other namespaces alone.
func TestStoreTotalsAndDeleteByPrefix(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			gone1 := counterproto.KeyFor("budget", "a/1", november)
			gone2 := counterproto.KeyFor("budget", "a/2:x", december)
			kept := counterproto.KeyFor("budget", "b/1", november)
			otherNamespace := counterproto.KeyFor("budget-unpriced", "a/1", november)
			adds := []counterproto.Add{{Key: gone1, Delta: 1}, {Key: gone2, Delta: 2}, {Key: kept, Delta: 3}, {Key: otherNamespace, Delta: 4}}
			_, err := store.Sync(ctx, "engine-1", "r1", adds, nil)
			require.NoError(t, err)

			totals, err := store.Totals(ctx, "budget", "a/")
			require.NoError(t, err)
			assert.ElementsMatch(t, []counterproto.Total{{Key: gone1, Total: 1}, {Key: gone2, Total: 2}}, totals)

			require.NoError(t, store.Delete(ctx, "budget", "a/"))
			totals, err = store.Totals(ctx, "budget", "a/")
			require.NoError(t, err)
			assert.Empty(t, totals)

			left, err := store.Sync(ctx, "engine-1", "", nil, []counterproto.Key{gone1, kept, otherNamespace})
			require.NoError(t, err)
			assert.Equal(t, []float64{0, 3, 4}, []float64{left[0].Total, left[1].Total, left[2].Total})
		})
	}
}
