/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package websocket

import "fmt"

// OrgConnectionLimitError is returned when an organization has reached its connection limit
type OrgConnectionLimitError struct {
	OrganizationID string
	CurrentCount   int
	MaxAllowed     int
}

func (e *OrgConnectionLimitError) Error() string {
	return fmt.Sprintf("organization %s has reached maximum connection limit: %d/%d",
		e.OrganizationID, e.CurrentCount, e.MaxAllowed)
}

// IsOrgConnectionLimitError checks if an error is an OrgConnectionLimitError
func IsOrgConnectionLimitError(err error) bool {
	_, ok := err.(*OrgConnectionLimitError)
	return ok
}
