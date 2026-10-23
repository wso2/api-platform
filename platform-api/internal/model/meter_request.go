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

package model

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// A meter request in the API Portal's shape: {id, displayName, description, type, typeKey,
// filterFields: [{field, eventKey}]}. The Management Portal's meters are saved here and reach an API
// Portal only when a plan that charges for them is published there. So they are checked here with
// the portal's own rules and messages (its REST API's schema, then meterValidation.js), and the
// portal accepts them as they are.
var (
	// meterRequestFields are the fields a request may set; meterReadOnlyFields only come back in
	// responses.
	meterRequestFields   = []string{"id", "displayName", "description", "type", "typeKey", "filterFields"}
	meterReadOnlyFields  = []string{"planCount", "createdBy", "updatedBy", "createdAt", "updatedAt"}
	meterHandlePattern   = regexp.MustCompile(`^[a-z0-9-]{3,40}$`)
	meterHandleSeparator = regexp.MustCompile(`[^a-z0-9]+`)
	// defaultFilterEventKeys are the event fields our gateway fills for each fixed field; a TOKEN
	// field has no default.
	defaultFilterEventKeys = map[string]string{
		FilterFieldHTTPStatusCode: "response.status",
		FilterFieldHTTPMethod:     "request.verb",
		FilterFieldResource:       "metadata.apiResourceTemplate",
		FilterFieldAPIType:        "metadata.apiType",
	}
)

const (
	maxMeterNameChars        = 255
	maxMeterDescriptionChars = 1023
)

// ParseMeterRequest reads a meter create (create true) or update request and returns the meter it
// describes, trimmed and with each fixed filter field's default event key filled in. On create the
// handle is `id`, or is derived from the display name. On update it is the request's `id` as given,
// empty when there is none, for the caller to compare with the meter's. The returned error's text is
// safe to show.
func ParseMeterRequest(body []byte, create bool) (*Meter, error) {
	keys, values, ok := jsonObject(body)
	if !ok {
		return nil, errors.New("The request body must be a JSON object")
	}
	if _, ok := values["conditions"]; ok {
		return nil, errors.New("A meter has no conditions; each plan price gives its own filters")
	}
	if err := checkMeterShape(keys, values); err != nil {
		return nil, err
	}
	var req struct {
		ID           *string `json:"id"`
		DisplayName  string  `json:"displayName"`
		Description  *string `json:"description"`
		Type         string  `json:"type"`
		TypeKey      *string `json:"typeKey"`
		FilterFields []struct {
			Field    string  `json:"field"`
			EventKey *string `json:"eventKey"`
		} `json:"filterFields"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, errors.New("The request body is not a valid meter")
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || utf8.RuneCountInString(displayName) > maxMeterNameChars {
		return nil, fmt.Errorf("displayName is required and must be at most %d characters", maxMeterNameChars)
	}
	meter := &Meter{DisplayName: displayName, Type: req.Type, FilterFields: []MeterFilterField{}}
	if req.Description != nil {
		meter.Description = *req.Description
	}
	if _, given := values["typeKey"]; given && !isJSONNull(values["typeKey"]) {
		if meter.Type != MeterTypeTokens {
			return nil, errors.New("typeKey applies only to a TOKENS meter")
		}
		key, ok := eventKey(*req.TypeKey)
		if !ok {
			return nil, fmt.Errorf("typeKey must be 1 to %d characters with no spaces", maxEventKeyChars)
		}
		meter.TypeKey = &key
	} else if meter.Type == MeterTypeTokens {
		return nil, errors.New("A TOKENS meter needs a typeKey")
	}
	tokenKeys := make(map[string]bool)
	for _, entry := range req.FilterFields {
		given := ""
		if entry.EventKey != nil {
			given = strings.TrimSpace(*entry.EventKey)
		}
		if entry.Field == FilterFieldToken {
			f := MeterFilterField{Field: FilterFieldToken, EventKey: given}
			if given == "" {
				return nil, errors.New("A Token field needs an eventKey")
			}
			if err := validateTokenField(meter, f, tokenKeys); err != nil {
				return nil, err
			}
			meter.FilterFields = append(meter.FilterFields, f)
			continue
		}
		if slices.ContainsFunc(meter.FilterFields, func(f MeterFilterField) bool { return f.Field == entry.Field }) {
			return nil, fmt.Errorf("filterFields: %s is listed twice", entry.Field)
		}
		key := defaultFilterEventKeys[entry.Field]
		if given != "" {
			var ok bool
			if key, ok = eventKey(given); !ok {
				return nil, fmt.Errorf("The event key of %s must be 1 to %d characters with no spaces",
					filterFieldLabels[entry.Field], maxEventKeyChars)
			}
		}
		meter.FilterFields = append(meter.FilterFields, MeterFilterField{Field: entry.Field, EventKey: key})
	}
	if !create {
		if req.ID != nil {
			meter.Handle = *req.ID
		}
		return meter, nil
	}
	meter.Handle = meterHandleFromName(displayName)
	if req.ID != nil {
		meter.Handle = *req.ID
	}
	if !meterHandlePattern.MatchString(meter.Handle) {
		return nil, errors.New("id must be 3 to 40 lowercase letters, digits or hyphens")
	}
	return meter, nil
}

// checkMeterShape refuses what the API Portal's REST API schema refuses, in the words it answers with:
// a missing required field, a field it does not take or only gives back, a value of the wrong type, a
// name or description too long, an unknown type, and filter fields that are too many or malformed.
func checkMeterShape(keys []string, values map[string]json.RawMessage) error {
	for _, required := range []string{"displayName", "type"} {
		if _, ok := values[required]; !ok {
			return fmt.Errorf("%s is required", required)
		}
	}
	for _, key := range keys {
		switch {
		case slices.Contains(meterReadOnlyFields, key):
			return fmt.Errorf("%s is read-only", key)
		case !slices.Contains(meterRequestFields, key):
			return fmt.Errorf("%s is not a known field", key)
		}
	}
	for _, field := range []string{"id", "displayName", "type"} {
		if raw, ok := values[field]; ok && !isJSONString(raw) {
			return fmt.Errorf("%s must be a string", field)
		}
	}
	for _, field := range []string{"description", "typeKey"} {
		if raw, ok := values[field]; ok && !isJSONString(raw) && !isJSONNull(raw) {
			return fmt.Errorf("%s must be a string", field)
		}
	}
	var name, description string
	_ = json.Unmarshal(values["displayName"], &name)
	if utf8.RuneCountInString(name) > maxMeterNameChars {
		return fmt.Errorf("displayName must be at most %d characters", maxMeterNameChars)
	}
	if raw, ok := values["description"]; ok && !isJSONNull(raw) {
		_ = json.Unmarshal(raw, &description)
		if utf8.RuneCountInString(description) > maxMeterDescriptionChars {
			return fmt.Errorf("description must be at most %d characters", maxMeterDescriptionChars)
		}
	}
	var meterType string
	_ = json.Unmarshal(values["type"], &meterType)
	if !slices.Contains(meterTypes, meterType) {
		return fmt.Errorf("type must be one of %s", strings.Join(meterTypes, ", "))
	}
	raw, ok := values["filterFields"]
	if !ok {
		return nil
	}
	var entries []json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || isJSONNull(raw) {
		return errors.New("filterFields must be an array")
	}
	if len(entries) > maxMeterFilterFields {
		return fmt.Errorf("filterFields must have at most %d entries", maxMeterFilterFields)
	}
	for i, entry := range entries {
		where := fmt.Sprintf("filterField %d", i+1)
		keys, values, ok := jsonObject(entry)
		if !ok {
			return fmt.Errorf("%s must be an object", where)
		}
		if _, ok := values["field"]; !ok {
			return fmt.Errorf("field of %s is required", where)
		}
		for _, key := range keys {
			if key != "field" && key != "eventKey" {
				return fmt.Errorf("%s of %s is not a known field", key, where)
			}
		}
		var field string
		if !isJSONString(values["field"]) {
			return fmt.Errorf("field of %s must be a string", where)
		}
		_ = json.Unmarshal(values["field"], &field)
		if !slices.Contains(filterFields, field) {
			return fmt.Errorf("field of %s must be one of %s", where, strings.Join(filterFields, ", "))
		}
		if key, ok := values["eventKey"]; ok && !isJSONString(key) && !isJSONNull(key) {
			return fmt.Errorf("eventKey of %s must be a string", where)
		}
	}
	return nil
}

// eventKey returns an event key as stored, trimmed, and whether it is 1 to 255 characters with no
// whitespace or control characters.
func eventKey(raw string) (string, bool) {
	key := strings.TrimSpace(raw)
	return key, validEventKey(key)
}

// meterHandleFromName derives a handle from a display name, as the API Portal does: "Successful calls"
// is "successful-calls". A name with too few letters or digits gets "meter-" and six random hex digits.
func meterHandleFromName(displayName string) string {
	slug := strings.Trim(meterHandleSeparator.ReplaceAllString(strings.ToLower(displayName), "-"), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	if len(slug) >= 3 {
		return slug
	}
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	return "meter-" + hex.EncodeToString(suffix)
}

// jsonObject returns the names of a JSON object's fields in the order written, and their values; ok
// is false when raw is not a JSON object.
func jsonObject(raw []byte) (keys []string, values map[string]json.RawMessage, ok bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, nil, false
	}
	values = map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, nil, false
		}
		key, _ := token.(string)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, nil, false
		}
		keys = append(keys, key)
		values[key] = value
	}
	return keys, values, true
}

func isJSONString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}
