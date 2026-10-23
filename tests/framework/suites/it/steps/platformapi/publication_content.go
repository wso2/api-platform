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

package platformapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

var platformDocumentKind = cleanup.Kind{Name: "platform-api-document", Order: 47}

// RegisterPublicationContent binds the steps that attach documents, a landing page and a
// thumbnail to a publication.
func RegisterPublicationContent(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I create a document "([^"]*)" of type "([^"]*)" with content "([^"]*)" on the "([^"]*)" "([^"]*)" via the control plane$`,
		s.createDocument)
	sc.Step(`^I create a document "([^"]*)" of type "([^"]*)" with (\d+) bytes of incompressible content on the "([^"]*)" "([^"]*)" via the control plane$`,
		s.createIncompressibleDocument)
	sc.Step(`^I save the landing page "([^"]*)" on the draft of the "([^"]*)" "([^"]*)" on the API Portal "([^"]*)"$`,
		s.saveDraftLandingPage)
	sc.Step(`^I save a PNG thumbnail on the draft of the "([^"]*)" "([^"]*)" on the API Portal "([^"]*)"$`,
		s.saveDraftThumbnail)
}

// createDocument creates a document on an API and removes it at cleanup.
func (s *Steps) createDocument(ctx context.Context, handle, docType, content, kind, apiHandle string) error {
	resolved, err := stepscommon.Expand(ctx, content)
	if err != nil {
		return err
	}
	return s.postDocument(ctx, handle, docType, resolved, kind, apiHandle)
}

// createIncompressibleDocument creates a document of size random bytes written as hexadecimal.
// The text is twice that long, and zipping brings it back to about size bytes, so size is what
// the document adds to a publication's content archive.
func (s *Steps) createIncompressibleDocument(
	ctx context.Context, handle, docType string, size int, kind, apiHandle string,
) error {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generating document content: %w", err)
	}
	return s.postDocument(ctx, handle, docType, hex.EncodeToString(raw), kind, apiHandle)
}

func (s *Steps) postDocument(ctx context.Context, handle, docType, content, kind, apiHandle string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	apiType, err := publicationType(kind)
	if err != nil {
		return err
	}
	api, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for _, field := range [][2]string{
		{"id", resolvedHandle}, {"displayName", resolvedHandle}, {"type", docType}, {"inlineContent", content},
	} {
		if err := mw.WriteField(field[0], field[1]); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	path := documentsPath(apiType, api)
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiBase + path,
		Headers: map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": mw.FormDataContentType()},
		Body:    body.Bytes(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating document %q: %w", resolvedHandle, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("creating document %q failed: %s", resolvedHandle, resp.Describe())
	}
	return s.registerTracked(ctx, platformDocumentKind, documentID(apiType, api, resolvedHandle),
		func(ctx context.Context) error {
			return s.deleteResource(ctx, base, bearer, path+"/"+url.PathEscape(resolvedHandle))
		},
		func(reg *cleanup.Registry) error { return reg.RegisterDeleter(platformDocumentKind, s.deleteDocument) })
}

// documentsPath returns the control-plane path of one API's documents.
func documentsPath(apiType, apiHandle string) string {
	return "/apis/" + apiType + "/" + url.PathEscape(apiHandle) + "/docs"
}

// documentID builds the cleanup resource id of a document from its API and handle, so the
// deleter needs nothing from the registration that happened to come last.
func documentID(apiType, apiHandle, docHandle string) string {
	return strings.Join([]string{apiType, apiHandle, docHandle}, publicationIDSeparator)
}

// splitDocumentID undoes documentID.
func splitDocumentID(id string) (apiType, apiHandle, docHandle string, err error) {
	parts := strings.Split(id, publicationIDSeparator)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("malformed document id %q", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// deleteDocument removes the document a cleanup resource names, as the administrator.
func (s *Steps) deleteDocument(ctx context.Context, res cleanup.Resource) error {
	apiType, api, doc, err := splitDocumentID(res.ID)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.deleteResource(ctx, base, bearer, documentsPath(apiType, api)+"/"+url.PathEscape(doc))
}

// saveDraftLandingPage replaces the draft's landing page with Markdown.
func (s *Steps) saveDraftLandingPage(ctx context.Context, markdown, kind, apiHandle, portalHandle string) error {
	content, err := stepscommon.Expand(ctx, markdown)
	if err != nil {
		return err
	}
	return s.putDraftContent(ctx, "/draft/landing-page", "text/markdown", []byte(content), kind, apiHandle, portalHandle)
}

// saveDraftThumbnail uploads a small PNG as the draft's thumbnail.
func (s *Steps) saveDraftThumbnail(ctx context.Context, kind, apiHandle, portalHandle string) error {
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		return err
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="thumbnail.png"`)
	header.Set("Content-Type", "image/png")
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := part.Write(img.Bytes()); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	return s.putDraftContent(ctx, "/draft/thumbnail", mw.FormDataContentType(), body.Bytes(), kind, apiHandle, portalHandle)
}

// putDraftContent PUTs one piece of draft content and requires it to be accepted.
func (s *Steps) putDraftContent(
	ctx context.Context, suffix, contentType string, body []byte, kind, apiHandle, portalHandle string,
) error {
	portal, api, apiType, err := publicationTarget(ctx, kind, apiHandle, portalHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPut, URL: base + apiBase + publicationPath(portal, apiType, api) + suffix,
		Headers: map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": contentType},
		Body:    body,
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("saving draft content %s: %w", suffix, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("saving draft content %s failed: %s", suffix, resp.Describe())
	}
	return nil
}
