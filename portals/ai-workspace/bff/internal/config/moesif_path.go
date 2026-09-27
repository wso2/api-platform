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

package config

import (
	"sort"
	"strings"
)

// MoesifPathMapper builds the path rewriter for the Moesif hop from
// MoesifPathMappings, or returns nil when nothing is configured (leaving that hop
// forwarding paths untouched, exactly as it did before this existed).
//
// It exists because the upstream a deployment points MoesifURL at may publish
// these routes under names of its own. The SPA asks for `/id_token`,
// `/organization` and `/moesif_key` — the names Choreo's moesif-key API uses —
// so that upstream needs no mapping. wso2cloud's platform-api, which serves the
// viewer token at `/analytics/id-token`, does:
//
//	moesif_url           = "http://host:8081/cloud"
//	moesif_path_mappings = "/id_token=/analytics/id-token"
//
// Keeping this on the BFF rather than in the SPA is deliberate: the call has to
// stay server-side anyway (the browser holds no token — the session is an
// HttpOnly cookie), so the one place that already knows where the upstream lives
// is the place that should know what it calls things.
//
// Parsed once at construction, not per request.
func (c ControlPlaneConfig) MoesifPathMapper() func(string) string {
	mappings := parseMoesifPathMappings(c.MoesifPathMappings)
	if len(mappings) == 0 {
		return nil
	}
	// Longest `from` first, so a mapping for "/analytics/id-token" wins over one
	// for "/analytics" whatever order the operator listed them in.
	sort.SliceStable(mappings, func(i, j int) bool {
		return len(mappings[i].from) > len(mappings[j].from)
	})
	return func(p string) string {
		for _, m := range mappings {
			if p == m.from {
				return m.to
			}
			// Whole-segment match only, so "/analytics/id-token-v2" is not caught
			// by a mapping for "/analytics/id-token".
			if rest, ok := strings.CutPrefix(p, m.from+"/"); ok {
				return m.to + "/" + rest
			}
		}
		return p
	}
}

type moesifPathMapping struct{ from, to string }

// parseMoesifPathMappings reads comma-separated "<from>=<to>" pairs. Both sides
// are normalised to a single leading slash and no trailing one, so "analytics/
// id-token = /id_token/" and "/analytics/id-token=/id_token" mean the same.
// Malformed entries are skipped rather than failing startup: a bad mapping
// leaves that route forwarded as-is, which is far easier to diagnose from a 404
// than a portal that will not boot.
func parseMoesifPathMappings(raw string) []moesifPathMapping {
	var mappings []moesifPathMapping
	for _, pair := range strings.Split(raw, ",") {
		from, to, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		from = normalizeMoesifPath(from)
		to = normalizeMoesifPath(to)
		if from == "" || to == "" {
			continue
		}
		mappings = append(mappings, moesifPathMapping{from: from, to: to})
	}
	return mappings
}

func normalizeMoesifPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}
