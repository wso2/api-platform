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
 *
 */

package api

import (
	"bytes"
	"encoding/json"
)

// UnmarshalJSON reads a subscription plan reference from an object ({"id": "gold", ...}) or from a
// plain plan handle string ("gold"), the form clients sent before the reference had details, so
// existing clients keep working.
func (r *SubscriptionPlanRef) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var handle string
		if err := json.Unmarshal(trimmed, &handle); err != nil {
			return err
		}
		*r = SubscriptionPlanRef{Id: handle}
		return nil
	}
	// A local type without this method, so decoding the object does not recurse.
	type plain SubscriptionPlanRef
	var ref plain
	if err := json.Unmarshal(trimmed, &ref); err != nil {
		return err
	}
	*r = SubscriptionPlanRef(ref)
	return nil
}
