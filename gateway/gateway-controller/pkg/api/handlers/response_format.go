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

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/httpkit/httputil"
	"gopkg.in/yaml.v3"
)

// yamlMediaTypes are the Accept values that select a YAML response body.
// application/yaml (RFC 9512) is the one written back as Content-Type.
var yamlMediaTypes = []string{"application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml"}

// wantsYAML reports whether the request's Accept header prefers YAML over JSON.
// YAML is chosen only when its best q-value is strictly greater than JSON's, so
// a missing or unparseable header, a wildcard, or a tie keeps the JSON default
// that existing clients rely on.
func wantsYAML(r *http.Request) bool {
	ranges := parseAccept(r.Header.Values("Accept"))
	yamlQ := 0.0
	for _, mt := range yamlMediaTypes {
		if q := acceptQ(ranges, mt); q > yamlQ {
			yamlQ = q
		}
	}
	return yamlQ > acceptQ(ranges, "application/json")
}

// parseAccept maps each media range in the Accept header values to its q-value.
// Entries that do not parse (no type/subtype, bad q) are dropped.
func parseAccept(values []string) map[string]float64 {
	ranges := map[string]float64{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			fields := strings.Split(part, ";")
			mediaRange := strings.ToLower(strings.TrimSpace(fields[0]))
			if typ, sub, ok := strings.Cut(mediaRange, "/"); !ok || typ == "" || sub == "" {
				continue
			}
			q, ok := 1.0, true
			for _, param := range fields[1:] {
				name, val, _ := strings.Cut(strings.TrimSpace(param), "=")
				if strings.EqualFold(strings.TrimSpace(name), "q") {
					parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
					q, ok = parsed, err == nil && parsed >= 0 && parsed <= 1
				}
			}
			if !ok {
				continue
			}
			// An explicit q=0 is recorded too: it excludes the type even when a
			// wildcard would otherwise match it.
			if prev, seen := ranges[mediaRange]; !seen || q > prev {
				ranges[mediaRange] = q
			}
		}
	}
	return ranges
}

// acceptQ returns the q-value for mediaType using the most specific matching
// range (type/subtype, then type/*, then */*), or 0 when nothing matches.
func acceptQ(ranges map[string]float64, mediaType string) float64 {
	typ, _, _ := strings.Cut(mediaType, "/")
	for _, candidate := range []string{mediaType, typ + "/*", "*/*"} {
		if q, ok := ranges[candidate]; ok {
			return q
		}
	}
	return 0
}

// writeResource writes a successful resource body as JSON, or as YAML when the
// request's Accept header prefers it. Only success paths call it; error bodies
// stay JSON, as the OpenAPI spec declares.
//
// The YAML body is converted from the JSON encoding rather than marshalled from
// v directly: yaml.v3 ignores MarshalJSON, which the generated union types
// (Upstream and the per-kind upstreams) rely on. Going through JSON keeps the
// two formats identical, the same reason config.Parser.ParseAPIConfigYAML goes
// through JSON on the way in.
func writeResource(w http.ResponseWriter, r *http.Request, status int, v any) {
	if !wantsYAML(r) {
		httputil.WriteJSON(w, status, v)
		return
	}
	body, err := jsonToYAML(v)
	if err != nil {
		// Same response httputil.WriteJSON gives for an encode failure.
		http.Error(w, `{"code":"internal_error","message":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Add("Vary", "Accept")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// jsonToYAML encodes v as JSON and re-encodes that document as block-style YAML.
// Decoding into a yaml.Node keeps the JSON key order.
func jsonToYAML(v any) ([]byte, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(jsonBytes, &node); err != nil {
		return nil, err
	}
	clearStyle(&node)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// clearStyle drops the flow and quoting styles the JSON source carried, so the
// output is ordinary block YAML. yaml.v3 still quotes any string that would
// otherwise read back as another type (a version like "1.0", a timestamp).
func clearStyle(node *yaml.Node) {
	node.Style = 0
	for _, child := range node.Content {
		clearStyle(child)
	}
}
