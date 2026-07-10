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
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func TestIsValidPolicyVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"major only v0", "v0", true},
		{"major only v1", "v1", true},
		{"major only v42", "v42", true},
		{"full semver rejected", "v1.0.0", false},
		{"missing v prefix", "1", false},
		{"uppercase V rejected", "V1", false},
		{"empty string means unspecified, resolved to latest by gateway", "", true},
		{"whitespace-only also means unspecified", "   ", true},
		{"v with no digits", "v", false},
		{"non-numeric suffix", "va", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPolicyVersion(tt.version); got != tt.want {
				t.Errorf("isValidPolicyVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestValidatePolicyVersions(t *testing.T) {
	if err := validatePolicyVersions(nil); err != nil {
		t.Errorf("expected nil error for nil list, got %v", err)
	}
	valid := []api.Policy{{Name: "SET_HEADER", Version: "v1"}}
	if err := validatePolicyVersions(&valid); err != nil {
		t.Errorf("expected nil error for valid version, got %v", err)
	}
	unspecified := []api.Policy{{Name: "SET_HEADER", Version: ""}}
	if err := validatePolicyVersions(&unspecified); err != nil {
		t.Errorf("expected nil error for unspecified version (resolved to latest by gateway), got %v", err)
	}
	invalid := []api.Policy{{Name: "SET_HEADER", Version: "v1.0.0"}}
	err := validatePolicyVersions(&invalid)
	if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
		t.Errorf("expected ErrInvalidPolicyVersion, got %v", err)
	}
}

func TestValidateLLMPolicyVersions(t *testing.T) {
	if err := validateLLMPolicyVersions(nil); err != nil {
		t.Errorf("expected nil error for nil list, got %v", err)
	}
	invalid := []api.LLMPolicy{{Name: "API_KEY_AUTH", Version: "1.0"}}
	err := validateLLMPolicyVersions(&invalid)
	if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
		t.Errorf("expected ErrInvalidPolicyVersion, got %v", err)
	}
}

func TestValidateOperationPolicyVersions(t *testing.T) {
	if err := validateOperationPolicyVersions(nil); err != nil {
		t.Errorf("expected nil error for nil list, got %v", err)
	}
	invalid := []api.OperationPolicy{{Name: "RATE_LIMIT", Version: "v1.2"}}
	err := validateOperationPolicyVersions(&invalid)
	if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
		t.Errorf("expected ErrInvalidPolicyVersion, got %v", err)
	}
}

func TestValidateOperationAndChannelPolicyVersions(t *testing.T) {
	badPolicies := []api.Policy{{Name: "SET_HEADER", Version: "v1.0.0"}}
	goodPolicies := []api.Policy{{Name: "SET_HEADER", Version: "v1"}}

	t.Run("nil operations and channels", func(t *testing.T) {
		if err := validateOperationAndChannelPolicyVersions(nil, nil); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("bad operation policy version", func(t *testing.T) {
		operations := []api.Operation{{Request: api.OperationRequest{Policies: &badPolicies}}}
		err := validateOperationAndChannelPolicyVersions(&operations, nil)
		if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
			t.Errorf("expected ErrInvalidPolicyVersion, got %v", err)
		}
	})

	t.Run("bad channel policy version", func(t *testing.T) {
		channels := []api.Channel{{Request: api.ChannelRequest{Policies: &badPolicies}}}
		err := validateOperationAndChannelPolicyVersions(nil, &channels)
		if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
			t.Errorf("expected ErrInvalidPolicyVersion, got %v", err)
		}
	})

	t.Run("valid operation and channel policy versions", func(t *testing.T) {
		operations := []api.Operation{{Request: api.OperationRequest{Policies: &goodPolicies}}}
		channels := []api.Channel{{Request: api.ChannelRequest{Policies: &goodPolicies}}}
		if err := validateOperationAndChannelPolicyVersions(&operations, &channels); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}
