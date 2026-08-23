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

// Package migrationcore holds the ONE implementation of the Platform API v1→v2
// per-row transform + idempotent v2 write, shared by the batch migrator
// (cmd/dbmigrate) and the live dual-write intermediate v1 build. It contains only
// pure per-row logic and idempotent upserts — no table iteration, quarantine
// bookkeeping, checkpointing, or DDL init (those stay in the batch caller).
package migrationcore

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// ReinterpretTZ treats t's wall-clock as being in loc and returns the equivalent
// UTC instant. v1 stores naive TIMESTAMP; pgx hands them back with a UTC location,
// so for source-tz=UTC this is identity. Used identically by migrate and verify.
func ReinterpretTZ(t time.Time, loc *time.Location) time.Time {
	if loc == nil || loc == time.UTC {
		return t.UTC()
	}
	y, mo, d := t.Date()
	h, mi, s := t.Clock()
	return time.Date(y, mo, d, h, mi, s, t.Nanosecond(), loc).UTC()
}

// BoolToSmallint maps a SQL boolean to v2's SMALLINT convention.
func BoolToSmallint(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TruncateStr truncates s to max runes, reporting whether truncation occurred.
func TruncateStr(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	return string(r[:max]), true
}

var (
	slugInvalidChars = regexp.MustCompile(`[^a-z0-9\-_ ]`)
	slugMultiHyphen  = regexp.MustCompile(`-+`)
)

// Slug reproduces the deterministic part of internal/utils.sanitizeToHandle (no
// random padding). Keep in sync with internal/utils/handle.go.
func Slug(s string) string {
	h := strings.ToLower(s)
	h = strings.ReplaceAll(h, " ", "-")
	h = strings.ReplaceAll(h, "_", "-")
	h = slugInvalidChars.ReplaceAllString(h, "")
	h = slugMultiHyphen.ReplaceAllString(h, "-")
	h = strings.Trim(h, "-")
	if len(h) > 40 {
		h = h[:40]
		h = strings.TrimRight(h, "-")
	}
	return h
}

// ParseTransport parses the v1 `transport` column (a JSON array stored as TEXT,
// e.g. `["https","wss"]`). Empty/invalid input yields a nil slice.
func ParseTransport(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// jsonTagSet returns the set of top-level json field names on v (a struct or
// pointer to one), ignoring "-" and honoring embedded structs.
func jsonTagSet(v any) map[string]bool {
	set := map[string]bool{}
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return set
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			for k := range jsonTagSet(reflect.New(f.Type).Interface()) {
				set[k] = true
			}
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" {
			set[f.Name] = true
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		set[name] = true
	}
	return set
}

// TopLevelUnknownFields returns the top-level keys present in the v1 config JSON
// that the v2 struct target does NOT have — fields the permissive unmarshal would
// silently drop (§E discovery). Catches struct-level unknowns only.
func TopLevelUnknownFields(raw []byte, target any) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}
	known := jsonTagSet(target)
	var unknown []string
	for k := range top {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	return unknown
}

// ReshapeRestAPIConfig unmarshals a v1 rest_apis.configuration blob into the v2
// RestAPIConfig, injects the moved `transport` column, and re-marshals to BYTEA.
func ReshapeRestAPIConfig(raw []byte, transportCol string) (out []byte, cfg model.RestAPIConfig, unknown []string, err error) {
	unknown = TopLevelUnknownFields(raw, &cfg)
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return nil, cfg, unknown, err
	}
	if tr := ParseTransport(transportCol); len(tr) > 0 {
		cfg.Transport = tr
	}
	out, err = json.Marshal(&cfg)
	return out, cfg, unknown, err
}

// ReshapeWebSubConfig reshapes a v1 WebSub config into the v2 struct. Beyond the
// transport injection, two STRUCTURAL v1→v2 differences (discovered from real data,
// contradicting the "strict subset" premise) are handled losslessly:
//
//   - channels: v1 stores an ARRAY [{request:{name,method}}]; v2 wants a
//     map[string]WebSubChannel keyed by channel name. → key = request.name,
//     value = empty WebSubChannel (method is always SUBSCRIBE, carries no info).
//   - policies (top-level, legacy): folded into allChannels — object-form
//     {event:[p]} → allChannels.event.policies; flat array [p] (whole-API auth) →
//     allChannels.on_subscription.policies.
//
// notes describes any structural remap applied (recorded as a flag by the caller).
func ReshapeWebSubConfig(raw []byte, transportCol string) (out []byte, cfg model.WebSubAPIConfiguration, unknown, notes []string, err error) {
	fixed, notes, err := PreprocessWebSubRaw(raw)
	if err != nil {
		return nil, cfg, nil, notes, err
	}
	unknown = TopLevelUnknownFields(fixed, &cfg)
	if err = json.Unmarshal(fixed, &cfg); err != nil {
		return nil, cfg, unknown, notes, err
	}
	if tr := ParseTransport(transportCol); len(tr) > 0 {
		cfg.Transport = tr
	}
	out, err = json.Marshal(&cfg)
	return out, cfg, unknown, notes, err
}

// jsonKind returns the first non-whitespace byte of a JSON value ('{', '[', ...),
// or 0 for empty/whitespace-only input.
func jsonKind(raw []byte) byte {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}
	return 0
}

// PreprocessWebSubRaw applies the two structural fixups (channels array→map,
// top-level policies→allChannels) at the JSON level, before struct unmarshal.
func PreprocessWebSubRaw(raw []byte) ([]byte, []string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return raw, nil, err
	}
	var notes []string

	// channels: array → map keyed by request.name.
	if ch, ok := top["channels"]; ok && jsonKind(ch) == '[' {
		var arr []struct {
			Request struct {
				Name   string `json:"name"`
				Method string `json:"method"`
			} `json:"request"`
		}
		if err := json.Unmarshal(ch, &arr); err == nil {
			m := map[string]json.RawMessage{}
			for _, e := range arr {
				if e.Request.Name != "" {
					m[e.Request.Name] = json.RawMessage("{}")
				}
			}
			b, _ := json.Marshal(m)
			top["channels"] = b
			notes = append(notes, "channels[] -> map (keyed by request.name)")
		}
	}

	// policies (top-level legacy) → allChannels.
	if pol, ok := top["policies"]; ok && jsonKind(pol) != 0 && string(pol) != "null" {
		allch := map[string]json.RawMessage{}
		if ac, ok := top["allChannels"]; ok && jsonKind(ac) == '{' {
			_ = json.Unmarshal(ac, &allch)
		}
		wrap := func(policyArray json.RawMessage) json.RawMessage {
			b, _ := json.Marshal(map[string]json.RawMessage{"policies": policyArray})
			return b
		}
		switch jsonKind(pol) {
		case '{': // object-form: {event: [policy...]}
			var byEvent map[string]json.RawMessage
			if json.Unmarshal(pol, &byEvent) == nil {
				for event, arr := range byEvent {
					if _, exists := allch[event]; !exists {
						allch[event] = wrap(arr)
					}
				}
				notes = append(notes, "policies{event:[...]} -> allChannels")
			}
		case '[': // flat array of whole-API policies (auth) → on_subscription
			var arr []json.RawMessage
			if json.Unmarshal(pol, &arr) == nil && len(arr) > 0 {
				if _, exists := allch["on_subscription"]; !exists {
					allch["on_subscription"] = wrap(pol)
				}
				notes = append(notes, "policies[] (whole-API) -> allChannels.on_subscription")
			}
		}
		delete(top, "policies")
		if len(allch) > 0 {
			b, _ := json.Marshal(allch)
			top["allChannels"] = b
		}
	}

	out, err := json.Marshal(top)
	return out, notes, err
}

// ReshapeWebBrokerConfig mirrors ReshapeRestAPIConfig for WebBroker configs (a
// strict subset of v2 + transport).
func ReshapeWebBrokerConfig(raw []byte, transportCol string) (out []byte, cfg model.WebBrokerAPIConfiguration, unknown []string, err error) {
	unknown = TopLevelUnknownFields(raw, &cfg)
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return nil, cfg, unknown, err
	}
	if tr := ParseTransport(transportCol); len(tr) > 0 {
		cfg.Transport = tr
	}
	out, err = json.Marshal(&cfg)
	return out, cfg, unknown, err
}

// RemarshalConfig unmarshals a v1 config blob into the given v2 config struct
// (target must be a pointer) and re-marshals it to BYTEA, returning any dropped
// top-level fields. Used for LLM provider/proxy and MCP configs (no field moves).
func RemarshalConfig(raw []byte, target any) (out []byte, unknown []string, err error) {
	unknown = TopLevelUnknownFields(raw, target)
	if err = json.Unmarshal(raw, target); err != nil {
		return nil, unknown, err
	}
	out, err = json.Marshal(target)
	return out, unknown, err
}
