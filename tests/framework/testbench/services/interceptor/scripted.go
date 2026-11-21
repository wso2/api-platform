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

package interceptor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// phase names the interceptor hook a scripted call targets.
type phase string

const (
	phaseRequest  phase = "request"
	phaseResponse phase = "response"
)

const (
	// controlPrefix starts every control header name; the phase and the control follow it.
	controlPrefix = "x-testbench-interceptor-"
	// maxScriptedDelay keeps a delayed reply well inside the testbench server's 30-second
	// write timeout.
	maxScriptedDelay = 10 * time.Second
	// maxScriptedIDLen bounds the scenario name in the path.
	maxScriptedIDLen = 128
	modeReflect      = "reflect"
	minReplyStatus   = 200
	maxReplyStatus   = 599
	jsonContentType  = "application/json"
	defaultReplyBody = "{}"
)

// controls is the reply a scripted call asked for.
type controls struct {
	reply   []byte
	status  int
	delay   time.Duration
	reflect bool
}

// scripted serves one scripted hook. pathReflect selects reflect mode from the route, for
// attachments whose payload carries no request headers.
func (s *Service) scripted(ph phase, pathReflect bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := validScriptedID(r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !isJSONContentType(r.Header.Get("Content-Type")) {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		headers, err := parsePayload(ph, raw)
		if err != nil {
			http.Error(w, "invalid interceptor payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		ctl, err := parseControls(ph, headers, pathReflect)
		if err != nil {
			http.Error(w, "invalid interceptor control: "+err.Error(), http.StatusBadRequest)
			return
		}
		if ctl.delay > 0 && !wait(r.Context(), ctl.delay) {
			return
		}
		status, body := ctl.status, ctl.reply
		if ctl.reflect {
			if body, err = reflectReply(ph, raw); err != nil {
				http.Error(w, "could not encode the reflect reply", http.StatusInternalServerError)
				return
			}
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", jsonContentType)
		w.WriteHeader(status)
		if _, err := w.Write(body); err != nil {
			log.Printf("interceptor: failed to write scripted reply: %v", err)
		}
	}
}

// validScriptedID accepts a scenario name of letters, digits, dashes and underscores.
func validScriptedID(id string) error {
	if id == "" {
		return fmt.Errorf("scripted id is required")
	}
	if len(id) > maxScriptedIDLen {
		return fmt.Errorf("scripted id is longer than %d characters", maxScriptedIDLen)
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return fmt.Errorf("scripted id may contain only letters, digits, dashes and underscores")
		}
	}
	return nil
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, jsonContentType)
}

// parsePayload checks the payload against the interceptor contract and returns its
// requestHeaders, which carry the controls.
func parsePayload(ph phase, raw []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("not a JSON object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after the JSON object")
	}

	headers, err := decodeStringMap(fields, "requestHeaders")
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"responseHeaders", "interceptorContext"} {
		if _, err := decodeStringMap(fields, name); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"requestBody", "responseBody"} {
		var v string
		if err := decodeField(fields, name, &v); err != nil {
			return nil, err
		}
	}
	var invocation map[string]json.RawMessage
	if err := decodeField(fields, "invocationContext", &invocation); err != nil {
		return nil, err
	}
	if ph == phaseResponse {
		code, ok := fields["responseCode"]
		if !ok {
			return nil, fmt.Errorf("responseCode is required")
		}
		// Decoded through an interface so a quoted number ("200"), which json.Number would
		// accept, is rejected.
		number := json.NewDecoder(bytes.NewReader(code))
		number.UseNumber()
		var v any
		if err := number.Decode(&v); err != nil {
			return nil, fmt.Errorf("responseCode must be an integer")
		}
		n, isNumber := v.(json.Number)
		if !isNumber {
			return nil, fmt.Errorf("responseCode must be an integer")
		}
		if _, err := n.Int64(); err != nil {
			return nil, fmt.Errorf("responseCode must be an integer")
		}
	}
	return headers, nil
}

// decodeField decodes an optional field into dst, rejecting a value of the wrong type. A
// JSON null counts as absent.
func decodeField(fields map[string]json.RawMessage, name string, dst any) error {
	raw, ok := fields[name]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s has the wrong type", name)
	}
	return nil
}

// decodeStringMap decodes an optional object of strings. A null field counts as absent, but a
// null member is rejected: encoding/json would otherwise decode it as an empty string.
func decodeStringMap(fields map[string]json.RawMessage, name string) (map[string]string, error) {
	var members map[string]*string
	if err := decodeField(fields, name, &members); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(members))
	for key, value := range members {
		if value == nil {
			return nil, fmt.Errorf("%s has the wrong type: %q is null", name, key)
		}
		out[key] = *value
	}
	return out, nil
}

// parseControls reads this phase's control headers. Header names match without regard to
// case, so two names that differ only in case are rejected as a duplicate control; controls
// for the other phase are ignored.
func parseControls(ph phase, headers map[string]string, pathReflect bool) (controls, error) {
	ctl := controls{reply: []byte(defaultReplyBody), status: http.StatusOK, reflect: pathReflect}
	prefix := controlPrefix + string(ph) + "-"
	var haveReply, haveStatus bool
	seen := make(map[string]string, len(headers))
	for name, value := range headers {
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		control := strings.TrimPrefix(lower, prefix)
		if other, dup := seen[control]; dup {
			return controls{}, fmt.Errorf("%s and %s set the same control", other, name)
		}
		seen[control] = name
		switch control {
		case "reply":
			decoded, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return controls{}, fmt.Errorf("%s is not standard base64", name)
			}
			ctl.reply, haveReply = decoded, true
		case "status":
			n, err := strconv.Atoi(value)
			if err != nil || n < minReplyStatus || n > maxReplyStatus {
				return controls{}, fmt.Errorf("%s must be an integer from %d to %d", name, minReplyStatus, maxReplyStatus)
			}
			ctl.status, haveStatus = n, true
		case "delay-ms":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || int64(n) > maxScriptedDelay.Milliseconds() {
				return controls{}, fmt.Errorf("%s must be an integer from 0 to %d", name, maxScriptedDelay.Milliseconds())
			}
			ctl.delay = time.Duration(n) * time.Millisecond
		case "mode":
			if !strings.EqualFold(value, modeReflect) {
				return controls{}, fmt.Errorf("%s must be %q", name, modeReflect)
			}
			ctl.reflect = true
		default:
			return controls{}, fmt.Errorf("%s is not a known control", name)
		}
	}
	if ctl.reflect && (haveReply || haveStatus) {
		return controls{}, fmt.Errorf("reflect cannot be combined with a reply or a status")
	}
	return ctl, nil
}

// reflectReply builds the reply that returns the received payload to the client: a direct
// response in the request phase, a body replacement in the response phase.
func reflectReply(ph phase, payload []byte) ([]byte, error) {
	contentType := map[string]string{"Content-Type": jsonContentType}
	body := base64.StdEncoding.EncodeToString(payload)
	var r reply
	if ph == phaseRequest {
		r = reply{DirectRespond: true, ResponseCode: http.StatusOK, HeadersToAdd: contentType, Body: body}
	} else {
		r = reply{HeadersToReplace: contentType, Body: body}
	}
	return json.Marshal(r)
}

// wait blocks for delay unless the request ends first, reporting whether the delay elapsed.
func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
