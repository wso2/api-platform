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

package upstreamref

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestFindByName_Found(t *testing.T) {
	defs := &[]api.UpstreamDefinition{
		{Name: "users-svc"},
		{Name: "orders-svc"},
	}
	def, err := FindByName("orders-svc", defs)
	require.NoError(t, err)
	require.NotNil(t, def)
	assert.Equal(t, "orders-svc", def.Name)
}

func TestFindByName_TrimsWhitespace(t *testing.T) {
	defs := &[]api.UpstreamDefinition{{Name: "users-svc"}}
	def, err := FindByName("  users-svc  ", defs)
	require.NoError(t, err)
	assert.Equal(t, "users-svc", def.Name)
}

func TestFindByName_EmptyRef(t *testing.T) {
	defs := &[]api.UpstreamDefinition{{Name: "users-svc"}}
	_, err := FindByName("", defs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestFindByName_WhitespaceRef(t *testing.T) {
	defs := &[]api.UpstreamDefinition{{Name: "users-svc"}}
	_, err := FindByName("   ", defs)
	require.Error(t, err)
}

func TestFindByName_NilDefs(t *testing.T) {
	_, err := FindByName("users-svc", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no definitions provided")
}

func TestFindByName_EmptyDefs(t *testing.T) {
	defs := &[]api.UpstreamDefinition{}
	_, err := FindByName("users-svc", defs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no definitions provided")
}

func TestFindByName_NotFound(t *testing.T) {
	defs := &[]api.UpstreamDefinition{{Name: "users-svc"}}
	_, err := FindByName("orders-svc", defs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFindByName_ReturnsStablePointer(t *testing.T) {
	defs := &[]api.UpstreamDefinition{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	got, err := FindByName("b", defs)
	require.NoError(t, err)
	assert.Same(t, &(*defs)[1], got, "must return pointer into the slice, not a copy of a loop variable")
}

func TestParseConnectTimeout_NilInput(t *testing.T) {
	d, err := ParseConnectTimeout(nil)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestParseConnectTimeout_EmptyString(t *testing.T) {
	empty := ""
	d, err := ParseConnectTimeout(&empty)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestParseConnectTimeout_WhitespaceOnly(t *testing.T) {
	ws := "   "
	d, err := ParseConnectTimeout(&ws)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestParseConnectTimeout_Valid(t *testing.T) {
	v := "5s"
	d, err := ParseConnectTimeout(&v)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 5*time.Second, *d)
}

func TestParseConnectTimeout_ValidMilliseconds(t *testing.T) {
	v := "500ms"
	d, err := ParseConnectTimeout(&v)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 500*time.Millisecond, *d)
}

func TestParseConnectTimeout_ValidMinutesAndHours(t *testing.T) {
	m := "2m"
	d, err := ParseConnectTimeout(&m)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 2*time.Minute, *d)

	h := "1h"
	d, err = ParseConnectTimeout(&h)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 1*time.Hour, *d)
}

func TestParseConnectTimeout_Malformed(t *testing.T) {
	v := "abc"
	_, err := ParseConnectTimeout(&v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout format")
}

func TestParseConnectTimeout_NoUnit(t *testing.T) {
	v := "30"
	_, err := ParseConnectTimeout(&v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout format")
}

func TestParseConnectTimeout_Zero(t *testing.T) {
	v := "0s"
	_, err := ParseConnectTimeout(&v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestParseConnectTimeout_Negative(t *testing.T) {
	v := "-5s"
	_, err := ParseConnectTimeout(&v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func ptrStr(s string) *string {
	return &s
}

func TestHasContent(t *testing.T) {
	assert.False(t, HasContent(nil))
	assert.False(t, HasContent(&api.Upstream{}))
	assert.False(t, HasContent(&api.Upstream{Url: ptrStr(""), Ref: ptrStr("")}))
	assert.False(t, HasContent(&api.Upstream{Url: ptrStr("   ")}))
	assert.False(t, HasContent(&api.Upstream{Ref: ptrStr("   ")}))

	assert.True(t, HasContent(&api.Upstream{Url: ptrStr("http://foo")}))
	assert.True(t, HasContent(&api.Upstream{Ref: ptrStr("foo-svc")}))
}

func TestSandboxActive(t *testing.T) {
	// API-level sandbox has content -> active
	assert.True(t, SandboxActive(&api.Upstream{Url: ptrStr("http://foo")}, nil))

	// API-level sandbox is empty, but operations have sandbox override -> active
	ops := []api.Operation{
		{
			Upstream: &api.OperationUpstream{
				Sandbox: &struct {
					Ref api.UpstreamReference "json:\"ref\" yaml:\"ref\""
				}{Ref: "op-sandbox-svc"},
			},
		},
	}
	assert.True(t, SandboxActive(nil, ops))

	// Both empty -> inactive
	assert.False(t, SandboxActive(nil, []api.Operation{{}}))
}
