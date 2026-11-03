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

// Package model is the gateway's internal model system policy: the parts of a
// model endpoint the gateway decides or answers itself. A model endpoint serves the
// Models that name it on one set of routes, in one inbound format (OpenAI,
// Anthropic or Gemini), and dispatches each request to that Model's own policy
// chain. The controller attaches this policy to the chains it compiles for an
// endpoint; a Model author never does.
//
// It does four jobs, and each takes its own parameter block. Nothing is shared
// between the blocks beyond the name: a chain carries one instance per job, and
// which block an instance holds is the whole of what that instance does.
//
// # Target resolution and provider scope (route, providerScope)
//
// A Model chain is laid out as an outside speaking the endpoint's inbound format
// and a provider-speaking inside:
//
//	analytics · endpoint policies · Model policies · router · route · [per provider:
//	transformer, credential, cost] · providerScope
//
// Everything before `route` sees the request the client sent, in the endpoint's
// format. `route` decides which of the Model's targets the request goes to and
// rewrites the model for that target, wherever the format carries it. Everything
// after it is specific to one provider and is gated on that provider having been
// selected.
//
// The policies on either side read the same two metadata keys to learn which wire
// format they are looking at: template_handle, which names the provider template
// whose field locations describe the payload, and provider_name, which a rate
// limiter maps to that template. The kernel seeds both from the route, and one
// route serves every Model, so the route can only ever name the endpoint's scope.
// These two jobs bracket the provider-specific policies to correct that:
//
//   - On the request, `route` switches the scope to the selected provider's, so
//     the translator and the cost policy after it read the provider's format.
//   - On the response, which runs the chain in reverse, `providerScope` runs
//     first and switches the scope to the provider's before the cost policy reads
//     the provider's native response; `route` runs after the transformer has
//     turned that response back into the endpoint's format, and switches the
//     scope back for the Model's policies and the analytics collector.
//
// `route` also answers in the Model's name: the model the response reports — the
// target's upstream model, as the provider named it — is replaced with the name
// the client asked for, so a response never says which provider or model a Model
// is made of. The provider's value is recorded for the analytics collector.
//
// Both do it on every response phase — headers, a buffered body, and each
// streamed chunk — because the reverse walk happens once per phase and per
// chunk, and the metadata a policy reads is whatever the last writer left there.
//
// # The dispatch error (dispatchError)
//
// The endpoint's resolver binds a request it cannot place — no model named, a model
// that is not deployed, a body that is not a JSON object — to a reserved chain
// rather than refusing it itself, because a resolver cannot shape its refusal and
// runs before authentication. The reserved chain runs the endpoint's own policies
// first and this job last, so an unauthenticated caller is refused by them before
// this job can tell it whether a model exists, and an authenticated one gets an
// error its SDK can read, in the endpoint's format.
//
// # The catalog (catalog)
//
// The model list (`GET /models` on OpenAI, `GET /v1/models` on Anthropic, `GET
// /{version}/models` on Gemini) is answered here, from the Models the controller
// compiled into the endpoint, in the endpoint's format and filtered to the Models
// the caller's API key reaches. It is a policy rather than a direct-response route
// for the same reason the A2A Agent Card is: the route needs a real chain, so the
// endpoint's authentication and the analytics collector run on it.
package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Parameter names.
//
// These are written by the gateway controller's model gateway compiler and read
// here. The two live in separate Go modules and cannot share a constant, so the
// names are spelled once on each side and the coupling is asserted by a test on
// each side.
const (
	// ParamRoute is the block configuring target resolution and the request's
	// switch into the selected provider's scope.
	ParamRoute = "route"

	// ParamProviderScope is the block configuring the response's switch into the
	// selected provider's scope, ahead of the provider-specific policies.
	ParamProviderScope = "providerScope"

	// ParamDispatchError is the block configuring the reserved dispatch chain's
	// answer.
	ParamDispatchError = "dispatchError"

	// ParamCatalog is the block configuring the model catalog answer.
	ParamCatalog = "catalog"

	// Fields of the route block.
	ParamModelHandle    = "modelHandle"
	ParamModelName      = "modelName"
	ParamScope          = "scope"
	ParamTemplateHandle = "templateHandle"
	ParamProviderName   = "providerName"
	ParamTargets        = "targets"
	ParamModelLocation  = "modelLocation"

	// ParamFormat is the endpoint's inbound format, in the route, dispatchError and
	// catalog blocks: every body the job writes is in it.
	ParamFormat = "format"

	// Fields of one route target.
	ParamTargetName          = "name"
	ParamTargetProvider      = "provider"
	ParamTargetUpstreamModel = "upstreamModel"
	ParamTargetTemplate      = "template"
	ParamTargetDisplayName   = "providerDisplayName"
	ParamTargetRouted        = "routed"

	// Fields of the catalog block: the Models the endpoint serves.
	ParamCatalogModels      = "models"
	ParamCatalogHandle      = "handle"
	ParamCatalogName        = "name"
	ParamCatalogDisplayName = "displayName"
	ParamCatalogCreated     = "created"
)

// The authentication context api-key-auth leaves behind, which the catalog job
// filters on. AuthPropertyAPIKeyOperations is the key's Operations — "*", or a JSON
// array of the operations it reaches.
const (
	APIKeyAuthType               = "apikey"
	AuthPropertyAPIKeyOperations = "APIKeyOperations"
	allOperations                = "*"
)

// SharedContext.Metadata keys.
//
// The first three are the engine-wide contract this policy takes part in rather
// than defines: selected_provider is what the controller's conditional translator
// and credential policies match on, and template_handle and provider_name are the
// keys the kernel seeds from the route and every LLM-aware policy reads. The
// model.* keys are this policy's own, written by the route job for the analytics
// collector, which reports the real provider and Model from them, and for the
// providerScope job, which switches back into the scope they name.
// MetadataResponseModel is the model the provider's response named, before the
// route job replaced it with the Model's name.
const (
	MetadataSelectedProvider = "selected_provider"
	MetadataTemplateHandle   = "template_handle"
	MetadataProviderName     = "provider_name"

	MetadataModelHandle         = "model.handle"
	MetadataModelName           = "model.name"
	MetadataModelTarget         = "model.target"
	MetadataProviderHandle      = "model.provider.handle"
	MetadataProviderTemplate    = "model.provider.template"
	MetadataProviderDisplayName = "model.provider.displayName"
	MetadataUpstreamModel       = "model.upstreamModel"
	MetadataResponseModel       = "model.responseModel"
)

// InternalLoopbackHeader is the marker a proxy stamps on its forward to a
// provider's own route. A provider route trusts it to mean "this call came from
// the gateway", so a client must never be able to send it through: the route job
// removes it on a direct target, and a later conditional policy sets the
// gateway's own value on a routed one.
const InternalLoopbackHeader = "x-wso2-internal-loopback"

// Resolution attribute names, as the llm-model-dispatch resolver in the policy
// engine spells them into SharedContext.ResolutionAttributes.
//
// Read, never written: the resolver owns that map. The engine's copy lives in an
// internal package of another module, so the literals are mirrored here and
// pinned by a test on each side — the same arrangement the a2a system policy uses
// for its resolver's names.
const (
	AttrModelName     = "llm.model.name"
	AttrModelHandle   = "llm.model.handle"
	AttrRequestStream = "llm.request.stream"
	AttrDispatchError = "llm.dispatch.error"
)

// Values of AttrDispatchError: why the resolver bound the request to the dispatch
// chain rather than to a Model's.
const (
	DispatchErrorInvalidJSON   = "invalid_json"
	DispatchErrorMissingModel  = "missing_model"
	DispatchErrorInvalidModel  = "invalid_model"
	DispatchErrorModelNotFound = "model_not_found"
)

// GetPolicy returns the instance for one attachment of the policy.
//
// The parameter block is parsed here, once per chain build, rather than per
// request: the block decides what the instance does in every phase, including
// which phases it takes part in at all (see Mode), and that has to be known before
// a request exists.
//
// A block that is missing, duplicated, or unusable is refused with an error. That
// fails this one chain's build — the policy engine skips a chain it cannot build
// and serves the rest of the snapshot — so the chain is never served, and its
// requests are refused by the engine's own sterile response rather than reaching
// a provider. Every block here is written by the controller from a Model it has
// already validated, so a refusal is a controller defect, and refusing is the
// only answer that cannot turn one into a request forwarded somewhere it should
// not go.
func GetPolicy(
	_ policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	block, err := soleBlock(params)
	if err != nil {
		return nil, err
	}

	switch block {
	case ParamRoute:
		cfg, err := parseRouteConfig(params[ParamRoute])
		if err != nil {
			return nil, fmt.Errorf("%s: %s block: %w", PolicyName, ParamRoute, err)
		}
		return newRoutePolicy(cfg), nil
	case ParamProviderScope:
		if err := requireEmptyObject(params[ParamProviderScope]); err != nil {
			return nil, fmt.Errorf("%s: %s block: %w", PolicyName, ParamProviderScope, err)
		}
		return providerScopeInstance, nil
	case ParamDispatchError:
		format, err := parseFormatOnlyBlock(params[ParamDispatchError])
		if err != nil {
			return nil, fmt.Errorf("%s: %s block: %w", PolicyName, ParamDispatchError, err)
		}
		return &dispatchErrorPolicy{format: format}, nil
	case ParamCatalog:
		catalog, err := parseCatalogConfig(params[ParamCatalog])
		if err != nil {
			return nil, fmt.Errorf("%s: %s block: %w", PolicyName, ParamCatalog, err)
		}
		return catalog, nil
	default:
		// Unreachable: soleBlock returns only the names it was given.
		return nil, fmt.Errorf("%s: unhandled parameter block %q", PolicyName, block)
	}
}

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with plugin
// registries that call GetPolicyV2 on all plugins.
func GetPolicyV2(
	metadata policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// PolicyName is the name this policy is registered under, used in errors.
const PolicyName = "wso2_apip_sys_model"

// jobBlocks are the parameter blocks an instance may carry, one of them exactly.
var jobBlocks = []string{ParamRoute, ParamProviderScope, ParamDispatchError, ParamCatalog}

// soleBlock names the one job block params carries.
//
// Keys that are not job blocks are ignored rather than refused: the engine merges
// the definition's system parameters into what the controller wrote, and a key a
// newer controller adds beside the block should not make an older runtime refuse a
// chain it can serve. Two job blocks are refused, because the instance would then
// have two answers to the same phase and no rule for which wins.
func soleBlock(params map[string]any) (string, error) {
	var present []string
	for _, name := range jobBlocks {
		if _, ok := params[name]; ok {
			present = append(present, name)
		}
	}
	switch len(present) {
	case 1:
		return present[0], nil
	case 0:
		return "", fmt.Errorf("%s: instance carries no parameter block (expected one of %s)",
			PolicyName, strings.Join(jobBlocks, ", "))
	default:
		sort.Strings(present)
		return "", fmt.Errorf("%s: instance carries more than one parameter block (%s)",
			PolicyName, strings.Join(present, ", "))
	}
}

// requireEmptyObject accepts the parameterless blocks: present, an object, and
// carrying nothing. A field in one of them is a controller expecting a behaviour
// this version does not have, and is refused rather than ignored for the same
// reason a missing block is.
func requireEmptyObject(raw any) error {
	block, ok := raw.(map[string]any)
	if !ok {
		return errors.New("must be an object")
	}
	if len(block) > 0 {
		names := make([]string, 0, len(block))
		for name := range block {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf("carries unexpected fields (%s)", strings.Join(names, ", "))
	}
	return nil
}

// parseFormatOnlyBlock reads a block that carries nothing but an optional format.
func parseFormatOnlyBlock(raw any) (string, error) {
	block, ok := raw.(map[string]any)
	if !ok {
		return "", errors.New("must be an object")
	}
	for name := range block {
		if name != ParamFormat {
			return "", fmt.Errorf("carries unexpected field %q", name)
		}
	}
	return parseFormat(block)
}

// parseCatalogConfig reads the catalog block: the endpoint's format and the Models
// it serves, sorted by name as the controller lists them.
func parseCatalogConfig(raw any) (*catalogPolicy, error) {
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("must be an object")
	}
	format, err := parseFormat(block)
	if err != nil {
		return nil, err
	}
	rawModels, present := block[ParamCatalogModels]
	list, ok := rawModels.([]any)
	if present && !ok {
		return nil, fmt.Errorf("%s must be an array", ParamCatalogModels)
	}
	entries := make([]catalogEntry, 0, len(list))
	for i, rawEntry := range list {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", ParamCatalogModels, i)
		}
		var e catalogEntry
		if e.handle, err = requiredString(entry, ParamCatalogHandle); err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", ParamCatalogModels, i, err)
		}
		if e.name, err = requiredString(entry, ParamCatalogName); err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", ParamCatalogModels, i, err)
		}
		e.displayName, _ = entry[ParamCatalogDisplayName].(string)
		if e.displayName == "" {
			e.displayName = e.name
		}
		switch created := entry[ParamCatalogCreated].(type) {
		case float64:
			e.created = int64(created)
		case int64:
			e.created = created
		case int:
			e.created = int64(created)
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return &catalogPolicy{format: format, entries: entries}, nil
}

// requiredString reads one non-empty string field. A non-string is refused rather
// than coerced: stringifying whatever arrived would route a request to a provider
// named "map[...]".
func requiredString(block map[string]any, name string) (string, error) {
	value, ok := block[name].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s must be a non-empty string", name)
	}
	return value, nil
}

// metadataString reads one string metadata value, treating anything else as
// absent.
func metadataString(shared *policy.SharedContext, key string) string {
	if shared == nil || shared.Metadata == nil {
		return ""
	}
	value, _ := shared.Metadata[key].(string)
	return value
}

// setScope writes the two scope keys.
func setScope(shared *policy.SharedContext, templateHandle, providerName string) {
	if shared == nil {
		return
	}
	if shared.Metadata == nil {
		shared.Metadata = make(map[string]any)
	}
	shared.Metadata[MetadataTemplateHandle] = templateHandle
	shared.Metadata[MetadataProviderName] = providerName
}
