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
	"strings"
	"testing"

	"platform-api/src/internal/constants"
)

// TestValidateAPIKeyIssuerAndTargets pins the 255-character storage limit on the
// API-key issuer and allowedTargets fields: exactly 255 is accepted, 256 is
// rejected. Both are carried verbatim (issuer is an exact-match lookup key,
// allowedTargets a parsed gateway allow-list) so an over-length value must be
// rejected, never truncated.
func TestValidateAPIKeyIssuerAndTargets(t *testing.T) {
	issuer255 := strings.Repeat("a", 255)
	issuer256 := strings.Repeat("a", 256)
	targets255 := strings.Repeat("b", 255)
	targets256 := strings.Repeat("b", 256)

	tests := []struct {
		name           string
		issuer         *string
		allowedTargets string
		wantErr        error
	}{
		{"nil issuer, ALL targets", nil, "ALL", nil},
		{"issuer at limit", &issuer255, "ALL", nil},
		{"issuer over limit", &issuer256, "ALL", constants.ErrIssuerTooLong},
		{"targets at limit", nil, targets255, nil},
		{"targets over limit", nil, targets256, constants.ErrAllowedTargetsTooLong},
		{"issuer checked before targets", &issuer256, targets256, constants.ErrIssuerTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAPIKeyIssuerAndTargets(tt.issuer, tt.allowedTargets)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateAPIKeyIssuerAndTargets() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
