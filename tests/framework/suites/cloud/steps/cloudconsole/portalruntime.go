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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// keyPortalOrg and keyPortalView name the organisation and view the portal's front door sends a
	// visitor to; the public pages of a published API sit below them.
	keyPortalOrg  = "cloudPortalOrg"
	keyPortalView = "cloudPortalView"

	// portalServeTimeout bounds the wait for a portal to answer as a scenario expects: a publish
	// reaches the portal while the control plane answers, and a deleted portal's route goes after
	// its runtime does.
	portalServeTimeout = 3 * time.Minute

	// portalStopServingTimeout bounds the wait for a deleted portal's URL to stop serving.
	portalStopServingTimeout = 5 * time.Minute

	flowEmailInput = "email"
)

// portalServePollInterval is how often a request to a portal is repeated while a scenario waits for
// a status.
var portalServePollInterval = 3 * time.Second

// registerPortalRuntimeSteps binds the steps that talk to a provisioned portal itself, as a
// visitor or as the control plane would, and the checks that follow a portal's deletion.
func (s *Steps) registerPortalRuntimeSteps(sc stepRegistrar) {
	sc.Step(`^I request "([^"]*)" from the synthetic managed API Portal$`, s.requestFromPortal)
	sc.Step(`^I request "([^"]*)" from the synthetic managed API Portal until status (\d+)(?: containing "([^"]*)")?$`, s.requestFromPortalUntilStatus)
	sc.Step(`^I request "([^"]*)" from the synthetic managed API Portal with a wrong shared key$`, s.requestFromPortalWithWrongSharedKey)
	sc.Step(`^I follow the front door of the synthetic managed API Portal to its view$`, s.followPortalFrontDoor)
	sc.Step(`^the portal answer should (contain|not contain) "([^"]*)"$`, s.assertPortalAnswerText)
	sc.Step(`^the portal answer should send the visitor to the identity provider for a silent sign-in$`, s.assertSilentSignInRedirect)
	sc.Step(`^the portal answer should carry no "([^"]*)" header$`, s.assertPortalAnswerLacksHeader)
	sc.Step(`^the portal answer header "([^"]*)" should contain "([^"]*)"$`, s.assertPortalAnswerHeader)
	sc.Step(`^the sign-in of the synthetic managed API Portal should (offer|not offer) sign-up$`, s.assertSignUpOffer)
	sc.Step(`^the synthetic managed API Portal should no longer serve$`, s.awaitPortalStopsServing)
	sc.Step(`^I request a managed API Portal under the handle of the deleted synthetic managed API Portal$`, s.requestPortalUnderDeletedHandle)
}

// portalTLS is the TLS a request to a portal runs with. A cloud on the developer's machine serves
// its portals from the data plane's self-signed certificate, so a request to an approved local
// target does not verify it. Every other target is verified.
func portalTLS(target *url.URL) *httpx.ClientTLS {
	if target.Scheme == "https" && localDataPlaneEnabled() && approvedLocalDataPlaneTarget(target) {
		return &httpx.ClientTLS{InsecureSkipVerify: true}
	}
	return nil
}

// portalRequest sends a request to the runner portal's public URL without publishing the answer.
// The path may be relative to the portal or an absolute URL the portal sent a visitor to; either
// must pass the same check as the portal's own URL.
func (s *Steps) portalRequest(ctx context.Context, method, path string, headers map[string]string) (*httpx.Response, error) {
	portal, err := portalBaseURL(ctx)
	if err != nil {
		return nil, err
	}
	ref, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("the portal path %q does not parse: %w", path, err)
	}
	target := portal.ResolveReference(ref)
	if err := requireSecureURL(target); err != nil {
		return nil, fmt.Errorf("the portal request: %w", err)
	}
	if !sameOrigin(target, portal) {
		return nil, fmt.Errorf("the portal request would leave the portal for %s", target.Redacted())
	}
	return s.client.Do(ctx, httpx.Request{Method: method, URL: target.String(), Headers: headers, TLS: portalTLS(target)}, 0, 0)
}

// publishPortalAnswer publishes an answer of the portal for the assertion steps.
func (s *Steps) publishPortalAnswer(ctx context.Context, resp *httpx.Response) error {
	if s.funnel == nil {
		return nil
	}
	return s.funnel.Publish(ctx, resp)
}

// requestFromPortal makes an anonymous GET of a path of the runner's portal and publishes the
// answer.
func (s *Steps) requestFromPortal(ctx context.Context, path string) error {
	return s.requestFromPortalWith(ctx, path, nil)
}

// requestFromPortalWith makes a GET of a path of the runner's portal with the given headers and
// publishes the answer.
func (s *Steps) requestFromPortalWith(ctx context.Context, path string, headers map[string]string) error {
	path, err := expandCloudValue(ctx, path)
	if err != nil {
		return err
	}
	httpx.ClearPublished(ctx)
	resp, err := s.portalRequest(ctx, http.MethodGet, path, headers)
	if err != nil {
		return err
	}
	return s.publishPortalAnswer(ctx, resp)
}

// requestFromPortalWithWrongSharedKey makes a GET that claims to come from the control plane with a
// shared key the portal does not hold.
func (s *Steps) requestFromPortalWithWrongSharedKey(ctx context.Context, path string) error {
	random, err := randomHex(16)
	if err != nil {
		return err
	}
	return s.requestFromPortalWith(ctx, path, map[string]string{"Authorization": "SharedKey " + random})
}

// requestFromPortalUntilStatus repeats an anonymous GET of a path of the runner's portal until it
// answers with the status, and publishes that answer.
func (s *Steps) requestFromPortalUntilStatus(ctx context.Context, path string, status int, text string) error {
	path, err := expandCloudValue(ctx, path)
	if err != nil {
		return err
	}
	if text, err = expandCloudValue(ctx, text); err != nil {
		return err
	}
	httpx.ClearPublished(ctx)
	accept := func(resp *httpx.Response) bool {
		return resp != nil && resp.StatusCode == status && strings.Contains(string(resp.Body), text)
	}
	resp, err := retry.Until(ctx, retry.Options{Timeout: portalServeTimeout, Interval: portalServePollInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.portalRequest(ctx, http.MethodGet, path, nil)
			if err != nil {
				return nil, retry.Transient(err)
			}
			return resp, nil
		}, accept)
	if err != nil {
		return fmt.Errorf("waiting for the portal to answer %s with %d%s: %w", path, status, containing(text), err)
	}
	if !accept(resp) {
		return fmt.Errorf("the portal never answered %s with %d%s; the last answer was %s", path, status, containing(text), resp.Describe())
	}
	return s.publishPortalAnswer(ctx, resp)
}

// containing words the text an answer must carry, or nothing when none is required.
func containing(text string) string {
	if text == "" {
		return ""
	}
	return fmt.Sprintf(" and containing %q", text)
}

// portalViewPath returns the organisation and view of a portal path of the form
// /api-portal/<org>/views/<view>, with anything after the view ignored.
func portalViewPath(path string) (org, view string, ok bool) {
	if !strings.HasPrefix(path, portalBasePath+"/") {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, portalBasePath+"/"), "/"), "/")
	if len(parts) < 3 || parts[1] != "views" || parts[0] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

// followPortalFrontDoor opens the portal's front door as a visitor with no session, follows the
// portal's redirects to the view the visitor lands on, and records the organisation and view for
// the public pages that sit below them. The first answer is published.
func (s *Steps) followPortalFrontDoor(ctx context.Context) error {
	portal, err := portalBaseURL(ctx)
	if err != nil {
		return err
	}
	httpx.ClearPublished(ctx)
	target := portalBasePath + "/"
	for hop := range maxSignInHops {
		resp, err := s.portalRequest(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		if hop == 0 {
			if err := s.publishPortalAnswer(ctx, resp); err != nil {
				return err
			}
		}
		next, err := redirectTarget(resp)
		if err != nil {
			return fmt.Errorf("the portal's front door did not send the visitor to a view: %w", err)
		}
		if !sameOrigin(next, portal) {
			return fmt.Errorf("the portal's front door sent the visitor away from the portal to %s", next.Redacted())
		}
		if org, view, ok := portalViewPath(next.Path); ok {
			if err := tcontext.Set(ctx, keyPortalOrg, org); err != nil {
				return err
			}
			return tcontext.Set(ctx, keyPortalView, view)
		}
		target = next.String()
	}
	return fmt.Errorf("the portal's front door redirected the visitor more than %d times without reaching a view", maxSignInHops)
}

// assertPortalAnswerText checks the published portal answer's body does, or does not, contain the
// text.
func (s *Steps) assertPortalAnswerText(ctx context.Context, relation, text string) error {
	text, err := expandCloudValue(ctx, text)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if contains := strings.Contains(string(resp.Body), text); contains != (relation == "contain") {
		return fmt.Errorf("the portal answer should %s %q: %s", relation, text, resp.Describe())
	}
	return nil
}

// assertSilentSignInRedirect checks the published portal answer sends a visitor with no session to
// the identity provider's authorization endpoint, on another host, to be signed in silently with an
// existing identity provider session: a portal in identity-provider mode asks the identity
// provider before it shows a visitor any page.
func (s *Steps) assertSilentSignInRedirect(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	portal, err := portalBaseURL(ctx)
	if err != nil {
		return err
	}
	next, err := redirectTarget(resp)
	if err != nil {
		return fmt.Errorf("the portal did not redirect the visitor: %w", err)
	}
	if sameOrigin(next, portal) {
		return fmt.Errorf("the portal kept the visitor on the portal: %s", next.Redacted())
	}
	if !strings.HasSuffix(strings.TrimRight(next.Path, "/"), identityAuthorizePath) {
		return fmt.Errorf("the portal sent the visitor to %s, which is not an authorization endpoint", next.Redacted())
	}
	query := next.Query()
	if query.Get("prompt") != "none" || query.Get("response_type") != "code" {
		return fmt.Errorf("the authorization request asks for prompt %q and response_type %q, want a silent sign-in for a code", query.Get("prompt"), query.Get("response_type"))
	}
	return nil
}

// assertPortalAnswerLacksHeader checks the published portal answer carries no such header.
func (s *Steps) assertPortalAnswerLacksHeader(ctx context.Context, header string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if values := resp.Headers.Values(header); len(values) > 0 {
		return fmt.Errorf("the portal answer carries a %s header: %s", header, resp.Describe())
	}
	return nil
}

// assertPortalAnswerHeader checks a header of the published portal answer contains the text.
func (s *Steps) assertPortalAnswerHeader(ctx context.Context, header, text string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if got := resp.Headers.Get(header); !strings.Contains(strings.ToLower(got), strings.ToLower(text)) {
		return fmt.Errorf("the portal answer header %s is %q, want it to contain %q: %s", header, got, text, resp.Describe())
	}
	return nil
}

// signUpOffered opens the portal's sign-in as a new visitor, takes the sign-up action on its first
// prompt and reports whether the identity provider answered with a registration prompt, which asks
// for an email address the sign-in prompt does not. Nothing is submitted, so no consumer is
// created. The reason is what the identity provider gave when it refused the action.
func (s *Steps) signUpOffered(ctx context.Context) (offered bool, reason string, err error) {
	browser, base, _, step, err := s.beginFlow(ctx)
	if err != nil {
		return false, "", err
	}
	answer, refusal, err := browser.takeFlowAction(ctx, joinURLPath(base, identityFlowPath), step, flowAction{action: flowActionSignUp})
	if err != nil {
		return false, "", err
	}
	if refusal != "" {
		return false, refusal, nil
	}
	if !strings.Contains(strings.ToLower(string(answer.raw)), flowEmailInput) {
		return false, "the identity provider did not answer with a registration prompt", nil
	}
	return true, "", nil
}

// assertSignUpOffer checks whether the portal's sign-in offers a visitor sign-up.
func (s *Steps) assertSignUpOffer(ctx context.Context, relation string) error {
	offered, reason, err := s.signUpOffered(ctx)
	if err != nil {
		return err
	}
	if want := relation == "offer"; offered != want {
		if offered {
			return errors.New("the sign-in offers sign-up, but it should not")
		}
		return fmt.Errorf("the sign-in does not offer sign-up, but it should: %s", reason)
	}
	return nil
}

// awaitPortalStopsServing waits until the deleted portal's health URL no longer serves: the
// request fails, or the route answers 404.
func (s *Steps) awaitPortalStopsServing(ctx context.Context) error {
	return retry.Await(ctx, retry.Options{Timeout: portalStopServingTimeout, Interval: portalPollInterval},
		func(ctx context.Context) (string, error) {
			resp, err := s.portalRequest(ctx, http.MethodGet, portalBasePath+"/health", nil)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return "", err
				}
				return "unreachable", nil
			}
			if resp.StatusCode == http.StatusNotFound {
				return "not found", nil
			}
			return "", retry.Transient(fmt.Errorf("the deleted portal still answers: %s", resp.Describe()))
		},
		func(state string) bool { return state != "" },
		"waiting for the deleted portal's URL to stop serving")
}

// requestPortalUnderDeletedHandle asks for a portal under the handle of the runner's portal, which
// was deleted, and publishes the answer. The platform reads a deleted portal as gone while the
// cloud is still tearing down its runtime, and refuses a create under the handle until that ends,
// so a refusal that a later attempt may clear (a conflict or a server error) is repeated within the
// portal deletion budget. The handle is registered for runner cleanup before the request, and the
// registration is dropped again when the platform finally refuses the request.
func (s *Steps) requestPortalUnderDeletedHandle(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	if err := cleanup.RegisterRunner(ctx, portalResource(handle)); err != nil {
		return fmt.Errorf("registering managed API Portal %q for cleanup: %w", handle, err)
	}
	resp, err := retry.Until(ctx, retry.Options{Timeout: portalDeletionTimeout, Interval: portalPollInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.doAuthenticated(ctx, http.MethodPost, managedPortalsCollection, portalRequestBody(handle, "Synthetic "+handle))
			if err != nil {
				return nil, retry.Transient(err)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && !portalCreateMayClear(resp.StatusCode) })
	if err != nil {
		return err
	}
	if refusedCreate(resp) {
		deregister(ctx, kindAPIPortal, handle)
	}
	return nil
}

// portalCreateMayClear reports whether a refused portal create can succeed when repeated: a
// conflict with a runtime still being torn down, or a server error.
func portalCreateMayClear(status int) bool {
	return status == http.StatusConflict || status >= http.StatusInternalServerError
}
