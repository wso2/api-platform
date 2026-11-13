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

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	keyPlanID          = "cloudSubscriptionPlanID"
	keySecondPlanID    = "cloudSecondSubscriptionPlanID"
	keyGraphQLAPIID    = "cloudGraphQLAPIID"
	keyDocumentID      = "cloudDocumentID"
	publicationIDGlue  = "/"
	publicationTitle   = "Synthetic "
	landingPageText    = "Use this API."
	publicationVersion = "1.0.0"
)

var (
	// A publication is removed before the API it lists, and a document before its API.
	cloudPublicationKind = cleanup.Kind{Name: "cloud-api-publication", Order: 45}
	cloudDocumentKind    = cleanup.Kind{Name: "cloud-api-document", Order: 47}
)

// publicationSubject is an API a publication step acts on.
type publicationSubject struct {
	apiType string
	key     string
}

var publicationSubjects = map[string]publicationSubject{
	"synthetic REST API":    {apiType: "rest-api", key: keyAPIID},
	"synthetic GraphQL API": {apiType: "graphql-api", key: keyGraphQLAPIID},
}

const subjectPattern = `(synthetic REST API|synthetic GraphQL API)`

// draftListKeys are the draft fields a values table sets as JSON arrays.
var draftListKeys = map[string]bool{"subscriptionPlanIds": true, "docIds": true, "tags": true}

// draftTextKeys are the draft fields a values table sets as text.
var draftTextKeys = map[string]bool{"displayName": true, "version": true, "description": true, "agentVisibility": true}

// draftObjectKeys are the draft fields a values table sets as JSON objects.
var draftObjectKeys = map[string]bool{"endpoints": true, "owners": true}

func (s *Steps) publicationStepDefinitions() []stepDefinition {
	return []stepDefinition{
		{`^I find an active managed API Portal$`, s.findActiveAPIPortal},
		{`^I create a synthetic subscription plan$`, s.createPlan},
		{`^I create another synthetic subscription plan$`, s.createSecondPlan},
		{`^I deactivate the synthetic subscription plan$`, s.deactivatePlan},
		{`^I try to delete the synthetic subscription plan$`, s.tryDeletePlan},
		{`^I create a synthetic GraphQL API in the default project$`, s.createGraphQLAPI},
		{`^I create a synthetic document for the synthetic REST API$`, s.createDocument},
		{`^I save a publication draft of the ` + subjectPattern + `$`, s.saveDraft},
		{`^I save a publication draft of the ` + subjectPattern + ` with:$`, s.saveDraftWith},
		{`^I try to save a publication draft of the ` + subjectPattern + `$`, s.trySaveDraft},
		{`^I try to save a publication draft naming an unknown (subscription plan|API|API Portal)$`, s.trySaveDraftNamingUnknown},
		{`^I save an? (valid|invalid|OpenAPI) definition on the publication draft of the ` + subjectPattern + `$`, s.saveDefinition},
		{`^I save a landing page on the publication draft of the ` + subjectPattern + `$`, s.saveLandingPage},
		{`^I save a PNG thumbnail on the publication draft of the ` + subjectPattern + `$`, s.saveThumbnail},
		{`^I (publish|deprecate|unpublish) the ` + subjectPattern + ` on the managed API Portal$`, s.publicationAction},
		{`^I publish the synthetic GraphQL API through the REST API route$`, s.publishGraphQLThroughRESTRoute},
		{`^I send a "([^"]*)" request (without a token|with a forged token) to the publication path "([^"]*)"$`, s.sendWithoutValidToken},
		{`^the ` + subjectPattern + ` should be (PUBLISHED|DEPRECATED) on the managed API Portal$`, s.assertPublicationStatus},
		{`^the ` + subjectPattern + ` should not be published on the managed API Portal$`, s.assertNotPublished},
		{`^the API publications of the ` + subjectPattern + ` should show the managed API Portal as (PUBLISHED|DEPRECATED|NOT_PUBLISHED)$`, s.assertPublicationSummary},
		{`^the publication of the ` + subjectPattern + ` should offer (the synthetic subscription plan|both synthetic subscription plans)$`, s.assertPublicationPlans},
		{`^the publication of the ` + subjectPattern + ` should have the display name "([^"]*)"$`, s.assertPublicationDisplayName},
		{`^the publication of the synthetic REST API should (include|not include) the synthetic document$`, s.assertPublicationDocument},
		{`^the publication of the synthetic REST API should have a landing page and a thumbnail$`, s.assertPublicationLandingPageAndThumbnail},
		{`^the publication draft of the ` + subjectPattern + ` should still exist$`, s.assertDraftExists},
		{`^the publication draft of the synthetic REST API should keep its details and definition$`, s.assertDraftDetails},
		{`^the landing page of the publication draft of the synthetic REST API should have no raw HTML$`, s.assertDraftLandingPageSanitized},
	}
}

// findActiveAPIPortal picks a managed API Portal whose provisioning has completed. Scenarios
// publish to an existing portal: provisioning one starts a portal runtime for the organization.
func (s *Steps) findActiveAPIPortal(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-api-portals", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
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

func activeAPIPortalID(items []map[string]any) (string, error) {
	statuses := make([]string, 0, len(items))
	for _, item := range items {
		id, _ := item["id"].(string)
		status, _ := item["status"].(string)
		if strings.TrimSpace(id) != "" && status == "active" {
			return id, nil
		}
		statuses = append(statuses, fmt.Sprintf("%s: %s", id, status))
	}
	if len(items) == 0 {
		return "", fmt.Errorf("the organization has no managed API Portal")
	}
	return "", fmt.Errorf("no active managed API Portal among %d listed: %s", len(items), strings.Join(statuses, "; "))
}

func (s *Steps) createPlan(ctx context.Context) error {
	return s.createPlanAs(ctx, keyPlanID)
}

func (s *Steps) createSecondPlan(ctx context.Context) error {
	return s.createPlanAs(ctx, keySecondPlanID)
}

func planBody(id, status string) map[string]any {
	return map[string]any{
		"id": id, "displayName": publicationTitle + id, "status": status,
		"limits": []map[string]any{{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": 10}},
	}
}

func (s *Steps) createPlanAs(ctx context.Context, key string) error {
	if err := generateResourceAndStore(ctx, "syn-plan", key); err != nil {
		return err
	}
	id, err := requiredValue(ctx, key)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindSubscriptionPlan, ID: id, Actor: resourceActor, Description: "subscription plan " + id,
	}); err != nil {
		return fmt.Errorf("registering subscription plan %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/subscription-plans", planBody(id, "ACTIVE"))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		if refusedCreate(resp) {
			deregister(ctx, cleanup.KindSubscriptionPlan, id)
		}
		return responseError(resp)
	}
	return nil
}

func (s *Steps) deactivatePlan(ctx context.Context) error {
	id, err := requiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPut, "/subscription-plans/"+url.PathEscape(id), planBody(id, "INACTIVE"))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
	}
	return nil
}

func (s *Steps) tryDeletePlan(ctx context.Context) error {
	id, err := requiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/subscription-plans/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		deregister(ctx, cleanup.KindSubscriptionPlan, id)
	}
	return nil
}

// createGraphQLAPI creates a GraphQL API with an inline schema. The control plane accepts a
// GraphQL API only as a multipart form whose metadata part carries the definition.
func (s *Steps) createGraphQLAPI(ctx context.Context) error {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-graphql", keyGraphQLAPIID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]any{
		"id": id, "displayName": publicationTitle + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":     map[string]any{"main": map[string]string{"url": "https://graphql.example.com/graphql"}},
		"schemaSource": "inline", "sdl": graphQLSchema,
	})
	if err != nil {
		return err
	}
	body, contentType, err := multipartFields([][2]string{{"metadata", string(metadata)}})
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGraphQLAPI, ID: id, Actor: resourceActor, Description: "GraphQL API " + id,
	}); err != nil {
		return fmt.Errorf("registering GraphQL API %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPost, "/graphql-apis", contentType, body)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		if refusedCreate(resp) {
			deregister(ctx, cleanup.KindGraphQLAPI, id)
		}
		return responseError(resp)
	}
	return nil
}

const graphQLSchema = "type Query { hello: String }"

func documentsPath(apiType, apiID string) string {
	return "/apis/" + apiType + "/" + url.PathEscape(apiID) + "/docs"
}

func (s *Steps) createDocument(ctx context.Context) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-doc", keyDocumentID); err != nil {
		return err
	}
	docID, err := requiredValue(ctx, keyDocumentID)
	if err != nil {
		return err
	}
	body, contentType, err := multipartFields([][2]string{
		{"id", docID}, {"displayName", publicationTitle + docID}, {"type", "HowTo"},
		{"inlineContent", "Step one: call the API."},
	})
	if err != nil {
		return err
	}
	cleanupID := strings.Join([]string{"rest-api", apiID, docID}, publicationIDGlue)
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cloudDocumentKind, ID: cleanupID, Actor: resourceActor, Description: "document " + docID,
	}); err != nil {
		return fmt.Errorf("registering document %q for cleanup: %w", docID, err)
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPost, documentsPath("rest-api", apiID), contentType, body)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		if refusedCreate(resp) {
			deregister(ctx, cloudDocumentKind, cleanupID)
		}
		return responseError(resp)
	}
	return nil
}

// deleteDocumentRequest builds the cleanup request for a document id made of the API type, the
// API and the document.
func deleteDocumentRequest(id string) (string, string, func(*httpx.Response) bool, error) {
	parts, err := splitPublicationID(id)
	if err != nil {
		return "", "", nil, err
	}
	settled := func(resp *httpx.Response) bool {
		return resp.Succeeded() || resp.StatusCode == http.StatusNotFound
	}
	return http.MethodDelete, documentsPath(parts[0], parts[1]) + "/" + url.PathEscape(parts[2]), settled, nil
}

// publicationTarget names the portal, the API type and the API one publication call is about.
type publicationTarget struct {
	portal, apiType, api string
}

func (t publicationTarget) path(suffix string) string {
	return "/api-portals/" + url.PathEscape(t.portal) + "/apis/" + t.apiType + "/" + url.PathEscape(t.api) + suffix
}

func (t publicationTarget) id() string {
	return strings.Join([]string{t.portal, t.apiType, t.api}, publicationIDGlue)
}

func splitPublicationID(id string) ([]string, error) {
	parts := strings.Split(id, publicationIDGlue)
	if len(parts) != 3 || slices.Contains(parts, "") {
		return nil, fmt.Errorf("malformed publication id %q", id)
	}
	return parts, nil
}

func targetOf(ctx context.Context, subject string) (publicationTarget, error) {
	ref, ok := publicationSubjects[subject]
	if !ok {
		return publicationTarget{}, fmt.Errorf("unsupported publication subject %q", subject)
	}
	portal, err := requiredValue(ctx, keyAPIPortalID)
	if err != nil {
		return publicationTarget{}, err
	}
	api, err := requiredValue(ctx, ref.key)
	if err != nil {
		return publicationTarget{}, err
	}
	return publicationTarget{portal: portal, apiType: ref.apiType, api: api}, nil
}

// draftBody is the draft a scenario saves: the API's title and version, the synthetic plan, and
// whatever the values table replaces.
func draftBody(api, plan string, overrides map[string]string) (map[string]any, error) {
	body := map[string]any{
		"displayName": publicationTitle + api, "version": publicationVersion,
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
		return responseError(resp)
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
	plan, err := requiredValue(ctx, keyPlanID)
	if err != nil {
		return nil, err
	}
	overrides, err := tableValues(table)
	if err != nil {
		return nil, err
	}
	for key, value := range overrides {
		if overrides[key], err = expandCloudValue(ctx, value); err != nil {
			return nil, err
		}
	}
	body, err := draftBody(target.api, plan, overrides)
	if err != nil {
		return nil, err
	}
	return s.doAuthenticated(ctx, http.MethodPut, target.path("/draft"), body)
}

// trySaveDraftNamingUnknown saves the synthetic REST API's draft with one of its references
// pointing at something the organization does not have.
func (s *Steps) trySaveDraftNamingUnknown(ctx context.Context, unknown string) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	plan, err := requiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	switch unknown {
	case "subscription plan":
		plan += "-missing"
	case "API":
		target.api += "-missing"
	case "API Portal":
		target.portal += "-missing"
	default:
		return fmt.Errorf("unsupported unknown reference %q", unknown)
	}
	body, err := draftBody(target.api, plan, nil)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, target.path("/draft"), body)
	return err
}

// definitionFor returns the definition a step saves and its content type. A valid definition
// matches the API's type; the invalid and OpenAPI ones are refused at save or at publish.
func definitionFor(kind, apiType, title string) ([]byte, string, error) {
	switch {
	case kind == "valid" && apiType == "graphql-api":
		return []byte(graphQLSchema), "application/graphql", nil
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

// saveDefinition requires a valid definition to be stored. Any other is sent and its answer is
// left unchecked: refused now or stored and refused at publish, the publish fails the same way.
func (s *Steps) saveDefinition(ctx context.Context, kind, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	body, contentType, err := definitionFor(kind, target.apiType, target.api)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPut, target.path("/draft/definition"), contentType, body)
	if err != nil {
		return err
	}
	if kind == "valid" && resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return nil
}

// landingPage carries raw HTML, which the control plane strips before storing it.
const landingPage = "# Welcome\n\n<script>alert(1)</script>\n\n" + landingPageText + "\n"

func (s *Steps) saveLandingPage(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPut, target.path("/draft/landing-page"), "text/markdown", []byte(landingPage))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
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
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPut, target.path("/draft/thumbnail"), contentType, body)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
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

func multipartFields(fields [][2]string) ([]byte, string, error) {
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	for _, field := range fields {
		if err := form.WriteField(field[0], field[1]); err != nil {
			return nil, "", err
		}
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), form.FormDataContentType(), nil
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
			Kind: cloudPublicationKind, ID: target.id(), Actor: resourceActor,
			Description: target.api + " on API Portal " + target.portal,
		}); err != nil {
			return fmt.Errorf("registering the publication of %q for cleanup: %w", target.api, err)
		}
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, target.path("/"+action), nil)
	if err != nil {
		return err
	}
	if action == "unpublish" && resp.Succeeded() {
		deregister(ctx, cloudPublicationKind, target.id())
	}
	return nil
}

func (s *Steps) publishGraphQLThroughRESTRoute(ctx context.Context) error {
	target, err := targetOf(ctx, "synthetic GraphQL API")
	if err != nil {
		return err
	}
	target.apiType = "rest-api"
	_, err = s.doAuthenticated(ctx, http.MethodPost, target.path("/publish"), nil)
	return err
}

// publicationStateConflict is the error code of an unpublish that found nothing to unpublish.
const publicationStateConflict = "PUBLICATION_STATE_CONFLICT"

// unpublishRequest builds the cleanup request for a publication id. An unpublish that finds
// nothing published, or no API, leaves nothing behind; any other conflict, such as the portal
// still holding subscriptions to the API, means the listing is still there.
func unpublishRequest(id string) (string, string, func(*httpx.Response) bool, error) {
	parts, err := splitPublicationID(id)
	if err != nil {
		return "", "", nil, err
	}
	target := publicationTarget{portal: parts[0], apiType: parts[1], api: parts[2]}
	return http.MethodPost, target.path("/unpublish"), unpublishSettled, nil
}

func unpublishSettled(resp *httpx.Response) bool {
	if resp.Succeeded() || resp.StatusCode == http.StatusNotFound {
		return true
	}
	if resp.StatusCode != http.StatusConflict {
		return false
	}
	code, err := stringField(resp, "code")
	return err == nil && code == publicationStateConflict
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
	path, err := expandCloudValue(ctx, path)
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
	_, err = s.send(ctx, strings.ToUpper(method), path, headers, body)
	return err
}

func (s *Steps) readPublication(ctx context.Context, subject string) (map[string]any, error) {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, target.path("/publication"), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the publication of %q: %s", target.api, resp.Describe())
	}
	var publication map[string]any
	if err := json.Unmarshal(resp.Body, &publication); err != nil {
		return nil, fmt.Errorf("decoding the publication of %q: %w", target.api, err)
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
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, target.path("/publication"), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("expected %q to have no publication, got %s", target.api, resp.Describe())
	}
	return nil
}

// assertPublicationSummary reads the per-portal summary the console shows for an API.
func (s *Steps) assertPublicationSummary(ctx context.Context, subject, want string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	query := url.Values{"apiType": {target.apiType}, "apiId": {target.api}, "limit": {"100"}}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/api-publications?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding API publications: %w", err)
	}
	return summaryStatusIs(items, target.portal, want)
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
		plan, err := requiredValue(ctx, key)
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
	want, err := expandCloudValue(ctx, want)
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
	doc, err := requiredValue(ctx, keyDocumentID)
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
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, target.path(suffix), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading %s of %q: %s", suffix, target.api, resp.Describe())
	}
	return resp, nil
}

func (s *Steps) assertDraftExists(ctx context.Context, subject string) error {
	target, err := targetOf(ctx, subject)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, target.path("/draft"), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected the draft of %q to exist, got %s", target.api, resp.Describe())
	}
	return nil
}

func (s *Steps) assertDraftDetails(ctx context.Context) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	plan, err := requiredValue(ctx, keyPlanID)
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
	if draft["displayName"] != publicationTitle+api || draft["version"] != publicationVersion {
		return fmt.Errorf("draft details are %v %v, want %q %q", draft["displayName"], draft["version"], publicationTitle+api, publicationVersion)
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

// doAuthenticatedContent sends a body that is not JSON, such as a form or Markdown, and
// publishes the answer.
func (s *Steps) doAuthenticatedContent(ctx context.Context, method, path, contentType string, body []byte) (*httpx.Response, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return s.send(ctx, method, path, map[string]string{
		"Authorization": "Bearer " + token, "Content-Type": contentType,
	}, body)
}

// send makes one request to the cloud API with the given headers and publishes the answer.
func (s *Steps) send(ctx context.Context, method, path string, headers map[string]string, body []byte) (*httpx.Response, error) {
	httpx.ClearPublished(ctx)
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: method, URL: cloudAPIURL(base, path), Headers: headers, Body: body,
	}, 0, 0)
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
