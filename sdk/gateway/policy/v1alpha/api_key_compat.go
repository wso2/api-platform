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

package policyv1alpha

import (
	"github.com/wso2/api-platform/common/apikey"
)

// DEPRECATED: This file provides backward compatibility for code that imports API key types from the SDK.
// New code should import directly from github.com/wso2/api-platform/common/apikey instead.
// These re-exports will be removed in a future version.

// APIKey is deprecated. Use apikey.APIKey instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.APIKey
type APIKey = apikey.APIKey

// APIKeyStatus is deprecated. Use apikey.APIKeyStatus instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.APIKeyStatus
type APIKeyStatus = apikey.APIKeyStatus

// ParsedAPIKey is deprecated. Use apikey.ParsedAPIKey instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.ParsedAPIKey
type ParsedAPIKey = apikey.ParsedAPIKey

// APIkeyStore is deprecated. Use apikey.APIkeyStore instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.APIkeyStore
type APIkeyStore = apikey.APIkeyStore

// Status constants - deprecated
// Deprecated: Use github.com/wso2/api-platform/common/apikey constants
const (
	Active  = apikey.Active
	Expired = apikey.Expired
	Revoked = apikey.Revoked
)

// APIKeySeparator is deprecated. Use apikey.APIKeySeparator instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.APIKeySeparator
const APIKeySeparator = apikey.APIKeySeparator

// Errors - deprecated
// Deprecated: Use github.com/wso2/api-platform/common/apikey errors
var (
	ErrNotFound = apikey.ErrNotFound
	ErrConflict = apikey.ErrConflict
)

// NewAPIkeyStore is deprecated. Use apikey.NewAPIkeyStore instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.NewAPIkeyStore
func NewAPIkeyStore() *APIkeyStore {
	return apikey.NewAPIkeyStore()
}

// GetAPIkeyStoreInstance is deprecated. Use apikey.GetAPIkeyStoreInstance instead.
// Deprecated: Use github.com/wso2/api-platform/common/apikey.GetAPIkeyStoreInstance
func GetAPIkeyStoreInstance() *APIkeyStore {
	return apikey.GetAPIkeyStoreInstance()
}
