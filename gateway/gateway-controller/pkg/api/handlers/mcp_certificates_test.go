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
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The expected route keys below are written by hand rather than read from
// certActions, so this test compares the dispatch table against the management
// routes as spelled in cmd/controller/main.go rather than against itself.
func TestResolveCertAction(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		wantAction   string
		wantRouteKey string
		wantMutating bool
		wantConfirm  bool
	}{
		{"list", "list", certActionList, "GET /certificates", false, false},
		{"apply", "apply", certActionApply, "POST /certificates", true, false},
		{"delete", "delete", certActionDelete, "DELETE /certificates/{id}", true, true},
		{"reload", "reload", certActionReload, "POST /certificates/reload", true, true},

		// Alias and spelling tolerance: a model reaching for a different verb,
		// or a different casing, must land on the same operation.
		{"upload aliases to apply", "upload", certActionApply, "POST /certificates", true, false},
		{"create aliases to apply", "create", certActionApply, "POST /certificates", true, false},
		{"remove aliases to delete", "remove", certActionDelete, "DELETE /certificates/{id}", true, true},
		{"refresh aliases to reload", "refresh", certActionReload, "POST /certificates/reload", true, true},
		{"upper case", "LIST", certActionList, "GET /certificates", false, false},
		{"surrounding space", "  Apply  ", certActionApply, "POST /certificates", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := resolveCertAction(tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAction, op.Action)
			assert.Equal(t, tt.wantRouteKey, op.RouteKey)
			assert.Equal(t, tt.wantMutating, op.Mutating)
			assert.Equal(t, tt.wantConfirm, op.NeedsConfirm)
		})
	}
}

func TestResolveCertActionRejectsUnknown(t *testing.T) {
	for _, action := range []string{"", "get", "rotate", "drop"} {
		_, err := resolveCertAction(action)
		require.Error(t, err, "action %q should not resolve", action)
		assert.Contains(t, err.Error(), "list, apply, delete, reload",
			"the rejection should name the actions that would have worked")
	}
}

// The authorization gate and the tool must derive the same route key from the
// same arguments; if they can disagree, the gate authorizes one operation while
// the handler performs another.
func TestCertificateGateAgreesWithHandler(t *testing.T) {
	h := &McpHandler{}

	for _, action := range []string{"list", "apply", "delete", "reload", "UPLOAD"} {
		t.Run(action, func(t *testing.T) {
			args, err := json.Marshal(manageCertificatesInput{Action: action, ID: "cert-1"})
			require.NoError(t, err)

			gateKeys, ok := h.routeKeysForCall("wso2_apip_gw_manage_certificates", args)
			require.True(t, ok, "the gate must be able to map a resolvable call")
			require.Len(t, gateKeys, 1, "a certificate call maps to exactly one route key")

			op, err := resolveCertAction(action)
			require.NoError(t, err)
			assert.Equal(t, op.RouteKey, gateKeys[0])
		})
	}
}

// An unresolvable action is passed through rather than authorized, so the tool
// itself can explain what was wrong — the same contract an unknown kind has.
func TestCertificateGatePassesThroughUnresolvableAction(t *testing.T) {
	h := &McpHandler{}
	args, err := json.Marshal(manageCertificatesInput{Action: "sideways"})
	require.NoError(t, err)

	_, ok := h.routeKeysForCall("wso2_apip_gw_manage_certificates", args)
	assert.False(t, ok)
}

func TestManageCertificatesGuards(t *testing.T) {
	// Skipped exempts the role check, which is what lets these cases run
	// without a role map; every guard asserted here sits outside that check.
	authorized := withMcpCaller(context.Background(), mcpCaller{Skipped: true})

	t.Run("immutable mode refuses every mutating action", func(t *testing.T) {
		h := &McpHandler{immutable: true, logger: slog.Default()}
		for _, action := range []string{"apply", "delete", "reload"} {
			_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
				Action: action, Confirm: true, ID: "cert-1", Name: "n", Certificate: "pem",
			})
			require.Errorf(t, err, "%s should be refused in immutable mode", action)
			assert.Contains(t, err.Error(), "immutable mode")
		}
	})

	t.Run("delete and reload require confirm", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		for _, action := range []string{"delete", "reload"} {
			_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
				Action: action, ID: "cert-1",
			})
			require.Errorf(t, err, "%s should require confirm", action)
			assert.Contains(t, err.Error(), "confirm=true")
		}
	})

	t.Run("apply rejects an id rather than creating a duplicate", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "apply", ID: "cert-1", Name: "n", Certificate: "pem",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be updated")
	})

	t.Run("apply requires name and certificate", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "apply", Name: "n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate")
	})

	t.Run("delete requires an id", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "delete", Confirm: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action=list")
	})
}

// A tool handler reached without an authorization decision on the context must
// deny: its absence is what proves the gate ran.
func TestManageCertificatesDeniesWithoutGate(t *testing.T) {
	h := &McpHandler{logger: slog.Default()}
	_, _, err := h.manageCertificates(context.Background(), nil, manageCertificatesInput{Action: "list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}
