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

package apikey

import (
	"fmt"
	"strings"
)

const entityIDSeparator = "_"

// BuildAPIKeyEntityID returns the composite entity ID used for API key events.
func BuildAPIKeyEntityID(apiID, keyID string) string {
	return apiID + entityIDSeparator + keyID
}

// ParseAPIKeyEntityID splits the composite entity ID used for API key events.
func ParseAPIKeyEntityID(entityID string) (string, string, error) {
	separatorIndex := strings.LastIndex(entityID, entityIDSeparator)
	if separatorIndex <= 0 || separatorIndex == len(entityID)-1 {
		return "", "", fmt.Errorf("invalid API key entity ID: %q", entityID)
	}

	return entityID[:separatorIndex], entityID[separatorIndex+1:], nil
}
