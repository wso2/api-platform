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

package redact

import (
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretTracker_Track(t *testing.T) {
	st := NewSecretTracker()

	st.Track("secret-value-1")
	st.Track("secret-value-2")

	vals := st.Values()
	sort.Strings(vals)
	assert.Equal(t, []string{"secret-value-1", "secret-value-2"}, vals)
}

func TestSecretTracker_TrackEmpty(t *testing.T) {
	st := NewSecretTracker()

	st.Track("")
	st.Track("real-secret")

	vals := st.Values()
	assert.Equal(t, []string{"real-secret"}, vals)
}

func TestSecretTracker_TrackDuplicates(t *testing.T) {
	st := NewSecretTracker()

	st.Track("same-secret")
	st.Track("same-secret")
	st.Track("same-secret")

	vals := st.Values()
	assert.Equal(t, 1, len(vals))
	assert.Equal(t, "same-secret", vals[0])
}

func TestSecretTracker_ValuesReturnsEmpty(t *testing.T) {
	st := NewSecretTracker()

	vals := st.Values()
	assert.NotNil(t, vals)
	assert.Empty(t, vals)
}

func TestSecretTracker_ConcurrentAccess(t *testing.T) {
	st := NewSecretTracker()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			st.Track(v)
		}("secret-" + string(rune('a'+i%26)))
	}

	wg.Wait()

	vals := st.Values()
	assert.NotEmpty(t, vals)
	assert.LessOrEqual(t, len(vals), 26)
}
