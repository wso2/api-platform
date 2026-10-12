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

// Package translate is the engine that runs a kind definition against a
// deployment artifact. It holds no knowledge of any particular kind: the
// kinds package supplies a Kind, and Run applies it for one target gateway.
//
// A Kind is two things. Normalize brings an artifact stored at an older
// platform data version up to the latest shape, so every Step starts from the
// same canonical input. Steps are the adaptations an older gateway needs, each
// tagged with the release it applies Below. Run adds one more step itself:
// the CRD apiVersion swap every kind gets on gateways older than
// gwversion.MinGatewayV1Version, so no kind definition can forget it.
package translate

// Step is one adaptation an artifact needs on gateways older than Below.
//
// Below is always a gwversion.Min* constant, never a literal, so the kind
// file reads as "below the release that added X". Apply mutates the artifact
// in place and records anything lossy in the Report; it returns an error only
// for a programming mistake (wrong payload type), never for an artifact the
// gateway will simply reject.
type Step struct {
	Below string
	Name  string
	Apply func(artifact any, r *Report) error
}

// Kind is everything the translator knows about one artifact kind.
type Kind struct {
	// GatewayKind is the kind as the gateway names it in the artifact's
	// "kind:" field, e.g. "Mcp" or "Agent".
	GatewayKind string

	// MinGatewayVersion is the first release that has this kind at all. Empty
	// means every release. A deploy to an older gateway is refused outright
	// (see gatewaytranslator.EnsureKindSupported); Steps never see it.
	MinGatewayVersion string

	// Normalize brings an artifact generated at sourceDataVersion up to the
	// latest shape, in place. It must be idempotent and must reject a payload
	// of another kind's type. nil means the kind has only ever had one shape.
	Normalize func(sourceDataVersion string, artifact any) error

	// Steps run in order, each only when the target gateway is below its
	// Below release.
	Steps []Step
}
