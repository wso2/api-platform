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

package actor

import "testing"

func TestAdministrator(t *testing.T) {
	got := Administrator()
	if got.Username != "admin" || got.Password != "admin" {
		t.Fatalf("Administrator() = %#v, want admin/admin", got)
	}
}

func TestAdministratorReturnsAValue(t *testing.T) {
	first := Administrator()
	second := Administrator()
	if first != second {
		t.Fatalf("Administrator() returned inconsistent credentials: %#v and %#v", first, second)
	}
}
