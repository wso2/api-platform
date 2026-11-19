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

package portal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	keyPlanID          = "cloudSubscriptionPlanID"
	keySecondPlanID    = "cloudSecondSubscriptionPlanID"
	keyDocumentID      = "cloudDocumentID"
	landingPageText    = "Use this API."
	publicationVersion = "1.0.0"
)

var (
	// A publication is removed before the API it lists, and a document before its API.
	CloudPublicationKind = cleanup.Kind{Name: "cloud-api-publication", Order: 45}
	CloudDocumentKind    = cleanup.Kind{Name: "cloud-api-document", Order: 47}
)

// publicationSubject is an API a publication step acts on.
type publicationSubject struct {
	apiType string
	key     string
}

var publicationSubjects = map[string]publicationSubject{
	"synthetic REST API":    {apiType: "rest-api", key: cloudbase.KeyAPIID},
	"synthetic GraphQL API": {apiType: "graphql-api", key: cloudbase.KeyGraphQLAPIID},
}

const subjectPattern = `(synthetic REST API|synthetic GraphQL API)`

// draftListKeys are the draft fields a values table sets as JSON arrays.
var draftListKeys = map[string]bool{"subscriptionPlanIds": true, "docIds": true, "tags": true}

// draftTextKeys are the draft fields a values table sets as text.
var draftTextKeys = map[string]bool{"displayName": true, "version": true, "description": true, "agentVisibility": true}

// draftObjectKeys are the draft fields a values table sets as JSON objects.
var draftObjectKeys = map[string]bool{"endpoints": true, "owners": true}

func (s *Steps) publicationStepDefinitions() []cloudbase.StepDefinition {
	return []cloudbase.StepDefinition{
		{Pattern: `^I find an active managed API Portal$`, Handler: s.findActiveAPIPortal},
		{Pattern: `^I create a synthetic subscription plan$`, Handler: s.createPlan},
		{Pattern: `^I create another synthetic subscription plan$`, Handler: s.createSecondPlan},
		{Pattern: `^I deactivate the synthetic subscription plan$`, Handler: s.deactivatePlan},
		{Pattern: `^I try to delete the synthetic subscription plan$`, Handler: s.tryDeletePlan},
		{Pattern: `^I create a synthetic GraphQL API in the APIP test project$`, Handler: s.CreateGraphQLAPI},
		{Pattern: `^I create a synthetic document for the synthetic REST API$`, Handler: s.createDocument},
		{Pattern: `^I save a publication draft of the ` + subjectPattern + `$`, Handler: s.saveDraft},
		{Pattern: `^I save a publication draft of the ` + subjectPattern + ` with:$`, Handler: s.saveDraftWith},
		{Pattern: `^I try to save a publication draft of the ` + subjectPattern + `$`, Handler: s.trySaveDraft},
		{Pattern: `^I try to save a publication draft naming an unknown (subscription plan|API|API Portal)$`, Handler: s.trySaveDraftNamingUnknown},
		{Pattern: `^I save an? (valid|invalid|OpenAPI) definition on the publication draft of the ` + subjectPattern + `$`, Handler: s.saveDefinition},
		{Pattern: `^I save a landing page on the publication draft of the ` + subjectPattern + `$`, Handler: s.saveLandingPage},
		{Pattern: `^I save a PNG thumbnail on the publication draft of the ` + subjectPattern + `$`, Handler: s.saveThumbnail},
		{Pattern: `^I (publish|deprecate|unpublish) the ` + subjectPattern + ` on the managed API Portal$`, Handler: s.publicationAction},
		{Pattern: `^I publish the synthetic GraphQL API through the REST API route$`, Handler: s.publishGraphQLThroughRESTRoute},
		{Pattern: `^I send a "([^"]*)" request (without a token|with a forged token) to the publication path "([^"]*)"$`, Handler: s.sendWithoutValidToken},
		{Pattern: `^the ` + subjectPattern + ` should be (PUBLISHED|DEPRECATED) on the managed API Portal$`, Handler: s.assertPublicationStatus},
		{Pattern: `^the ` + subjectPattern + ` should not be published on the managed API Portal$`, Handler: s.assertNotPublished},
		{Pattern: `^the publication cleanup of the synthetic REST API is no longer needed$`, Handler: s.dropPublicationCleanup},
		{Pattern: `^the API publications of the ` + subjectPattern + ` should show the managed API Portal as (PUBLISHED|DEPRECATED|NOT_PUBLISHED)$`, Handler: s.assertPublicationSummary},
		{Pattern: `^the publication of the ` + subjectPattern + ` should offer (the synthetic subscription plan|both synthetic subscription plans)$`, Handler: s.assertPublicationPlans},
		{Pattern: `^the publication of the ` + subjectPattern + ` should have the display name "([^"]*)"$`, Handler: s.assertPublicationDisplayName},
		{Pattern: `^the publication of the synthetic REST API should (include|not include) the synthetic document$`, Handler: s.assertPublicationDocument},
		{Pattern: `^the publication of the synthetic REST API should have a landing page and a thumbnail$`, Handler: s.assertPublicationLandingPageAndThumbnail},
		{Pattern: `^the publication draft of the ` + subjectPattern + ` should still exist$`, Handler: s.assertDraftExists},
		{Pattern: `^the publication draft of the synthetic REST API should keep its details and definition$`, Handler: s.assertDraftDetails},
		{Pattern: `^the landing page of the publication draft of the synthetic REST API should have no raw HTML$`, Handler: s.assertDraftLandingPageSanitized},
	}
}

// findActiveAPIPortal picks a managed API Portal whose provisioning has completed. Scenarios
// publish to an existing portal: provisioning one starts a portal runtime for the organization.
func (s *Steps) findActiveAPIPortal(ctx context.Context) error {
	resp, err := s.DoAuthenticated(ctx, http.MethodGet, "/managed-api-portals", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	items, err := cloudbase.ListItems(resp)
	if err != nil {
		return fmt.Errorf("decoding managed API Portals: %w", err)
	}
	id, err := activeAPIPortalID(items)
	if err != nil {
		return err
	}
	tcontext.Remove(ctx, keyAPIPortalURL)
	for _, item := range items {
		if item["id"] == id {
			if portalURL, _ := item["url"].(string); portalURL != "" {
				if err := tcontext.Set(ctx, keyAPIPortalURL, portalURL); err != nil {
					return err
				}
			}
		}
	}
	return tcontext.Set(ctx, keyAPIPortalID, id)
}

// activeAPIPortalID returns the first active portal the suite may use: one named with the suite's
// prefix that no scenario created.
func activeAPIPortalID(items []map[string]any) (string, error) {
	statuses := make([]string, 0, len(items))
	for _, item := range items {
		id, _ := item["id"].(string)
		status, _ := item["status"].(string)
		if strings.HasPrefix(id, cloudbase.SyntheticPortalPrefix) || !cloudbase.OwnedPortal(item) {
			continue
		}
		if strings.TrimSpace(id) != "" && status == "active" {
			return id, nil
		}
		statuses = append(statuses, fmt.Sprintf("%s: %s", id, status))
	}
	if len(statuses) == 0 {
		return "", fmt.Errorf("the organization has no managed API Portal named with the prefix %q", cloudbase.TestResourcePrefix)
	}
	return "", fmt.Errorf("no active managed API Portal named with the prefix %q among %d listed: %s", cloudbase.TestResourcePrefix, len(statuses), strings.Join(statuses, "; "))
}

func (s *Steps) createPlan(ctx context.Context) error {
	return s.createPlanAs(ctx, keyPlanID)
}

func (s *Steps) createSecondPlan(ctx context.Context) error {
	return s.createPlanAs(ctx, keySecondPlanID)
}

// syntheticPlanRequestLimit is per minute, high enough that polling the gateway never hits it.
const syntheticPlanRequestLimit = 1000

func planBody(id, status string) map[string]any {
	return map[string]any{
		"id": id, "displayName": cloudbase.PublicationTitle + id, "status": status,
		"limits": []map[string]any{{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": syntheticPlanRequestLimit}},
	}
}

func (s *Steps) createPlanAs(ctx context.Context, key string) error {
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-plan", key); err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, key)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindSubscriptionPlan, ID: id, Actor: cloudbase.ResourceActor, Description: "subscription plan " + id,
	}); err != nil {
		return fmt.Errorf("registering subscription plan %q for cleanup: %w", id, err)
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost, "/subscription-plans", planBody(id, "ACTIVE"))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		if cloudbase.RefusedCreate(resp) {
			cloudbase.Deregister(ctx, cleanup.KindSubscriptionPlan, id)
		}
		return cloudbase.ResponseError(resp)
	}
	return nil
}

func (s *Steps) deactivatePlan(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPut, "/subscription-plans/"+url.PathEscape(id), planBody(id, "INACTIVE"))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return cloudbase.ResponseError(resp)
	}
	return nil
}

func (s *Steps) tryDeletePlan(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodDelete, "/subscription-plans/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		cloudbase.Deregister(ctx, cleanup.KindSubscriptionPlan, id)
	}
	return nil
}

func documentsPath(apiType, apiID string) string {
	return "/apis/" + apiType + "/" + url.PathEscape(apiID) + "/docs"
}

func (s *Steps) createDocument(ctx context.Context) error {
	apiID, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-doc", keyDocumentID); err != nil {
		return err
	}
	docID, err := cloudbase.RequiredValue(ctx, keyDocumentID)
	if err != nil {
		return err
	}
	body, contentType, err := cloudbase.MultipartFields([][2]string{
		{"id", docID}, {"displayName", cloudbase.PublicationTitle + docID}, {"type", "HowTo"},
		{"inlineContent", "Step one: call the API."},
	})
	if err != nil {
		return err
	}
	cleanupID := strings.Join([]string{"rest-api", apiID, docID}, cloudbase.PublicationIDGlue)
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: CloudDocumentKind, ID: cleanupID, Actor: cloudbase.ResourceActor, Description: "document " + docID,
	}); err != nil {
		return fmt.Errorf("registering document %q for cleanup: %w", docID, err)
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPost, documentsPath("rest-api", apiID), contentType, body)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		if cloudbase.RefusedCreate(resp) {
			cloudbase.Deregister(ctx, CloudDocumentKind, cleanupID)
		}
		return cloudbase.ResponseError(resp)
	}
	return nil
}

// DeleteDocumentRequest builds the cleanup request for a document id made of the API type, the
// API and the document.
func DeleteDocumentRequest(id string) (string, string, func(*httpx.Response) bool, error) {
	parts, err := cloudbase.SplitPublicationID(id)
	if err != nil {
		return "", "", nil, err
	}
	settled := func(resp *httpx.Response) bool {
		return resp.Succeeded() || resp.StatusCode == http.StatusNotFound
	}
	return http.MethodDelete, documentsPath(parts[0], parts[1]) + "/" + url.PathEscape(parts[2]), settled, nil
}

func targetOf(ctx context.Context, subject string) (cloudbase.PublicationTarget, error) {
	ref, ok := publicationSubjects[subject]
	if !ok {
		return cloudbase.PublicationTarget{}, fmt.Errorf("unsupported publication subject %q", subject)
	}
	portal, err := cloudbase.RequiredValue(ctx, keyAPIPortalID)
	if err != nil {
		return cloudbase.PublicationTarget{}, err
	}
	api, err := cloudbase.RequiredValue(ctx, ref.key)
	if err != nil {
		return cloudbase.PublicationTarget{}, err
	}
	return cloudbase.PublicationTarget{Portal: portal, APIType: ref.apiType, API: api}, nil
}

// draftBody is the draft a scenario saves: the API's title and version, the synthetic plan, and
// whatever the values table replaces.
func draftBody(api, plan string, overrides map[string]string) (map[string]any, error) {
	body := map[string]any{
		"displayName": cloudbase.PublicationTitle + api, "version": publicationVersion,
		"subscriptionPlanIds": []string{plan},
	}
	for key, value := range overrides {
		switch {
		case draftTextKeys[key]:
			body[key] = value
		case draftObjectKeys[key]:
			var object map[string]any
			if err := json.Unmarshal([]byte(value), &object); err != nil {
				return nil, fmt.Errorf("draft field %q must be a JSON object: %w", key, err)
			}
			body[key] = object
		case draftListKeys[key]:
			var list []string
			if err := json.Unmarshal([]byte(value), &list); err != nil {
				return nil, fmt.Errorf("draft field %q must be a JSON array of strings: %w", key, err)
			}
			body[key] = list
		default:
			return nil, fmt.Errorf("unsupported draft field %q", key)
		}
	}
	return body, nil
}

func (s *Steps) saveDraft(ctx context.Context, subject string) error {
	return s.saveDraftWith(ctx, subject, nil)
}

func (s *Steps) saveDraftWith(ctx context.Context, subject string, table *godog.Table) error {
	resp, err := s.sendDraft(ctx, subject, table)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	return nil
}

func (s *Steps) trySaveDraft(ctx context.Context, subject string) error {
	_, err := s.sendDraft(ctx, subject, nil)
	return err
}

func (s *Steps) sendDraft(ctx context.Context, subject string, table *godog.Table) (*httpx.Response, error) {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return nil, err
	}
	plan, err := cloudbase.RequiredValue(ctx, keyPlanID)
	if err != nil {
		return nil, err
	}
	overrides, err := cloudbase.TableValues(table)
	if err != nil {
		return nil, err
	}
	for key, value := range overrides {
		if overrides[key], err = cloudbase.ExpandCloudValue(ctx, value); err != nil {
			return nil, err
		}
	}
	body, err := draftBody(target.API, plan, overrides)
	if err != nil {
		return nil, err
	}
	return s.DoAuthenticated(ctx, http.MethodPut, target.Path("/draft"), body)
}

// trySaveDraftNamingUnknown saves the synthetic REST API's draft with one of its references
// pointing at something the organization does not have.
func (s *Steps) trySaveDraftNamingUnknown(ctx context.Context, unknown string) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	plan, err := cloudbase.RequiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	switch unknown {
	case "subscription plan":
		plan += "-missing"
	case "API":
		target.API += "-missing"
	case "API Portal":
		target.Portal += "-missing"
	default:
		return fmt.Errorf("unsupported unknown reference %q", unknown)
	}
	body, err := draftBody(target.API, plan, nil)
	if err != nil {
		return err
	}
	_, err = s.DoAuthenticated(ctx, http.MethodPut, target.Path("/draft"), body)
	return err
}

// definitionFor returns the definition a step saves and its content type. A valid definition
// matches the API's type; the invalid and OpenAPI ones are refused at save or at publish.
func definitionFor(kind, apiType, title string) ([]byte, string, error) {
	switch {
	case kind == "valid" && apiType == "graphql-api":
		return []byte(cloudbase.GraphQLSchema), "application/graphql", nil
	case kind == "valid" || kind == "OpenAPI":
		body, err := json.Marshal(map[string]any{
			"openapi": "3.0.3",
			"info":    map[string]any{"title": title, "version": publicationVersion},
			"paths": map[string]any{"/ping": map[string]any{"get": map[string]any{
				"responses": map[string]any{"200": map[string]any{"description": "ok"}},
			}}},
		})
		return body, "application/json", err
	case kind == "invalid":
		return []byte(`{"swagger": "not an OpenAPI document"}`), "application/json", nil
	default:
		return nil, "", fmt.Errorf("unsupported definition %q", kind)
	}
}

// saveDefinition requires a valid definition to be stored. Any other is either stored or refused
// for its content at once (400 PUBLICATION_VALIDATION_FAILED), and the publish refuses it the same
// way; any other answer is an error.
func (s *Steps) saveDefinition(ctx context.Context, kind, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	body, contentType, err := definitionFor(kind, target.APIType, target.API)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPut, target.Path("/draft/definition"), contentType, body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if kind != "valid" && refusedAtSave(resp) {
		return nil
	}
	return cloudbase.ResponseError(resp)
}

// publicationValidationFailed is the error code of a draft part the control plane refuses.
const publicationValidationFailed = "PUBLICATION_VALIDATION_FAILED"

// refusedAtSave reports whether the answer is the control plane refusing the saved content for
// what it is. Any other answer, such as a 401 or a 500, is a failed upload, not a refusal.
func refusedAtSave(resp *httpx.Response) bool {
	if resp.StatusCode != http.StatusBadRequest {
		return false
	}
	code, err := cloudbase.StringField(resp, "code")
	return err == nil && code == publicationValidationFailed
}

// landingPage carries raw HTML, which the control plane strips before storing it.
const landingPage = "# Welcome\n\n<script>alert(1)</script>\n\n" + landingPageText + "\n"

func (s *Steps) saveLandingPage(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPut, target.Path("/draft/landing-page"), "text/markdown", []byte(landingPage))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return cloudbase.ResponseError(resp)
	}
	return nil
}

func (s *Steps) saveThumbnail(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	body, contentType, err := thumbnailForm()
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPut, target.Path("/draft/thumbnail"), contentType, body)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return cloudbase.ResponseError(resp)
	}
	return nil
}

// thumbnailPNG is a 1x1 PNG.
func thumbnailPNG() ([]byte, error) {
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		return nil, err
	}
	return img.Bytes(), nil
}

// thumbnailForm is a multipart form holding a 1x1 PNG.
func thumbnailForm() ([]byte, string, error) {
	img, err := thumbnailPNG()
	if err != nil {
		return nil, "", err
	}
	return thumbnailFormOf(img)
}

// thumbnailFormOf is a multipart form that declares its file as a PNG whatever the bytes are.
func thumbnailFormOf(content []byte) ([]byte, string, error) {
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="thumbnail.png"`)
	header.Set("Content-Type", "image/png")
	part, err := form.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), form.FormDataContentType(), nil
}

// portalUnavailableCode is the code the platform answers with when it cannot reach the API Portal
// at that moment; the answer itself asks the caller to try again.
const portalUnavailableCode = "PUBLICATION_PORTAL_UNAVAILABLE"

// publicationRetryInterval is the pause before a publication action is sent again after the
// platform could not reach the API Portal, and publicationRetryTimeout how long that is retried.
var (
	publicationRetryInterval = 5 * time.Second
	publicationRetryTimeout  = 2 * time.Minute
)

// sendPublicationAction sends a publication action and sends it again while the platform answers
// that it cannot reach the API Portal. Any other answer, including a refusal a scenario expects, is
// returned and published at once. When the portal stays unreachable the last answer is published
// for the scenario to assert.
func (s *Steps) sendPublicationAction(ctx context.Context, target cloudbase.PublicationTarget, action string) (*httpx.Response, error) {
	var last *httpx.Response
	err := retry.Await(ctx, retry.Options{Timeout: publicationRetryTimeout, Interval: publicationRetryInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.DoAuthenticated(ctx, http.MethodPost, target.Path("/"+action), nil)
			if err != nil {
				return nil, err
			}
			last = resp
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && !portalUnavailable(resp) },
		"waiting for the platform to reach the API Portal")
	if err != nil && last != nil && portalUnavailable(last) {
		return last, nil
	}
	return last, err
}

// portalUnavailable reports whether the platform could not reach the API Portal for the request.
func portalUnavailable(resp *httpx.Response) bool {
	if resp.StatusCode != http.StatusServiceUnavailable {
		return false
	}
	code, err := cloudbase.StringField(resp, "code")
	return err == nil && code == portalUnavailableCode
}

// publicationAction publishes, deprecates or unpublishes and leaves the answer for the scenario
// to assert. The unpublish that removes a listing is registered before a publish is sent, since a
// publish can fail after the portal accepted it.
func (s *Steps) publicationAction(ctx context.Context, action, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	if action == "publish" {
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: CloudPublicationKind, ID: target.ID(), Actor: cloudbase.ResourceActor,
			Description: target.API + " on API Portal " + target.Portal,
		}); err != nil {
			return fmt.Errorf("registering the publication of %q for cleanup: %w", target.API, err)
		}
	}
	resp, err := s.sendPublicationAction(ctx, target, action)
	if err != nil {
		return err
	}
	if action == "unpublish" && resp.Succeeded() {
		cloudbase.Deregister(ctx, CloudPublicationKind, target.ID())
	}
	return nil
}

func (s *Steps) publishGraphQLThroughRESTRoute(ctx context.Context) error {
	target, err := targetOf(ctx, "synthetic GraphQL API")
	if err != nil {
		return err
	}
	target.APIType = "rest-api"
	_, err = s.DoAuthenticated(ctx, http.MethodPost, target.Path("/publish"), nil)
	return err
}

// dropPublicationCleanup deregisters the cleanup of the synthetic REST API's publication. A
// scenario that deleted the API uses it only after it has seen the portal stop serving the API:
// the platform answers an unpublish for a missing API with 404 API_NOT_FOUND before it asks the
// portal to remove the listing, so the cleanup could not remove one that was left behind. When
// an earlier step fails, the cleanup stays registered and reports the listing as unsettled.
func (s *Steps) dropPublicationCleanup(ctx context.Context) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	cloudbase.Deregister(ctx, CloudPublicationKind, target.ID())
	return nil
}

// forgedToken is shaped like a console token but signed by no key the cloud trusts.
func forgedToken() string {
	encode := func(v map[string]any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	header := encode(map[string]any{"alg": "RS256", "typ": "JWT"})
	claims := encode(map[string]any{"sub": "apip-cloud-test", "exp": time.Now().Add(time.Hour).Unix()})
	return header + "." + claims + "." + base64.RawURLEncoding.EncodeToString([]byte("not-a-signature"))
}

func (s *Steps) sendWithoutValidToken(ctx context.Context, method, credential, path string) error {
	path, err := cloudbase.ExpandCloudValue(ctx, path)
	if err != nil {
		return err
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if credential == "with a forged token" {
		headers["Authorization"] = "Bearer " + forgedToken()
	}
	var body []byte
	if method == http.MethodPut || method == http.MethodPost {
		body = []byte("{}")
	}
	_, err = s.Send(ctx, strings.ToUpper(method), path, headers, body)
	return err
}

func (s *Steps) readPublication(ctx context.Context, subject string) (map[string]any, error) {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, target.Path("/publication"), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the publication of %q: %s", target.API, resp.Describe())
	}
	var publication map[string]any
	if err := json.Unmarshal(resp.Body, &publication); err != nil {
		return nil, fmt.Errorf("decoding the publication of %q: %w", target.API, err)
	}
	return publication, nil
}

func (s *Steps) assertPublicationStatus(ctx context.Context, subject, want string) error {
	publication, err := s.readPublication(ctx, subject)
	if err != nil {
		return err
	}
	if got := publication["status"]; got != want {
		return fmt.Errorf("publication status = %v, want %s", got, want)
	}
	return nil
}

func (s *Steps) assertNotPublished(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, target.Path("/publication"), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("expected %q to have no publication, got %s", target.API, resp.Describe())
	}
	return nil
}

// assertPublicationSummary reads the per-portal summary the console shows for an API.
func (s *Steps) assertPublicationSummary(ctx context.Context, subject, want string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	query := url.Values{"apiType": {target.APIType}, "apiId": {target.API}, "limit": {"100"}}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, "/api-publications?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	items, err := cloudbase.ListItems(resp)
	if err != nil {
		return fmt.Errorf("decoding API publications: %w", err)
	}
	return summaryStatusIs(items, target.Portal, want)
}

func summaryStatusIs(items []map[string]any, portal, want string) error {
	for _, item := range items {
		if item["apiPortalId"] != portal {
			continue
		}
		if item["status"] != want {
			return fmt.Errorf("API Portal %q shows the API as %v, want %s", portal, item["status"], want)
		}
		return nil
	}
	return fmt.Errorf("API publications do not list API Portal %q", portal)
}

func selectionIDs(publication map[string]any, field string) []string {
	items, _ := publication[field].([]any)
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if selection, ok := item.(map[string]any); ok {
			if id, ok := selection["id"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func (s *Steps) assertPublicationPlans(ctx context.Context, subject, which string) error {
	keys := []string{keyPlanID}
	if which == "both synthetic subscription plans" {
		keys = append(keys, keySecondPlanID)
	}
	publication, err := s.readPublication(ctx, subject)
	if err != nil {
		return err
	}
	offered := selectionIDs(publication, "subscriptionPlans")
	for _, key := range keys {
		plan, err := cloudbase.RequiredValue(ctx, key)
		if err != nil {
			return err
		}
		if !slices.Contains(offered, plan) {
			return fmt.Errorf("publication offers %v, want it to include %q", offered, plan)
		}
	}
	return nil
}

func (s *Steps) assertPublicationDisplayName(ctx context.Context, subject, want string) error {
	want, err := cloudbase.ExpandCloudValue(ctx, want)
	if err != nil {
		return err
	}
	publication, err := s.readPublication(ctx, subject)
	if err != nil {
		return err
	}
	if got := publication["displayName"]; got != want {
		return fmt.Errorf("publication display name = %v, want %q", got, want)
	}
	return nil
}

func (s *Steps) assertPublicationDocument(ctx context.Context, presence string) error {
	doc, err := cloudbase.RequiredValue(ctx, keyDocumentID)
	if err != nil {
		return err
	}
	publication, err := s.readPublication(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	documents := selectionIDs(publication, "documents")
	if slices.Contains(documents, doc) != (presence == "include") {
		return fmt.Errorf("publication documents are %v, want them to %s %q", documents, presence, doc)
	}
	return nil
}

func (s *Steps) assertPublicationLandingPageAndThumbnail(ctx context.Context) error {
	publication, err := s.readPublication(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	if publication["hasLandingPage"] != true || publication["hasThumbnail"] != true {
		return fmt.Errorf("publication hasLandingPage = %v and hasThumbnail = %v, want both true",
			publication["hasLandingPage"], publication["hasThumbnail"])
	}
	return nil
}

func (s *Steps) readDraftPart(ctx context.Context, suffix string) (*httpx.Response, error) {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, target.Path(suffix), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading %s of %q: %s", suffix, target.API, resp.Describe())
	}
	return resp, nil
}

func (s *Steps) assertDraftExists(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, target.Path("/draft"), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected the draft of %q to exist, got %s", target.API, resp.Describe())
	}
	return nil
}

func (s *Steps) assertDraftDetails(ctx context.Context) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	plan, err := cloudbase.RequiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	resp, err := s.readDraftPart(ctx, "/draft")
	if err != nil {
		return err
	}
	var draft map[string]any
	if err := json.Unmarshal(resp.Body, &draft); err != nil {
		return fmt.Errorf("decoding the draft: %w", err)
	}
	if draft["displayName"] != cloudbase.PublicationTitle+api || draft["version"] != publicationVersion {
		return fmt.Errorf("draft details are %v %v, want %q %q", draft["displayName"], draft["version"], cloudbase.PublicationTitle+api, publicationVersion)
	}
	if plans := selectionIDs(draft, "subscriptionPlans"); !slices.Equal(plans, []string{plan}) {
		return fmt.Errorf("draft offers %v, want [%s]", plans, plan)
	}
	resp, err = s.readDraftPart(ctx, "/draft/definition")
	if err != nil {
		return err
	}
	var definition struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
	}
	if err := json.Unmarshal(resp.Body, &definition); err != nil || definition.Info.Title != api {
		return fmt.Errorf("draft definition title = %q, want %q (%s)", definition.Info.Title, api, resp.Describe())
	}
	return nil
}

func (s *Steps) assertDraftLandingPageSanitized(ctx context.Context) error {
	resp, err := s.readDraftPart(ctx, "/draft/landing-page")
	if err != nil {
		return err
	}
	text := resp.Text()
	if !strings.Contains(text, landingPageText) || strings.Contains(text, "<script>") {
		return fmt.Errorf("draft landing page should keep %q without raw HTML, got %q", landingPageText, text)
	}
	return nil
}
