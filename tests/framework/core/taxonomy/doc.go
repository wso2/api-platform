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

// Package taxonomy renders the coverage tree and lints scenario tags in one pass, off
// the curated capability map in docs/capability-map.yml.
//
// The map is a CLOSED vocabulary, hand-maintained and disconnected from code. A
// capability or feature listed with no scenarios renders as an empty branch: a
// visible gap to fill by eye. The tree is generated and must never be hand-edited —
// regenerate it instead, which is what keeps it from going stale.
//
// The tree answers "where does a test belong", not "what is already covered". Only
// the feature files answer the latter; coverage must never be judged from the tree or
// from tag names alone.
//
// Lint rules, applied to every product scenario:
//
//   - exactly one capability tag and exactly one feature tag
//   - the pair resolves to a real node in the map
//   - any test-nature tag is one of the permitted values
//   - any dependency tag names a real capability
//
// Plus, on every scenario product or not, the bidirectional setup rule: a file named
// as a setup fixture must carry the setup marker and vice versa. Both directions
// matter — a prerequisite file missing the marker would be miscounted as coverage,
// and a real test wearing the marker would silently vanish from the tree.
//
// A dependency tag declares that a scenario NEEDS another capability, never that it
// covers it. That distinction is what keeps coverage honest: one capability's
// coverage can never be borrowed from another's tests.
//
// This package also carries the check the YAML topology makes possible and the design
// it is ported from cannot do: every feature file on disk must be bound to exactly
// one runner. That catches dead tests never wired into any suite, and features
// accidentally bound twice.
//
// Violations exit non-zero so this can gate CI. It is convention validation, not a
// coverage gap-finder: empty branches are shown, never failed.
package taxonomy
