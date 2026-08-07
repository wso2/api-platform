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
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

func ptrTime(t time.Time) *time.Time { return &t }

// TestValidateExpiryInFuture pins the rule shared by every API key create/update
// path: an absolute expiry must be strictly in the future, a nil expiry means
// "never expires" and is left alone, and a rejection is a 400 (caller error), not
// a 500 — the LLM provider/proxy create paths call this directly, the REST path
// reaches it through resolveExpiresAt.
func TestValidateExpiryInFuture(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		wantErr   bool
	}{
		{name: "nil never expires", expiresAt: nil},
		{name: "future accepted", expiresAt: ptrTime(time.Now().Add(time.Hour))},
		{name: "past rejected", expiresAt: ptrTime(time.Now().Add(-time.Hour)), wantErr: true},
		{name: "far past rejected", expiresAt: ptrTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExpiryInFuture(tt.expiresAt)
			if tt.wantErr != (err != nil) {
				t.Fatalf("validateExpiryInFuture() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				assertBadRequest(t, err)
			}
		})
	}
}

// TestResolveExpiresAt covers both inputs the REST create/update paths accept:
// an absolute expiresAt (which must not already be in the past) and a relative
// expiresIn (which must convert to a future instant and reject an unknown unit).
func TestResolveExpiresAt(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		expiresIn *api.ExpirationDuration
		wantNil   bool
		wantErr   bool
	}{
		{name: "no expiry given", wantNil: true},
		{name: "future expiresAt", expiresAt: ptrTime(time.Now().Add(24 * time.Hour))},
		{name: "past expiresAt rejected", expiresAt: ptrTime(time.Now().Add(-time.Minute)), wantErr: true},
		{name: "expiresAt takes precedence over expiresIn", expiresAt: ptrTime(time.Now().Add(time.Hour)),
			expiresIn: &api.ExpirationDuration{Duration: 5, Unit: api.Days}},
		{name: "past expiresAt rejected even with valid expiresIn", expiresAt: ptrTime(time.Now().Add(-time.Hour)),
			expiresIn: &api.ExpirationDuration{Duration: 5, Unit: api.Days}, wantErr: true},
		{name: "positive expiresIn", expiresIn: &api.ExpirationDuration{Duration: 30, Unit: api.Minutes}},
		{name: "negative expiresIn rejected", expiresIn: &api.ExpirationDuration{Duration: -1, Unit: api.Hours}, wantErr: true},
		{name: "zero expiresIn rejected", expiresIn: &api.ExpirationDuration{Duration: 0, Unit: api.Hours}, wantErr: true},
		{name: "unknown unit rejected", expiresIn: &api.ExpirationDuration{Duration: 1, Unit: "fortnights"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveExpiresAt(tt.expiresAt, tt.expiresIn)
			if tt.wantErr != (err != nil) {
				t.Fatalf("resolveExpiresAt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				assertBadRequest(t, err)
				return
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("resolveExpiresAt() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("resolveExpiresAt() = nil, want a timestamp")
			}
			if !got.After(time.Now()) {
				t.Fatalf("resolveExpiresAt() = %v, which is not in the future", got)
			}
			if tt.expiresAt != nil && !got.Equal(*tt.expiresAt) {
				t.Fatalf("resolveExpiresAt() = %v, want the supplied expiresAt %v", got, *tt.expiresAt)
			}
		})
	}
}

// assertBadRequest fails the test unless err carries a catalog entry mapping to
// 400 — an already-expired expiry is a caller mistake, and surfacing it as a 500
// would both mislead the caller and mark a webhook event as retryable.
func assertBadRequest(t *testing.T, err error) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperror.Error, so it would surface as a 500", err)
	}
	if appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("error status = %d, want %d", appErr.HTTPStatus, http.StatusBadRequest)
	}
}
