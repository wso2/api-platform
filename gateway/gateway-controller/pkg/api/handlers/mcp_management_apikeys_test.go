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

package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func TestParseFutureTimestamp(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	got, err := parseFutureTimestamp("expiresAt", future)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, future, got.Format(time.RFC3339))

	got, err = parseFutureTimestamp("expiresAt", "  ")
	require.NoError(t, err)
	assert.Nil(t, got, "a blank value means no expiry, not an error")

	_, err = parseFutureTimestamp("expiresAt", "2020-01-01T00:00:00Z")
	require.Error(t, err)
	assert.Equal(t, `expiresAt "2020-01-01T00:00:00Z" is in the past; supply a timestamp later than the current time`, err.Error())

	_, err = parseFutureTimestamp("expiresAt", "next tuesday")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expiresAt "next tuesday" is not a valid RFC 3339 timestamp`)
}

func TestParseExpiresIn(t *testing.T) {
	got, err := parseExpiresIn(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = parseExpiresIn(&expiresInInput{Duration: 90, Unit: " DAYS "})
	require.NoError(t, err)
	assert.Equal(t, &expiresInInput{Duration: 90, Unit: "days"}, got, "the unit is normalised to the service's spelling")

	got, err = parseExpiresIn(&expiresInInput{Duration: 0, Unit: "seconds"})
	require.NoError(t, err)
	assert.Equal(t, 0, got.Duration, "zero is allowed: the key expires immediately")

	_, err = parseExpiresIn(&expiresInInput{Duration: 1, Unit: "fortnights"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expiresIn.unit "fortnights" is not supported`)

	_, err = parseExpiresIn(&expiresInInput{Duration: -1, Unit: "days"})
	require.Error(t, err)
	assert.Equal(t, "expiresIn.duration must not be negative", err.Error())
}

func TestExpiresInConversionsCarryBothFields(t *testing.T) {
	assert.Nil(t, creationExpiresIn(nil))
	assert.Nil(t, regenerationExpiresIn(nil))

	in := &expiresInInput{Duration: 7, Unit: "weeks"}
	c := creationExpiresIn(in)
	require.NotNil(t, c)
	assert.Equal(t, 7, c.Duration)
	assert.Equal(t, api.APIKeyCreationRequestExpiresInUnit("weeks"), c.Unit)

	r := regenerationExpiresIn(in)
	require.NotNil(t, r)
	assert.Equal(t, 7, r.Duration)
	assert.Equal(t, api.APIKeyRegenerationRequestExpiresInUnit("weeks"), r.Unit)
}

func TestIssueAPIKeyGenerateMode(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key",
		ExpiresIn: &expiresInInput{Duration: 90, Unit: "Days"},
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "generate", res["operation"])
	assert.Equal(t, "orders", res["id"])
	assert.Contains(t, res["notice"], "shown once", "a generated key must carry the one-time notice")

	require.Equal(t, 1, f.rest.keyCalls["issue"])
	assert.Equal(t, "orders", f.rest.keyParent)
	require.NotNil(t, f.rest.caller)
	assert.Equal(t, "alice", f.rest.caller.UserID, "the key is recorded against the verified caller")

	req := f.rest.issueReq
	require.NotNil(t, req.Name)
	assert.Equal(t, "prod-key", *req.Name)
	assert.Nil(t, req.ApiKey, "generate mode sends no key value")
	require.NotNil(t, req.ExpiresIn)
	assert.Equal(t, api.APIKeyCreationRequestExpiresInUnit("days"), req.ExpiresIn.Unit)
	assert.Nil(t, req.ExternalRefId)
	assert.Nil(t, req.Issuer)
}

func TestIssueAPIKeyRegisterMode(t *testing.T) {
	f := newToolsHandler(false)
	value := strings.Repeat("k", 40)
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	_, out, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{
		Kind: "RestApi", ID: "orders",
		ApiKey:        "  " + value + "  ",
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		ExternalRefId: " ext-42 ",
		Issuer:        "   ",
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "register", res["operation"])
	assert.NotContains(t, res, "notice", "a caller-supplied key is never echoed, so there is nothing to warn about")

	req := f.rest.issueReq
	require.NotNil(t, req.ApiKey)
	assert.Equal(t, value, *req.ApiKey, "the supplied key is trimmed before it is sent")
	require.NotNil(t, req.ExpiresAt)
	assert.True(t, expiresAt.Equal(*req.ExpiresAt))
	require.NotNil(t, req.ExternalRefId)
	assert.Equal(t, "ext-42", *req.ExternalRefId)
	assert.Nil(t, req.Issuer, "a blank issuer is omitted, not sent as an empty restriction")
}

func TestIssueAPIKeyRefusesBeforeReachingTheService(t *testing.T) {
	tests := []struct {
		name      string
		immutable bool
		ctx       context.Context
		in        issueKeyInput
		wantErr   string
	}{
		{"immutable mode", true, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "immutable mode"},
		{"invalid key name", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", KeyName: "Prod Key"}, "API key name must be lowercase"},
		{"expiry in the past", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresAt: "2020-01-01T00:00:00Z"}, "is in the past"},
		{"unparseable expiry", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresAt: "tomorrow"}, "not a valid RFC 3339 timestamp"},
		{"unknown expiresIn unit", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresIn: &expiresInInput{Duration: 1, Unit: "years"}}, "is not supported"},
		{"negative expiresIn", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresIn: &expiresInInput{Duration: -5, Unit: "days"}}, "must not be negative"},
		{"kind without API keys", false, toolsCtx("admin"),
			issueKeyInput{Kind: "Mcp", ID: "tools"}, `kind "Mcp" does not have API keys; the API key tools accept: RestApi`},
		{"unknown kind", false, toolsCtx("admin"),
			issueKeyInput{Kind: "Widget", ID: "w1"}, `unknown kind "Widget"`},
		{"caller lacks the role", false, toolsCtx("viewer"),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "insufficient scope"},
		// Role checks can be skipped, identity cannot: an empty UserID would act
		// as "no creator filter" in the key service.
		{"no caller identity", false,
			withMcpCaller(context.Background(), mcpCaller{Skipped: true}),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "requires an authenticated user identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)

			_, out, err := f.h.issueAPIKey(tt.ctx, nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, out)
			assert.Zero(t, f.rest.keyCalls["issue"])
		})
	}
}

func TestIssueAPIKeyMapsServiceErrors(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: API configuration handle 'orders' not found", storage.ErrNotFound)

	_, _, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{Kind: "RestApi", ID: "orders"})

	require.Error(t, err)
	assert.Equal(t, `cannot issue an API key for RestApi "orders": no such resource, or no such key on it`, err.Error())
}

func TestListAPIKeys(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.listAPIKeys(toolsCtx("developer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, map[string]any{"listed": true}, res["result"])
	assert.Equal(t, "orders", res["id"])
	assert.Equal(t, 1, f.rest.keyCalls["list"])
	assert.Equal(t, "alice", f.rest.caller.UserID)
}

func TestListAPIKeysFailures(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.listAPIKeys(toolsCtx("viewer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")
	assert.Zero(t, f.rest.keyCalls["list"])

	f.rest.keyErr = errors.New("connection reset by peer")
	_, _, err = f.h.listAPIKeys(toolsCtx("developer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `failed to list API keys for RestApi "orders": the Gateway refused the operation`)
	assert.NotContains(t, err.Error(), "connection reset", "an unmapped error stays generic")
}

func TestRotateIsInjection(t *testing.T) {
	assert.False(t, rotateIsInjection(rotateKeyInput{}))
	assert.False(t, rotateIsInjection(rotateKeyInput{ApiKey: "   "}), "whitespace is not a key value")
	assert.True(t, rotateIsInjection(rotateKeyInput{ApiKey: "a-real-value"}))
}

func TestRotateAPIKeyRegeneratesWithoutAnAPIKey(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: "  ",
		ExpiresIn: &expiresInInput{Duration: 30, Unit: "days"},
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "regenerate", res["operation"])
	assert.Equal(t, "prod-key", res["keyName"])
	assert.Contains(t, res["notice"], "previous key value is now invalid")
	assert.Equal(t, 1, f.rest.keyCalls["rotate"])
	assert.Zero(t, f.rest.keyCalls["update"])
	assert.Equal(t, "prod-key", f.rest.keyName)
	require.NotNil(t, f.rest.rotateReq.ExpiresIn)
	assert.Equal(t, 30, f.rest.rotateReq.ExpiresIn.Duration)
}

func TestRotateAPIKeyInstallsASuppliedValue(t *testing.T) {
	f := newToolsHandler(false)
	value := strings.Repeat("v", 40)

	_, out, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: value,
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "update", res["operation"])
	assert.NotContains(t, res, "notice", "the caller supplied the value, so it is never echoed")
	assert.Equal(t, 1, f.rest.keyCalls["update"])
	assert.Zero(t, f.rest.keyCalls["rotate"])
	require.NotNil(t, f.rest.updateReq.ApiKey)
	assert.Equal(t, value, *f.rest.updateReq.ApiKey)
}

// Regeneration and value injection are two different REST routes with two
// different role requirements; the tool must authorize the one it performs.
func TestRotateAPIKeyAuthorizesTheRouteItPerforms(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: strings.Repeat("v", 40),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope", "injection needs PUT .../{apiKeyName}, admin only")
	assert.Zero(t, f.rest.keyCalls["update"])

	_, _, err = f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key",
	})
	require.NoError(t, err, "regeneration is POST .../regenerate, which developer holds")
}

func TestRotateAPIKeyRefusesBeforeReachingTheService(t *testing.T) {
	for _, tt := range []struct {
		name      string
		immutable bool
		in        rotateKeyInput
		wantErr   string
	}{
		{"immutable mode", true, rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"}, "immutable mode"},
		{"expiry in the past", false,
			rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", ExpiresAt: "2020-01-01T00:00:00Z"}, "is in the past"},
		{"unknown expiresIn unit", false,
			rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", ExpiresIn: &expiresInInput{Duration: 1, Unit: "eons"}}, "is not supported"},
		{"kind without API keys", false,
			rotateKeyInput{Kind: "Mcp", ID: "tools", KeyName: "k1"}, "does not have API keys"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)

			_, _, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Zero(t, f.rest.keyCalls["rotate"]+f.rest.keyCalls["update"])
		})
	}
}

func TestRotateAPIKeyMapsServiceErrors(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: generated keys cannot be overwritten", storage.ErrOperationNotAllowed)

	_, _, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "k1", ApiKey: strings.Repeat("v", 40),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cannot update the API key on RestApi "orders"`)
	assert.Contains(t, err.Error(), `call the same tool again without "apiKey"`,
		"the refusal must name the retry that works")

	f.rest.keyErr = fmt.Errorf("%w: key k1", storage.ErrNotFound)
	_, _, err = f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"})
	require.Error(t, err)
	assert.Equal(t, `cannot rotate the API key on RestApi "orders": no such resource, or no such key on it`, err.Error())
}

func TestRevokeAPIKey(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "success", res["status"])
	assert.Equal(t, "k1", res["keyName"])
	assert.Equal(t, map[string]any{"revoked": "k1"}, res["result"])
	assert.Equal(t, 1, f.rest.keyCalls["revoke"])
}

func TestRevokeAPIKeyRefusals(t *testing.T) {
	f := newToolsHandler(false)
	_, _, err := f.h.revokeAPIKey(toolsCtx("developer"), nil, revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"})
	require.Error(t, err)
	assert.Equal(t, `refusing to revoke API key "k1" on RestApi "orders": confirm the details with the user, then retry with confirm=true`,
		err.Error())

	f = newToolsHandler(true)
	_, _, err = f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "immutable mode")
	assert.Zero(t, f.rest.keyCalls["revoke"])

	f = newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: duplicate", storage.ErrConflict)
	_, _, err = f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a key with that name already exists")
}

func TestKeyOpErrorNeverEchoesTheCause(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{"not found", fmt.Errorf("%w: secret-detail", storage.ErrNotFound), "no such resource, or no such key on it"},
		{"conflict", fmt.Errorf("%w: secret-detail", storage.ErrConflict), "a key with that name already exists"},
		{"operation not allowed", fmt.Errorf("%w: secret-detail", storage.ErrOperationNotAllowed),
			"a key value can only be installed over a key whose value was supplied"},
		{"anything else", errors.New("secret-detail"), "the Gateway refused the operation"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := keyOpError("rotate the API key on", "RestApi", "orders", tt.err)
			assert.Contains(t, got.Error(), tt.want)
			assert.Contains(t, got.Error(), `RestApi "orders"`)
			assert.NotContains(t, got.Error(), "secret-detail")
		})
	}
}

// The API-key service's expiry sentinels are client-input errors. The REST
// api-key handlers return them as 400 with the service's message, so keyOpError
// echoes it too, unlike every other cause.
func TestKeyOpErrorEchoesExpiryErrors(t *testing.T) {
	for _, err := range []error{
		fmt.Errorf("%w, got: 2020-01-01T00:00:00Z (current time: 2026-10-16T00:00:00Z)", utils.ErrAPIKeyExpirationInPast),
		fmt.Errorf("%w: fortnights", utils.ErrUnsupportedAPIKeyExpirationUnit),
	} {
		got := keyOpError("issue an API key for", "RestApi", "orders", err)
		assert.Equal(t, `cannot issue an API key for RestApi "orders": `+err.Error(), got.Error())
	}
}
