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

package service

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// displayNameRepo records what InternalUserID passes down, can simulate a
// failure to save the display name alone, and serves a fixed set of rows.
type displayNameRepo struct {
	passthroughIdentityRepo
	identities map[string]repository.UserIdentity
	gotName    string
	saveErr    error
}

func (r *displayNameRepo) GetOrCreateUUIDWithDisplayName(identity, displayName string) (string, error) {
	r.gotName = displayName
	return "uuid-" + identity, r.saveErr
}

func (r *displayNameRepo) GetIdentitiesByUUIDs([]string) (map[string]repository.UserIdentity, error) {
	return r.identities, nil
}

func (r *displayNameRepo) GetSubByUUID(uuid string) (string, bool, error) {
	identity, ok := r.identities[uuid]
	return identity.IdpID, ok, nil
}

func namedRows() *displayNameRepo {
	return &displayNameRepo{identities: map[string]repository.UserIdentity{
		"u-named":   {IdpID: "sub-named", DisplayName: "Grace Hopper"},
		"u-unnamed": {IdpID: "sub-unnamed"},
		"u-blank":   {IdpID: "sub-blank", DisplayName: "   "},
	}}
}

func TestInternalUserID_PassesTheDisplayNameClaim(t *testing.T) {
	cases := []struct {
		name  string
		claim string
	}{
		{"claim present", "Ada Lovelace"},
		{"claim absent", ""},
	}
	for _, tc := range cases {
		repo := &displayNameRepo{}
		req := middleware.WithUserID(httptest.NewRequest("GET", "/", nil), "sub-1")
		if tc.claim != "" {
			req = middleware.WithDisplayName(req, tc.claim)
		}
		got, err := NewIdentityService(repo).InternalUserID(req)
		if err != nil || got != "uuid-sub-1" {
			t.Fatalf("%s: got %q, %v", tc.name, got, err)
		}
		if repo.gotName != tc.claim {
			t.Fatalf("%s: repo got display name %q, want %q", tc.name, repo.gotName, tc.claim)
		}
	}
}

// Failing to save the display name must not fail the write that needed the
// actor's UUID; any other repository failure still does.
func TestInternalUserID_DisplayNameSaveFailureIsNotFatal(t *testing.T) {
	req := middleware.WithDisplayName(middleware.WithUserID(httptest.NewRequest("GET", "/", nil), "sub-1"), "Ada")

	var logged bytes.Buffer
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))

	// SQL Server's truncation error quotes the rejected value.
	repo := &displayNameRepo{saveErr: fmt.Errorf("%w: String or binary data would be truncated, value 'Ada'", repository.ErrDisplayNameNotSaved)}
	if got, err := NewIdentityService(repo).InternalUserID(req); err != nil || got != "uuid-sub-1" {
		t.Fatalf("display-name failure: got %q, %v; want the UUID and no error", got, err)
	}
	if !strings.Contains(logged.String(), "uuid-sub-1") || strings.Contains(logged.String(), "Ada") {
		t.Fatalf("the warning must name the user's UUID and not the name or driver error: %s", logged.String())
	}

	repo = &displayNameRepo{saveErr: errors.New("database unavailable")}
	if _, err := NewIdentityService(repo).InternalUserID(req); err == nil {
		t.Fatal("a lookup failure must still be returned")
	}
}

// Rows with no display name (written before the column existed, or by tokens
// without the claim) fall back to the idp_id, which is what createdBy showed
// before display names existed.
func TestDisplayNamesForUUIDs_FallsBackToIdpID(t *testing.T) {
	svc := NewIdentityService(namedRows())

	got, err := svc.DisplayNamesForUUIDs([]string{"u-named", "u-unnamed", "u-blank", "u-gone", ""})
	if err != nil {
		t.Fatalf("DisplayNamesForUUIDs: %v", err)
	}
	want := map[string]string{
		"u-named":   "Grace Hopper",
		"u-unnamed": "sub-unnamed",
		"u-blank":   "sub-blank",
		"u-gone":    constants.DeletedUser,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, w := range want {
		if got[id] != w {
			t.Fatalf("%s: got %q, want %q", id, got[id], w)
		}
	}
	if name, err := svc.DisplayNameForUUID(""); err != nil || name != constants.DeletedUser {
		t.Fatalf("empty UUID: got %q, %v", name, err)
	}
}

// Response fields (createdBy and friends, and what plugins see through
// DisplayIdentity) carry the display name; SubForUUID, which gateway events
// use, keeps the raw idp_id the data plane matches on.
func TestResponsesShowDisplayNameWhileEventsKeepIdpID(t *testing.T) {
	svc := NewIdentityService(namedRows())

	named, unnamed := "u-named", "u-unnamed"
	createdBy, updatedBy := &named, &unnamed
	if err := svc.ResolveIdentityFields([]**string{&createdBy, &updatedBy}); err != nil {
		t.Fatalf("ResolveIdentityFields: %v", err)
	}
	if *createdBy != "Grace Hopper" || *updatedBy != "sub-unnamed" {
		t.Fatalf("list fields: createdBy=%q updatedBy=%q", *createdBy, *updatedBy)
	}

	single := "u-named"
	field := &single
	if err := svc.ResolveIdentityField(&field); err != nil || *field != "Grace Hopper" {
		t.Fatalf("detail field: %q, %v", *field, err)
	}
	if shown, err := svc.DisplayIdentity("u-named"); err != nil || shown != "Grace Hopper" {
		t.Fatalf("DisplayIdentity: %q, %v", shown, err)
	}
	if shown, err := svc.DisplayIdentity(""); err != nil || shown != "" {
		t.Fatalf("DisplayIdentity of an empty id: %q, %v", shown, err)
	}

	if sub, err := svc.SubForUUID("u-named"); err != nil || sub != "sub-named" {
		t.Fatalf("SubForUUID must stay the idp_id: %q, %v", sub, err)
	}
}
