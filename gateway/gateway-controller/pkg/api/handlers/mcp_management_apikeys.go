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
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// API key tools

// beginKeyOp is the shared preamble for the four api-key tools: resolve the
// kind, authorize, then resolve the caller's identity.
func (h *McpHandler) beginKeyOp(ctx context.Context, rawKind, tool, method, routeSuffix string) (
	*kindOps, *commonmodels.AuthContext, *slog.Logger, string, error) {

	ops, err := h.resolveKeyBearing(rawKind)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if err := h.authz.authorize(ctx, keyRouteKey(method, ops, routeSuffix)); err != nil {
		return nil, nil, nil, "", err
	}
	caller, err := h.authz.callerIdentity(ctx)
	if err != nil {
		return nil, nil, nil, "", err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	return ops, caller, log, correlationID, nil
}

// parseTimestamp converts a tool's RFC 3339 string into the pointer the
// generated request types use. An unparseable value is returned as an error
// rather than dropped: silently ignoring it would write a different lifetime
// than the user asked for. field names the argument, so the message points at
// the one the caller actually sent.
func parseTimestamp(field, raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf(
			"%s %q is not a valid RFC 3339 timestamp; use a form like 2027-01-31T23:59:59Z", field, raw)
	}
	return &t, nil
}

// parseFutureTimestamp parses an RFC 3339 expiry and rejects one already in the
// past. The service rejects it too (utils.ErrAPIKeyExpirationInPast); checking
// here first means the message names the exact argument the caller sent.
func parseFutureTimestamp(field, raw string) (*time.Time, error) {
	t, err := parseTimestamp(field, raw)
	if err != nil || t == nil {
		return t, err
	}
	if t.Before(time.Now()) {
		return nil, fmt.Errorf(
			"%s %q is in the past; supply a timestamp later than the current time", field, raw)
	}
	return t, nil
}

// parseExpiresIn validates an expiresIn argument before it reaches the service,
// so an unknown unit or a negative duration is reported against the argument the
// caller sent. The unit is normalised to the lowercase form the service matches
// on; nil in means nil out.
func parseExpiresIn(in *expiresInInput) (*expiresInInput, error) {
	if in == nil {
		return nil, nil
	}
	unit := strings.ToLower(strings.TrimSpace(in.Unit))
	switch unit {
	case "seconds", "minutes", "hours", "days", "weeks", "months":
	default:
		return nil, fmt.Errorf(
			"expiresIn.unit %q is not supported; use one of seconds, minutes, hours, days, weeks, months", in.Unit)
	}
	if in.Duration < 0 {
		return nil, fmt.Errorf("expiresIn.duration must not be negative")
	}
	return &expiresInInput{Duration: in.Duration, Unit: unit}, nil
}

// The generated request types declare ExpiresIn as a pointer to an unnamed
// struct, so a value can only be built by restating its exact field types and
// tags. These two helpers keep that restatement in one place per request type.
func creationExpiresIn(in *expiresInInput) *struct {
	Duration int                                    `json:"duration" yaml:"duration"`
	Unit     api.APIKeyCreationRequestExpiresInUnit `json:"unit" yaml:"unit"`
} {
	if in == nil {
		return nil
	}
	return &struct {
		Duration int                                    `json:"duration" yaml:"duration"`
		Unit     api.APIKeyCreationRequestExpiresInUnit `json:"unit" yaml:"unit"`
	}{Duration: in.Duration, Unit: api.APIKeyCreationRequestExpiresInUnit(in.Unit)}
}

func regenerationExpiresIn(in *expiresInInput) *struct {
	Duration int                                        `json:"duration" yaml:"duration"`
	Unit     api.APIKeyRegenerationRequestExpiresInUnit `json:"unit" yaml:"unit"`
} {
	if in == nil {
		return nil
	}
	return &struct {
		Duration int                                        `json:"duration" yaml:"duration"`
		Unit     api.APIKeyRegenerationRequestExpiresInUnit `json:"unit" yaml:"unit"`
	}{Duration: in.Duration, Unit: api.APIKeyRegenerationRequestExpiresInUnit(in.Unit)}
}

// immutableKeyWrite mirrors the guard on write/remove. The MCP route is excluded
// from the immutable middleware (it rejects every POST, which would disable
// reads too), but the REST api-key routes are NOT excluded — so
// POST /rest-apis/{id}/api-keys is already refused in immutable mode. Repeating
// the check here is parity, not a new policy.
func (h *McpHandler) immutableKeyWrite() error {
	if h.immutable {
		return fmt.Errorf(
			"this Gateway runs in immutable mode; API keys cannot be issued, rotated or revoked at runtime")
	}
	return nil
}

func (h *McpHandler) issueAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in issueKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	expiresAt, err := parseFutureTimestamp("expiresAt", in.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}
	expiresIn, err := parseExpiresIn(in.ExpiresIn)
	if err != nil {
		return nil, nil, err
	}
	if trimmed := strings.TrimSpace(in.KeyName); trimmed != "" {
		if err := utils.ValidateAPIKeyName(trimmed); err != nil {
			return nil, nil, err
		}
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, toolIssueAPIKey, http.MethodPost, "")
	if err != nil {
		return nil, nil, err
	}

	req := api.APIKeyCreationRequest{
		Name:      &in.KeyName,
		ExpiresAt: expiresAt,
		ExpiresIn: creationExpiresIn(expiresIn),
	}
	supplied := strings.TrimSpace(in.ApiKey)
	if supplied != "" {
		req.ApiKey = &supplied
	}
	if v := strings.TrimSpace(in.ExternalRefId); v != "" {
		req.ExternalRefId = &v
	}
	if v := strings.TrimSpace(in.Issuer); v != "" {
		req.Issuer = &v
	}

	resp, err := ops.Keys.Issue(in.ID, req, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key creation failed",
			slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, keyOpError("issue an API key for", ops.Kind, in.ID, err)
	}

	if supplied != "" {
		return nil, map[string]any{
			"status": "success", "operation": "register",
			"kind": ops.Kind, "id": in.ID,
			"result": resp,
		}, nil
	}
	return nil, map[string]any{
		"status": "success", "operation": "generate",
		"kind": ops.Kind, "id": in.ID,
		"result": resp,
		"notice": "The plaintext key in this response is shown once and cannot be retrieved again. " +
			"Give it to the user now and do not repeat it.",
	}, nil
}

func (h *McpHandler) listAPIKeys(ctx context.Context, _ *mcp.CallToolRequest, in listKeysInput) (*mcp.CallToolResult, any, error) {
	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, toolListAPIKeys, http.MethodGet, "")
	if err != nil {
		return nil, nil, err
	}

	resp, err := ops.Keys.List(in.ID, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key listing failed",
			slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, keyOpError("list API keys for", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"kind": ops.Kind, "id": in.ID, "result": resp,
	}, nil
}

// rotateIsInjection reports whether a rotate call installs a caller-supplied key
// value (PUT .../{apiKeyName}, the external-key-injection operation) rather than
// having the Gateway generate a new one (POST .../{apiKeyName}/regenerate).
//
// Called by BOTH the authorization gate and the tool, so the route key that was
// authorized is always the route key that executes. This is the same
// dispatch-on-argument shape write() uses to split create from update, which is
// why one tool covering two REST operations still maps to exactly one route key
// per call — no conjunctive evaluation is needed.
func rotateIsInjection(in rotateKeyInput) bool {
	return strings.TrimSpace(in.ApiKey) != ""
}

func (h *McpHandler) rotateAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in rotateKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	expiresAt, err := parseFutureTimestamp("expiresAt", in.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}
	expiresIn, err := parseExpiresIn(in.ExpiresIn)
	if err != nil {
		return nil, nil, err
	}

	// An absent apiKey IS the signal to regenerate, so — unlike the REST
	// handler, whose single route cannot tell the two intents apart — an empty
	// value routes to regeneration rather than being an error.
	injecting := rotateIsInjection(in)
	method, suffix := http.MethodPost, "/{apiKeyName}/regenerate"
	if injecting {
		method, suffix = http.MethodPut, "/{apiKeyName}"
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, toolRotateAPIKey, method, suffix)
	if err != nil {
		return nil, nil, err
	}

	if injecting {
		req := api.APIKeyCreationRequest{
			ApiKey:    &in.ApiKey,
			ExpiresAt: expiresAt,
			ExpiresIn: creationExpiresIn(expiresIn),
		}
		resp, err := ops.Keys.Update(in.ID, in.KeyName, req, caller, correlationID, log)
		if err != nil {
			log.Error("MCP API key update failed",
				slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
			return nil, nil, keyOpError("update the API key on", ops.Kind, in.ID, err)
		}
		// No plaintext notice here: UpdateAPIKey returns the masked value, and
		// the caller supplied the key in the first place.
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
			"result": resp,
		}, nil
	}

	req := api.APIKeyRegenerationRequest{
		ExpiresAt: expiresAt,
		ExpiresIn: regenerationExpiresIn(expiresIn),
	}
	resp, err := ops.Keys.Rotate(in.ID, in.KeyName, req, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key regeneration failed",
			slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
		return nil, nil, keyOpError("rotate the API key on", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"status": "success", "operation": "regenerate",
		"kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
		"result": resp,
		"notice": "The previous key value is now invalid. The plaintext key in this response is " +
			"shown once and cannot be retrieved again.",
	}, nil
}

func (h *McpHandler) revokeAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in revokeKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	if !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to revoke API key %q on %s %q: confirm the details with the user, then retry with confirm=true",
			in.KeyName, in.Kind, in.ID)
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, toolRevokeAPIKey, http.MethodDelete, "/{apiKeyName}")
	if err != nil {
		return nil, nil, err
	}

	resp, err := ops.Keys.Revoke(in.ID, in.KeyName, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key revocation failed",
			slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
		return nil, nil, keyOpError("revoke the API key on", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"status": "success", "kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
		"result": resp,
	}, nil
}

// keyOpError maps an APIKeyService failure to a message for the model.
//
// It reads the service's sentinels rather than copying any of the three
// different string-matching schemes the REST api-key handlers grew.
// A "not found" is echoed because the caller
// already proved they may read this collection, so confirming the parent exists
// leaks nothing they could not learn from list_resources. Anything else stays
// generic, with the real error in the log only.
func keyOpError(action, kind, id string, err error) error {
	switch {
	case storage.IsNotFoundError(err):
		return fmt.Errorf("cannot %s %s %q: no such resource, or no such key on it", action, kind, id)
	case storage.IsConflictError(err):
		return fmt.Errorf("cannot %s %s %q: a key with that name already exists, or the supplied "+
			"apiKey value is already in use by another key on this Gateway", action, kind, id)
	case storage.IsOperationNotAllowedError(err):
		// In practice this is only reachable from the injection path, where the
		// target key was generated by this Gateway rather than supplied from
		// outside. Say what to do instead: without an explanation the model has
		// no way to recover, and the retry it needs is one argument away.
		return fmt.Errorf("cannot %s %s %q: a key value can only be installed over a key whose value "+
			"was supplied rather than generated (issued with \"apiKey\", or issued elsewhere). This key's "+
			"value was generated by the Gateway — call the same tool again without \"apiKey\" to generate "+
			"a replacement value instead, or revoke it and issue a new key with \"apiKey\"", action, kind, id)
	case errors.Is(err, utils.ErrAPIKeyExpirationInPast), errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit):
		// Mirrors the REST api-key handlers, which return these as 400 with the
		// service's message: it names the expiry the caller sent.
		return fmt.Errorf("cannot %s %s %q: %v", action, kind, id, err)
	default:
		// Covers the authorization refusal from canRegenerateAPIKey, which
		// wraps no sentinel.
		return fmt.Errorf("failed to %s %s %q: the Gateway refused the operation. Check the "+
			"arguments first; the key may also belong to another user, or the key quota may be "+
			"exhausted", action, kind, id)
	}
}
