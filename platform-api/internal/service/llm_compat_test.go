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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"gopkg.in/yaml.v3"
)

// Backward-compatibility guard for multi-provider proxy support.
//
// Every fixture here is a proxy in the *legacy* shape — the only shape that
// exists in a customer database today. The goldens were generated from the
// pre-change build, so this test encodes the behaviour customers have
// now, not the behaviour this feature introduces.
//
// The two comparisons are deliberately different, because the two invariants
// are different:
//
//   - The **response** must stay valid for a client written against today's
//     contract. That is a superset rule, not equality: the change adds
//     `providers` to every read, so the response legitimately grows.
//     Every field the golden carries must still be present and equal.
//
//   - The **artefact** must keep describing exactly the same deployment,
// but the provider shape is now emitted canonically and the
//     frozen gateway rejects a payload carrying both shapes at once. Byte
//     equality is therefore impossible for the provider block alone. Both
//     documents are canonicalised to one attachment list and then compared in
//     full, so every other byte is still held to equality.

const compatFixtureDir = "testdata/compat"

type compatFixture struct {
	name string
	file string
}

func compatFixtures() []compatFixture {
	return []compatFixture{
		{name: "legacy-single-provider", file: "legacy-single-provider.json"},
		{name: "legacy-additional-providers", file: "legacy-additional-providers.json"},
		{name: "legacy-provider-aliases", file: "legacy-provider-aliases.json"},
	}
}

// TestLLMProxyLegacyShapeCompatibility creates, reads and deploys each legacy
// fixture and holds the result against the committed goldens.
//
// Regenerate the goldens with:
//
//	UPDATE_COMPAT_GOLDEN=1 go test ./internal/service -run TestLLMProxyLegacyShapeCompatibility
//
// Regenerating is correct only when the change is an intended, reviewed one —
// the whole point of the goldens is that they came from the pre-change build.
func TestLLMProxyLegacyShapeCompatibility(t *testing.T) {
	for _, fixture := range compatFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			request := loadCompatFixture(t, fixture.file)

			response, stored := createProxyForCompat(t, request)

			artifact, err := generateLLMProxyDeploymentYAML(stored)
			if err != nil {
				t.Fatalf("failed to generate deployment artefact: %v", err)
			}

			responseJSON := marshalIndentedJSON(t, response)
			artifactYAML := marshalYAML(t, artifact)

			if updateCompatGolden() {
				writeGolden(t, fixture.name+".response.json", responseJSON)
				writeGolden(t, fixture.name+".artifact.yaml", artifactYAML)
				return
			}

			assertResponseSupersetOfGolden(t, fixture.name+".response.json", responseJSON)
			assertArtifactMatchesGolden(t, fixture.name+".artifact.yaml", artifactYAML)
		})
	}
}

// createProxyForCompat runs a fixture through the real Create path against
// in-memory repositories, returning the API response and the model that was
// handed to the repository for persistence.
func createProxyForCompat(t *testing.T, request *api.LLMProxy) (*api.LLMProxy, *model.LLMProxy) {
	t.Helper()

	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		return proxyRepo.created, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{
				UUID:        providerID + "-uuid",
				ID:          providerID,
				OpenAPISpec: "openapi: 3.0.3\ninfo:\n  title: Compat\n  version: v1.0\npaths: {}\n",
			}, nil
		},
	}
	projectRepo := &mockProjectRepo{project: &model.Project{
		ID:             "project-1",
		Handle:         "project-1",
		OrganizationID: "org-1",
	}}
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo, nil, nil, nil,
		slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	response, err := service.Create("org-1", "compat-user", request)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	if proxyRepo.created == nil {
		t.Fatal("expected the proxy to reach the repository")
	}
	return response, proxyRepo.created
}

// assertResponseSupersetOfGolden holds the response to its compatibility rule:
// every field the
// golden carries must still be present and equal. Fields added since the golden
// was generated are the feature working as specified and pass.
func assertResponseSupersetOfGolden(t *testing.T, goldenName string, actual []byte) {
	t.Helper()

	var want, got any
	if err := json.Unmarshal(readGolden(t, goldenName), &want); err != nil {
		t.Fatalf("failed to parse golden %s: %v", goldenName, err)
	}
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if diff := missingFromSuperset("$", want, got); diff != "" {
		t.Fatalf("response no longer satisfies a client written against today's contract:\n%s\n\ngolden %s\nactual:\n%s",
			diff, goldenName, actual)
	}
}

// assertArtifactMatchesGolden holds the artefact to its compatibility rule: the
// deployment it describes must be unchanged, with the provider shape free to
// move from legacy to canonical. Both sides are canonicalised to one attachment
// list, so every byte outside the provider block is still compared for equality.
func assertArtifactMatchesGolden(t *testing.T, goldenName string, actual []byte) {
	t.Helper()

	want := canonicaliseArtifactShape(t, readGolden(t, goldenName))
	got := canonicaliseArtifactShape(t, actual)

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("deployment artefact changed:\nwant (golden %s, canonicalised):\n%s\ngot (canonicalised):\n%s\nraw actual:\n%s",
			goldenName, marshalYAML(t, want), marshalYAML(t, got), actual)
	}
}

// canonicaliseArtifactShape rewrites whichever provider shape a document uses
// into one `providers` list, so a legacy golden and a canonical artefact that
// describe the same attachments compare equal. Nothing else is touched.
func canonicaliseArtifactShape(t *testing.T, document []byte) map[string]any {
	t.Helper()

	var parsed map[string]any
	if err := yaml.Unmarshal(document, &parsed); err != nil {
		t.Fatalf("failed to parse artefact: %v", err)
	}
	spec, _ := parsed["spec"].(map[string]any)
	if spec == nil {
		return parsed
	}

	entries := make([]map[string]any, 0, 4)
	if providers, ok := spec["providers"].([]any); ok {
		for _, entry := range providers {
			if mapped, ok := entry.(map[string]any); ok {
				entries = append(entries, canonicaliseAttachment(mapped, mapped["isPrimary"] == true))
			}
		}
	} else {
		if primary, ok := spec["provider"].(map[string]any); ok {
			entries = append(entries, canonicaliseAttachment(primary, true))
		}
		if additional, ok := spec["additionalProviders"].([]any); ok {
			for _, entry := range additional {
				if mapped, ok := entry.(map[string]any); ok {
					entries = append(entries, canonicaliseAttachment(mapped, false))
				}
			}
		}
	}

	delete(spec, "provider")
	delete(spec, "additionalProviders")
	if len(entries) > 0 {
		// Primary first, then declaration order — the same ordering rule the
		// gateway's own normaliser applies (pkg/models/llm_proxy_attachments.go).
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i]["isPrimary"] == true && entries[j]["isPrimary"] != true
		})
		asAny := make([]any, 0, len(entries))
		for _, entry := range entries {
			asAny = append(asAny, entry)
		}
		spec["providers"] = asAny
	}
	return parsed
}

// canonicaliseAttachment maps one attachment, in either shape, onto the
// canonical field names. `as` and `alias` are the same thing under two names,
// and an absent alias means "defaults to the id", so an entry that
// names its alias explicitly and one that leaves it out are the same attachment.
func canonicaliseAttachment(in map[string]any, isPrimary bool) map[string]any {
	out := map[string]any{"isPrimary": isPrimary}
	for key, value := range in {
		switch key {
		case "as", "alias":
			out["alias"] = value
		case "isPrimary":
		default:
			out[key] = value
		}
	}
	if alias, ok := out["alias"]; !ok || alias == "" || alias == nil {
		out["alias"] = out["id"]
	}
	return out
}

// missingFromSuperset reports the first place `want` is not contained in `got`,
// recursing through objects and comparing arrays element-wise.
func missingFromSuperset(path string, want, got any) string {
	switch wantTyped := want.(type) {
	case map[string]any:
		gotTyped, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: expected an object, got %T", path, got)
		}
		keys := make([]string, 0, len(wantTyped))
		for key := range wantTyped {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			gotValue, present := gotTyped[key]
			if !present {
				return fmt.Sprintf("%s.%s: missing from the response", path, key)
			}
			if diff := missingFromSuperset(path+"."+key, wantTyped[key], gotValue); diff != "" {
				return diff
			}
		}
		return ""
	case []any:
		gotTyped, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: expected an array, got %T", path, got)
		}
		if len(gotTyped) != len(wantTyped) {
			return fmt.Sprintf("%s: expected %d entries, got %d", path, len(wantTyped), len(gotTyped))
		}
		for i := range wantTyped {
			if diff := missingFromSuperset(fmt.Sprintf("%s[%d]", path, i), wantTyped[i], gotTyped[i]); diff != "" {
				return diff
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(want, got) {
			return fmt.Sprintf("%s: expected %#v, got %#v", path, want, got)
		}
		return ""
	}
}

func loadCompatFixture(t *testing.T, name string) *api.LLMProxy {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(compatFixtureDir, name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	var request api.LLMProxy
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", name, err)
	}
	return &request
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(compatFixtureDir, "golden", name))
	if err != nil {
		t.Fatalf("failed to read golden %s (generate it with UPDATE_COMPAT_GOLDEN=1 against the pre-change build): %v", name, err)
	}
	return raw
}

func writeGolden(t *testing.T, name string, content []byte) {
	t.Helper()

	path := filepath.Join(compatFixtureDir, "golden", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create golden directory: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write golden %s: %v", name, err)
	}
	t.Logf("wrote golden %s", path)
}

func marshalIndentedJSON(t *testing.T, value any) []byte {
	t.Helper()

	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return append(out, '\n')
}

func marshalYAML(t *testing.T, value any) []byte {
	t.Helper()

	out, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}
	return out
}

// updateCompatGolden is an environment variable rather than a flag, so this
// file registers nothing on the package's shared test binary.
func updateCompatGolden() bool {
	return strings.TrimSpace(os.Getenv("UPDATE_COMPAT_GOLDEN")) != ""
}
