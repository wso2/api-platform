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

package repository

import (
	"strings"
	"testing"
)

// storedDisplayName reads the stored display_name for idp; ok is false when
// the column is NULL.
func storedDisplayName(t *testing.T, s *schemaDB, idp string) (string, bool) {
	t.Helper()
	var name *string
	if err := s.db.QueryRow(s.db.Rebind(`SELECT display_name FROM user_idp_references WHERE idp_id = ?`), idp).Scan(&name); err != nil {
		t.Fatalf("read display_name: %v", err)
	}
	if name == nil {
		return "", false
	}
	return *name, true
}

// The display-name claim is optional: a token without it must create the row
// exactly as before (NULL name), and a later empty value must never clear a
// stored name. A non-empty, different value refreshes it.
func TestIdentity_GetOrCreateUUIDWithDisplayName_Lifecycle(t *testing.T) {
	tdb := openSchemaDB(t)
	repo := NewUserIdentityMappingRepo(tdb.db)

	first, err := repo.GetOrCreateUUIDWithDisplayName("sub-lifecycle", "")
	if err != nil || first == "" {
		t.Fatalf("create without a name: uuid=%q err=%v", first, err)
	}
	if _, ok := storedDisplayName(t, tdb, "sub-lifecycle"); ok {
		t.Fatal("a missing claim must store NULL, not a value")
	}

	steps := []struct {
		name, want string
	}{
		{"  Tharindu Dharmarathna  ", "Tharindu Dharmarathna"}, // first value is stored, trimmed
		{"", "Tharindu Dharmarathna"},                          // absent claim keeps it
		{"   ", "Tharindu Dharmarathna"},                       // blank claim keeps it
		{"Tharindu D.", "Tharindu D."},                         // a changed name refreshes it
	}
	for _, step := range steps {
		got, err := repo.GetOrCreateUUIDWithDisplayName("sub-lifecycle", step.name)
		if err != nil {
			t.Fatalf("name %q: %v", step.name, err)
		}
		if got != first {
			t.Fatalf("name %q: the UUID changed from %q to %q", step.name, first, got)
		}
		if stored, _ := storedDisplayName(t, tdb, "sub-lifecycle"); stored != step.want {
			t.Fatalf("name %q: stored %q, want %q", step.name, stored, step.want)
		}
	}

	if sub, found, err := repo.GetSubByUUID(first); err != nil || !found || sub != "sub-lifecycle" {
		t.Fatalf("idp_id must be unchanged: sub=%q found=%v err=%v", sub, found, err)
	}
}

func TestIdentity_GetOrCreateUUIDWithDisplayName_StoresOnCreateAndSharesRowWithLegacyPath(t *testing.T) {
	tdb := openSchemaDB(t)
	repo := NewUserIdentityMappingRepo(tdb.db)

	withName, err := repo.GetOrCreateUUIDWithDisplayName("sub-create", "Ada Lovelace")
	if err != nil {
		t.Fatalf("create with a name: %v", err)
	}
	if stored, ok := storedDisplayName(t, tdb, "sub-create"); !ok || stored != "Ada Lovelace" {
		t.Fatalf("stored %q (set=%v), want Ada Lovelace", stored, ok)
	}
	legacy, err := repo.GetOrCreateUUID("sub-create")
	if err != nil || legacy != withName {
		t.Fatalf("GetOrCreateUUID must resolve the same row: got %q err=%v, want %q", legacy, err, withName)
	}
}

func TestIdentity_GetOrCreateUUIDWithDisplayName_CapsOverlongNames(t *testing.T) {
	tdb := openSchemaDB(t)
	repo := NewUserIdentityMappingRepo(tdb.db)

	long := strings.Repeat("é", maxDisplayNameRunes+40)
	if _, err := repo.GetOrCreateUUIDWithDisplayName("sub-long", long); err != nil {
		t.Fatalf("an overlong name must not fail the write: %v", err)
	}
	stored, _ := storedDisplayName(t, tdb, "sub-long")
	if n := len([]rune(stored)); n != maxDisplayNameRunes {
		t.Fatalf("stored %d runes, want %d", n, maxDisplayNameRunes)
	}
}

func TestIdentity_GetIdentitiesByUUIDs(t *testing.T) {
	tdb := openSchemaDB(t)
	repo := NewUserIdentityMappingRepo(tdb.db)

	named, _ := repo.GetOrCreateUUIDWithDisplayName("sub-named", "Grace Hopper")
	unnamed, _ := repo.GetOrCreateUUID("sub-unnamed")

	got, err := repo.GetIdentitiesByUUIDs([]string{named, unnamed, "no-such-uuid", "", named})
	if err != nil {
		t.Fatalf("GetIdentitiesByUUIDs: %v", err)
	}
	if got[named] != (UserIdentity{IdpID: "sub-named", DisplayName: "Grace Hopper"}) {
		t.Fatalf("named: got %+v", got[named])
	}
	if got[unnamed] != (UserIdentity{IdpID: "sub-unnamed"}) {
		t.Fatalf("unnamed: got %+v", got[unnamed])
	}
	if _, ok := got["no-such-uuid"]; ok || len(got) != 2 {
		t.Fatalf("unmapped UUIDs must be absent: got %+v", got)
	}
}
