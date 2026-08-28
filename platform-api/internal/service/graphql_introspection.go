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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// graphQLIntrospectionTimeout bounds the outbound introspection call end to
// end — this is a one-shot onboarding-time probe against a
// tenant-configured upstream, not a proxied request in the data path, so a
// generous-but-bounded timeout is appropriate.
const graphQLIntrospectionTimeout = 15 * time.Second

// standardGraphQLIntrospectionQuery is the standard GraphQL introspection
// query (the same shape graphql-js's getIntrospectionQuery() emits), sent
// verbatim to the tenant's upstream so any spec-compliant GraphQL server
// can answer it.
const standardGraphQLIntrospectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type { ...TypeRef }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
            }
          }
        }
      }
    }
  }
}
`

// graphQLIntrospectionRequestBody is the JSON body sent to the upstream endpoint.
type graphQLIntrospectionRequestBody struct {
	Query string `json:"query"`
}

// graphQLIntrospectionResponse is the minimal shape of a standard GraphQL
// introspection response this converter understands.
type graphQLIntrospectionResponse struct {
	Data *struct {
		Schema graphQLIntrospectionSchema `json:"__schema"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

type graphQLIntrospectionSchema struct {
	QueryType        *graphQLIntrospectionTypeRef `json:"queryType"`
	MutationType     *graphQLIntrospectionTypeRef `json:"mutationType"`
	SubscriptionType *graphQLIntrospectionTypeRef `json:"subscriptionType"`
	Types            []graphQLIntrospectionType   `json:"types"`
}

type graphQLIntrospectionTypeRef struct {
	Kind   string                       `json:"kind"`
	Name   string                       `json:"name"`
	OfType *graphQLIntrospectionTypeRef `json:"ofType"`
}

type graphQLIntrospectionType struct {
	Kind          string                            `json:"kind"`
	Name          string                            `json:"name"`
	Description   string                            `json:"description"`
	Fields        []graphQLIntrospectionField       `json:"fields"`
	InputFields   []graphQLIntrospectionInputValue  `json:"inputFields"`
	Interfaces    []graphQLIntrospectionTypeRef     `json:"interfaces"`
	EnumValues    []graphQLIntrospectionEnumValue   `json:"enumValues"`
	PossibleTypes []graphQLIntrospectionTypeRef     `json:"possibleTypes"`
}

type graphQLIntrospectionField struct {
	Name        string                           `json:"name"`
	Description string                           `json:"description"`
	Args        []graphQLIntrospectionInputValue `json:"args"`
	Type        graphQLIntrospectionTypeRef      `json:"type"`
}

type graphQLIntrospectionInputValue struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Type        graphQLIntrospectionTypeRef `json:"type"`
	// DefaultValue is intentionally not converted — see convertGraphQLIntrospectionToSDL.
}

type graphQLIntrospectionEnumValue struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// graphQLBuiltinScalarNames are the five GraphQL scalars every server
// implicitly defines; introspection always lists them, but re-declaring
// them in SDL is both unnecessary and (for String/Int/Float/Boolean/ID)
// invalid.
var graphQLBuiltinScalarNames = map[string]bool{
	"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true,
}

// fetchAndConvertGraphQLSchema runs the standard introspection query
// against upstreamURL through the SSRF-hardened upstream client, converts
// the JSON result into SDL text, and validates the result defines a Query
// type. The returned error is for internal logging only — callers map it
// to the sterile GraphQLAPISchemaResolveFailed response (ssrf-prevention.md
// / error-handling.md — never echo the resolved IP or the specific failure
// reason to the client).
func fetchAndConvertGraphQLSchema(upstreamURL string) (string, error) {
	body, err := json.Marshal(graphQLIntrospectionRequestBody{Query: standardGraphQLIntrospectionQuery})
	if err != nil {
		return "", fmt.Errorf("failed to build introspection request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build introspection request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// The GraphQL endpoint URL is tenant-supplied — dial through the
	// SSRF-guarded client (ssrf-prevention.md directive 6: reuse the shared
	// upstream-fetch helper rather than a one-off client). upstream.main.url
	// is the tenant's own configured backend (analogous to REST/MCP's
	// upstream), so NewUpstreamFetchClient's private/in-cluster-permitting
	// policy is the correct one here — not the stricter public-only policy
	// FetchOpenAPISpecFromURL uses for fetching a public vendor's OpenAPI doc.
	client, err := utils.NewUpstreamFetchClient(graphQLIntrospectionTimeout)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to reach GraphQL endpoint for introspection: %w", err)
	}
	defer resp.Body.Close()

	const maxIntrospectionResponseBytes = 5 << 20 // 5 MiB ceiling on the introspection response
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxIntrospectionResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read introspection response: %w", err)
	}
	if len(respBody) > maxIntrospectionResponseBytes {
		// Reject outright rather than silently parsing a truncated body — a cut
		// that happens to land on a JSON boundary could otherwise produce a
		// subtly incomplete (but parseable) derived schema (file-access.md
		// directive 5).
		return "", fmt.Errorf("introspection response exceeds the maximum allowed size of %d bytes", maxIntrospectionResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("introspection request failed with status %d", resp.StatusCode)
	}

	var parsed graphQLIntrospectionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse introspection response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return "", fmt.Errorf("introspection query returned %d error(s)", len(parsed.Errors))
	}
	if parsed.Data == nil {
		return "", fmt.Errorf("introspection response has no data")
	}

	sdl, err := convertGraphQLIntrospectionToSDL(parsed.Data.Schema)
	if err != nil {
		return "", err
	}
	if err := validateGraphQLSDL(sdl); err != nil {
		return "", fmt.Errorf("derived schema failed validation: %w", err)
	}
	return sdl, nil
}

// convertGraphQLIntrospectionToSDL converts a standard introspection
// __schema result into SDL text via gqlparser's AST + formatter. This is a
// reasonably complete converter (object/interface/union/enum/input types,
// scalars, non-null/list wrappers, field arguments) — not byte-perfect for
// every exotic GraphQL feature. Known gaps, left as best-effort omissions
// rather than hard failures:
//   - default values on arguments/input fields are not reproduced (would
//     require parsing the introspection-supplied literal back into an
//     ast.Value);
//   - custom directives and directive definitions are not reproduced
//     (introspection's `directives` list is not requested/consumed);
//   - descriptions are preserved, but deprecation reasons are not rendered
//     as `@deprecated(reason: ...)` directives.
func convertGraphQLIntrospectionToSDL(schema graphQLIntrospectionSchema) (string, error) {
	if schema.QueryType == nil || schema.QueryType.Name == "" {
		return "", fmt.Errorf("introspection response has no queryType")
	}

	astSchema := &ast.Schema{Types: map[string]*ast.Definition{}}
	for _, t := range schema.Types {
		if t.Name == "" || strings.HasPrefix(t.Name, "__") || graphQLBuiltinScalarNames[t.Name] {
			continue
		}
		def, ok := convertGraphQLIntrospectionDefinition(t)
		if !ok {
			// Best-effort: skip a type we can't faithfully represent rather
			// than fail the whole schema derivation.
			continue
		}
		astSchema.Types[t.Name] = def
	}

	queryDef, ok := astSchema.Types[schema.QueryType.Name]
	if !ok {
		return "", fmt.Errorf("query type %q not found among introspected types", schema.QueryType.Name)
	}
	astSchema.Query = queryDef

	if schema.MutationType != nil {
		if def, ok := astSchema.Types[schema.MutationType.Name]; ok {
			astSchema.Mutation = def
		}
	}
	if schema.SubscriptionType != nil {
		if def, ok := astSchema.Types[schema.SubscriptionType.Name]; ok {
			astSchema.Subscription = def
		}
	}

	var buf bytes.Buffer
	formatter.NewFormatter(&buf).FormatSchema(astSchema)
	return buf.String(), nil
}

// convertGraphQLIntrospectionDefinition converts one introspected type into
// an ast.Definition. ok is false for a kind this converter does not
// understand (e.g. a future GraphQL kind), signaling the caller to skip it.
func convertGraphQLIntrospectionDefinition(t graphQLIntrospectionType) (*ast.Definition, bool) {
	def := &ast.Definition{
		Name:        t.Name,
		Description: t.Description,
	}

	switch t.Kind {
	case "OBJECT":
		def.Kind = ast.Object
		def.Fields = convertGraphQLIntrospectionFields(t.Fields)
		def.Interfaces = convertGraphQLIntrospectionTypeRefNames(t.Interfaces)
	case "INTERFACE":
		def.Kind = ast.Interface
		def.Fields = convertGraphQLIntrospectionFields(t.Fields)
		def.Interfaces = convertGraphQLIntrospectionTypeRefNames(t.Interfaces)
	case "UNION":
		def.Kind = ast.Union
		def.Types = convertGraphQLIntrospectionTypeRefNames(t.PossibleTypes)
	case "ENUM":
		def.Kind = ast.Enum
		for _, ev := range t.EnumValues {
			def.EnumValues = append(def.EnumValues, &ast.EnumValueDefinition{
				Name:        ev.Name,
				Description: ev.Description,
			})
		}
	case "INPUT_OBJECT":
		def.Kind = ast.InputObject
		for _, f := range t.InputFields {
			def.Fields = append(def.Fields, &ast.FieldDefinition{
				Name:        f.Name,
				Description: f.Description,
				Type:        convertGraphQLIntrospectionTypeRef(&f.Type),
			})
		}
	case "SCALAR":
		def.Kind = ast.Scalar
	default:
		return nil, false
	}
	return def, true
}

// convertGraphQLIntrospectionFields converts introspected object/interface
// fields, including their arguments.
func convertGraphQLIntrospectionFields(fields []graphQLIntrospectionField) ast.FieldList {
	out := make(ast.FieldList, 0, len(fields))
	for _, f := range fields {
		fd := &ast.FieldDefinition{
			Name:        f.Name,
			Description: f.Description,
			Type:        convertGraphQLIntrospectionTypeRef(&f.Type),
		}
		for _, a := range f.Args {
			fd.Arguments = append(fd.Arguments, &ast.ArgumentDefinition{
				Name:        a.Name,
				Description: a.Description,
				Type:        convertGraphQLIntrospectionTypeRef(&a.Type),
			})
		}
		out = append(out, fd)
	}
	return out
}

// convertGraphQLIntrospectionTypeRefNames extracts sorted, de-duplicated
// names from a list of type references (used for interfaces/union
// possibleTypes).
func convertGraphQLIntrospectionTypeRefNames(refs []graphQLIntrospectionTypeRef) []string {
	seen := make(map[string]bool, len(refs))
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Name == "" || seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names
}

// convertGraphQLIntrospectionTypeRef recursively converts an introspection
// TypeRef (which wraps NON_NULL/LIST around a named type) into an ast.Type.
func convertGraphQLIntrospectionTypeRef(ref *graphQLIntrospectionTypeRef) *ast.Type {
	if ref == nil {
		return ast.NamedType("String", nil)
	}
	switch ref.Kind {
	case "NON_NULL":
		inner := convertGraphQLIntrospectionTypeRef(ref.OfType)
		wrapped := *inner
		wrapped.NonNull = true
		return &wrapped
	case "LIST":
		return ast.ListType(convertGraphQLIntrospectionTypeRef(ref.OfType), nil)
	default:
		if ref.Name == "" {
			return ast.NamedType("String", nil)
		}
		return ast.NamedType(ref.Name, nil)
	}
}
