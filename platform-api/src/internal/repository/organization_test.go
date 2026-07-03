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
	"testing"

	"platform-api/src/internal/model"
)

// TestOrganizationRepo_IdpOrgRefUUIDRoundTrips verifies that the IDP organization
// reference (sourced from the token's org claim) is persisted on create and read
// back through every organization lookup path.
func TestOrganizationRepo_IdpOrgRefUUIDRoundTrips(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewOrganizationRepo(db)

	const idpRef = "idp-org-1234-5678"
	org := &model.Organization{
		ID:                     "org-uuid-1",
		Handle:                 "acme",
		Name:                   "Acme Corp",
		Region:                 "us",
		IdpOrganizationRefUUID: idpRef,
	}
	if err := repo.CreateOrganization(org); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}

	cases := map[string]func() (*model.Organization, error){
		"by uuid":    func() (*model.Organization, error) { return repo.GetOrganizationByUUID("org-uuid-1") },
		"by handle":  func() (*model.Organization, error) { return repo.GetOrganizationByHandle("acme") },
		"by idp ref": func() (*model.Organization, error) { return repo.GetOrganizationByIdpOrgRefUUID(idpRef) },
	}
	for name, lookup := range cases {
		got, err := lookup()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == nil {
			t.Fatalf("%s: expected organization, got nil", name)
		}
		if got.ID != "org-uuid-1" {
			t.Errorf("%s: ID = %q, want org-uuid-1", name, got.ID)
		}
		if got.IdpOrganizationRefUUID != idpRef {
			t.Errorf("%s: IdpOrganizationRefUUID = %q, want %q", name, got.IdpOrganizationRefUUID, idpRef)
		}
	}
}

// TestOrganizationRepo_IdpOrgRefUUIDEmptyForFileBased verifies that an org created
// without an IDP reference (file-based auth) stores an empty string and is never
// matched by a lookup on the IDP reference.
func TestOrganizationRepo_IdpOrgRefUUIDEmptyForFileBased(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewOrganizationRepo(db)

	org := &model.Organization{
		ID:     "org-uuid-2",
		Handle: "beta",
		Name:   "Beta",
		Region: "us",
	}
	if err := repo.CreateOrganization(org); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}

	got, err := repo.GetOrganizationByUUID("org-uuid-2")
	if err != nil {
		t.Fatalf("GetOrganizationByUUID: %v", err)
	}
	if got.IdpOrganizationRefUUID != "" {
		t.Errorf("IdpOrganizationRefUUID = %q, want empty", got.IdpOrganizationRefUUID)
	}

	// A lookup on an empty IDP reference must not match the file-based org.
	if match, err := repo.GetOrganizationByIdpOrgRefUUID(""); err != nil || match != nil {
		t.Errorf("GetOrganizationByIdpOrgRefUUID(\"\") = (%v, %v), want (nil, nil)", match, err)
	}
}
