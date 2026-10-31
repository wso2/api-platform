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

package cloudconsole

import (
	"encoding/json"
	"strings"
)

// parseConfigCell reads a table cell as the JSON value it spells (a number, a boolean, null or a
// quoted string) and otherwise as plain text, so a scenario can send a value of any type.
func parseConfigCell(raw string) any {
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &value); err == nil {
		return value
	}
	return raw
}
