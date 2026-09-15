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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// routeKey builds the REST route key for a CRUD operation on a kind, in the
// relative form generateAuthConfig stores ("POST /llm-providers").
func routeKey(method string, ops *kindOps, item bool) string {
	if item {
		return method + " " + ops.Collection + "/{id}"
	}
	return method + " " + ops.Collection
}

// keyRouteKey builds the route key for an api-key sub-resource. suffix is ""
// for the collection, "/{apiKeyName}" for one key, or
// "/{apiKeyName}/regenerate" for rotation. Placeholder spellings must match
// generateAuthConfig exactly.
func keyRouteKey(method string, ops *kindOps, suffix string) string {
	return method + " " + ops.Collection + "/{id}/api-keys" + suffix
}

// apiKeyRouteSuffixes is every api-key route an MCP tool can reach, paired
// with its method. MCPRouteKeys uses it so the advertised scope set covers
// these routes.
var apiKeyRouteSuffixes = []struct {
	Method string
	Suffix string
}{
	{http.MethodPost, ""},
	{http.MethodGet, ""},
	{http.MethodPost, "/{apiKeyName}/regenerate"},
	{http.MethodPut, "/{apiKeyName}"},
	{http.MethodDelete, "/{apiKeyName}"},
}

// Certificate actions, canonical spellings.
const (
	certActionList   = "list"
	certActionApply  = "apply"
	certActionDelete = "delete"
	certActionReload = "reload"
)

// certAction is the REST operation one certificate action performs.
type certAction struct {
	Action string
	// RouteKey is the REST route authorized for this action.
	RouteKey string
	// Mutating actions are refused in immutable mode.
	Mutating bool
	// NeedsConfirm actions require confirm=true.
	NeedsConfirm bool
}

var certActions = map[string]certAction{
	certActionList:   {Action: certActionList, RouteKey: "GET /certificates"},
	certActionApply:  {Action: certActionApply, RouteKey: "POST /certificates", Mutating: true},
	certActionDelete: {Action: certActionDelete, RouteKey: "DELETE /certificates/{id}", Mutating: true, NeedsConfirm: true},
	certActionReload: {Action: certActionReload, RouteKey: "POST /certificates/reload", Mutating: true, NeedsConfirm: true},
}

// certActionAliases resolves an accepted spelling to its canonical action.
var certActionAliases = map[string]string{
	"list":    certActionList,
	"apply":   certActionApply,
	"upload":  certActionApply,
	"create":  certActionApply,
	"add":     certActionApply,
	"delete":  certActionDelete,
	"remove":  certActionDelete,
	"reload":  certActionReload,
	"refresh": certActionReload,
}

// resolveCertAction maps the certificate tool's action argument to the REST
// operation it performs.
func resolveCertAction(rawAction string) (certAction, error) {
	canonical, ok := certActionAliases[normalizeToolWord(rawAction)]
	if !ok {
		return certAction{}, fmt.Errorf(
			"unknown action %q; wso2_apip_gw_manage_certificates accepts: list, apply, delete, reload",
			rawAction)
	}
	return certActions[canonical], nil
}

// Subscription types and actions, canonical spellings.
const (
	subTypeSubscription = "Subscription"
	subTypePlan         = "SubscriptionPlan"

	subActionList   = "list"
	subActionGet    = "get"
	subActionApply  = "apply"
	subActionDelete = "delete"
)

// subAction is the REST operation one (type, action, id-presence) combination
// of the subscription tool performs.
type subAction struct {
	Type         string
	Action       string
	RouteKey     string
	Mutating     bool
	NeedsID      bool
	NeedsConfirm bool
}

// subCollections holds each type's collection path and item placeholder. The
// placeholders differ ({subscriptionId} vs {planId}), which is why this is a
// table rather than a shared "/{id}" suffix.
var subCollections = map[string]struct {
	Collection  string
	Placeholder string
}{
	subTypeSubscription: {Collection: "/subscriptions", Placeholder: "{subscriptionId}"},
	subTypePlan:         {Collection: "/subscription-plans", Placeholder: "{planId}"},
}

// subTypeAliases resolves an accepted type spelling to its canonical value.
var subTypeAliases = map[string]string{
	"subscription":     subTypeSubscription,
	"subscriptions":    subTypeSubscription,
	"subscriptionplan": subTypePlan,
	"plan":             subTypePlan,
	"plans":            subTypePlan,
}

// subActionAliases resolves an accepted action spelling to its canonical value.
var subActionAliases = map[string]string{
	"list":   subActionList,
	"get":    subActionGet,
	"read":   subActionGet,
	"apply":  subActionApply,
	"create": subActionApply,
	"update": subActionApply,
	"delete": subActionDelete,
	"remove": subActionDelete,
}

// resolveSubscriptionAction maps the subscription tool's arguments to the REST
// operation they perform. apply is PUT when an id is supplied and POST otherwise.
func resolveSubscriptionAction(rawType, rawAction, id string) (subAction, error) {
	resourceType, ok := subTypeAliases[normalizeToolWord(rawType)]
	if !ok {
		return subAction{}, fmt.Errorf(
			"unknown type %q; wso2_apip_gw_manage_subscriptions accepts: %s, %s",
			rawType, subTypeSubscription, subTypePlan)
	}
	action, ok := subActionAliases[normalizeToolWord(rawAction)]
	if !ok {
		return subAction{}, fmt.Errorf(
			"unknown action %q; wso2_apip_gw_manage_subscriptions accepts: list, get, apply, delete",
			rawAction)
	}

	// Item routes carry the type's own placeholder.
	paths := subCollections[resourceType]
	item := paths.Collection + "/" + paths.Placeholder
	op := subAction{Type: resourceType, Action: action}

	switch action {
	case subActionList:
		op.RouteKey = http.MethodGet + " " + paths.Collection
	case subActionGet:
		op.RouteKey = http.MethodGet + " " + item
		op.NeedsID = true
	case subActionDelete:
		op.RouteKey = http.MethodDelete + " " + item
		op.Mutating, op.NeedsID, op.NeedsConfirm = true, true, true
	default: // apply
		op.Mutating = true
		if strings.TrimSpace(id) != "" {
			op.RouteKey = http.MethodPut + " " + item
		} else {
			op.RouteKey = http.MethodPost + " " + paths.Collection
		}
	}

	return op, nil
}

// normalizeToolWord folds an action or type argument to the form the alias
// maps are keyed on, so "SubscriptionPlan", "subscription_plan" and "plan" all
// match. Shares kindSeparatorStripper with normalizeKind.
func normalizeToolWord(raw string) string {
	return strings.ToLower(kindSeparatorStripper.Replace(strings.TrimSpace(raw)))
}

// MCPCertificateRouteKeys is every route key the certificate tool can reach,
// read from the dispatch table. Exported so cmd/controller can assert each key
// exists in the role map.
func MCPCertificateRouteKeys() []string {
	keys := make([]string, 0, len(certActions))
	for _, op := range certActions {
		keys = append(keys, op.RouteKey)
	}
	sort.Strings(keys)
	return keys
}

// MCPSubscriptionRouteKeys is every route key the subscription tool can reach,
// produced by resolveSubscriptionAction exactly as a real call would.
func MCPSubscriptionRouteKeys() []string {
	var keys []string
	for resourceType := range subCollections {
		for _, action := range []string{subActionList, subActionGet, subActionDelete} {
			if op, err := resolveSubscriptionAction(resourceType, action, ""); err == nil {
				keys = append(keys, op.RouteKey)
			}
		}
		// apply reaches two routes, chosen by whether an id was supplied.
		for _, id := range []string{"", "an-id"} {
			if op, err := resolveSubscriptionAction(resourceType, subActionApply, id); err == nil {
				keys = append(keys, op.RouteKey)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// certAndSubscriptionRouteKeys returns the certificate and subscription route
// keys for the tools this handler actually registered.
func (h *McpHandler) certAndSubscriptionRouteKeys() []string {
	var keys []string
	if h.certificateService != nil {
		keys = append(keys, MCPCertificateRouteKeys()...)
	}
	if h.subscriptionService != nil {
		keys = append(keys, MCPSubscriptionRouteKeys()...)
	}
	return keys
}

// MCPRouteKeys is every route key a registered tool can reach, sorted and
// deduplicated. Kinds absent from the registry and tools without a service
// contribute nothing, so it describes this gateway as configured.
func (h *McpHandler) MCPRouteKeys() []string {
	seen := map[string]struct{}{}
	add := func(keys ...string) {
		for _, k := range keys {
			seen[k] = struct{}{}
		}
	}

	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			for _, item := range []bool{false, true} {
				add(routeKey(m, ops, item))
			}
		}
		// Include the api-key routes so their roles reach the advertised set.
		if ops.Keys != nil {
			for _, r := range apiKeyRouteSuffixes {
				add(keyRouteKey(r.Method, ops, r.Suffix))
			}
		}
	}
	add(h.certAndSubscriptionRouteKeys()...)

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MCPBaselineRoles is the union of every role that can call at least one
// tool. main.go uses it as the default advertised scope set and as the
// allow-list for configured entries. It returns local role names; translation
// to IdP scopes happens once, in main.go.
func (h *McpHandler) MCPBaselineRoles() []string {
	seen := map[string]struct{}{}
	for _, key := range h.MCPRouteKeys() {
		if roles, ok := h.authz.rolesFor(key); ok {
			for _, r := range roles {
				seen[r] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// routeKeysForCall maps a tool call to the REST route key(s) its caller's
// roles are checked against. It returns several keys only for
// wso2_apip_gw_list_resources with no kind, where any readable kind admits the
// call. ok=false means the call could not be mapped.
func (h *McpHandler) routeKeysForCall(tool string, args json.RawMessage) ([]string, bool) {
	switch tool {
	case "wso2_apip_gw_deploy_api", "wso2_apip_gw_apply_config":
		var in deployInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "wso2_apip_gw_apply_config" {
			class = classConfig
		}
		env, err := readManifestEnvelope([]byte(in.Yaml))
		if err != nil {
			return nil, false
		}
		ops, err := h.resolveKind(env.Kind, class)
		if err != nil {
			return nil, false
		}
		if in.ID != "" {
			return []string{routeKey(http.MethodPut, ops, true)}, true
		}
		return []string{routeKey(http.MethodPost, ops, false)}, true

	case "wso2_apip_gw_undeploy_api", "wso2_apip_gw_delete_config":
		var in deleteInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "wso2_apip_gw_delete_config" {
			class = classConfig
		}
		ops, err := h.resolveKind(in.Kind, class)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodDelete, ops, true)}, true

	case "wso2_apip_gw_get_resource":
		var in getInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		ops, err := h.resolveKind(in.Kind, classAny)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodGet, ops, true)}, true

	case "wso2_apip_gw_list_resources":
		var in listInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		if in.Kind != "" {
			ops, err := h.resolveKind(in.Kind, classAny)
			if err != nil {
				return nil, false
			}
			return []string{routeKey(http.MethodGet, ops, false)}, true
		}
		keys := make([]string, 0, len(h.kinds))
		for _, kind := range canonicalKinds() {
			if ops, ok := h.kinds[kind]; ok {
				keys = append(keys, routeKey(http.MethodGet, ops, false))
			}
		}
		return keys, true

	case "wso2_apip_gw_issue_api_key":
		var in issueKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodPost, "")

	case "wso2_apip_gw_list_api_keys":
		var in listKeysInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodGet, "")

	case "wso2_apip_gw_rotate_api_key":
		var in rotateKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		// Dispatched through the same helper the tool uses: a caller-supplied
		// key is an injection (PUT), otherwise the Gateway generates one (POST).
		if rotateIsInjection(in) {
			return h.keyRouteKeysFor(in.Kind, http.MethodPut, "/{apiKeyName}")
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodPost, "/{apiKeyName}/regenerate")

	case "wso2_apip_gw_revoke_api_key":
		var in revokeKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodDelete, "/{apiKeyName}")

	// The action-dispatched tools resolve through the same functions their
	// handlers call, so the key authorized is the key that executes.
	case "wso2_apip_gw_manage_certificates":
		var in manageCertificatesInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		op, err := resolveCertAction(in.Action)
		if err != nil {
			return nil, false
		}
		return []string{op.RouteKey}, true

	case "wso2_apip_gw_manage_subscriptions":
		var in manageSubscriptionsInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		op, err := resolveSubscriptionAction(in.Type, in.Action, in.ID)
		if err != nil {
			return nil, false
		}
		return []string{op.RouteKey}, true

	default:
		return nil, false
	}
}

// keyRouteKeysFor resolves a raw kind to its single api-key route key.
// ok=false means the kind is unknown or bears no keys; the tool reports the
// specific error.
func (h *McpHandler) keyRouteKeysFor(rawKind, method, suffix string) ([]string, bool) {
	ops, err := h.resolveKeyBearing(rawKind)
	if err != nil {
		return nil, false
	}
	return []string{keyRouteKey(method, ops, suffix)}, true
}
