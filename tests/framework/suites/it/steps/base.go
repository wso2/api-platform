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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// Base holds what EVERY step file needs from the running topology. Domain step files embed it,
// so a domain method reaches this plumbing by promotion and base never reaches back.
type Base struct {
	topo   *frameworkruntime.Topology
	funnel *httpx.Funnel
}

// Request-shaping state, scenario-scoped. Base assembles every request, so it owns these keys.
const (
	keyAuthHeader   = "authHeader"
	keyExtraHeaders = "extraHeaders"
	keyRequestHost  = "requestHost"
)

// exposedContextValues are the names a feature may reference with ${CTX:name}, mapped to the
// scenario-context key each resolves to.
//
// Populated by the domain that owns each value, so base never names one. An explicit allowlist
// rather than the raw context: a feature addressing internal keys directly would couple its text
// to Go constants and expose every value the framework happens to store.
var exposedContextValues = map[string]string{}

// exposeContextValue declares one name a feature may reference with ${CTX:name}.
func exposeContextValue(name, key string) {
	exposedContextValues[name] = key
}

// exposeContextValues installs the resolver features use for ${CTX:name}.
func exposeContextValues() {
	unique.ContextValue = func(ctx context.Context, name string) (string, error) {
		key, ok := exposedContextValues[name]
		if !ok {
			return "", fmt.Errorf("%q is not an exposed context value; this suite exposes %v",
				name, sortedExposedNames())
		}
		v, ok := tcontext.Get(ctx, key)
		if !ok {
			return "", fmt.Errorf("%q is not set in this scenario", name)
		}
		s, _ := v.(string)
		if s == "" {
			return "", fmt.Errorf("%q is empty in this scenario", name)
		}
		return s, nil
	}
}

func sortedExposedNames() []string {
	out := make([]string, 0, len(exposedContextValues))
	for n := range exposedContextValues {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// BasicAuthHeader builds the Authorization value for the credential base assembles every request
// with. Exported so the suite's cleanup deleters authenticate as the steps do.
func BasicAuthHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// scenarioLabel describes the current runner, for a cleanup warning that names where a leaked
// resource came from.
func scenarioLabel(ctx context.Context) string {
	if local, ok := tcontext.LocalOf(ctx); ok {
		return local.Runner()
	}
	return "unknown runner"
}

func (b *Base) registerBaseSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the response status code should be (\d+)$`, b.statusCodeIs)
	sc.Step(`^the response should be (valid JSON|empty)$`, b.responseShapeIs)
	sc.Step(`^the response body should be:$`, b.responseBodyIs)
	sc.Step(`^the response body should contain "([^"]*)"$`, b.responseContains)
	sc.Step(`^the response body should not contain "([^"]*)"$`, b.responseNotContains)
	sc.Step(`^the response body should match pattern "([^"]*)"$`, b.responseMatchesPattern)
	sc.Step(`^the response header "([^"]*)" should be "([^"]*)"$`, b.responseHeaderEquals)
	sc.Step(`^the response header "([^"]*)" should contain "([^"]*)"$`, b.responseHeaderContains)
	sc.Step(`^the response header "([^"]*)" should (exist|not exist)$`, b.responseHeaderPresence)
	sc.Step(`^the response should contain echoed header "([^"]*)" with value "([^"]*)"$`,
		b.echoedHeaderEquals)
	sc.Step(`^the response should contain echoed header "([^"]*)" with exact value:$`,
		b.echoedHeaderEqualsDoc)
	sc.Step(`^the response should not contain echoed header "([^"]*)"$`, b.echoedHeaderAbsent)
	sc.Step(`^the JSON response should have field "([^"]*)"$`, b.jsonFieldExists)
	sc.Step(`^the JSON response field "([^"]*)" should not exist$`, b.jsonFieldAbsent)
	sc.Step(`^the JSON response field "([^"]*)" should be (\d+)$`, b.jsonFieldIsNumber)
	sc.Step(`^the JSON response field "([^"]*)" should be "([^"]*)"$`, b.jsonFieldIs)
	sc.Step(`^the JSON response field "([^"]*)" should be:$`, b.jsonFieldIsDoc)
	sc.Step(`^the JSON response field "([^"]*)" should contain "([^"]*)"$`, b.jsonFieldContains)
	sc.Step(`^the JSON response field "([^"]*)" should be greater than (\d+)$`,
		b.jsonFieldGreaterThan)
	sc.Step(`^I set header "([^"]*)" to "([^"]*)"$`, b.setHeader)
	sc.Step(`^I reset the request$`, b.resetRequest)
	sc.Step(`^I set request host to "([^"]*)"$`, b.setRequestHost)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)"$`, b.sendRequest)
	sc.Step(`^I send (\d+) "([^"]*)" requests to "([^"]*)"$`, b.sendRepeated)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with body:$`, b.sendRequestWithBody)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until status (\d+)$`, b.sendUntilStatus)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until status (\d+) with body:$`,
		b.sendUntilStatusWithBody)
}

// sendRequest invokes a data-plane path once, without retrying.
func (b *Base) sendRequest(ctx context.Context, method, path string) error {
	return b.sendRequestWithBody(ctx, method, path, nil)
}

// sendRequestWithBody invokes a data-plane path once, with a body when one is given.
func (b *Base) sendRequestWithBody(
	ctx context.Context, method, path string, body *godog.DocString,
) error {
	resolved, err := unique.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := b.gatewayURL(resolved)
	if err != nil {
		return err
	}

	headers := b.scenarioHeaders(ctx)
	var payload []byte
	if body != nil {
		content, expErr := unique.Expand(ctx, body.Content)
		if expErr != nil {
			return expErr
		}
		payload = []byte(content)
		// Default rather than override: a feature that sets Content-Type explicitly is testing
		// how the gateway treats it. Bodyless requests get none at all.
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}

	return b.invokeWith(ctx, strings.ToUpper(method), url, headers, payload)
}

// sendRepeated invokes a data-plane path n times, without retrying.
//
// Each call publishes, so the assertions that follow see the LAST response — which is what the
// counted rate-limit scenarios check: n requests to exhaust a quota, then the status of the nth.
// Any header the scenario set applies to every one of them, like a single send.
func (b *Base) sendRepeated(ctx context.Context, n int, method, path string) error {
	for i := 0; i < n; i++ {
		if err := b.sendRequest(ctx, method, path); err != nil {
			return fmt.Errorf("request %d of %d: %w", i+1, n, err)
		}
	}
	return nil
}

// sendUntilStatus invokes a data-plane path until it answers with the wanted status.
func (b *Base) sendUntilStatus(ctx context.Context, method, path string, want int) error {
	return b.sendUntilStatusWithBody(ctx, method, path, want, nil)
}

// sendUntilStatusWithBody polls a data-plane path, with a body when one is given, until it answers with the wanted status.
func (b *Base) sendUntilStatusWithBody(
	ctx context.Context, method, path string, want int, body *godog.DocString,
) error {
	resolved, err := unique.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := b.gatewayURL(resolved)
	if err != nil {
		return err
	}

	headers := b.scenarioHeaders(ctx)
	var payload []byte
	if body != nil {
		content, expErr := unique.Expand(ctx, body.Content)
		if expErr != nil {
			return expErr
		}
		payload = []byte(content)
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}

	return retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) {
			r, sendErr := b.funnel.Send(ctx, httpx.Request{
				Method: strings.ToUpper(method), URL: url, Body: payload, Headers: headers,
				// Empty when the scenario set no host, and the client drops an empty value,
				// so this honours a sticky host without imposing one.
				Host: b.requestHost(ctx),
			})
			if sendErr != nil {
				// A route that is not programmed yet closes the connection instead of answering.
				return nil, retry.Transient(sendErr)
			}
			return r, nil
		},
		func(r *httpx.Response) bool { return r != nil && r.StatusCode == want },
		fmt.Sprintf("waiting for %s %s to return %d", strings.ToUpper(method), url, want))
}

// sendUntilHeader polls a data-plane path until a response header carries the wanted value.
//
// The companion to sendUntilStatus, for a fact a status cannot express. A status poll sees 200
// both before and after a policy update, so it cannot tell which configuration is serving; a
// header can. X-RateLimit-Limit reports the CONFIGURED limit and X-RateLimit-Remaining the
// consumption, so polling either distinguishes "the engine is still on the pre-update config"
// from "the new one is live".
//
// Costs exactly one request against whatever bucket is live once the condition holds. Polls
// taken before that are charged to the OLD bucket, so they spend nothing the scenario counts.
func (b *Base) sendUntilHeader(ctx context.Context, method, path, name, want string) error {
	resolved, err := unique.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := b.gatewayURL(resolved)
	if err != nil {
		return err
	}
	wantValue, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	headers := b.scenarioHeaders(ctx)

	return retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) {
			r, sendErr := b.funnel.Send(ctx, httpx.Request{
				Method: strings.ToUpper(method), URL: url, Headers: headers,
				Host: b.requestHost(ctx),
			})
			if sendErr != nil {
				// A route that is not programmed yet closes the connection instead of answering.
				return nil, retry.Transient(sendErr)
			}
			return r, nil
		},
		// Get is case-insensitive per RFC 7230, matching responseHeaderEquals.
		func(r *httpx.Response) bool { return r != nil && r.Headers.Get(name) == wantValue },
		fmt.Sprintf("waiting for %s %s to answer with header %s: %q",
			strings.ToUpper(method), url, name, wantValue))
}

func (b *Base) statusCodeIs(ctx context.Context, want int) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != want {
		// The exact value, never a widened range: a permissive check swallows a regression
		// that returns a different code in the same class.
		return fmt.Errorf("expected status %d, got %s", want, resp.Describe())
	}
	return nil
}

// responseShapeIs asserts the body has the named shape. The regex enumerates the shapes, so an
// unknown one is an undefined step rather than a silent pass.
func (b *Base) responseShapeIs(ctx context.Context, shape string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	switch shape {
	case "valid JSON":
		// Not gated on success: a negative scenario asserts the ERROR body is well-formed JSON.
		if !resp.HasBody() {
			return fmt.Errorf("expected a JSON body, got none: %s", resp.Describe())
		}
		var parsed any
		if err := json.Unmarshal(resp.Body, &parsed); err != nil {
			return fmt.Errorf("response is not valid JSON: %w (%s)", err, resp.Describe())
		}
	case "empty":
		// Distinct from containing the empty string, which every response satisfies.
		if len(resp.Body) != 0 {
			return fmt.Errorf("expected an empty body, got %d bytes: %s", len(resp.Body), resp.Describe())
		}
	default:
		return fmt.Errorf("unknown response shape %q", shape)
	}
	return nil
}

// responseBodyIs asserts the whole body equals the given text, byte for byte.
func (b *Base) responseBodyIs(ctx context.Context, want *godog.DocString) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	expected, err := unique.Expand(ctx, want.Content)
	if err != nil {
		return err
	}
	// No trimming: a body that gained or lost surrounding whitespace changed, and this step
	// exists precisely to catch what a substring match cannot.
	if got := resp.Text(); got != expected {
		return fmt.Errorf("expected the body to be %q, got %q (%s)", expected, got, resp.Describe())
	}
	return nil
}

// responseContains asserts the value appears somewhere in the body.
//
// Deliberately whole-body, and kept for the cases a field path cannot state: presence of a
// resource in a collection whose element order is not stable — /config_dump reorders its apis[]
// between runs, and /rest-apis is shared by every runner in the block. Prefer
// `the JSON response field "..." should be` wherever the path is stable.
func (b *Base) responseContains(ctx context.Context, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	if !strings.Contains(resp.Text(), resolved) {
		return fmt.Errorf("expected the body to contain %q, got %s", resolved, resp.Describe())
	}
	return nil
}

// responseNotContains asserts the value appears NOWHERE in the body.
//
// Whole-body by necessity, not convenience: the scenarios using it check that a masked PII value
// or a resolved secret never surfaces, and the whole risk is it appearing in a field nobody
// thought to name. A field-path equivalent would only prove the one place checked is clean.
func (b *Base) responseNotContains(ctx context.Context, unwanted string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, unwanted)
	if err != nil {
		return err
	}
	if strings.Contains(resp.Text(), resolved) {
		return fmt.Errorf("expected the body NOT to contain %q, got %s", resolved, resp.Describe())
	}
	return nil
}

func (b *Base) jsonFieldExists(ctx context.Context, field string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return fmt.Errorf("parsing the response as JSON: %w (%s)", err, resp.Describe())
	}
	if _, ok := traverseJSON(data, field); !ok {
		// Name the places that key DOES occur. A path is easy to get wrong by a level, and
		// "it does not exist" sends you reading the whole body to find out it was nested.
		if where := keyPaths(data, "", field); len(where) > 0 {
			return fmt.Errorf("expected field %q to exist; the response carries that key at %v (%s)",
				field, where, resp.Describe())
		}
		return fmt.Errorf("expected field %q to exist, got %s", field, resp.Describe())
	}
	return nil
}

// keyPaths lists every dotted path whose final segment is name.
func keyPaths(node any, path, name string) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if k == name {
				out = append(out, p)
			}
			out = append(out, keyPaths(child, p, name)...)
		}
	case []any:
		for i, child := range v {
			out = append(out, keyPaths(child, fmt.Sprintf("%s[%d]", path, i), name)...)
		}
	}
	return out
}

// jsonFieldExists asserts a field is present, whatever its value.
// jsonFieldAbsent asserts a dotted path is NOT present in the response.
//
// Absent, not merely empty. The scenarios using this check that a secret's resolved value never
// appears in a management-API response at all — a field present with an empty string would mean
// the product knows about the value and chose to blank it, which is a different and weaker
// guarantee than never exposing the field.
func (b *Base) jsonFieldAbsent(ctx context.Context, field string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return fmt.Errorf("parsing the response as JSON: %w (%s)", err, resp.Describe())
	}
	if v, ok := traverseJSON(data, field); ok {
		return fmt.Errorf("expected field %q to be absent, but it is present with value %v (%s)",
			field, v, resp.Describe())
	}
	return nil
}

// responseMatchesPattern asserts the body matches a regular expression.
//
// Kept for SET MEMBERSHIP, which no other step expresses: the failover scenarios assert the
// request landed on one of several backups, and which one is genuinely non-deterministic. A
// prefix substring would accept any name sharing the stem, so it is weaker, not equivalent.
func (b *Base) responseMatchesPattern(ctx context.Context, pattern string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the expected pattern %q is not a valid regular expression: %w", pattern, err)
	}
	if !re.MatchString(resp.Text()) {
		return fmt.Errorf("expected the body to match %q, got %s", pattern, resp.Describe())
	}
	return nil
}

// responseHeaderEquals asserts an exact header value.
func (b *Base) responseHeaderEquals(ctx context.Context, name, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	// http.Header.Get is case-insensitive per RFC 7230, which is what a test asserting on a
	// gateway-set header wants: the product may legitimately vary the casing.
	got := resp.Headers.Get(name)
	if got != resolved {
		return fmt.Errorf("expected header %q to be %q, got %q (%s)",
			name, resolved, got, resp.Describe())
	}
	return nil
}

// responseHeaderContains asserts a header value contains a substring.
//
// Separate from an exact match because several headers are compound — a Content-Type with a
// charset, a Location with a generated path — and asserting the whole value would make the
// test brittle against parts the scenario is not about.
func (b *Base) responseHeaderContains(ctx context.Context, name, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	got := resp.Headers.Get(name)
	if !strings.Contains(got, resolved) {
		return fmt.Errorf("expected header %q to contain %q, got %q (%s)",
			name, resolved, got, resp.Describe())
	}
	return nil
}

// responseHeaderPresence asserts a header was or was not sent. The regex enumerates the two
// spellings, so a typo is an undefined step rather than a silent pass.
//
// Raw map lookup with textproto canonicalisation rather than Header.Get, which cannot express
// this: Get returns "" both for a header that is absent and for one sent with an empty value, so
// a header sent as "X-Foo:" must still count as present. Canonicalising matches Get's
// case-insensitivity, because the product may legitimately vary casing and neither branch may
// pass merely because it did.
func (b *Base) responseHeaderPresence(ctx context.Context, name, presence string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	values, present := resp.Headers[textproto.CanonicalMIMEHeaderKey(name)]
	switch presence {
	case "exist":
		if !present {
			return fmt.Errorf("expected header %q to be present, and it was not sent (%s)",
				name, resp.Describe())
		}
	case "not exist":
		if present {
			return fmt.Errorf("expected header %q to be absent, but it was sent as %q (%s)",
				name, strings.Join(values, ", "), resp.Describe())
		}
	default:
		return fmt.Errorf("unknown header presence %q", presence)
	}
	return nil
}

// jsonFieldContains asserts a field's rendered value contains a substring.
func (b *Base) jsonFieldContains(ctx context.Context, field, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return fmt.Errorf("parsing the response as JSON: %w (%s)", err, resp.Describe())
	}
	value, ok := traverseJSON(data, field)
	if !ok {
		return fmt.Errorf("expected field %q to exist, got %s", field, resp.Describe())
	}
	if got := fmt.Sprintf("%v", value); !strings.Contains(got, resolved) {
		return fmt.Errorf("expected field %q to contain %q, got %q", field, resolved, got)
	}
	return nil
}

// jsonFieldGreaterThan asserts a numeric field exceeds a threshold.
//
// A LOWER bound rather than an equality check, deliberately: these counts are totals across
// everything the engine currently holds, and runners share a gateway, so an exact number would
// depend on what another runner deployed a moment earlier.
func (b *Base) jsonFieldGreaterThan(ctx context.Context, path string, threshold int) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Body), &doc); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}
	got, ok := traverseJSON(doc, path)
	if !ok {
		return fmt.Errorf("no field %q in the response", path)
	}
	num, ok := got.(float64)
	if !ok {
		return fmt.Errorf("field %q is %v, which is not a number", path, got)
	}
	if int(num) <= threshold {
		return fmt.Errorf("expected %q to be greater than %d, got %v", path, threshold, num)
	}
	return nil
}

// echoedHeaderEquals asserts the gateway forwarded a header upstream with a given value.
func (b *Base) echoedHeaderEquals(ctx context.Context, name, want string) error {
	return b.assertEchoedHeader(ctx, name, want)
}

// echoedHeaderEqualsDoc is echoedHeaderEquals with the expected value in a docstring.
//
// Exists because a quoted capture cannot carry a double quote, and the value that most needs
// asserting here is precisely one full of awkward characters: the scenario feeding a secret
// containing a backslash and embedded quotes through to an upstream Authorization header. The
// quoted form would force that expectation to be trimmed to something the regex tolerates,
// which would retire the injection case while appearing to keep it.
//
// The docstring is compared verbatim apart from a trailing newline, which the Gherkin parser
// adds and no header ever carries.
func (b *Base) echoedHeaderEqualsDoc(ctx context.Context, name string, want *godog.DocString) error {
	return b.assertEchoedHeader(ctx, name, strings.TrimRight(want.Content, "\n"))
}

// echoedHeaderAbsent asserts the gateway did NOT forward a header upstream.
func (b *Base) echoedHeaderAbsent(ctx context.Context, name string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	headers, err := echoedHeaders(resp)
	if err != nil {
		return err
	}
	if value, found := lookupEchoed(headers, name); found {
		return fmt.Errorf("expected echoed header %q to be absent, got %v", name, value)
	}
	return nil
}

func (b *Base) assertEchoedHeader(ctx context.Context, name, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	resolved, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	headers, err := echoedHeaders(resp)
	if err != nil {
		return err
	}
	value, found := lookupEchoed(headers, name)
	if !found {
		return fmt.Errorf("expected echoed header %q to exist in response", name)
	}

	// Both shapes are carried over from the original: a JSON echo may render a header as a
	// bare string or as an array of values, and only the first is compared.
	switch v := value.(type) {
	case string:
		if v != resolved {
			return fmt.Errorf("expected echoed header %q to be %q, got %q", name, resolved, v)
		}
	case []any:
		if len(v) == 0 {
			return fmt.Errorf("expected echoed header %q to be %q, got empty array", name, resolved)
		}
		if got := fmt.Sprintf("%v", v[0]); got != resolved {
			return fmt.Errorf("expected echoed header %q to be %q, got %q", name, resolved, got)
		}
	default:
		return fmt.Errorf("expected echoed header %q to be string or array, got %T", name, value)
	}
	return nil
}

// echoedHeaders pulls the request headers the backend reflected back in its JSON body.
//
// These assert on what the GATEWAY SENT UPSTREAM, not on what it returned downstream — which
// is the only way to test a policy that adds or removes a request header. Two response shapes
// are accepted because the original suite accepted both: the backend nests them under
// Request.Header, other echo backends use a top-level "headers".
func echoedHeaders(resp *httpx.Response) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return nil, fmt.Errorf("parsing the echoed response as JSON: %w (%s)", err, resp.Describe())
	}
	if request, ok := data["Request"].(map[string]any); ok {
		if header, ok := request["Header"].(map[string]any); ok {
			return header, nil
		}
	}
	if headers, ok := data["headers"].(map[string]any); ok {
		return headers, nil
	}
	return nil, fmt.Errorf("the response carries no echoed headers: %s", resp.Describe())
}

// lookupEchoed finds a header case-insensitively, as HTTP requires.
func lookupEchoed(headers map[string]any, name string) (any, bool) {
	want := strings.ToLower(name)
	for k, v := range headers {
		if strings.ToLower(k) == want {
			return v, true
		}
	}
	return nil, false
}

// jsonFieldIs asserts the field at a dotted path holds exactly the given value.
func (b *Base) jsonFieldIs(ctx context.Context, field, want string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !resp.HasBody() {
		return fmt.Errorf("reading JSON field %s: response has no body: %s", field, resp.Describe())
	}
	var doc map[string]any
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return fmt.Errorf("response is not a JSON object: %w (%s)", err, resp.Describe())
	}
	expected, err := unique.Expand(ctx, want)
	if err != nil {
		return err
	}
	got, present := traverseJSON(doc, field)
	if !present {
		return fmt.Errorf("JSON field %q is absent from %s", field, resp.Describe())
	}
	if fmt.Sprintf("%v", got) != expected {
		return fmt.Errorf("JSON field %q: expected %q, got %q: %s",
			field, expected, fmt.Sprintf("%v", got), resp.Describe())
	}
	return nil
}

// jsonFieldIsDoc is jsonFieldIs for a value a Gherkin string cannot hold.
//
// The quoted form stops at the first `"`, so any field carrying JSON — an echoed request body,
// for instance — or spanning lines is only expressible as a DocString.
func (b *Base) jsonFieldIsDoc(ctx context.Context, field string, want *godog.DocString) error {
	return b.jsonFieldIs(ctx, field, want.Content)
}

// jsonFieldIsNumber asserts the field at a dotted path holds exactly the given number.
func (b *Base) jsonFieldIsNumber(ctx context.Context, field string, want int) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !resp.HasBody() {
		return fmt.Errorf("reading JSON field %s: response has no body: %s", field, resp.Describe())
	}
	var doc map[string]any
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return fmt.Errorf("response is not a JSON object: %w (%s)", err, resp.Describe())
	}
	got, present := traverseJSON(doc, field)
	if !present {
		return fmt.Errorf("JSON field %q is absent from %s", field, resp.Describe())
	}
	// encoding/json decodes every JSON number to float64, so anything else is a TYPE mismatch,
	// not a value mismatch. Saying which one it is matters: a count that became the string "1"
	// is a contract change, and stringifying both sides would hide it.
	num, ok := got.(float64)
	if !ok {
		return fmt.Errorf("JSON field %q holds %v (%T), not a number: %s",
			field, got, got, resp.Describe())
	}
	if num != float64(want) {
		return fmt.Errorf("JSON field %q: expected %d, got %v: %s", field, want, num, resp.Describe())
	}
	return nil
}

// gatewayURL builds a data-plane URL, where deployed APIs are invoked.
func (b *Base) gatewayURL(path string) (string, error) {
	base, err := b.topo.URL("platform-gateway", "http")
	if err != nil {
		return "", err
	}
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path, nil
}

func (b *Base) scenarioHeaders(ctx context.Context) map[string]string {
	h := map[string]string{}
	if v, ok := tcontext.Get(ctx, keyAuthHeader); ok {
		if s, ok := v.(string); ok {
			h["Authorization"] = s
		}
	}
	if v, ok := tcontext.Get(ctx, keyExtraHeaders); ok {
		if extra, ok := v.(map[string]string); ok {
			for k, val := range extra {
				h[k] = val
			}
		}
	}
	return h
}

func (b *Base) invokeWith(
	ctx context.Context, method, url string, headers map[string]string, body []byte,
) error {
	_, err := b.funnel.Send(ctx, httpx.Request{
		Method: method, URL: url,
		Headers: headers, Body: body,
		Host: b.requestHost(ctx),
	})
	if err != nil {
		return fmt.Errorf("invoking %s %s: %w", method, url, err)
	}
	return nil
}

// requestHost returns the Host override for this scenario, or "" when none is set.
func (b *Base) requestHost(ctx context.Context) string {
	if v, ok := tcontext.Get(ctx, keyRequestHost); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// resetRequest drops the request-shaping state accumulated so far in this scenario.
func (b *Base) resetRequest(ctx context.Context) error {
	if err := tcontext.Set(ctx, keyExtraHeaders, map[string]string{}); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyRequestHost, "")
}

// setHeader adds one header to every subsequent request in this scenario.
func (b *Base) setHeader(ctx context.Context, name, value string) error {
	resolved, err := unique.Expand(ctx, value)
	if err != nil {
		return err
	}
	extra := map[string]string{}
	if v, ok := tcontext.Get(ctx, keyExtraHeaders); ok {
		if existing, ok := v.(map[string]string); ok {
			for k, val := range existing {
				extra[k] = val
			}
		}
	}
	extra[name] = resolved
	return tcontext.Set(ctx, keyExtraHeaders, extra)
}

// setRequestHost overrides the HTTP Host header for subsequent requests.
func (b *Base) setRequestHost(ctx context.Context, host string) error {
	return tcontext.Set(ctx, keyRequestHost, strings.TrimSpace(host))
}

// traverseJSON walks a dotted path, honouring [n] array indices.
func traverseJSON(doc any, path string) (any, bool) {
	current := doc
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		name, indices := splitIndices(segment)
		if name != "" {
			asMap, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			if current, ok = asMap[name]; !ok {
				return nil, false
			}
		}
		for _, i := range indices {
			asSlice, ok := current.([]any)
			if !ok || i < 0 || i >= len(asSlice) {
				return nil, false
			}
			current = asSlice[i]
		}
	}
	return current, true
}

// splitIndices separates "policies[0][1]" into its name and its indices.
func splitIndices(segment string) (string, []int) {
	if n, err := strconv.Atoi(segment); err == nil {
		return "", []int{n}
	}
	open := strings.Index(segment, "[")
	if open < 0 {
		return segment, nil
	}
	name := segment[:open]
	var indices []int
	for rest := segment[open:]; strings.HasPrefix(rest, "["); {
		end := strings.Index(rest, "]")
		if end < 0 {
			// Unbalanced: treat the whole segment as a key so the caller reports a missing
			// path rather than silently dropping the malformed part.
			return segment, nil
		}
		n, err := strconv.Atoi(rest[1:end])
		if err != nil {
			return segment, nil
		}
		indices = append(indices, n)
		rest = rest[end+1:]
	}
	return name, indices
}
