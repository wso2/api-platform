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
	"sort"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// managedPortalsCollection is the collection managed API Portals are created in, listed in and
	// deleted from.
	managedPortalsCollection = "/managed-api-portals"

	// keyAPIPortalID names the managed API Portal the runner built; the sign-in scenarios use it.
	keyAPIPortalID = "cloudAPIPortalID"
	// keyAPIPortalURL is the public URL of that portal, known once it is active.
	keyAPIPortalURL = "cloudAPIPortalURL"
	// keyRefusedAPIPortalID names the portal whose creation the scenario expects to be refused.
	keyRefusedAPIPortalID = "cloudRefusedAPIPortalID"
	// keyAbsentAPIPortalID names a portal the scenario never creates.
	keyAbsentAPIPortalID = "cloudAbsentAPIPortalID"
	// keyAbsentPortalEnvironment names a login environment the organisation does not have.
	keyAbsentPortalEnvironment = "cloudAbsentPortalEnvironment"

	portalStatusPending  = "pending"
	portalStatusActive   = "active"
	portalStatusFailed   = "failed"
	portalStatusDeleting = "deleting"

	// portalBuildTimeout bounds the wait for a portal to become active. The platform builds a
	// portal in the background, which takes minutes.
	portalBuildTimeout = 15 * time.Minute
	// portalDeletionTimeout bounds the wait for a deleted portal to be gone.
	portalDeletionTimeout = 15 * time.Minute

	// portalHandleMaxLength is the longest handle the platform accepts.
	portalHandleMaxLength = 40

	// keyPortalBuilds counts the portals the runner has asked the platform to build.
	keyPortalBuilds = "cloudAPIPortalBuilds"
	// maxPortalBuilds bounds them: the first, and one replacement for a build the platform failed,
	// so the scenarios that only use a portal still run while the creation scenario reports the
	// failure.
	maxPortalBuilds = 2

	portalDescription = "Created and deleted by the APIP cloud integration test. Safe to delete."
)

// portalPollInterval is how often a portal that is being built or torn down is read, and how often
// a refused cleanup delete is repeated.
var portalPollInterval = 15 * time.Second

// errPortalFailed marks a portal the platform reports as failed.
var errPortalFailed = errors.New("the platform reports the managed API Portal as failed")

// kindAPIPortal is the cleanup kind of a managed API Portal. A portal holds none of the resources
// the suite creates and none holds it, so its place in the order only keeps it apart from them.
var kindAPIPortal = cleanup.Kind{Name: "api-portal", Order: 89}

// portalRequestDefects turn a valid generated handle into a portal request the platform must
// refuse, keyed by how a scenario describes the defect. Each returns the handle and the body.
var portalRequestDefects = map[string]func(handle string) (string, map[string]any){
	"an uppercase handle": func(handle string) (string, map[string]any) {
		handle = strings.ToUpper(handle)
		return handle, portalRequestBody(handle, "Synthetic "+handle)
	},
	"a handle with an underscore": func(handle string) (string, map[string]any) {
		handle = strings.ReplaceAll(handle, "-", "_")
		return handle, portalRequestBody(handle, "Synthetic "+handle)
	},
	"a handle shorter than 3 characters": func(handle string) (string, map[string]any) {
		handle = handle[:2]
		return handle, portalRequestBody(handle, "Synthetic "+handle)
	},
	"a handle longer than 40 characters": func(handle string) (string, map[string]any) {
		if len(handle) <= portalHandleMaxLength {
			handle += "-" + strings.Repeat("x", portalHandleMaxLength-len(handle))
		}
		return handle, portalRequestBody(handle, "Synthetic "+handle)
	},
	"no name": func(handle string) (string, map[string]any) {
		return handle, portalRequestBody(handle, "")
	},
	"a name longer than 255 characters": func(handle string) (string, map[string]any) {
		return handle, portalRequestBody(handle, strings.Repeat("n", portalNameMaxLength+1))
	},
	"a description longer than 1023 characters": func(handle string) (string, map[string]any) {
		body := portalRequestBody(handle, "Synthetic "+handle)
		body["description"] = strings.Repeat("d", portalDescriptionMaxLength+1)
		return handle, body
	},
}

func portalRequestDefectNames() string {
	names := make([]string, 0, len(portalRequestDefects))
	for name := range portalRequestDefects {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// registerPortalSteps binds the steps that create, read, refuse and delete a managed API Portal.
func (s *Steps) registerPortalSteps(sc stepRegistrar) {
	sc.Step(`^I request a synthetic managed API Portal for the runner$`, s.requestRunnerPortal)
	sc.Step(`^the synthetic managed API Portal should be accepted for provisioning$`, s.assertPortalAccepted)
	sc.Step(`^the synthetic managed API Portal should become active$`, s.awaitPortalActive)
	sc.Step(`^the runner has an active synthetic managed API Portal$`, s.ensureActiveRunnerPortal)
	sc.Step(`^the synthetic managed API Portal should report an HTTPS URL$`, s.assertPortalURL)
	sc.Step(`^the managed API Portal list should include the synthetic managed API Portal$`, s.assertPortalListed)
	sc.Step(`^I request a managed API Portal under the handle of the synthetic managed API Portal$`, s.requestDuplicatePortal)
	sc.Step(`^the synthetic managed API Portal should still be active$`, s.assertPortalStillActive)
	sc.Step(`^I request a managed API Portal that has "([^"]*)"$`, s.requestDefectivePortal)
	sc.Step(`^I request a managed API Portal whose login environment does not exist$`, s.requestPortalInAbsentEnvironment)
	sc.Step(`^the refused managed API Portal should not be listed$`, s.assertRefusedPortalNotListed)
	sc.Step(`^I read the synthetic managed API Portal$`, s.readRunnerPortal)
	sc.Step(`^I read a managed API Portal that does not exist$`, s.readAbsentPortal)
	sc.Step(`^I delete the synthetic managed API Portal$`, s.deleteRunnerPortal)
	sc.Step(`^the managed API Portal deletion should be accepted$`, s.assertPortalDeletionAccepted)
	sc.Step(`^the synthetic managed API Portal should eventually be gone$`, s.awaitPortalGone)
	s.registerPortalManageSteps(sc)
	s.registerPortalRuntimeSteps(sc)
	s.registerPublicationDraftSteps(sc)
}

// portalPath is the route of one managed API Portal.
func portalPath(id string) string {
	return managedPortalsCollection + "/" + url.PathEscape(id)
}

// portalLoginEnvironment is the environment the consumers of the portals created here sign in on.
// Every organisation has it from the start.
const portalLoginEnvironment = "development"

// portalRequestBody is the body of a portal create. Consumers sign in on portalLoginEnvironment;
// the sign-up setting is left to the platform's default.
func portalRequestBody(handle, name string) map[string]any {
	return map[string]any{"handle": handle, "name": name, "description": portalDescription,
		"loginEnvironment": portalLoginEnvironment}
}

// portalResource is the cleanup record of a managed API Portal.
func portalResource(id string) cleanup.Resource {
	return cleanup.Resource{Kind: kindAPIPortal, ID: id, Actor: resourceActor, Description: "managed API Portal " + id}
}

// runnerPortal returns the handle of the portal the runner built.
func runnerPortal(ctx context.Context) (string, error) {
	handle, err := requiredValue(ctx, keyAPIPortalID)
	if err != nil {
		return "", fmt.Errorf("the runner has requested no managed API Portal: %w", err)
	}
	return handle, nil
}

// requestRunnerPortal asks for a portal that the later scenarios of the runner keep using, because
// a portal takes minutes to build. It is registered for runner cleanup before the
// request and the registration is dropped again when the platform refuses it. The response is
// published without requiring success.
func (s *Steps) requestRunnerPortal(ctx context.Context) error {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return errors.New("cannot request a runner portal without runner context")
	}
	local.Append(keyPortalBuilds, true)
	if err := generateResourceAndStore(ctx, "syn-portal", keyAPIPortalID); err != nil {
		return err
	}
	tcontext.Remove(ctx, keyAPIPortalURL)
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	if err := cleanup.RegisterRunner(ctx, portalResource(handle)); err != nil {
		return fmt.Errorf("registering managed API Portal %q for cleanup: %w", handle, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, managedPortalsCollection, portalRequestBody(handle, "Synthetic "+handle))
	if err != nil {
		return err
	}
	if refusedCreate(resp) {
		deregister(ctx, kindAPIPortal, handle)
	}
	return nil
}

// assertPortalAccepted checks the published create answer took the portal: 202 when the platform
// builds it in the background, which reads as pending, or 201 when the platform built it before
// answering. Either way the answer names the requested handle.
func (s *Steps) assertPortalAccepted(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("the managed API Portal request was not accepted: %s", resp.Describe())
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("reading the id of the accepted portal: %w (%s)", err, resp.Describe())
	}
	if id != handle {
		return fmt.Errorf("the accepted portal is %q, want the requested handle %q", id, handle)
	}
	if resp.StatusCode == http.StatusAccepted {
		status, err := stringField(resp, "status")
		if err != nil || status != portalStatusPending {
			return fmt.Errorf("a portal accepted for provisioning should read as %s: %s", portalStatusPending, resp.Describe())
		}
	}
	return nil
}

// portalReadTransient reports whether a failed portal read may succeed when repeated: a portal that
// is still being built can briefly read as not found.
func portalReadTransient(status int) bool {
	return status == http.StatusNotFound || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

// awaitPortalActive waits until the runner's portal reads as active, stops at once when it reads
// as failed, publishes the last read and records the portal's URL. A portal that is already active
// passes on the first read.
func (s *Steps) awaitPortalActive(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	var last *httpx.Response
	err = retry.Await(ctx, retry.Options{Timeout: portalBuildTimeout, Interval: portalPollInterval},
		func(ctx context.Context) (string, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, portalPath(handle), nil)
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				if portalReadTransient(resp.StatusCode) {
					return "", retry.Transient(responseError(resp))
				}
				return "", responseError(resp)
			}
			last = resp
			status, err := stringField(resp, "status")
			if err != nil {
				return "", retry.Transient(fmt.Errorf("reading the status of managed API Portal %q: %w (%s)", handle, err, resp.Describe()))
			}
			if status == portalStatusFailed {
				return "", fmt.Errorf("%w: %q reads as %s: %s", errPortalFailed, handle, portalStatusFailed, resp.Describe())
			}
			return status, nil
		},
		func(status string) bool { return status == portalStatusActive },
		"waiting for managed API Portal "+handle+" to become active")
	if err != nil {
		return err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, last); err != nil {
			return err
		}
	}
	if portalURL, err := stringField(last, "url"); err == nil {
		return tcontext.Set(ctx, keyAPIPortalURL, portalURL)
	}
	return nil
}

// ensureActiveRunnerPortal gives a scenario that only uses a portal an active one: the runner's
// portal, waited for while it is built. When the platform failed that build, or refused it, the
// failed portal is deleted, which frees the organisation's portal quota, and one replacement is
// built. The creation scenario still reports the failure; a second failed build fails here too.
func (s *Steps) ensureActiveRunnerPortal(ctx context.Context) error {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return errors.New("cannot find the runner portal without runner context")
	}
	for {
		handle, haveHandle := "", false
		if value, err := requiredValue(ctx, keyAPIPortalID); err == nil {
			handle, haveHandle = value, true
		}
		var waitErr error
		if haveHandle && !portalRegistered(ctx, handle) {
			// The platform refused this portal, so there is nothing to wait for or delete.
			haveHandle, waitErr = false, fmt.Errorf("%w: the request for %q was refused", errPortalAbsent, handle)
		}
		if haveHandle {
			if waitErr = s.awaitPortalActive(ctx); waitErr == nil {
				return nil
			}
			if !errors.Is(waitErr, errPortalFailed) && !errors.Is(waitErr, errPortalAbsent) {
				return waitErr
			}
		}
		if builds := len(local.List(keyPortalBuilds)); builds >= maxPortalBuilds {
			if waitErr == nil {
				waitErr = errPortalAbsent
			}
			return fmt.Errorf("the runner asked for %d managed API Portals and none became active: %w", builds, waitErr)
		}
		if haveHandle {
			if err := s.removeRunnerPortal(ctx, handle); err != nil {
				return fmt.Errorf("removing the failed managed API Portal %q before building another: %w", handle, err)
			}
		}
		if err := s.requestRunnerPortal(ctx); err != nil {
			return err
		}
		if err := s.assertPortalAccepted(ctx); err != nil {
			return err
		}
	}
}

// errPortalAbsent marks a runner portal the platform does not have, such as one whose creation it
// refused.
var errPortalAbsent = errors.New("the platform has no such managed API Portal")

// portalRegistered reports whether a portal is waiting to be cleaned up: a portal the platform took
// stays registered, and one it refused is dropped.
func portalRegistered(ctx context.Context, id string) bool {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return false
	}
	for _, resource := range registry.Pending() {
		if resource.Kind == kindAPIPortal && resource.ID == id {
			return true
		}
	}
	return false
}

// removeRunnerPortal deletes a portal of the runner, waits until it is gone and drops its cleanup
// registration.
func (s *Steps) removeRunnerPortal(ctx context.Context, handle string) error {
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return err
	}
	bearer, err := bearerToken(ctx)
	if err != nil {
		return err
	}
	if err := deletePortal(ctx, s.client, base, bearer, handle, portalBuildTimeout); err != nil {
		return err
	}
	if err := awaitPortalDeleted(ctx, s.client, base, bearer, handle, portalDeletionTimeout); err != nil {
		return err
	}
	deregister(ctx, kindAPIPortal, handle)
	return nil
}

// awaitPortalDeleted waits until reading a portal answers 404. While it is torn down it reads as
// deleting.
func awaitPortalDeleted(ctx context.Context, client *httpx.Client, base, bearer, id string, timeout time.Duration) error {
	return retry.Await(ctx, retry.Options{Timeout: timeout, Interval: portalPollInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := client.Do(ctx, httpx.Request{
				Method:  http.MethodGet,
				URL:     cloudAPIURL(base, portalPath(id)),
				Headers: map[string]string{"Authorization": "Bearer " + bearer},
			}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				return nil, retry.Transient(responseError(resp))
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == http.StatusNotFound },
		"waiting for managed API Portal "+id+" to be gone")
}

// portalBaseURL parses the runner portal's recorded URL and checks that a consumer's password may
// be sent to it.
func portalBaseURL(ctx context.Context) (*url.URL, error) {
	raw, err := requiredValue(ctx, keyAPIPortalURL)
	if err != nil {
		return nil, fmt.Errorf("the runner's managed API Portal has reported no URL: %w", err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("the managed API Portal URL %q does not parse: %w", raw, err)
	}
	if err := requireSecureURL(parsed); err != nil {
		return nil, fmt.Errorf("the managed API Portal URL: %w", err)
	}
	return parsed, nil
}

// requireSecureURL accepts an absolute https URL without credentials, and an http URL on a named
// .localhost host only when the run targets a cloud on the developer's machine: a consumer's
// password and session cookie travel to it.
func requireSecureURL(parsed *url.URL) error {
	if parsed == nil {
		return fmt.Errorf("no URL")
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("%q is not an absolute URL without credentials", parsed.Redacted())
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if localDataPlaneEnabled() && approvedLocalDataPlaneTarget(parsed) {
		return nil
	}
	return fmt.Errorf("%q does not use https", parsed.Redacted())
}

// assertPortalURL checks the runner's portal reported a URL a consumer can open.
func (s *Steps) assertPortalURL(ctx context.Context) error {
	_, err := portalBaseURL(ctx)
	return err
}

// assertPortalListed lists the portals, publishes the answer and checks the runner's portal is
// among them and active.
func (s *Steps) assertPortalListed(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	items, err := s.listPortals(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item["id"] == handle {
			if item["status"] != portalStatusActive {
				return fmt.Errorf("managed API Portal %q is listed with the status %v, want %s", handle, item["status"], portalStatusActive)
			}
			return nil
		}
	}
	return fmt.Errorf("the managed API Portal list does not include %q", handle)
}

// listPortals lists the portals, publishes the answer and requires it to succeed.
func (s *Steps) listPortals(ctx context.Context) ([]map[string]any, error) {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, managedPortalsCollection, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, fmt.Errorf("decoding managed API Portals: %w (%s)", err, resp.Describe())
	}
	return items, nil
}

// requestDuplicatePortal asks for a second portal under the runner portal's handle, with a name of
// its own so only the handle can collide, and publishes the answer. The handle is already
// registered for runner cleanup.
func (s *Steps) requestDuplicatePortal(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, managedPortalsCollection, portalRequestBody(handle, "Synthetic duplicate of "+handle))
	return err
}

// assertPortalStillActive reads the runner's portal once, publishes the answer and checks it is
// still active.
func (s *Steps) assertPortalStillActive(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, portalPath(handle), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	if status, _ := stringField(resp, "status"); status != portalStatusActive {
		return fmt.Errorf("managed API Portal %q reads as %q, want %s", handle, status, portalStatusActive)
	}
	return nil
}

// requestRefusedPortal registers a portal for scenario cleanup, asks for it and publishes the
// answer without requiring success. The registration is dropped again when the platform refuses
// the request, and kept when it takes it, so a portal accepted against expectation is removed.
func (s *Steps) requestRefusedPortal(ctx context.Context, handle string, body map[string]any) error {
	if err := tcontext.Set(ctx, keyRefusedAPIPortalID, handle); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, portalResource(handle)); err != nil {
		return fmt.Errorf("registering managed API Portal %q for cleanup: %w", handle, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, managedPortalsCollection, body)
	if err != nil {
		return err
	}
	if refusedCreate(resp) {
		deregister(ctx, kindAPIPortal, handle)
	}
	return nil
}

// requestDefectivePortal asks for a portal whose request breaks the named rule.
func (s *Steps) requestDefectivePortal(ctx context.Context, defect string) error {
	corrupt, ok := portalRequestDefects[defect]
	if !ok {
		return fmt.Errorf("unknown managed API Portal request defect %q: the defects are %s", defect, portalRequestDefectNames())
	}
	if err := generateResourceAndStore(ctx, "syn-portal", keyRefusedAPIPortalID); err != nil {
		return err
	}
	valid, err := requiredValue(ctx, keyRefusedAPIPortalID)
	if err != nil {
		return err
	}
	handle, body := corrupt(valid)
	return s.requestRefusedPortal(ctx, handle, body)
}

// requestPortalInAbsentEnvironment asks for a portal whose consumers sign in on an environment the
// organisation does not have.
func (s *Steps) requestPortalInAbsentEnvironment(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-portal", keyRefusedAPIPortalID); err != nil {
		return err
	}
	handle, err := requiredValue(ctx, keyRefusedAPIPortalID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-no-env", keyAbsentPortalEnvironment); err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyAbsentPortalEnvironment)
	if err != nil {
		return err
	}
	body := portalRequestBody(handle, "Synthetic "+handle)
	body["loginEnvironment"] = environment
	return s.requestRefusedPortal(ctx, handle, body)
}

// assertRefusedPortalNotListed checks the refused portal left nothing behind. It reads the list
// rather than the portal, because a handle that breaks the naming rules is refused as a path too.
// The list answering is the positive control. It reads once: a refused create stores nothing.
func (s *Steps) assertRefusedPortalNotListed(ctx context.Context) error {
	handle, err := requiredValue(ctx, keyRefusedAPIPortalID)
	if err != nil {
		return fmt.Errorf("the scenario has requested no managed API Portal that should be refused: %w", err)
	}
	items, err := s.listPortals(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if id, _ := item["id"].(string); strings.EqualFold(id, handle) {
			return fmt.Errorf("managed API Portal %q is listed after its creation was refused", handle)
		}
	}
	return nil
}

// readRunnerPortal reads the runner's portal and publishes the answer.
func (s *Steps) readRunnerPortal(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, portalPath(handle), nil)
	return err
}

// readAbsentPortal reads a portal nothing has created and publishes the answer.
func (s *Steps) readAbsentPortal(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-portal", keyAbsentAPIPortalID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyAbsentAPIPortalID)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, portalPath(id), nil)
	return err
}

// deleteRunnerPortal deletes the runner's portal and publishes the answer.
func (s *Steps) deleteRunnerPortal(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodDelete, portalPath(handle), nil)
	return err
}

// assertPortalDeletionAccepted checks the published delete answer took the deletion: 202 while the
// platform tears the portal down in the background, which reads as deleting, or 204 when the
// portal was removed before the answer.
func (s *Steps) assertPortalDeletionAccepted(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusAccepted:
		if status, err := stringField(resp, "status"); err != nil || status != portalStatusDeleting {
			return fmt.Errorf("a portal accepted for deletion should read as %s: %s", portalStatusDeleting, resp.Describe())
		}
		return nil
	default:
		return fmt.Errorf("the managed API Portal deletion was not accepted: %s", resp.Describe())
	}
}

// awaitPortalGone waits until reading the runner's portal answers 404, drops its cleanup
// registration and checks the list no longer includes it. While it is torn down it reads as
// deleting.
func (s *Steps) awaitPortalGone(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return err
	}
	bearer, err := bearerToken(ctx)
	if err != nil {
		return err
	}
	if err := awaitPortalDeleted(ctx, s.client, base, bearer, handle, portalDeletionTimeout); err != nil {
		return err
	}
	deregister(ctx, kindAPIPortal, handle)
	items, err := s.listPortals(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item["id"] == handle {
			return fmt.Errorf("managed API Portal %q reads as gone but is still listed with the status %v", handle, item["status"])
		}
	}
	return nil
}

// portalDeleter deletes the managed API Portals a scenario or runner registered.
func portalDeleter(topo *frameworkruntime.Topology, client *httpx.Client) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		// A portal that is still being built refuses the delete until its build ends, and its
		// teardown takes time after that, so the deletion has a budget of its own rather than the
		// sweep's. It waits until the portal is gone, which frees the organisation's portal quota
		// and leaves the run-end finalizer nothing to find.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), portalBuildTimeout+portalDeletionTimeout+30*time.Second)
		defer cancel()
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		if err := deletePortal(ctx, client, base, bearer, resource.ID, portalBuildTimeout); err != nil {
			return err
		}
		return awaitPortalDeleted(ctx, client, base, bearer, resource.ID, portalDeletionTimeout)
	}
}

// deletePortal asks the platform to delete one portal and does not wait for its teardown, which
// the platform finishes after answering. A portal that is still being built, or is changing,
// refuses the delete with 409 until that ends, so the delete is repeated within timeout. An absent
// portal counts as deleted.
func deletePortal(ctx context.Context, client *httpx.Client, base, bearer, id string, timeout time.Duration) error {
	return retry.Await(ctx, retry.Options{Timeout: timeout, Interval: portalPollInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := client.Do(ctx, httpx.Request{
				Method:  http.MethodDelete,
				URL:     cloudAPIURL(base, portalPath(id)),
				Headers: map[string]string{"Authorization": "Bearer " + bearer},
			}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			switch {
			case resp.StatusCode == http.StatusNotFound || resp.Succeeded():
				return resp, nil
			case resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusTooManyRequests ||
				resp.StatusCode >= http.StatusInternalServerError:
				return nil, retry.Transient(responseError(resp))
			default:
				return nil, responseError(resp)
			}
		},
		func(resp *httpx.Response) bool { return resp != nil },
		"deleting managed API Portal "+id)
}
