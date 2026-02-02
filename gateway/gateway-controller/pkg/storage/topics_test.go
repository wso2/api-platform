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

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTopicManager(t *testing.T) {
	tm := NewTopicManager()
	assert.NotNil(t, tm)
	assert.Equal(t, 0, tm.Count())
}

func TestTopicManager_Add(t *testing.T) {
	tm := NewTopicManager()

	added := tm.Add("config-1", "topic-1")
	assert.True(t, added)
	assert.Equal(t, 1, tm.Count())
}

func TestTopicManager_Add_Duplicate(t *testing.T) {
	tm := NewTopicManager()

	added1 := tm.Add("config-1", "topic-1")
	assert.True(t, added1)

	// Adding same topic should return false
	added2 := tm.Add("config-1", "topic-1")
	assert.False(t, added2)
	assert.Equal(t, 1, tm.Count())
}

func TestTopicManager_Add_SameTopicDifferentConfig(t *testing.T) {
	tm := NewTopicManager()

	added1 := tm.Add("config-1", "topic-1")
	assert.True(t, added1)

	// Same topic for different config should be allowed
	added2 := tm.Add("config-2", "topic-1")
	assert.True(t, added2)

	// Count should still be 1 (unique topics)
	assert.Equal(t, 1, tm.Count())
}

func TestTopicManager_Remove(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")

	removed := tm.Remove("config-1", "topic-1")
	assert.True(t, removed)
	assert.Equal(t, 0, tm.Count())
}

func TestTopicManager_Remove_NotExists(t *testing.T) {
	tm := NewTopicManager()

	// Remove from nonexistent config
	removed := tm.Remove("config-1", "topic-1")
	assert.False(t, removed)
}

func TestTopicManager_Remove_TopicNotExists(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")

	// Remove nonexistent topic from existing config
	removed := tm.Remove("config-1", "topic-2")
	assert.False(t, removed)
}

func TestTopicManager_RemoveAllForConfig(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	tm.Add("config-1", "topic-3")
	tm.Add("config-2", "topic-4")

	tm.RemoveAllForConfig("config-1")

	assert.Equal(t, 0, tm.CountForConfig("config-1"))
	assert.Equal(t, 1, tm.CountForConfig("config-2"))
}

func TestTopicManager_RemoveAllForConfig_Empty(t *testing.T) {
	tm := NewTopicManager()

	// Should not panic on nonexistent config
	tm.RemoveAllForConfig("nonexistent")
	assert.Equal(t, 0, tm.Count())
}

func TestTopicManager_Contains(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")

	assert.True(t, tm.Contains("config-1", "topic-1"))
	assert.False(t, tm.Contains("config-1", "topic-2"))
	assert.False(t, tm.Contains("config-2", "topic-1"))
}

func TestTopicManager_Contains_EmptyConfig(t *testing.T) {
	tm := NewTopicManager()

	assert.False(t, tm.Contains("nonexistent", "topic-1"))
}

func TestTopicManager_GetAllByConfig(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	tm.Add("config-1", "topic-3")

	topics := tm.GetAllByConfig("config-1")
	assert.Len(t, topics, 3)
	assert.Contains(t, topics, "topic-1")
	assert.Contains(t, topics, "topic-2")
	assert.Contains(t, topics, "topic-3")
}

func TestTopicManager_GetAllByConfig_Empty(t *testing.T) {
	tm := NewTopicManager()

	topics := tm.GetAllByConfig("nonexistent")
	assert.NotNil(t, topics)
	assert.Len(t, topics, 0)
}

func TestTopicManager_IsTopicExist(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")

	assert.True(t, tm.IsTopicExist("config-1", "topic-1"))
	assert.False(t, tm.IsTopicExist("config-1", "topic-2"))
	assert.False(t, tm.IsTopicExist("config-2", "topic-1"))
}

func TestTopicManager_GetAll(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	tm.Add("config-2", "topic-1") // Same topic, different config
	tm.Add("config-2", "topic-3")

	allTopics := tm.GetAll()
	assert.Len(t, allTopics, 3) // topic-1, topic-2, topic-3
	assert.True(t, allTopics["topic-1"])
	assert.True(t, allTopics["topic-2"])
	assert.True(t, allTopics["topic-3"])
}

func TestTopicManager_GetAllForConfig(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	tm.Add("config-2", "topic-3")

	allForConfig := tm.GetAllForConfig()
	assert.Len(t, allForConfig, 2)
	assert.Len(t, allForConfig["config-1"], 2)
	assert.Len(t, allForConfig["config-2"], 1)
}

func TestTopicManager_Count(t *testing.T) {
	tm := NewTopicManager()
	assert.Equal(t, 0, tm.Count())

	tm.Add("config-1", "topic-1")
	assert.Equal(t, 1, tm.Count())

	tm.Add("config-1", "topic-2")
	assert.Equal(t, 2, tm.Count())

	// Same topic for different config should not increase unique count
	tm.Add("config-2", "topic-1")
	assert.Equal(t, 2, tm.Count())

	tm.Add("config-2", "topic-3")
	assert.Equal(t, 3, tm.Count())
}

func TestTopicManager_CountForConfig(t *testing.T) {
	tm := NewTopicManager()

	assert.Equal(t, 0, tm.CountForConfig("config-1"))

	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	assert.Equal(t, 2, tm.CountForConfig("config-1"))

	tm.Add("config-2", "topic-3")
	assert.Equal(t, 2, tm.CountForConfig("config-1"))
	assert.Equal(t, 1, tm.CountForConfig("config-2"))
}

func TestTopicManager_Clear(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")
	tm.Add("config-1", "topic-2")
	tm.Add("config-2", "topic-3")

	tm.Clear()

	assert.Equal(t, 0, tm.Count())
	assert.Equal(t, 0, tm.CountForConfig("config-1"))
	assert.Equal(t, 0, tm.CountForConfig("config-2"))
	assert.Empty(t, tm.GetAll())
}

func TestTopicManager_RemoveCleansUpEmptyConfig(t *testing.T) {
	tm := NewTopicManager()
	tm.Add("config-1", "topic-1")

	tm.Remove("config-1", "topic-1")

	// Config should be cleaned up
	allForConfig := tm.GetAllForConfig()
	_, exists := allForConfig["config-1"]
	assert.False(t, exists)
}
