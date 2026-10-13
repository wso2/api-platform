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

package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// transformArtifactConfig applies the three config-blob transforms an artifact
// migration needs, operating on the decoded JSON so every non-touched field is
// preserved (v2 re-parses the blob into its typed config on read, so the values
// are what matter — §B.1):
//   - §B.10 inline-secret externalization on the kind's credential path(s),
//     emitting secrets / secret_scopes / artifact_secret_refs rows via q;
//   - §B.11 LLM policy split (LlmProvider / LlmProxy only);
//   - transport-column fold-in for RestApi / WebSubApi / WebBrokerApi.
//
// q is the target tx (nil in dry-run: the transform + ciphertext are still
// computed for validation but no rows are written). ts is the artifact's
// (already zone-converted) created_at, used for deterministic secret ids.
func (k *Kernels) transformArtifactConfig(ctx context.Context, q queryer, kind, artifactUUID, orgUUID, createdBy string,
	ts time.Time, blob []byte, transportCol string) ([]byte, error) {

	if len(blob) == 0 {
		blob = []byte("{}")
	}
	var cfg map[string]any
	if err := json.Unmarshal(blob, &cfg); err != nil {
		return nil, fmt.Errorf("decode config for %s/%s: %w", kind, artifactUUID, err)
	}

	// 1. Inline-secret externalization on the credential path(s) for this kind.
	switch kind {
	case constants.RestApi, constants.MCPProxy, constants.LLMProvider:
		if err := k.externalizeAuthAt(ctx, q, cfg, artifactUUID, orgUUID, createdBy, ts,
			[]string{"upstream", "main", "auth"}, "upstream.main.auth.value"); err != nil {
			return nil, err
		}
		if err := k.externalizeAuthAt(ctx, q, cfg, artifactUUID, orgUUID, createdBy, ts,
			[]string{"upstream", "sandbox", "auth"}, "upstream.sandbox.auth.value"); err != nil {
			return nil, err
		}
	case constants.LLMProxy:
		if err := k.externalizeAuthAt(ctx, q, cfg, artifactUUID, orgUUID, createdBy, ts,
			[]string{"upstreamAuth"}, "upstreamAuth.value"); err != nil {
			return nil, err
		}
	}
	// websub_apis / webbroker_apis: the eventgateway plugin externalizes NOTHING
	// (§B.10) — no secret handling here.

	// 2. LLM policy split (LlmProvider / LlmProxy only, §B.11).
	if kind == constants.LLMProvider || kind == constants.LLMProxy {
		splitLegacyPoliciesInPlace(cfg)
	}

	// 3. Transport-column fold-in (RestApi / WebSubApi / WebBrokerApi) — v1 stored
	// transport as a separate column (JSON-array-as-TEXT); v2 keeps it inside the
	// config as `transport: []string`.
	if transportCol != "" {
		if _, present := cfg["transport"]; !present {
			cfg["transport"] = parseTransport(transportCol)
		}
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("re-marshal config for %s/%s: %w", kind, artifactUUID, err)
	}
	return out, nil
}

// addManagedBy re-marshals a llm_provider_templates config blob with `managedBy`
// injected — the sole payload diff v2 makes to a template config (§ templates §6).
func addManagedBy(blob []byte, managedBy string) ([]byte, error) {
	if len(blob) == 0 {
		blob = []byte("{}")
	}
	var cfg map[string]any
	if err := json.Unmarshal(blob, &cfg); err != nil {
		return nil, fmt.Errorf("decode template config: %w", err)
	}
	cfg["managedBy"] = managedBy
	return json.Marshal(cfg)
}

// externalizeAuthAt externalizes the credential at cfg[path...]["value"] if it is
// a real, non-placeholder credential whose auth type is not credential-less. It
// mutates cfg in place (value -> `{{ secret "handle" }}`) and, when q != nil,
// writes the secrets / secret_scopes / artifact_secret_refs rows.
func (k *Kernels) externalizeAuthAt(ctx context.Context, q queryer, cfg map[string]any,
	artifactUUID, orgUUID, createdBy string, ts time.Time, path []string, fieldKey string) error {

	auth := navigateMap(cfg, path)
	if auth == nil {
		return nil
	}
	value, _ := auth["value"].(string)
	if value == "" {
		return nil
	}
	authType, _ := auth["type"].(string)
	if isCredentialLessUpstreamAuthType(authType) {
		return nil
	}
	if constants.SecretPlaceholderRe.MatchString(value) {
		return nil // already externalized
	}
	if k.vault == nil {
		// Only reachable in a source-only dry-run without a key — cannot encrypt.
		k.log.Warn("no vault key — skipping secret externalization (dry-run)", "artifact", artifactUUID, "field", fieldKey)
		return nil
	}

	handle := secretHandle(artifactUUID, fieldKey, ts)
	uuid := secretUUID(artifactUUID, fieldKey, ts)
	ciphertext, err := k.vault.Encrypt(ctx, value)
	if err != nil {
		return fmt.Errorf("encrypt secret %s/%s: %w", artifactUUID, fieldKey, err)
	}
	hash := secretHash(k.vault.HashKey(), value)

	// Rewrite the config value to the placeholder BEFORE the blob is marshalled.
	auth["value"] = fmt.Sprintf(`{{ secret "%s" }}`, handle)

	if q == nil {
		return nil // dry-run: computed + validated, not persisted
	}

	// secrets row (data_version is a DB default — omitted). description is ""
	// rather than NULL: v2 scans it into a plain string, and database/sql
	// refuses NULL there, which made every migrated secret unreadable.
	if err := insertRow(ctx, q, "secrets",
		[]string{"uuid", "organization_uuid", "handle", "display_name", "description", "ciphertext", "hash",
			"type", "provider", "status", "created_at", "created_by", "updated_at", "updated_by"},
		[]any{uuid, orgUUID, handle, handle /* display_name */, "" /* description */, ciphertext, hash,
			model.SecretTypeGeneric, model.SecretProviderInHouse, model.SecretStatusActive,
			ts, createdBy, ts, createdBy},
		conflictUUIDNothing); err != nil {
		return err
	}
	// secret_scopes: one org-scope row (the BFF default scope).
	if err := insertRow(ctx, q, "secret_scopes",
		[]string{"secret_uuid", "scope", "scope_value"},
		[]any{uuid, model.SecretScopeTypeOrg, orgUUID},
		conflictCols("secret_uuid", "scope", "scope_value")); err != nil {
		return err
	}
	// artifact_secret_refs: artifact-level ref (gateway_id = '').
	if err := insertRow(ctx, q, "artifact_secret_refs",
		[]string{"organization_uuid", "artifact_uuid", "secret_handle", "gateway_id"},
		[]any{orgUUID, artifactUUID, handle, ""},
		conflictCols("organization_uuid", "artifact_uuid", "secret_handle", "gateway_id")); err != nil {
		return err
	}
	return nil
}

// isCredentialLessUpstreamAuthType mirrors v2's skip set: auth types that carry
// no credential (None / Other, case/-/_-insensitive) — do not externalize them.
func isCredentialLessUpstreamAuthType(t string) bool {
	n := strings.ToLower(strings.TrimSpace(t))
	n = strings.ReplaceAll(n, "-", "")
	n = strings.ReplaceAll(n, "_", "")
	return n == "none" || n == "other"
}

// navigateMap walks a nested map by keys, returning the final map or nil if any
// hop is missing or not an object.
func navigateMap(m map[string]any, path []string) map[string]any {
	cur := m
	for _, key := range path {
		next, ok := cur[key].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

// splitLegacyPoliciesInPlace folds a flat `policies[]` into `globalPolicies` /
// `operationPolicies` and clears `policies`, reproducing v2
// service/llm.go:migrateLegacyPolicies (§B.11):
//   - a path entry with path=="/*" AND methods==["*"] -> globalPolicies (dedup by name)
//   - any other path entry -> operationPolicies (merged by name+version)
func splitLegacyPoliciesInPlace(cfg map[string]any) {
	raw, ok := cfg["policies"].([]any)
	if !ok || len(raw) == 0 {
		delete(cfg, "policies")
		return
	}

	var global []any
	globalSeen := map[string]bool{}
	var operation []any
	opIndex := map[string]int{} // name\x00version -> index in operation

	for _, p := range raw {
		pol, ok := p.(map[string]any)
		if !ok {
			continue
		}
		name, _ := pol["name"].(string)
		version, _ := pol["version"].(string)
		paths, _ := pol["paths"].([]any)

		for _, pe := range paths {
			pathEntry, ok := pe.(map[string]any)
			if !ok {
				continue
			}
			pathStr, _ := pathEntry["path"].(string)
			if pathStr == "/*" && methodsAreWildcardOnly(pathEntry["methods"]) {
				if !globalSeen[name] {
					globalSeen[name] = true
					gp := map[string]any{"name": name, "version": version}
					if params, ok := pathEntry["params"]; ok {
						gp["params"] = params
					}
					if ec, ok := pol["executionCondition"]; ok {
						gp["executionCondition"] = ec
					}
					global = append(global, gp)
				}
				continue
			}
			// operation policy: merge by name+version, accumulating path entries.
			key := name + "\x00" + version
			if idx, exists := opIndex[key]; exists {
				op := operation[idx].(map[string]any)
				op["paths"] = append(op["paths"].([]any), pathEntry)
			} else {
				opIndex[key] = len(operation)
				op := map[string]any{"name": name, "version": version, "paths": []any{pathEntry}}
				if ec, ok := pol["executionCondition"]; ok {
					op["executionCondition"] = ec
				}
				operation = append(operation, op)
			}
		}
	}

	if len(global) > 0 {
		cfg["globalPolicies"] = global
	}
	if len(operation) > 0 {
		cfg["operationPolicies"] = operation
	}
	delete(cfg, "policies")
}

func methodsAreWildcardOnly(v any) bool {
	methods, ok := v.([]any)
	if !ok || len(methods) != 1 {
		return false
	}
	s, ok := methods[0].(string)
	return ok && s == "*"
}

// parseTransport turns a v1 transport column (JSON-array-as-TEXT, or a bare
// value) into the v2 config's []string transport list.
func parseTransport(col string) []string {
	col = strings.TrimSpace(col)
	if col == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(col), &arr); err == nil {
		return arr
	}
	// Fallback: comma-separated or a single bare value.
	parts := strings.Split(col, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
