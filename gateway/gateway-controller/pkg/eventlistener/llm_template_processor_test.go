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

package eventlistener

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func TestHandleEvent_LLMTemplateCreate_LoadsTemplateFromDBAndPublishesLazyResource(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	template := testLLMProviderTemplate("tmpl-create-id", "openai")
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	lazyStore := storage.NewLazyResourceStore(newTestLogger())
	lazySnapshot := lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, newTestLogger())
	lazyManager := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazySnapshot, newTestLogger())

	listener := &EventListener{
		store:               store,
		db:                  db,
		lazyResourceManager: lazyManager,
		logger:              newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMTemplate,
		Action:    "CREATE",
		EntityID:  template.UUID,
		EventID:   "corr-llm-template-create",
	})

	stored, err := store.GetTemplate(template.UUID)
	require.NoError(t, err)
	assert.Equal(t, "openai", stored.GetHandle())

	resource, exists := lazyManager.GetResourceByIDAndType("openai", utils.LazyResourceTypeLLMProviderTemplate)
	require.True(t, exists)
	spec, ok := resource.Resource["spec"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Test Template", spec["displayName"])
}

func TestHandleEvent_LLMTemplateUpdate_RefreshesLocalTemplateFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)

	existing := testLLMProviderTemplate("tmpl-update-id", "openai")
	require.NoError(t, store.AddTemplate(existing))

	updated := testLLMProviderTemplate("tmpl-update-id", "openai")
	updated.Configuration.Spec.DisplayName = "Updated Template"
	require.NoError(t, db.SaveLLMProviderTemplate(updated))

	lazyStore := storage.NewLazyResourceStore(newTestLogger())
	lazySnapshot := lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, newTestLogger())
	lazyManager := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazySnapshot, newTestLogger())

	listener := &EventListener{
		store:               store,
		db:                  db,
		lazyResourceManager: lazyManager,
		logger:              newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMTemplate,
		Action:    "UPDATE",
		EntityID:  updated.UUID,
		EventID:   "corr-llm-template-update",
	})

	stored, err := store.GetTemplate(updated.UUID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Template", stored.Configuration.Spec.DisplayName)

	resource, exists := lazyManager.GetResourceByIDAndType("openai", utils.LazyResourceTypeLLMProviderTemplate)
	require.True(t, exists)
	spec, ok := resource.Resource["spec"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Updated Template", spec["displayName"])
}

func TestHandleEvent_LLMTemplateDelete_RemovesLocalState(t *testing.T) {
	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-delete-id", "openai")
	require.NoError(t, store.AddTemplate(template))

	lazyStore := storage.NewLazyResourceStore(newTestLogger())
	lazySnapshot := lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, newTestLogger())
	lazyManager := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazySnapshot, newTestLogger())

	listener := &EventListener{
		store:               store,
		lazyResourceManager: lazyManager,
		logger:              newTestLogger(),
	}

	resource, err := listener.buildLLMTemplateLazyResource(template.Configuration)
	require.NoError(t, err)
	require.NoError(t, lazyManager.StoreResource(resource, ""))

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMTemplate,
		Action:    "DELETE",
		EntityID:  template.UUID,
		EventID:   "corr-llm-template-delete",
	})

	_, err = store.GetTemplate(template.UUID)
	require.Error(t, err)

	_, exists := lazyManager.GetResourceByIDAndType("openai", utils.LazyResourceTypeLLMProviderTemplate)
	assert.False(t, exists)
}
