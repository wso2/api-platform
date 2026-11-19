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

package cloudbase

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

const (
	KeyGraphQLAPIID   = "cloudGraphQLAPIID"
	PublicationIDGlue = "/"
	PublicationTitle  = "Synthetic "
)

// SyntheticPortalPrefix starts the handle of every portal a scenario creates for itself. Those
// portals belong to the runner, are deleted by its last scenario and are not the portal the
// publication scenarios publish to.
const SyntheticPortalPrefix = "syn-portal-"

// SyntheticPlanPrefix starts the id of every subscription plan a scenario creates.
const SyntheticPlanPrefix = "syn-plan-"

// PublicationTarget names the portal, the API type and the API one publication call is about.
type PublicationTarget struct {
	Portal, APIType, API string
}

// Path returns the publication route of the target, extended by suffix.
func (t PublicationTarget) Path(suffix string) string {
	return "/api-portals/" + url.PathEscape(t.Portal) + "/apis/" + t.APIType + "/" + url.PathEscape(t.API) + suffix
}

// ID returns the identifier of the publication, made of its portal and API.
func (t PublicationTarget) ID() string {
	return strings.Join([]string{t.Portal, t.APIType, t.API}, PublicationIDGlue)
}

// SplitPublicationID splits a publication identifier into its parts.
func SplitPublicationID(id string) ([]string, error) {
	parts := strings.Split(id, PublicationIDGlue)
	if len(parts) != 3 || slices.Contains(parts, "") {
		return nil, fmt.Errorf("malformed publication id %q", id)
	}
	return parts, nil
}

// MultipartFields encodes form fields as a multipart body and returns it with its content type.
func MultipartFields(fields [][2]string) ([]byte, string, error) {
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

// publicationStateConflict is the error code of an unpublish that found nothing to unpublish.
const publicationStateConflict = "PUBLICATION_STATE_CONFLICT"

// apiNotFound is the error code of a request for an API the platform no longer has.
const apiNotFound = "API_NOT_FOUND"

// UnpublishRequest builds the cleanup request for a publication id. An unpublish that finds
// nothing published, or no portal, leaves nothing behind. A missing API does not: the platform
// answers before it asks the portal to remove the listing, so the listing may remain. Any other
// conflict, such as the portal still holding subscriptions to the API, means the listing is still
// there.
func UnpublishRequest(id string) (string, string, func(*httpx.Response) bool, error) {
	parts, err := SplitPublicationID(id)
	if err != nil {
		return "", "", nil, err
	}
	target := PublicationTarget{Portal: parts[0], APIType: parts[1], API: parts[2]}
	return http.MethodPost, target.Path("/unpublish"), unpublishSettled, nil
}

func unpublishSettled(resp *httpx.Response) bool {
	if resp.Succeeded() {
		return true
	}
	code, err := StringField(resp, "code")
	switch resp.StatusCode {
	case http.StatusNotFound:
		return err != nil || code != apiNotFound
	case http.StatusConflict:
		return err == nil && code == publicationStateConflict
	}
	return false
}

// DoAuthenticatedContent sends a body that is not JSON, such as a form or Markdown, and
// publishes the answer.
func (s *Steps) DoAuthenticatedContent(ctx context.Context, method, path, contentType string, body []byte) (*httpx.Response, error) {
	token, err := BearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return s.Send(ctx, method, path, map[string]string{
		"Authorization": "Bearer " + token, "Content-Type": contentType,
	}, body)
}

// Send makes one request to the cloud API with the given headers and publishes the answer.
func (s *Steps) Send(ctx context.Context, method, path string, headers map[string]string, body []byte) (*httpx.Response, error) {
	httpx.ClearPublished(ctx)
	base, err := s.Topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(ctx, httpx.Request{
		Method: method, URL: CloudAPIURL(base, path), Headers: headers, Body: body,
	}, 0, 0)
	if err != nil {
		return nil, err
	}
	if s.Funnel != nil {
		if err := s.Funnel.Publish(ctx, resp); err != nil {
			return nil, err
		}
	}
	return resp, nil
}
