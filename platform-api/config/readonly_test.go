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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

func TestLoadConfig_ReadOnly_DefaultsOff(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.False(t, cfg.ReadOnly.Enabled)
	assert.Nil(t, cfg.ReadOnly.WritableOrganizations)
	assert.False(t, cfg.ReadOnly.IsReadOnlyOrg(readOnlyTestOrgA))
}

// A TOML list is trimmed, canonicalised to lowercase hyphenated form (uppercase and
// braced spellings included) and de-duplicated; empty entries are dropped.
func TestLoadConfig_ReadOnly_ListIsTrimmedCanonicalisedAndDeduped(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[platform_api.read_only]
enabled = true
writable_organizations = [" `+strings.ToUpper(readOnlyTestOrgA)+` ", "`+readOnlyTestOrgB+`", "{`+readOnlyTestOrgA+`}", ""]
`)
	require.NoError(t, err)
	assert.True(t, cfg.ReadOnly.Enabled)
	assert.Equal(t, []string{readOnlyTestOrgA, readOnlyTestOrgB}, cfg.ReadOnly.WritableOrganizations)
	assert.False(t, cfg.ReadOnly.IsReadOnlyOrg(readOnlyTestOrgA))
	assert.False(t, cfg.ReadOnly.IsReadOnlyOrg(readOnlyTestOrgB))
	assert.True(t, cfg.ReadOnly.IsReadOnlyOrg("33333333-3333-3333-3333-333333333333"))
}

// Values supplied through {{ env }} tokens arrive as strings: the bool is weakly
// typed and the list is split on commas by the decode hook, untrimmed.
func TestLoadConfig_ReadOnly_EnvTokensWithCommaSeparatedList(t *testing.T) {
	t.Setenv("APIP_CP_READ_ONLY_ENABLED", "true")
	t.Setenv("APIP_CP_READ_ONLY_WRITABLE_ORGANIZATIONS",
		" "+readOnlyTestOrgA+" , "+readOnlyTestOrgB+","+readOnlyTestOrgA+" ,, ")
	cfg, err := loadWithKeys(t, `
[platform_api.read_only]
enabled                = '{{ env "APIP_CP_READ_ONLY_ENABLED" "false" }}'
writable_organizations = '{{ env "APIP_CP_READ_ONLY_WRITABLE_ORGANIZATIONS" "" }}'
`)
	require.NoError(t, err)
	assert.True(t, cfg.ReadOnly.Enabled)
	assert.Equal(t, []string{readOnlyTestOrgA, readOnlyTestOrgB}, cfg.ReadOnly.WritableOrganizations)
}

// The documented env example with the variables unset leaves the mode off, and an
// empty list string must not become a single empty entry.
func TestLoadConfig_ReadOnly_EnvTokenDefaultsAreOff(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[platform_api.read_only]
enabled                = '{{ env "APIP_CP_READ_ONLY_ENABLED_UNSET_FOR_TEST" "false" }}'
writable_organizations = '{{ env "APIP_CP_READ_ONLY_WRITABLE_ORGANIZATIONS_UNSET_FOR_TEST" "" }}'
`)
	require.NoError(t, err)
	assert.False(t, cfg.ReadOnly.Enabled)
	assert.Nil(t, cfg.ReadOnly.WritableOrganizations)
}

// A handle (or any non-UUID) must fail startup even while the mode is disabled, so
// the mistake is caught before the flag is flipped on.
func TestLoadConfig_ReadOnly_InvalidUUIDFailsStartup(t *testing.T) {
	_, err := loadWithKeys(t, `
[platform_api.read_only]
enabled = false
writable_organizations = ["my-org-handle"]
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read_only.writable_organizations")
	assert.Contains(t, err.Error(), "my-org-handle")
}

func TestLoadConfig_ReadOnly_EnabledWithEmptyListFreezesEveryOrganization(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[platform_api.read_only]
enabled = true
`)
	require.NoError(t, err)
	assert.True(t, cfg.ReadOnly.Enabled)
	assert.Nil(t, cfg.ReadOnly.WritableOrganizations)
	assert.True(t, cfg.ReadOnly.IsReadOnlyOrg(readOnlyTestOrgA))
}

func TestReadOnly_IsReadOnlyOrg(t *testing.T) {
	enabled := &ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgA}}
	disabled := &ReadOnly{Enabled: false, WritableOrganizations: []string{readOnlyTestOrgA}}
	allFrozen := &ReadOnly{Enabled: true}
	var nilRO *ReadOnly

	tests := []struct {
		name string
		ro   *ReadOnly
		org  string
		want bool
	}{
		{"nil receiver", nilRO, readOnlyTestOrgB, false},
		{"disabled: unlisted org is writable", disabled, readOnlyTestOrgB, false},
		{"disabled: empty org is writable", disabled, "", false},
		{"enabled: listed exact", enabled, readOnlyTestOrgA, false},
		{"enabled: listed uppercase", enabled, strings.ToUpper(readOnlyTestOrgA), false},
		{"enabled: listed padded", enabled, "  " + readOnlyTestOrgA + " ", false},
		{"enabled: unlisted org is read-only", enabled, readOnlyTestOrgB, true},
		{"enabled: empty org is read-only (fail closed)", enabled, "", true},
		{"enabled, empty list: every org is read-only", allFrozen, readOnlyTestOrgA, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.ro.IsReadOnlyOrg(tc.org))
		})
	}
}
