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

package policysecret

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
)

// awsAuthenticationSchema is the shape aws-authentication carries: write-only
// parameters directly under the top-level properties.
const awsAuthenticationSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "authenticationType": {"type": "string", "enum": ["system", "iam-user-access-key"]},
    "awsAccessKeyID": {"type": "string", "writeOnly": true, "minLength": 1},
    "awsSecretAccessKey": {"type": "string", "writeOnly": true, "minLength": 1},
    "awsSessionToken": {"type": "string", "writeOnly": true, "minLength": 1},
    "region": {"type": "string"}
  }
}`

// bedrockGuardrailSchema is the shape aws-bedrock-guardrail carries: the same
// credentials nested one object down. A walk that only looked at the top level
// would find none of them.
const bedrockGuardrailSchema = `{
  "type": "object",
  "properties": {
    "guardrailID": {"type": "string"},
    "awsCredentials": {
      "type": "object",
      "properties": {
        "authenticationType": {"type": "string"},
        "awsRoleARN": {"type": "string"},
        "awsAccessKeyID": {"type": "string", "writeOnly": true},
        "awsSecretAccessKey": {"type": "string", "writeOnly": true},
        "awsSessionToken": {"type": "string", "writeOnly": true}
      }
    }
  }
}`

func mustSchema(t *testing.T, raw string) any {
	t.Helper()
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("failed to parse test schema: %v", err)
	}
	return out
}

// sortedPaths renders paths in a stable order. The walk descends a Go map, so
// sibling order is not defined.
func sortedPaths(paths []Path) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		rendered, _ := json.Marshal(path)
		out = append(out, string(rendered))
	}
	sort.Strings(out)
	return out
}

func TestPathsFromParameterSchemaFindsTopLevelDeclarations(t *testing.T) {
	paths := PathsFromParameterSchema(mustSchema(t, awsAuthenticationSchema))

	want := []string{`["awsAccessKeyID"]`, `["awsSecretAccessKey"]`, `["awsSessionToken"]`}
	if got := sortedPaths(paths); !reflect.DeepEqual(got, want) {
		t.Fatalf("write-only paths = %v, want %v", got, want)
	}
}

func TestPathsFromParameterSchemaFindsNestedDeclarations(t *testing.T) {
	paths := PathsFromParameterSchema(mustSchema(t, bedrockGuardrailSchema))

	want := []string{
		`["awsCredentials","awsAccessKeyID"]`,
		`["awsCredentials","awsSecretAccessKey"]`,
		`["awsCredentials","awsSessionToken"]`,
	}
	if got := sortedPaths(paths); !reflect.DeepEqual(got, want) {
		t.Fatalf("write-only paths = %v, want %v", got, want)
	}
}

// Sibling branches must not share a path's backing array. A walk that appended
// in place would report one branch's prefix under another branch's name.
func TestPathsFromParameterSchemaKeepsSiblingBranchesDistinct(t *testing.T) {
	schema := mustSchema(t, `{
      "type": "object",
      "properties": {
        "first":  {"type": "object", "properties": {"secret": {"type": "string", "writeOnly": true}}},
        "second": {"type": "object", "properties": {"secret": {"type": "string", "writeOnly": true}}}
      }
    }`)

	want := []string{`["first","secret"]`, `["second","secret"]`}
	if got := sortedPaths(PathsFromParameterSchema(schema)); !reflect.DeepEqual(got, want) {
		t.Fatalf("write-only paths = %v, want %v", got, want)
	}
}

func TestPathsFromParameterSchemaDescendsArrayItems(t *testing.T) {
	schema := mustSchema(t, `{
      "type": "object",
      "properties": {
        "backends": {
          "type": "array",
          "items": {"type": "object", "properties": {"apiKey": {"type": "string", "writeOnly": true}}}
        }
      }
    }`)

	want := []string{`["backends","[]","apiKey"]`}
	if got := sortedPaths(PathsFromParameterSchema(schema)); !reflect.DeepEqual(got, want) {
		t.Fatalf("write-only paths = %v, want %v", got, want)
	}
}

func TestPathsFromParameterSchemaIgnoresSchemaWithNoDeclarations(t *testing.T) {
	schema := mustSchema(t, `{"type": "object", "properties": {"region": {"type": "string"}}}`)

	if paths := PathsFromParameterSchema(schema); len(paths) != 0 {
		t.Fatalf("write-only paths = %v, want none", paths)
	}
}

func TestBlankEmptiesDeclaredParametersAndKeepsTheKey(t *testing.T) {
	params := map[string]any{
		"region":             "us-east-1",
		"awsAccessKeyID":     "AKIAEXAMPLE",
		"awsSecretAccessKey": "a-stored-secret",
	}
	paths := []Path{{"awsAccessKeyID"}, {"awsSecretAccessKey"}, {"awsSessionToken"}}

	blanked := Blank(params, paths)

	if blanked["region"] != "us-east-1" {
		t.Errorf("region = %v, want it untouched", blanked["region"])
	}
	for _, key := range []string{"awsAccessKeyID", "awsSecretAccessKey"} {
		value, present := blanked[key]
		if !present {
			t.Errorf("%s was dropped; it must stay with an empty value", key)
		}
		if value != "" {
			t.Errorf("%s = %q, want the empty string", key, value)
		}
	}
	if _, present := blanked["awsSessionToken"]; present {
		t.Error("awsSessionToken was added; a parameter the attachment never set must not appear")
	}
}

// Blanking a response must not reach the stored configuration it was mapped
// from. A shallow copy would leave the nested map shared.
func TestBlankDoesNotMutateItsInput(t *testing.T) {
	params := map[string]any{
		"awsCredentials": map[string]any{"awsSecretAccessKey": "a-stored-secret"},
	}
	paths := []Path{{"awsCredentials", "awsSecretAccessKey"}}

	blanked := Blank(params, paths)

	stored := params["awsCredentials"].(map[string]any)
	if stored["awsSecretAccessKey"] != "a-stored-secret" {
		t.Fatalf("stored parameters were mutated: %v", stored)
	}
	returned := blanked["awsCredentials"].(map[string]any)
	if returned["awsSecretAccessKey"] != "" {
		t.Fatalf("nested secret = %v, want the empty string", returned["awsSecretAccessKey"])
	}
}

func TestBlankEmptiesInsideArrayElements(t *testing.T) {
	params := map[string]any{
		"backends": []any{
			map[string]any{"url": "https://one", "apiKey": "one-secret"},
			map[string]any{"url": "https://two", "apiKey": "two-secret"},
		},
	}

	blanked := Blank(params, []Path{{"backends", ArrayMarker, "apiKey"}})

	for i, element := range blanked["backends"].([]any) {
		backend := element.(map[string]any)
		if backend["apiKey"] != "" {
			t.Errorf("backends[%d].apiKey = %v, want the empty string", i, backend["apiKey"])
		}
		if backend["url"] == "" {
			t.Errorf("backends[%d].url was emptied; only declared parameters may be", i)
		}
	}
}

func TestPreserveCarriesStoredValueForwardWhenTheUpdateEchoesTheBlank(t *testing.T) {
	stored := map[string]any{"awsSecretAccessKey": "a-stored-secret", "region": "us-east-1"}
	updated := map[string]any{"awsSecretAccessKey": "", "region": "eu-west-1"}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if updated["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value carried forward", updated["awsSecretAccessKey"])
	}
	if updated["region"] != "eu-west-1" {
		t.Errorf("region = %v, want the update's own value", updated["region"])
	}
}

func TestPreserveCarriesStoredValueForwardWhenTheUpdateOmitsIt(t *testing.T) {
	stored := map[string]any{"awsSecretAccessKey": "a-stored-secret"}
	updated := map[string]any{}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if updated["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value carried forward", updated["awsSecretAccessKey"])
	}
}

func TestPreserveTakesANewlySuppliedValue(t *testing.T) {
	stored := map[string]any{"awsSecretAccessKey": "a-stored-secret"}
	updated := map[string]any{"awsSecretAccessKey": "a-rotated-secret"}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if updated["awsSecretAccessKey"] != "a-rotated-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the newly supplied value", updated["awsSecretAccessKey"])
	}
}

func TestPreserveClearsOnExplicitNull(t *testing.T) {
	stored := map[string]any{"awsSecretAccessKey": "a-stored-secret"}
	updated := map[string]any{"awsSecretAccessKey": nil}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if updated["awsSecretAccessKey"] != "" {
		t.Errorf("awsSecretAccessKey = %v, want it cleared", updated["awsSecretAccessKey"])
	}
}

func TestPreserveRestoresOnlyTheSecretWhenTheUpdateDropsTheBranch(t *testing.T) {
	stored := map[string]any{"awsCredentials": map[string]any{
		"authenticationType": "iam-user-access-key",
		"awsSecretAccessKey": "a-stored-secret",
	}}
	updated := map[string]any{}

	Preserve(stored, updated, []Path{{"awsCredentials", "awsSecretAccessKey"}})

	rebuilt, ok := updated["awsCredentials"].(map[string]any)
	if !ok {
		t.Fatalf("awsCredentials = %v, want the branch rebuilt for the secret", updated["awsCredentials"])
	}
	if rebuilt["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value", rebuilt["awsSecretAccessKey"])
	}
	if _, present := rebuilt["authenticationType"]; present {
		t.Error("authenticationType came back; a non-secret the caller dropped must stay dropped")
	}
}

func TestPreserveLeavesUndeclaredParametersAlone(t *testing.T) {
	stored := map[string]any{"region": "us-east-1"}
	updated := map[string]any{}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if _, present := updated["region"]; present {
		t.Error("region came back; only declared write-only parameters are carried forward")
	}
}

// An edited array has no key to match elements on, so a resized one re-supplies
// its secrets rather than having them attached by position.
func TestPreserveSkipsArraysWhoseLengthChanged(t *testing.T) {
	stored := map[string]any{"backends": []any{
		map[string]any{"apiKey": "one-secret"},
		map[string]any{"apiKey": "two-secret"},
	}}
	updated := map[string]any{"backends": []any{
		map[string]any{"apiKey": ""},
	}}

	Preserve(stored, updated, []Path{{"backends", ArrayMarker, "apiKey"}})

	only := updated["backends"].([]any)[0].(map[string]any)
	if only["apiKey"] != "" {
		t.Errorf("apiKey = %v, want it left empty rather than matched by position", only["apiKey"])
	}
}

func TestPreserveMatchesArrayElementsOfEqualLength(t *testing.T) {
	stored := map[string]any{"backends": []any{
		map[string]any{"apiKey": "one-secret"},
		map[string]any{"apiKey": "two-secret"},
	}}
	updated := map[string]any{"backends": []any{
		map[string]any{"apiKey": ""},
		map[string]any{"apiKey": "two-rotated"},
	}}

	Preserve(stored, updated, []Path{{"backends", ArrayMarker, "apiKey"}})

	elements := updated["backends"].([]any)
	if got := elements[0].(map[string]any)["apiKey"]; got != "one-secret" {
		t.Errorf("backends[0].apiKey = %v, want the stored value", got)
	}
	if got := elements[1].(map[string]any)["apiKey"]; got != "two-rotated" {
		t.Errorf("backends[1].apiKey = %v, want the newly supplied value", got)
	}
}

// Blank and Preserve are each other's inverse over a round trip: what a reader
// is shown, sent back unchanged, must leave the stored value as it was.
func TestBlankThenPreserveRoundTripsToTheStoredValue(t *testing.T) {
	paths := []Path{{"awsSecretAccessKey"}}
	stored := map[string]any{"awsSecretAccessKey": "a-stored-secret", "region": "us-east-1"}

	asRead := Blank(stored, paths)
	asRead["region"] = "eu-west-1" // the edit the caller actually came to make
	Preserve(stored, asRead, paths)

	if asRead["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value after a round trip", asRead["awsSecretAccessKey"])
	}
	if asRead["region"] != "eu-west-1" {
		t.Errorf("region = %v, want the caller's edit", asRead["region"])
	}
}

func TestNopResolverDeclaresNothingWriteOnly(t *testing.T) {
	if paths := Nop().WriteOnlyPaths("aws-authentication", "v0.11.1"); paths != nil {
		t.Fatalf("paths = %v, want none", paths)
	}
}

func TestIndexResolvesByNameAndVersion(t *testing.T) {
	manifest := []byte(`[
      {"name": "aws-authentication", "version": "v0.11.1",
       "writeOnlyParams": [["awsAccessKeyID"], ["awsSecretAccessKey"]]},
      {"name": "cors", "version": "v1.0.0"}
    ]`)

	index, err := NewIndex([][]byte{manifest})
	if err != nil {
		t.Fatalf("NewIndex returned %v", err)
	}

	if got := sortedPaths(index.WriteOnlyPaths("aws-authentication", "v0.11.1")); len(got) != 2 {
		t.Errorf("paths = %v, want both declarations", got)
	}
	if got := index.WriteOnlyPaths("cors", "v1.0.0"); len(got) != 0 {
		t.Errorf("paths = %v, want none for a policy with no declarations", got)
	}
}

// A definition carries "v1.4.1" and an attachment often carries "1.4.1".
func TestIndexIgnoresAVersionPrefixAndNameCase(t *testing.T) {
	manifest := []byte(`[{"name": "aws-authentication", "version": "v0.11.1",
      "writeOnlyParams": [["awsSecretAccessKey"]]}]`)

	index, err := NewIndex([][]byte{manifest})
	if err != nil {
		t.Fatalf("NewIndex returned %v", err)
	}

	if got := index.WriteOnlyPaths("AWS-Authentication", "0.11.1"); len(got) != 1 {
		t.Fatalf("paths = %v, want the declaration found", got)
	}
}

// An attachment naming no version, or one no gateway reported, still has its
// secrets withheld: the union of every known version is used.
func TestIndexFallsBackToTheUnionAcrossVersions(t *testing.T) {
	manifest := []byte(`[
      {"name": "oauth2-generator", "version": "v0.10.1", "writeOnlyParams": [["clientSecret"]]},
      {"name": "oauth2-generator", "version": "v0.11.0", "writeOnlyParams": [["password"]]}
    ]`)

	index, err := NewIndex([][]byte{manifest})
	if err != nil {
		t.Fatalf("NewIndex returned %v", err)
	}

	if got := sortedPaths(index.WriteOnlyPaths("oauth2-generator", "")); len(got) != 2 {
		t.Errorf("paths for an unversioned attachment = %v, want both versions' declarations", got)
	}
	if got := sortedPaths(index.WriteOnlyPaths("oauth2-generator", "v9.9.9")); len(got) != 2 {
		t.Errorf("paths for an unknown version = %v, want both versions' declarations", got)
	}
}

func TestIndexMergesManifestsWithoutDuplicating(t *testing.T) {
	manifest := []byte(`[{"name": "basic-auth", "version": "v1.1.1", "writeOnlyParams": [["password"]]}]`)

	index, err := NewIndex([][]byte{manifest, manifest})
	if err != nil {
		t.Fatalf("NewIndex returned %v", err)
	}

	if got := index.WriteOnlyPaths("basic-auth", "v1.1.1"); len(got) != 1 {
		t.Fatalf("paths = %v, want one entry for the same declaration reported twice", got)
	}
}

// A manifest that cannot be read is an error, not an index that quietly stops
// withholding whatever that manifest declared.
func TestNewIndexRejectsAnUnreadableManifest(t *testing.T) {
	if _, err := NewIndex([][]byte{[]byte("{not json")}); err == nil {
		t.Fatal("NewIndex accepted an unreadable manifest")
	}
}

func TestStoreCachesPerOrganizationAndRebuildsAfterInvalidate(t *testing.T) {
	manifest := []byte(`[{"name": "basic-auth", "version": "v1.1.1", "writeOnlyParams": [["password"]]}]`)
	loads := map[string]int{}
	store := NewStore(func(orgID string) ([][]byte, error) {
		loads[orgID]++
		return [][]byte{manifest}, nil
	})

	for i := 0; i < 3; i++ {
		if _, err := store.For("org-a"); err != nil {
			t.Fatalf("For returned %v", err)
		}
	}
	if loads["org-a"] != 1 {
		t.Errorf("manifests loaded %d times, want them cached after the first", loads["org-a"])
	}

	if _, err := store.For("org-b"); err != nil {
		t.Fatalf("For returned %v", err)
	}
	if loads["org-b"] != 1 {
		t.Errorf("org-b loaded %d times, want its own index built", loads["org-b"])
	}

	store.Invalidate("org-a")
	if _, err := store.For("org-a"); err != nil {
		t.Fatalf("For returned %v", err)
	}
	if loads["org-a"] != 2 {
		t.Errorf("manifests loaded %d times after invalidate, want a rebuild", loads["org-a"])
	}
}

// A read cannot continue past an unreadable index: carrying on would produce a
// response indistinguishable from one for a policy that has no secrets.
func TestStoreReportsALoadFailureRatherThanResolvingNothing(t *testing.T) {
	store := NewStore(func(string) ([][]byte, error) {
		return nil, errors.New("database unavailable")
	})

	if _, err := store.For("org-a"); err == nil {
		t.Fatal("For succeeded despite the manifests being unreadable")
	}
}
