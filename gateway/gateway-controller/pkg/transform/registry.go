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

package transform

import "github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"

var transformers map[string]models.ConfigTransformer

// Init sets the transformer registry once at startup.
// Tests call Init with a stub map (or nil to reset).
func Init(t map[string]models.ConfigTransformer) {
	transformers = t
}

// Get returns the transformer for the given API kind, or (nil, false) if none is registered.
func Get(kind string) (models.ConfigTransformer, bool) {
	t, ok := transformers[kind]
	return t, ok
}
