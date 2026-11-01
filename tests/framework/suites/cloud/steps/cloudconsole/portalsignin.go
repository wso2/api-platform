/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// portalBasePath is the prefix the API Portal serves every page under.
	portalBasePath = "/api-portal"
	// portalSignedInPage is a page of a view that only a signed-in consumer may open.
	portalSignedInPage = "applications"

	// identityAuthorizePath is the identity provider's authorization endpoint, which the portal
	// sends a visitor to; the flow and callback endpoints sit next to it.
	identityAuthorizePath = "/oauth2/authorize"
	identityFlowPath      = "/flow/execute"
	identityCallbackPath  = "/oauth2/auth/callback"

	// The actions of the portal's sign-in and registration flows, as the platform defines them:
	// the primary action submits the prompt's inputs, and the sign-up link moves from the sign-in
	// prompt to the registration prompt.
	flowActionSubmit = "action_001"
	flowActionSignUp = "action_signup"

	flowStatusComplete = "COMPLETE"
	flowStatusError    = "ERROR"

	// maxSignInHops bounds the redirects a sign-in follows on one host.
	maxSignInHops = 5

	// keyPortalBrowser holds the browser the scenario's consumer or visitor uses.
	keyPortalBrowser = "cloudPortalBrowser"
	// keyPortalConsumer and keyPortalConsumerPassword are the consumer the runner signed up. The
	// password stays in runner state and is never logged or published.
	keyPortalConsumer         = "cloudPortalConsumer"
	keyPortalConsumerPassword = "cloudPortalConsumerPassword"
)

// portalBrowser stands in for one visitor's browser on a managed API Portal. It keeps the cookies
// the portal and its identity provider set, sends them back to the host that set them, and
// follows no redirect by itself, so a step can check each hop of a sign-in.
type portalBrowser struct {
	client *httpx.Client
	jar    http.CookieJar
	portal *url.URL

	// org and view are the organisation and view of the sign-in page the portal redirected to.
	org, view string
	// authorize is the identity provider's authorization request the portal sent the visitor to.
	authorize *url.URL
	// refusal is why the identity provider refused the last sign-in, or "" when it accepted it.
	refusal string
}

func newPortalBrowser(client *httpx.Client, portal *url.URL) (*portalBrowser, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating a cookie jar: %w", err)
	}
	return &portalBrowser{client: client, jar: jar, portal: portal}, nil
}

// send makes one request with the cookies for its URL and keeps the cookies the answer sets.
func (b *portalBrowser) send(ctx context.Context, method, target string, body []byte) (*httpx.Response, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("the sign-in reached a URL that does not parse: %w", err)
	}
	if err := requireSecureURL(parsed); err != nil {
		return nil, fmt.Errorf("the sign-in reached an insecure URL: %w", err)
	}
	headers := map[string]string{}
	if cookies := b.jar.Cookies(parsed); len(cookies) > 0 {
		pairs := make([]string, 0, len(cookies))
		for _, cookie := range cookies {
			pairs = append(pairs, cookie.Name+"="+cookie.Value)
		}
		headers["Cookie"] = strings.Join(pairs, "; ")
	}
	request := httpx.Request{Method: method, URL: parsed.String(), Headers: headers, Body: body}
	if body != nil {
		request.ContentType = "application/json"
		headers["Accept"] = "application/json"
	}
	resp, err := b.client.Do(ctx, request, 0, 0)
	if err != nil {
		return nil, err
	}
	if set := (&http.Response{Header: resp.Headers}).Cookies(); len(set) > 0 {
		b.jar.SetCookies(parsed, set)
	}
	return resp, nil
}

// postJSON sends a JSON body and decodes a JSON answer into out. It never repeats the body in an
// error, because it can carry the consumer's password.
func (b *portalBrowser) postJSON(ctx context.Context, target string, body, out any) (*httpx.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the request to %s: %w", target, err)
	}
	resp, err := b.send(ctx, http.MethodPost, target, raw)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("the identity provider refused %s: %s", target, resp.Describe())
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return resp, fmt.Errorf("the identity provider's answer from %s is not JSON: %w", target, err)
	}
	return resp, nil
}

// redirectTarget resolves the Location of a redirect answer against the request that got it.
func redirectTarget(resp *httpx.Response) (*url.URL, error) {
	if resp.StatusCode < http.StatusMultipleChoices || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("expected a redirect, got %s", resp.Describe())
	}
	location := strings.TrimSpace(resp.Headers.Get("Location"))
	if location == "" {
		return nil, fmt.Errorf("the redirect names no location: %s", resp.Describe())
	}
	from, err := url.Parse(resp.URL)
	if err != nil {
		return nil, err
	}
	return from.Parse(location)
}

// viewSignInPage returns the organisation and view of a portal path of the form
// /api-portal/<org>/views/<view>/login.
func viewSignInPage(path string) (org, view string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, portalBasePath+"/"), "/")
	if !strings.HasPrefix(path, portalBasePath+"/") || len(parts) != 4 || parts[1] != "views" || parts[3] != "login" ||
		parts[0] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

// openSignIn opens the portal's sign-in entry point in a new browser and follows the portal's
// redirects until one leaves the portal for its identity provider. It returns the browser and the
// answer that sent it there.
func (s *Steps) openSignIn(ctx context.Context) (*portalBrowser, *httpx.Response, error) {
	portal, err := portalBaseURL(ctx)
	if err != nil {
		return nil, nil, err
	}
	browser, err := newPortalBrowser(s.client, portal)
	if err != nil {
		return nil, nil, err
	}
	if err := tcontext.Set(ctx, keyPortalBrowser, browser); err != nil {
		return nil, nil, err
	}
	target := portal.ResolveReference(&url.URL{Path: portalBasePath + "/login"})
	for range maxSignInHops {
		resp, err := browser.send(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		next, err := redirectTarget(resp)
		if err != nil {
			return browser, resp, fmt.Errorf("the portal did not send the visitor to sign in: %w", err)
		}
		if org, view, ok := viewSignInPage(next.Path); ok && sameOrigin(next, portal) {
			browser.org, browser.view = org, view
		}
		if !sameOrigin(next, portal) {
			browser.authorize = next
			return browser, resp, nil
		}
		target = next
	}
	return nil, nil, fmt.Errorf("the portal redirected the visitor more than %d times without sending it to sign in", maxSignInHops)
}

// openSignInAsVisitor opens the portal's sign-in as a new visitor and publishes the answer that
// sent the visitor away from the portal.
func (s *Steps) openSignInAsVisitor(ctx context.Context) error {
	httpx.ClearPublished(ctx)
	_, resp, err := s.openSignIn(ctx)
	if resp != nil && s.funnel != nil {
		if publishErr := s.funnel.Publish(ctx, resp); publishErr != nil {
			return publishErr
		}
	}
	return err
}

// scenarioBrowser returns the browser the scenario opened.
func scenarioBrowser(ctx context.Context) (*portalBrowser, error) {
	value, ok := tcontext.Get(ctx, keyPortalBrowser)
	if !ok {
		return nil, errors.New("the scenario has opened no managed API Portal")
	}
	browser, ok := value.(*portalBrowser)
	if !ok || browser == nil {
		return nil, fmt.Errorf("context key %q holds %T, not a portal browser", keyPortalBrowser, value)
	}
	return browser, nil
}

// assertSentToIdentityProvider checks the portal sent the visitor to an identity provider's
// authorization endpoint on another host, asking for a code for the portal's own client and to be
// sent back to a callback on the portal.
func (s *Steps) assertSentToIdentityProvider(ctx context.Context) error {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	authorize := browser.authorize
	if authorize == nil {
		return errors.New("the portal did not send the visitor to an identity provider")
	}
	if err := requireSecureURL(authorize); err != nil {
		return fmt.Errorf("the identity provider's sign-in: %w", err)
	}
	if !strings.HasSuffix(strings.TrimRight(authorize.Path, "/"), identityAuthorizePath) {
		return fmt.Errorf("the portal sent the visitor to %s, which is not an authorization endpoint", authorize.Redacted())
	}
	query := authorize.Query()
	if query.Get("response_type") != "code" {
		return fmt.Errorf("the authorization request asks for response_type %q, want code", query.Get("response_type"))
	}
	if strings.TrimSpace(query.Get("client_id")) == "" {
		return errors.New("the authorization request names no client")
	}
	callback, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || !sameOrigin(callback, browser.portal) || !strings.HasPrefix(callback.Path, portalBasePath+"/") {
		return fmt.Errorf("the authorization request sends the visitor back to %q, not to the portal", query.Get("redirect_uri"))
	}
	if browser.org == "" || browser.view == "" {
		return errors.New("the portal did not pass through a view's sign-in page")
	}
	return nil
}

// identityFlowStep is one answer of the identity provider's flow execution API.
type identityFlowStep struct {
	ExecutionID    string            `json:"executionId"`
	ChallengeToken string            `json:"challengeToken,omitempty"`
	FlowStatus     string            `json:"flowStatus"`
	Assertion      string            `json:"assertion,omitempty"`
	FailureReason  string            `json:"failureReason,omitempty"`
	SessionData    []identitySession `json:"sessionData,omitempty"`
}

// identitySession is an SSO session the flow reports, which the callback carries back.
type identitySession struct {
	FlowIdentifier string `json:"flowIdentifier"`
	SessionHandle  string `json:"sessionHandle"`
}

// flowAction is one action a consumer takes on a flow prompt.
type flowAction struct {
	action string
	inputs map[string]string
}

// takeFlowAction takes one action on the flow's current prompt. A client error or an ERROR flow
// status is the identity provider refusing the action, returned as its reason rather than as an
// error; any other failure is an error.
func (b *portalBrowser) takeFlowAction(ctx context.Context, target string, current identityFlowStep, next flowAction) (identityFlowStep, string, error) {
	request := map[string]any{"executionId": current.ExecutionID, "action": next.action}
	if current.ChallengeToken != "" {
		request["challengeToken"] = current.ChallengeToken
	}
	if len(next.inputs) > 0 {
		request["inputs"] = next.inputs
	}
	var answer identityFlowStep
	resp, err := b.postJSON(ctx, target, request, &answer)
	if err != nil {
		if resp != nil && resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
			return identityFlowStep{}, fmt.Sprintf("the identity provider answered %d", resp.StatusCode), nil
		}
		return identityFlowStep{}, "", err
	}
	if answer.FlowStatus == flowStatusError {
		reason := answer.FailureReason
		if reason == "" {
			reason = "the flow ended as " + flowStatusError
		}
		return identityFlowStep{}, reason, nil
	}
	if answer.ExecutionID == "" {
		answer.ExecutionID = current.ExecutionID
	}
	return answer, "", nil
}

// identityBase is the identity provider's base URL: the authorization endpoint without its path.
func identityBase(authorize *url.URL) *url.URL {
	base := *authorize
	base.Path = strings.TrimSuffix(strings.TrimRight(authorize.Path, "/"), identityAuthorizePath)
	base.RawPath, base.RawQuery, base.Fragment = "", "", ""
	return &base
}

func joinURLPath(base *url.URL, path string) string {
	joined := *base
	joined.Path = strings.TrimRight(base.Path, "/") + path
	return joined.String()
}

// signIn drives a consumer's sign-in, as the identity provider's sign-in page drives it: the
// authorization request, the flow's prompts with the given actions, the callback that turns the
// flow's assertion into the portal's code, and the portal's callback that turns the code into a
// session. A flow that ends without an assertion is a refusal: it is recorded on the browser and
// the portal is not called back. The last answer is published.
func (s *Steps) signIn(ctx context.Context, actions []flowAction) error {
	httpx.ClearPublished(ctx)
	browser, _, err := s.openSignIn(ctx)
	if err != nil {
		return err
	}
	resp, err := browser.send(ctx, http.MethodGet, browser.authorize.String(), nil)
	if err != nil {
		return err
	}
	started, err := redirectTarget(resp)
	if err != nil {
		return fmt.Errorf("the identity provider did not start a sign-in: %w", err)
	}
	executionID, authID := started.Query().Get("executionId"), started.Query().Get("authId")
	if executionID == "" || authID == "" {
		return fmt.Errorf("the identity provider started no sign-in flow: %s", resp.Describe())
	}
	base := identityBase(browser.authorize)
	var step identityFlowStep
	if _, err := browser.postJSON(ctx, joinURLPath(base, identityFlowPath), map[string]string{"executionId": executionID}, &step); err != nil {
		return err
	}
	// The callback carries every SSO session the flow reported, and an empty list when none.
	sessions := append([]identitySession{}, step.SessionData...)
	for i, next := range actions {
		last := i == len(actions)-1
		answer, refusal, err := browser.takeFlowAction(ctx, joinURLPath(base, identityFlowPath), step, next)
		if err != nil {
			return err
		}
		if refusal != "" {
			if !last {
				return fmt.Errorf("the identity provider ended the flow at action %q: %s", next.action, refusal)
			}
			browser.refusal = refusal
			return nil
		}
		step = answer
		sessions = append(sessions, step.SessionData...)
	}
	if step.FlowStatus != flowStatusComplete || step.Assertion == "" {
		browser.refusal = step.FailureReason
		if browser.refusal == "" {
			browser.refusal = "the flow ended as " + step.FlowStatus
		}
		return nil
	}
	browser.refusal = ""
	var callback struct {
		RedirectURI string `json:"redirect_uri"`
	}
	if _, err := browser.postJSON(ctx, joinURLPath(base, identityCallbackPath),
		map[string]any{"authId": authID, "assertion": step.Assertion, "sessionData": sessions}, &callback); err != nil {
		return err
	}
	portalCallback, err := url.Parse(callback.RedirectURI)
	if err != nil || !sameOrigin(portalCallback, browser.portal) {
		return fmt.Errorf("the identity provider sends the consumer back to %q, not to the portal", callback.RedirectURI)
	}
	landed, err := browser.send(ctx, http.MethodGet, portalCallback.String(), nil)
	if err != nil {
		return err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, landed); err != nil {
			return err
		}
	}
	if _, err := redirectTarget(landed); err != nil {
		return fmt.Errorf("the portal did not take the consumer's sign-in: %w", err)
	}
	return nil
}

// consumerCredentials returns the consumer the runner signed up.
func consumerCredentials(ctx context.Context) (username, password string, err error) {
	if username, err = requiredValue(ctx, keyPortalConsumer); err != nil {
		return "", "", fmt.Errorf("the runner has signed up no consumer: %w", err)
	}
	if password, err = requiredValue(ctx, keyPortalConsumerPassword); err != nil {
		return "", "", fmt.Errorf("the runner has signed up no consumer: %w", err)
	}
	return username, password, nil
}

// newConsumerPassword returns a password for a new consumer, with a letter of each case, a digit
// and a symbol for any password policy the identity provider applies.
func newConsumerPassword() (string, error) {
	random, err := randomHex(12)
	if err != nil {
		return "", err
	}
	return "Syn-" + random + "-Pw9!", nil
}

// signUpConsumer signs a new consumer up from the portal's sign-in page and keeps the consumer for
// the later scenarios of the runner. A consumer is a user of the organisation's identity provider,
// and the cloud API offers no way to remove one, so the runner signs up one consumer and reuses it.
func (s *Steps) signUpConsumer(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-consumer", keyPortalConsumer); err != nil {
		return err
	}
	username, err := requiredValue(ctx, keyPortalConsumer)
	if err != nil {
		return err
	}
	password, err := newConsumerPassword()
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyPortalConsumerPassword, password); err != nil {
		return err
	}
	if err := s.signIn(ctx, []flowAction{
		{action: flowActionSignUp},
		{action: flowActionSubmit, inputs: map[string]string{
			"username": username, "password": password, "email": username + "@example.com",
		}},
	}); err != nil {
		return err
	}
	return requireSignInAccepted(ctx)
}

// signInConsumer signs the runner's consumer in with its username and password.
func (s *Steps) signInConsumer(ctx context.Context) error {
	username, password, err := consumerCredentials(ctx)
	if err != nil {
		return err
	}
	if err := s.signIn(ctx, []flowAction{{action: flowActionSubmit, inputs: map[string]string{
		"username": username, "password": password,
	}}}); err != nil {
		return err
	}
	return requireSignInAccepted(ctx)
}

// trySignInWithWrongPassword signs the runner's consumer in with a password that is not theirs,
// and leaves the identity provider's answer for the next step to judge.
func (s *Steps) trySignInWithWrongPassword(ctx context.Context) error {
	username, password, err := consumerCredentials(ctx)
	if err != nil {
		return err
	}
	return s.signIn(ctx, []flowAction{{action: flowActionSubmit, inputs: map[string]string{
		"username": username, "password": password + "-wrong",
	}}})
}

// requireSignInAccepted fails when the identity provider refused the scenario's sign-in.
func requireSignInAccepted(ctx context.Context) error {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	if browser.refusal != "" {
		return fmt.Errorf("the identity provider refused the consumer's sign-in: %s", browser.refusal)
	}
	return nil
}

// assertSignInRefused checks the identity provider refused the scenario's sign-in.
func (s *Steps) assertSignInRefused(ctx context.Context) error {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	if browser.refusal == "" {
		return errors.New("the identity provider accepted the sign-in")
	}
	return nil
}

// signedInPage is a page of the browser's view that only a signed-in consumer may open.
func (b *portalBrowser) signedInPage() (string, error) {
	if b.org == "" || b.view == "" {
		return "", errors.New("the browser has not learned the portal's organisation and view")
	}
	return b.portal.ResolveReference(&url.URL{
		Path: portalBasePath + "/" + b.org + "/views/" + b.view + "/" + portalSignedInPage,
	}).String(), nil
}

// openSignedInPage opens the signed-in page in a browser and publishes the answer.
func (s *Steps) openSignedInPage(ctx context.Context, browser *portalBrowser) (*httpx.Response, error) {
	httpx.ClearPublished(ctx)
	page, err := browser.signedInPage()
	if err != nil {
		return nil, err
	}
	resp, err := browser.send(ctx, http.MethodGet, page, nil)
	if err != nil {
		return nil, err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, resp); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// assertConsumerSignedIn checks the scenario's browser opens a page only a signed-in consumer may.
func (s *Steps) assertConsumerSignedIn(ctx context.Context) error {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	resp, err := s.openSignedInPage(ctx, browser)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the consumer could not open the %s page: %s", portalSignedInPage, resp.Describe())
	}
	return nil
}

// requireSentToSignIn checks an answer sends the browser to its view's sign-in page.
func requireSentToSignIn(browser *portalBrowser, resp *httpx.Response) error {
	next, err := redirectTarget(resp)
	if err != nil {
		return fmt.Errorf("the %s page was not refused: %w", portalSignedInPage, err)
	}
	org, view, ok := viewSignInPage(next.Path)
	if !ok || !sameOrigin(next, browser.portal) || org != browser.org || view != browser.view {
		return fmt.Errorf("the %s page sent the browser to %s, not to the view's sign-in page", portalSignedInPage, next.Redacted())
	}
	return nil
}

// assertConsumerNotSignedIn checks the scenario's browser is sent to sign in when it opens a page
// only a signed-in consumer may.
func (s *Steps) assertConsumerNotSignedIn(ctx context.Context) error {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	resp, err := s.openSignedInPage(ctx, browser)
	if err != nil {
		return err
	}
	return requireSentToSignIn(browser, resp)
}

// assertVisitorWithoutSessionSentToSignIn opens the same page in a browser that holds none of the
// consumer's cookies and checks it is sent to sign in.
func (s *Steps) assertVisitorWithoutSessionSentToSignIn(ctx context.Context) error {
	signedIn, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	visitor, err := newPortalBrowser(s.client, signedIn.portal)
	if err != nil {
		return err
	}
	visitor.org, visitor.view = signedIn.org, signedIn.view
	resp, err := s.openSignedInPage(ctx, visitor)
	if err != nil {
		return err
	}
	return requireSentToSignIn(visitor, resp)
}

// signOutConsumer signs the scenario's consumer out of the portal and publishes the answer. The
// portal ends its session before it sends the browser to the identity provider's sign-out, which
// is not followed.
func (s *Steps) signOutConsumer(ctx context.Context) error {
	httpx.ClearPublished(ctx)
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return err
	}
	if browser.org == "" || browser.view == "" {
		return errors.New("the browser has not learned the portal's organisation and view")
	}
	page := browser.portal.ResolveReference(&url.URL{
		Path: portalBasePath + "/" + browser.org + "/views/" + browser.view + "/logout",
	}).String()
	resp, err := browser.send(ctx, http.MethodGet, page, nil)
	if err != nil {
		return err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, resp); err != nil {
			return err
		}
	}
	if _, err := redirectTarget(resp); err != nil {
		return fmt.Errorf("the portal did not sign the consumer out: %w", err)
	}
	return nil
}

// registerPortalSignInSteps binds the steps that sign a consumer up, in and out of a managed API
// Portal.
func (s *Steps) registerPortalSignInSteps(sc stepRegistrar) {
	sc.Step(`^I open the sign-in page of the synthetic managed API Portal as a new visitor$`, s.openSignInAsVisitor)
	sc.Step(`^the visitor should be sent to the portal's identity provider to sign in$`, s.assertSentToIdentityProvider)
	sc.Step(`^I sign up on the synthetic managed API Portal as a new consumer$`, s.signUpConsumer)
	sc.Step(`^I sign in to the synthetic managed API Portal as the consumer$`, s.signInConsumer)
	sc.Step(`^I try to sign in to the synthetic managed API Portal as the consumer with a wrong password$`, s.trySignInWithWrongPassword)
	sc.Step(`^the identity provider should refuse the sign-in$`, s.assertSignInRefused)
	sc.Step(`^the consumer should be signed in to the synthetic managed API Portal$`, s.assertConsumerSignedIn)
	sc.Step(`^the consumer should not be signed in to the synthetic managed API Portal$`, s.assertConsumerNotSignedIn)
	sc.Step(`^a visitor without the consumer's session should be sent to sign in$`, s.assertVisitorWithoutSessionSentToSignIn)
	sc.Step(`^I sign out of the synthetic managed API Portal$`, s.signOutConsumer)
}
