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

package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// EnvRecordExpectedErrors records expected error responses instead of comparing against them.
//
// They are recorded from the policies as they were before a change, so the changed policies can
// be proven to leave every error response unchanged. One file serves every Gateway version; a
// version whose response genuinely differs gets its own file under a directory named for it.
const EnvRecordExpectedErrors = "IT_RECORD_EXPECTED_ERRORS"

// EnvAcceptErrorChanges records a response that differs from its expected error response as
// an accepted change, beside it as <id>.accepted.json, instead of failing.
//
// It is how an intended difference is recorded: the accepted file is reviewed like any other
// change, and a response matching either file passes. A response matching neither still fails.
const EnvAcceptErrorChanges = "IT_ACCEPT_ERROR_CHANGES"

// acceptedSuffix names the accepted-change file recorded beside an expected error response.
const acceptedSuffix = ".accepted.json"

// headerPin says how an expected error response holds a response header.
type headerPin int

const (
	// pinValue compares the value, after the volatile-value masks.
	pinValue headerPin = iota
	// pinPresence compares only that the header is present: its value depends on the clock.
	pinPresence
	// pinRateLimit compares the value with each seconds-until-reset parameter (t=) masked.
	pinRateLimit
)

// expectedErrorHeaders are the response headers an expected error response pins, and how.
// Each one is part of a policy's client-visible contract; transport and tracing headers are
// not, and vary per request.
//
// The rate-limit set follows draft-ietf-httpapi-ratelimit-headers and the X-RateLimit
// convention. Limits, remaining counts, quota names and policies are deterministic for a
// fresh API and a fixed request count. Reset times and Retry-After are aligned to the clock,
// so only their presence is pinned — a fault path that dropped them still fails.
var expectedErrorHeaders = map[string]headerPin{
	"content-type":          pinValue,
	"www-authenticate":      pinValue,
	"x-error-code":          pinValue,
	"location":              pinValue,
	"x-ratelimit-limit":     pinValue,
	"x-ratelimit-remaining": pinValue,
	"x-ratelimit-quota":     pinValue,
	"ratelimit-policy":      pinValue,
	"ratelimit":             pinRateLimit,
	"x-ratelimit-reset":     pinPresence,
	"retry-after":           pinPresence,
}

// rateLimitReset matches the seconds-until-reset parameter of a RateLimit header field.
var rateLimitReset = regexp.MustCompile(`\bt=\d+`)

// expectedErrorID restricts identifiers to names that cannot escape the directory.
var expectedErrorID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// expectedErrorVersion restricts the Gateway version directory of an override in the same way.
var expectedErrorVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var (
	uuidPattern      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	// hexIDPattern matches a 32-digit hex identifier, as a UUID written without dashes.
	hexIDPattern = regexp.MustCompile(`\b[0-9a-f]{32}\b`)
	// createdPattern matches a "created" Unix time, as streamed chat-completion chunks carry.
	createdPattern = regexp.MustCompile(`"created":\d+`)
)

// normalizedErrorResponse is the normalized form of a response, as a file stores it.
type normalizedErrorResponse struct {
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     json.RawMessage   `json:"body,omitempty"`
	BodyText *string           `json:"bodyText,omitempty"`
}

// responseMatchesExpectedError asserts the published response equals the expected error
// response for the Gateway version under test, or records it when EnvRecordExpectedErrors is
// set.
func (b *Base) responseMatchesExpectedError(ctx context.Context, id string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	version, err := b.topo.ComponentVersion("platform-gateway")
	if err != nil {
		return err
	}
	path, err := expectedErrorPath(b.featureRoot, version, id)
	if err != nil {
		return err
	}
	suffix := ""
	if g, genErr := unique.Of(ctx); genErr == nil {
		suffix = g.Suffix()
	}
	got, err := normalizeErrorResponse(resp.StatusCode, resp.Headers, resp.Body, suffix)
	if err != nil {
		return err
	}
	return matchExpectedError(path, got, expectedErrorModeRequested())
}

// expectedErrorMode says what the expected error response step does with a response.
type expectedErrorMode int

const (
	// expectedErrorCompare compares against the expected error response, or its accepted change.
	expectedErrorCompare expectedErrorMode = iota
	// expectedErrorRecord records the response as the expected error response.
	expectedErrorRecord
	// expectedErrorAccept records a differing response as an accepted change.
	expectedErrorAccept
)

func expectedErrorModeRequested() expectedErrorMode {
	switch {
	case envFlag(EnvRecordExpectedErrors):
		return expectedErrorRecord
	case envFlag(EnvAcceptErrorChanges):
		return expectedErrorAccept
	default:
		return expectedErrorCompare
	}
}

func envFlag(name string) bool {
	v := strings.TrimSpace(os.Getenv(name))
	return v == "1" || strings.EqualFold(v, "true")
}

// expectedErrorPath resolves the file for id under root: the Gateway version's own file when
// one exists, otherwise the file every version shares.
func expectedErrorPath(root, version, id string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("expected errors: feature root is required")
	}
	if !expectedErrorID.MatchString(id) {
		return "", fmt.Errorf("expected errors: id %q must be lowercase words joined by '-'", id)
	}
	if !expectedErrorVersion.MatchString(version) || strings.Contains(version, "..") {
		return "", fmt.Errorf("expected errors: gateway version %q is not usable as a directory name", version)
	}
	dir := filepath.Join(root, "resources", "expected-error-responses")
	override := filepath.Join(dir, version, id+".json")
	if _, err := os.Stat(override); err == nil {
		return override, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("expected errors: reading %s: %w", override, err)
	}
	return filepath.Join(dir, id+".json"), nil
}

// normalizeErrorResponse reduces a response to what is compared: the status, the pinned
// headers and the body, with per-request values masked.
//
// A JSON body is stored structurally, so key order and whitespace do not count as a change;
// any other body is compared as exact text.
func normalizeErrorResponse(status int, headers http.Header, body []byte, suffix string) (normalizedErrorResponse, error) {
	out := normalizedErrorResponse{Status: status}
	for name, pin := range expectedErrorHeaders {
		values := headers.Values(name)
		if len(values) == 0 {
			continue
		}
		if out.Headers == nil {
			out.Headers = make(map[string]string, len(expectedErrorHeaders))
		}
		value := maskVolatile(strings.Join(values, ", "), suffix)
		switch pin {
		case pinPresence:
			value = "<present>"
		case pinRateLimit:
			value = rateLimitReset.ReplaceAllString(value, "t=<seconds>")
		}
		out.Headers[name] = value
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return out, nil
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err == nil {
		masked, err := json.Marshal(maskJSON(parsed, suffix))
		if err != nil {
			return normalizedErrorResponse{}, fmt.Errorf("expected errors: encoding the normalized body: %w", err)
		}
		out.Body = masked
		return out, nil
	}
	text := maskVolatile(string(body), suffix)
	out.BodyText = &text
	return out, nil
}

// maskJSON masks every string in a decoded JSON value. Map keys are kept: they are the
// response's structure, not its per-request data.
func maskJSON(value any, suffix string) any {
	switch v := value.(type) {
	case string:
		return maskVolatile(v, suffix)
	case []any:
		for i := range v {
			v[i] = maskJSON(v[i], suffix)
		}
		return v
	case map[string]any:
		for key, item := range v {
			if _, isNumber := item.(float64); isNumber && key == "created" {
				v[key] = "<unix-time>"
				continue
			}
			v[key] = maskJSON(item, suffix)
		}
		return v
	default:
		return v
	}
}

// maskVolatile replaces values that differ on every request — identifiers, timestamps and
// the runner's generated resource names — with stable markers.
func maskVolatile(s, suffix string) string {
	s = uuidPattern.ReplaceAllString(s, "<uuid>")
	s = timestampPattern.ReplaceAllString(s, "<timestamp>")
	s = hexIDPattern.ReplaceAllString(s, "<hex-id>")
	s = createdPattern.ReplaceAllString(s, `"created":"<unix-time>"`)
	if suffix != "" {
		generated := regexp.MustCompile(`[_-]` + regexp.QuoteMeta(suffix) + `[_-]\d+`)
		s = generated.ReplaceAllString(s, "<unique>")
	}
	return s
}

// matchExpectedError compares got against the expected error response at path and its
// accepted change, or records it, as mode says.
//
// A missing expected error response is a failure rather than an implicit recording: it is the
// baseline a release is held to, and creating it from the build under test would compare
// that build with itself. Accepting needs one too, for the same reason.
func matchExpectedError(path string, got normalizedErrorResponse, mode expectedErrorMode) error {
	encoded, err := encodeErrorResponse(got)
	if err != nil {
		return err
	}
	if mode == expectedErrorRecord {
		return writeExpectedError(path, encoded)
	}
	expected, err := readExpectedError(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("expected errors: %s has not been recorded; record it from the policies before the change with %s=1", path, EnvRecordExpectedErrors)
	}
	if err != nil {
		return err
	}
	if bytes.Equal(expected, encoded) {
		return nil
	}
	accepted := strings.TrimSuffix(path, ".json") + acceptedSuffix
	if mode == expectedErrorAccept {
		return writeExpectedError(accepted, encoded)
	}
	acceptedChange, err := readExpectedError(accepted)
	switch {
	case err == nil && bytes.Equal(acceptedChange, encoded):
		return nil
	case err == nil:
		return fmt.Errorf("expected errors: response matches neither %s nor its accepted change %s\nexpected:\n%s\naccepted:\n%s\ngot:\n%s",
			path, accepted, expected, acceptedChange, encoded)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return fmt.Errorf("expected errors: response differs from %s\nexpected:\n%s\ngot:\n%s", path, expected, encoded)
}

// readExpectedError reads and re-encodes the file at path, so a hand-edited file compares by its
// content rather than its formatting.
func readExpectedError(path string) ([]byte, error) {
	recorded, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("expected errors: reading %s: %w", path, err)
	}
	var want normalizedErrorResponse
	if err := json.Unmarshal(recorded, &want); err != nil {
		return nil, fmt.Errorf("expected errors: %s is not an expected error response: %w", path, err)
	}
	return encodeErrorResponse(want)
}

func writeExpectedError(path string, encoded []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("expected errors: creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("expected errors: writing %s: %w", path, err)
	}
	return nil
}

// encodeErrorResponse renders an expected error response deterministically: indented, with object keys sorted at
// every depth (encoding/json sorts map keys), so a recorded file and a fresh response
// compare byte for byte.
func encodeErrorResponse(g normalizedErrorResponse) ([]byte, error) {
	canonical := map[string]any{"status": g.Status}
	if len(g.Headers) > 0 {
		canonical["headers"] = g.Headers
	}
	if len(g.Body) > 0 {
		var body any
		if err := json.Unmarshal(g.Body, &body); err != nil {
			return nil, fmt.Errorf("expected errors: body is not JSON: %w", err)
		}
		canonical["body"] = body
	}
	if g.BodyText != nil {
		canonical["bodyText"] = *g.BodyText
	}
	encoded, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("expected errors: encoding: %w", err)
	}
	return append(encoded, '\n'), nil
}
