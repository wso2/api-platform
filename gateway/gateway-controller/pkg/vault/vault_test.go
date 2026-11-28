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

package vault_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/redact"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
)

type fakeLocalStore struct {
	values   map[string]string
	upserted map[string]string
}

func newFakeLocalStore(values map[string]string) *fakeLocalStore {
	return &fakeLocalStore{values: values, upserted: map[string]string{}}
}

func (f *fakeLocalStore) Resolve(handle string) (string, error) {
	v, ok := f.values[handle]
	if !ok {
		return "", errors.New("secret not found")
	}
	return v, nil
}

func (f *fakeLocalStore) UpsertFromPlatform(handle, _, plaintext string) error {
	f.upserted[handle] = plaintext
	return nil
}

type fakeAdapter struct {
	values map[string]string // keyed by Ref.String()
	calls  []vault.Ref
}

func (f *fakeAdapter) Type() string { return "fake" }

func (f *fakeAdapter) Get(_ context.Context, ref vault.Ref) (string, error) {
	f.calls = append(f.calls, ref)
	v, ok := f.values[ref.String()]
	if !ok {
		return "", vault.ErrNotFound
	}
	return v, nil
}

// The vault must stand in for the secret service in both roles it plays today.
var (
	_ funcs.SecretResolver = (*vault.Vault)(nil)
	_ interface {
		UpsertFromPlatform(handle, displayName, plaintext string) error
	} = (*vault.Vault)(nil)
)

func TestVault_LocalHandleUsesDBAdapter(t *testing.T) {
	local := newFakeLocalStore(map[string]string{"openai-key": "sk-local"})
	ext := &fakeAdapter{}
	v := vault.New(vault.NewDBAdapter(local), map[string]vault.Adapter{"amp": ext}, nil)

	got, err := v.Resolve("openai-key")
	require.NoError(t, err)
	assert.Equal(t, "sk-local", got)
	assert.Empty(t, ext.calls, "a local handle must never reach an external resolver")
}

func TestVault_ExternalRefUsesNamedResolver(t *testing.T) {
	local := newFakeLocalStore(nil)
	amp := &fakeAdapter{values: map[string]string{"kv://amp/gw/c1?version=2#api_key": "sk-amp"}}
	customer := &fakeAdapter{}
	v := vault.New(vault.NewDBAdapter(local), map[string]vault.Adapter{"amp": amp, "customer": customer}, nil)

	got, err := v.Resolve("kv://amp/gw/c1?version=2#api_key")
	require.NoError(t, err)
	assert.Equal(t, "sk-amp", got)
	require.Len(t, amp.calls, 1)
	assert.Equal(t, vault.Ref{Resolver: "amp", Path: "gw/c1", Field: "api_key", Version: 2}, amp.calls[0])
	assert.Empty(t, customer.calls)
	assert.Equal(t, []string{"amp", "customer"}, v.ResolverNames())
}

func TestVault_UnknownResolver(t *testing.T) {
	v := vault.New(vault.NewDBAdapter(newFakeLocalStore(nil)), nil, nil)
	_, err := v.Resolve("kv://missing/a#b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"missing" is not configured`)
}

func TestVault_AdapterErrorPropagates(t *testing.T) {
	v := vault.New(nil, map[string]vault.Adapter{"amp": &fakeAdapter{}}, nil)
	_, err := v.Resolve("kv://amp/a#b")
	assert.ErrorIs(t, err, vault.ErrNotFound)
}

func TestVault_InvalidRef(t *testing.T) {
	v := vault.New(nil, map[string]vault.Adapter{"amp": &fakeAdapter{}}, nil)
	_, err := v.Resolve("kv://amp/../sys#x")
	assert.Error(t, err)
}

func TestVault_NoLocalStore(t *testing.T) {
	v := vault.New(nil, nil, nil)
	_, err := v.Resolve("openai-key")
	assert.Error(t, err)
	assert.Error(t, v.UpsertFromPlatform("openai-key", "x", "y"))
}

func TestVault_UpsertGoesToLocalStoreOnly(t *testing.T) {
	local := newFakeLocalStore(nil)
	v := vault.New(vault.NewDBAdapter(local), nil, nil)

	require.NoError(t, v.UpsertFromPlatform("amp-1-1", "amp-1-1", "sk"))
	assert.Equal(t, "sk", local.upserted["amp-1-1"])

	err := v.UpsertFromPlatform("kv://amp/a#b", "x", "sk")
	assert.Error(t, err, "external references are never copied into the local store")
	assert.NotContains(t, local.upserted, "kv://amp/a#b")
}

func TestDBAdapter_RejectsExternalRef(t *testing.T) {
	a := vault.NewDBAdapter(newFakeLocalStore(nil))
	assert.Equal(t, "db", a.Type())
	_, err := a.Get(context.Background(), vault.Ref{Resolver: "amp", Path: "a", Field: "b"})
	assert.Error(t, err)
}

// TestVault_TemplateSecretFunc renders through the real {{ secret }} function,
// as artifact rendering does, and checks both values are tracked for redaction.
func TestVault_TemplateSecretFunc(t *testing.T) {
	local := newFakeLocalStore(map[string]string{"local-key": "sk-local"})
	amp := &fakeAdapter{values: map[string]string{"kv://amp/gw/c1#value": "sk-amp"}}
	v := vault.New(vault.NewDBAdapter(local), map[string]vault.Adapter{"amp": amp}, nil)

	tracker := redact.NewSecretTracker()
	fm := funcs.BuildFuncMap(&funcs.Deps{SecretResolver: v, Tracker: tracker})
	tmpl, err := template.New("t").Funcs(fm).Parse(`a={{ secret "local-key" }} b={{ secret "kv://amp/gw/c1" }}`)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, tmpl.Execute(&out, nil))
	assert.Equal(t, "a=sk-local b=sk-amp", out.String())
	assert.ElementsMatch(t, []string{"sk-local", "sk-amp"}, tracker.Values())

	err = template.Must(template.New("t").Funcs(fm).Parse(`{{ secret "kv://amp/missing" }}`)).Execute(&out, nil)
	var secretErr *funcs.SecretError
	require.ErrorAs(t, err, &secretErr)
	assert.ErrorIs(t, err, vault.ErrNotFound)
}
