/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package model

import (
	"reflect"
	"testing"
)

func TestServiceAccountRoleList(t *testing.T) {
	sa := &ServiceAccount{Roles: " ap_sa_reader  ap_sa_writer "}
	if got := sa.RoleList(); !reflect.DeepEqual(got, []string{"ap_sa_reader", "ap_sa_writer"}) {
		t.Fatalf("RoleList = %v", got)
	}
	if got := (&ServiceAccount{}).RoleList(); len(got) != 0 {
		t.Fatalf("empty roles = %v", got)
	}
}

func TestServiceAccountSubjectRoundTrip(t *testing.T) {
	sa := &ServiceAccount{Handle: "ci-bot", UUID: "0198a1b2-0000-7000-8000-000000000001"}
	sub := sa.Subject("acme")
	if sub != "sa:acme:ci-bot:0198a1b2-0000-7000-8000-000000000001" {
		t.Fatalf("Subject = %q", sub)
	}
	if id, ok := AccountUUIDFromSubject(sub); !ok || id != sa.UUID {
		t.Fatalf("AccountUUIDFromSubject = %q %v", id, ok)
	}
}

func TestAccountUUIDFromSubjectRejects(t *testing.T) {
	for _, sub := range []string{"", "alice", "sa:", "sa:acme:ci-bot:", "user:acme:ci-bot:id"} {
		if id, ok := AccountUUIDFromSubject(sub); ok {
			t.Errorf("%q: got %q, want no match", sub, id)
		}
	}
}
