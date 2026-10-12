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

// Package gatewaytranslator adapts a deployment artifact to the gateway it is
// deployed to. It is the only translator package that services and
// repositories import.
//
// # Two axes
//
// An artifact lives on two version axes:
//
//   - the platform data version it was stored at (the shape platform-api wrote
//     it in, recorded in the data_version column — see PlatformDataVersion and
//     ComputeDataVersion), and
//   - the release of the gateway it is going to (model.Gateway.Version, the
//     string the gateway reports in its manifest on connect).
//
// Only LTS gateway releases (semver: 1.0.0, 1.1.0, 1.2.0) are compared. STS
// releases are named after their release date and do not share the LTS
// version line, so a gateway reporting a date is treated as a current build
// (https://github.com/wso2/api-platform/issues/3681). A capability that no
// LTS release has yet uses gwversion.NoLTSRelease as its minimum.
//
// Generators always produce the gateway-latest shape. Translate first brings
// the artifact up to that shape if it was stored earlier, then adapts it down
// to what the target gateway understands, and reports every lossy decision.
//
// # Package map
//
//	gatewaytranslator        this facade: Translate, EnsureKindSupported, the
//	                         kind-name map, data versions
//	gatewaytranslator/gwversion   gateway version parsing and the release at
//	                         which each capability appeared — the only place
//	                         a gateway version literal may be written
//	gatewaytranslator/translate   the engine: Kind, Step, Report, Run
//	gatewaytranslator/kinds  one file per artifact kind
//	gatewaytranslator/secretinline  delivery-time {{ secret }} rendering for
//	                         gateways that cannot resolve placeholders
//
// Imports point strictly downward (facade -> kinds -> translate -> gwversion),
// so there are no cycles and each package is understandable from its own doc.
//
// # Reading rule
//
// To know what happens to an artifact of kind X on an older gateway, open
// kinds/<x>.go. It holds the kind's first supporting release, how an older
// stored shape is brought up to date, and the ordered steps, each tagged with
// the gwversion.Min* release it applies below. Version numbers never appear
// in a kind file.
//
// # Adding a kind or a step
//
// A kind is one file in kinds/, one row in kinds.All and one test. A step is
// one constant in gwversion/versions.go naming the LTS release that made the
// step unnecessary (gwversion.NoLTSRelease if none has yet), and one
// translate.Step in the kind file.
//
// # Known limitations
//
//   - A plaintext secret value containing "{{" is re-parsed by the 1.1.0
//     gateway's template engine after secretinline has inlined it.
//   - Gateway 1.0.0 keeps only the last policy of a given name on a route, so
//     a flattened global policy and an operation-level policy with the same
//     name do not both apply there.
//   - Gateway 1.2.0 operators who enabled mcp.append_resource_path_to_backend
//     get a doubled /mcp: platform-api sees the gateway version, not the toggle.
//   - The released gateways run their deployment sync once per controller
//     start and do not retry an entry missing from the batch, and they ack a
//     refused deploy-event fetch only as a generic processing error. A
//     deployment whose secret cannot be inlined is therefore marked FAILED
//     (SECRET_RESOLUTION_FAILED) by service/gateway_internal.go on either path
//     and needs a redeploy once the secret is restored.
//   - A gateway that has never pushed its manifest has no version and is
//     treated as a current build; artifacts deployed to it before its first
//     connect are shaped for latest and need a redeploy once it reports an
//     older release.
package gatewaytranslator
