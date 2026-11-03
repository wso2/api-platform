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

// Package inbound holds the gateway's inbound profiles: what a model endpoint's
// wire format needs that a provider template cannot say.
//
// A model endpoint names its format with spec.inboundTemplate, a provider template
// handle, as a multi-provider LLM proxy does. The template is data: its
// requestModel says where a request carries the model name, and its token fields
// drive extraction. It says nothing about which paths a client calls, how the
// format streams, or how its model list and error bodies look. A profile carries
// that, in code, one per supported format, registered under the template handle.
//
// The set is closed in v1: an endpoint whose inboundTemplate has no profile is
// refused when it is saved.
package inbound

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Operation is one route an endpoint serves, relative to its context. A path may
// use `{param}` for one segment, which the router matches as any non-slash run.
type Operation struct {
	Method string
	Path   string
}

// String renders the operation the way an API key scope names it: "GET /models".
func (o Operation) String() string {
	return o.Method + " " + o.Path
}

// Location is where a request carries a value: the {location, identifier} pair
// provider templates and routing policies use.
type Location struct {
	Location   string `json:"location"`
	Identifier string `json:"identifier"`
}

// Location values.
const (
	LocationPayload    = "payload"
	LocationPathParam  = "pathParam"
	LocationHeader     = "header"
	LocationQueryParam = "queryParam"
)

// Formats the error and catalog bodies are rendered in. The error formats are the
// policy engine's route error formats (faultformat), and the model system policy
// renders the same names; the three sides agree by these strings.
const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatGemini    = "gemini"
)

// ModelPathParam is the path parameter a dispatch operation names the model in.
const ModelPathParam = "{model}"

// Profile is one inbound format.
type Profile struct {
	// Template is the provider template handle the profile is registered under.
	Template string

	// Dispatch are the operations that name a Model and are dispatched to its
	// chain. Catalog are the operations that list the endpoint's Models.
	Dispatch []Operation
	Catalog  []Operation

	// Stream is where a request says it wants a streamed response. A payload
	// location is a top-level member that is literally true; a pathParam
	// location is a pattern that matches the request path.
	Stream Location

	// Format names the error and catalog body shapes (Format* constants).
	Format string
}

// ModelInPath reports whether the profile's dispatch operations carry the model
// in the path, which is what lets a template locate it there.
func (p Profile) ModelInPath() bool {
	for _, op := range p.Dispatch {
		if strings.Contains(op.Path, ModelPathParam) {
			return true
		}
	}
	return false
}

// CatalogScopes are the operations a Model-scoped API key names to be admitted on
// the catalog routes: one "METHOD path" per catalog operation.
func (p Profile) CatalogScopes() []string {
	out := make([]string, 0, len(p.Catalog))
	for _, op := range p.Catalog {
		out = append(out, op.String())
	}
	return out
}

// Gemini serves the same operations under two API versions, as the Google GenAI
// SDK calls one or the other.
var geminiVersions = []string{"v1", "v1beta"}

var profiles = map[string]Profile{
	FormatOpenAI: {
		Template: "openai",
		Dispatch: []Operation{{Method: "POST", Path: "/chat/completions"}},
		Catalog:  []Operation{{Method: "GET", Path: "/models"}},
		Stream:   Location{Location: LocationPayload, Identifier: "$.stream"},
		Format:   FormatOpenAI,
	},
	FormatAnthropic: {
		Template: "anthropic",
		Dispatch: []Operation{{Method: "POST", Path: "/v1/messages"}},
		Catalog:  []Operation{{Method: "GET", Path: "/v1/models"}},
		Stream:   Location{Location: LocationPayload, Identifier: "$.stream"},
		Format:   FormatAnthropic,
	},
	FormatGemini: geminiProfile(),
}

func geminiProfile() Profile {
	p := Profile{
		Template: "gemini",
		Stream:   Location{Location: LocationPathParam, Identifier: `:streamGenerateContent$`},
		Format:   FormatGemini,
	}
	for _, v := range geminiVersions {
		p.Dispatch = append(p.Dispatch,
			Operation{Method: "POST", Path: "/" + v + "/models/" + ModelPathParam + ":generateContent"},
			Operation{Method: "POST", Path: "/" + v + "/models/" + ModelPathParam + ":streamGenerateContent"})
		p.Catalog = append(p.Catalog, Operation{Method: "GET", Path: "/" + v + "/models"})
	}
	return p
}

// Lookup returns the profile registered under a template handle.
func Lookup(template string) (Profile, bool) {
	p, ok := profiles[strings.TrimSpace(template)]
	return p, ok
}

// Templates returns the template handles that have a profile, sorted.
func Templates() []string {
	out := make([]string, 0, len(profiles))
	for k := range profiles {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Limits on a model location, so a template cannot hand the resolver something
// unbounded to compile or scan.
const (
	maxPatternLength    = 512
	maxIdentifierLength = 128
)

var (
	// topLevelMember is the one payload shape the resolver reads: `$.<name>`. The
	// resolver runs before the endpoint's authentication, so it must not decode the
	// whole body to follow a nested path.
	topLevelMember = regexp.MustCompile(`^\$\.([A-Za-z_][A-Za-z0-9_\-]*)$`)
	headerName     = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]+$`)
	queryName      = regexp.MustCompile(`^[A-Za-z0-9._~\-]+$`)
)

// ValidateModelLocation checks that a template's requestModel is one the resolver
// can read for this profile, and returns it normalised (a header name
// lower-cased).
//
// The four locations are the ones the routing policies accept, so the resolver and
// the policies agree on what a template can declare. A path location is accepted
// only for a profile that names the model in its dispatch paths: a pattern run
// against "/chat/completions" would never find one.
func (p Profile) ValidateModelLocation(loc Location) (Location, error) {
	id := strings.TrimSpace(loc.Identifier)
	switch loc.Location {
	case LocationPayload:
		if !topLevelMember.MatchString(id) {
			return Location{}, fmt.Errorf("requestModel identifier %q must name a top-level member ($.<name>)", id)
		}
	case LocationPathParam:
		if !p.ModelInPath() {
			return Location{}, fmt.Errorf("the %s inbound format does not carry the model in its path", p.Template)
		}
		if len(id) == 0 || len(id) > maxPatternLength {
			return Location{}, fmt.Errorf("requestModel pattern must be 1 to %d characters", maxPatternLength)
		}
		re, err := regexp.Compile(id)
		if err != nil {
			return Location{}, fmt.Errorf("requestModel pattern %q does not compile: %v", id, err)
		}
		if re.NumSubexp() != 1 {
			return Location{}, fmt.Errorf("requestModel pattern %q must have exactly one capture group", id)
		}
	case LocationHeader:
		if len(id) == 0 || len(id) > maxIdentifierLength || !headerName.MatchString(id) {
			return Location{}, fmt.Errorf("requestModel header %q is not a valid header name", id)
		}
		id = strings.ToLower(id)
	case LocationQueryParam:
		if len(id) == 0 || len(id) > maxIdentifierLength || !queryName.MatchString(id) {
			return Location{}, fmt.Errorf("requestModel query parameter %q is not a valid name", id)
		}
	default:
		return Location{}, fmt.Errorf("requestModel location %q is not one of payload, pathParam, header, queryParam", loc.Location)
	}
	return Location{Location: loc.Location, Identifier: id}, nil
}

// LocationFromParams reads a {location, identifier} pair out of template params,
// as buildTemplateParams writes them.
func LocationFromParams(params map[string]interface{}, key string) (Location, bool) {
	raw, ok := params[key].(map[string]interface{})
	if !ok {
		return Location{}, false
	}
	var loc Location
	switch v := raw["location"].(type) {
	case string:
		loc.Location = v
	case fmt.Stringer:
		loc.Location = v.String()
	default:
		if raw["location"] != nil {
			loc.Location = fmt.Sprint(raw["location"])
		}
	}
	if v, ok := raw["identifier"].(string); ok {
		loc.Identifier = v
	}
	return loc, loc.Location != "" && loc.Identifier != ""
}
