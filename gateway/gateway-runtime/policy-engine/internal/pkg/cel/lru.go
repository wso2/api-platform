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

package cel

import (
	"container/list"

	"github.com/google/cel-go/cel"
)

// programLRUCache is a fixed-capacity LRU cache for compiled cel.Programs.
// All methods are O(1). The caller is responsible for external synchronisation.
type programLRUCache struct {
	capacity int
	list     *list.List
	items    map[string]*list.Element
}

type lruEntry struct {
	key     string
	program cel.Program
}

func newProgramLRUCache(capacity int) *programLRUCache {
	return &programLRUCache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

// get returns the cached program for key and promotes it to most-recently-used.
// Returns false when the key is absent.
func (c *programLRUCache) get(key string) (cel.Program, bool) {
	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.list.MoveToFront(elem)
	return elem.Value.(*lruEntry).program, true
}

// put inserts or updates the program for key.
// When the cache is at capacity the least-recently-used entry is evicted first.
func (c *programLRUCache) put(key string, program cel.Program) {
	if elem, ok := c.items[key]; ok {
		elem.Value.(*lruEntry).program = program
		c.list.MoveToFront(elem)
		return
	}
	if c.list.Len() >= c.capacity {
		back := c.list.Back()
		if back != nil {
			c.list.Remove(back)
			delete(c.items, back.Value.(*lruEntry).key)
		}
	}
	elem := c.list.PushFront(&lruEntry{key: key, program: program})
	c.items[key] = elem
}

// len returns the current number of cached entries.
func (c *programLRUCache) len() int {
	return c.list.Len()
}
