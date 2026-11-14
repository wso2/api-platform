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

// An array's elements carry no identity to match on, so nothing is carried
// forward inside one and the caller re-supplies its secrets.
func TestPreserveNeverCarriesSecretsInsideAnArray(t *testing.T) {
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
	if got := elements[0].(map[string]any)["apiKey"]; got != "" {
		t.Errorf("backends[0].apiKey = %v, want it left empty for the caller to re-supply", got)
	}
	if got := elements[1].(map[string]any)["apiKey"]; got != "two-rotated" {
		t.Errorf("backends[1].apiKey = %v, want the newly supplied value", got)
	}
}

// Equal length is not proof of identity. A reordered array would otherwise
// attach each element's credential to the element that took its position.
func TestPreserveDoesNotCrossSecretsAcrossReorderedElements(t *testing.T) {
	stored := map[string]any{"backends": []any{
		map[string]any{"id": "one", "apiKey": "one-secret"},
		map[string]any{"id": "two", "apiKey": "two-secret"},
	}}
	updated := map[string]any{"backends": []any{
		map[string]any{"id": "two", "apiKey": ""},
		map[string]any{"id": "one", "apiKey": ""},
	}}

	Preserve(stored, updated, []Path{{"backends", ArrayMarker, "apiKey"}})

	for _, element := range updated["backends"].([]any) {
		backend := element.(map[string]any)
		if backend["apiKey"] != "" {
			t.Errorf("id %v received %v; no credential may cross array positions",
				backend["id"], backend["apiKey"])
		}
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

// A property named for the array marker's spelling is a property, not an array.
// Resolving the marker by spelling alone would leave it returned in full.
func TestBlankEmptiesAPropertyNamedLikeTheArrayMarker(t *testing.T) {
	schema := mustSchema(t, `{"type": "object", "properties": {
      "[]": {"type": "string", "writeOnly": true}}}`)

	paths := PathsFromParameterSchema(schema)
	blanked := Blank(map[string]any{"[]": "a-stored-secret"}, paths)

	if blanked["[]"] != "" {
		t.Errorf("the \"[]\" property = %v, want the empty string", blanked["[]"])
	}
}

func TestPreserveCarriesAPropertyNamedLikeTheArrayMarker(t *testing.T) {
	stored := map[string]any{"[]": "a-stored-secret"}
	updated := map[string]any{"[]": ""}

	Preserve(stored, updated, []Path{{"[]"}})

	if updated["[]"] != "a-stored-secret" {
		t.Errorf("the \"[]\" property = %v, want the stored value", updated["[]"])
	}
}

// A root declared write-only says every parameter is credential material.
func TestPathsFromParameterSchemaExpandsARootDeclaration(t *testing.T) {
	schema := mustSchema(t, `{"type": "object", "writeOnly": true, "properties": {
      "username": {"type": "string"},
      "password": {"type": "string"}}}`)

	want := []string{`["password"]`, `["username"]`}
	if got := sortedPaths(PathsFromParameterSchema(schema)); !reflect.DeepEqual(got, want) {
		t.Fatalf("write-only paths = %v, want one per parameter", got)
	}
}

func TestBlankEmptiesEveryParameterUnderARootDeclaration(t *testing.T) {
	schema := mustSchema(t, `{"type": "object", "writeOnly": true, "properties": {
      "username": {"type": "string"},
      "password": {"type": "string"}}}`)
	params := map[string]any{"username": "admin", "password": "a-stored-secret"}

	blanked := Blank(params, PathsFromParameterSchema(schema))

	for _, key := range []string{"username", "password"} {
		if blanked[key] != "" {
			t.Errorf("%s = %v, want the empty string", key, blanked[key])
		}
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

func TestStoreKeepsAnIndexPerOrganization(t *testing.T) {
	manifests := map[string][]byte{
		"org-a": []byte(`[{"name": "basic-auth", "version": "v1", "writeOnlyParams": [["password"]]}]`),
		"org-b": []byte(`[{"name": "opa-policy", "version": "v1", "writeOnlyParams": [["accessKey"]]}]`),
	}
	store := NewStore(func(orgID string) ([][]byte, error) {
		return [][]byte{manifests[orgID]}, nil
	})

	a, err := store.For("org-a")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}
	b, err := store.For("org-b")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}

	if len(a.WriteOnlyPaths("basic-auth", "v1")) != 1 {
		t.Error("org-a did not resolve its own declaration")
	}
	if len(b.WriteOnlyPaths("basic-auth", "v1")) != 0 {
		t.Error("org-b resolved a declaration belonging to org-a")
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

// The cached index is reused only while the manifests behind it are unchanged,
// so a manifest reported to another replica is picked up on the next read
// rather than waiting for an invalidation that never arrives.
func TestStoreRebuildsWhenTheManifestsChange(t *testing.T) {
	before := []byte(`[{"name": "basic-auth", "version": "v1.1.0", "writeOnlyParams": [["password"]]}]`)
	after := []byte(`[{"name": "basic-auth", "version": "v1.1.0",
	  "writeOnlyParams": [["password"], ["clientSecret"]]}]`)

	current := before
	loads := 0
	store := NewStore(func(string) ([][]byte, error) {
		loads++
		return [][]byte{current}, nil
	})

	resolver, err := store.For("org-a")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}
	if got := len(resolver.WriteOnlyPaths("basic-auth", "v1.1.0")); got != 1 {
		t.Fatalf("paths = %d, want 1", got)
	}

	current = after
	resolver, err = store.For("org-a")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}
	if got := len(resolver.WriteOnlyPaths("basic-auth", "v1.1.0")); got != 2 {
		t.Errorf("paths = %d, want the replaced manifest's 2", got)
	}
	if loads != 2 {
		t.Errorf("manifests read %d times, want one per call", loads)
	}
}

// Unchanged manifests are not re-parsed. The read still happens; the resolved
// index is what the cache saves.
func TestStoreReusesTheIndexWhileManifestsAreUnchanged(t *testing.T) {
	manifest := []byte(`[{"name": "basic-auth", "version": "v1.1.1", "writeOnlyParams": [["password"]]}]`)
	store := NewStore(func(string) ([][]byte, error) { return [][]byte{manifest}, nil })

	first, err := store.For("org-a")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}
	second, err := store.For("org-a")
	if err != nil {
		t.Fatalf("For returned %v", err)
	}
	if first != second {
		t.Error("the index was rebuilt although the manifests were unchanged")
	}
}

// Splitting the same bytes across manifests differently is a different state,
// and the length-prefixed fingerprint must not collide across the two.
func TestStoreFingerprintDistinguishesManifestBoundaries(t *testing.T) {
	a := [][]byte{[]byte(`[{"name":"x"}]`), []byte(`[{"name":"y"}]`)}
	b := [][]byte{[]byte(`[{"name":"x"}][{"name":"y"}]`)}

	if fingerprintOf(a) == fingerprintOf(b) {
		t.Error("manifests concatenated differently produced the same fingerprint")
	}
}

// A {{ secret "handle" }} reference names a secret without disclosing it.
// Withholding it would hide which secret an attachment uses and protect
// nothing — a reader could not tell it from an attachment left unconfigured.
func TestBlankKeepsASecretReference(t *testing.T) {
	params := map[string]any{
		"awsSecretAccessKey": `{{ secret "aws-key" }}`,
		"awsAccessKeyID":     "AKIAEXAMPLE",
	}
	paths := []Path{{"awsSecretAccessKey"}, {"awsAccessKeyID"}}

	blanked := Blank(params, paths)

	if blanked["awsSecretAccessKey"] != `{{ secret "aws-key" }}` {
		t.Errorf("reference = %v, want it returned as it is", blanked["awsSecretAccessKey"])
	}
	if blanked["awsAccessKeyID"] != "" {
		t.Errorf("inline value = %v, want the empty string", blanked["awsAccessKeyID"])
	}
}

// The escaped-quote spelling is the same reference; the manifest and the
// renderer both accept it, so this must too.
func TestBlankKeepsAnEscapedQuoteSecretReference(t *testing.T) {
	params := map[string]any{"password": `{{ secret \"db-password\" }}`}

	blanked := Blank(params, []Path{{"password"}})

	if blanked["password"] != `{{ secret \"db-password\" }}` {
		t.Errorf("reference = %v, want it returned as it is", blanked["password"])
	}
}

// A value that merely contains a placeholder is not a reference: the rest of it
// may be credential material.
func TestBlankEmptiesAValueThatOnlyContainsAReference(t *testing.T) {
	params := map[string]any{"password": `literal-prefix-{{ secret "db-password" }}`}

	blanked := Blank(params, []Path{{"password"}})

	if blanked["password"] != "" {
		t.Errorf("mixed value = %v, want the empty string", blanked["password"])
	}
}

func TestBlankKeepsAReferenceInsideAnArrayElement(t *testing.T) {
	params := map[string]any{"backends": []any{
		map[string]any{"apiKey": `{{ secret "one" }}`},
		map[string]any{"apiKey": "two-inline-secret"},
	}}

	blanked := Blank(params, []Path{{"backends", ArrayMarker, "apiKey"}})

	elements := blanked["backends"].([]any)
	if got := elements[0].(map[string]any)["apiKey"]; got != `{{ secret "one" }}` {
		t.Errorf("backends[0].apiKey = %v, want the reference kept", got)
	}
	if got := elements[1].(map[string]any)["apiKey"]; got != "" {
		t.Errorf("backends[1].apiKey = %v, want the inline value emptied", got)
	}
}

// A reference the reader was shown comes back as a real value, so it is taken
// as supplied rather than preserved — which is what lets a caller move an
// attachment from one secret to another, or off a secret entirely.
func TestPreserveTakesASuppliedReferenceOverTheStoredValue(t *testing.T) {
	stored := map[string]any{"awsSecretAccessKey": "an-inline-secret"}
	updated := map[string]any{"awsSecretAccessKey": `{{ secret "aws-key" }}`}

	Preserve(stored, updated, []Path{{"awsSecretAccessKey"}})

	if updated["awsSecretAccessKey"] != `{{ secret "aws-key" }}` {
		t.Errorf("value = %v, want the supplied reference", updated["awsSecretAccessKey"])
	}
}

// Round trip: a reference is shown, echoed back unchanged, and stored as it was.
func TestBlankThenPreserveRoundTripsAReference(t *testing.T) {
	paths := []Path{{"awsSecretAccessKey"}}
	stored := map[string]any{"awsSecretAccessKey": `{{ secret "aws-key" }}`, "region": "us-east-1"}

	asRead := Blank(stored, paths)
	asRead["region"] = "eu-west-1"
	Preserve(stored, asRead, paths)

	if asRead["awsSecretAccessKey"] != `{{ secret "aws-key" }}` {
		t.Errorf("reference = %v, want it unchanged across a round trip", asRead["awsSecretAccessKey"])
	}
}
