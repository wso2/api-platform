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
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"

	"github.com/cucumber/godog"
)

const (
	// thumbnailMaxBytes is the largest thumbnail the control plane accepts by default.
	thumbnailMaxBytes = 2 << 20

	// notAnImage is the content of a thumbnail that no image decoder reads.
	notAnImage = "not an image"
)

// pngSignature starts every PNG.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// thumbnailDefects build the content of a thumbnail the control plane must refuse, keyed by how a
// scenario describes it.
var thumbnailDefects = map[string]func() []byte{
	"text declared as PNG": func() []byte { return []byte(notAnImage) },
	"larger than 2 MiB":    oversizedPNG,
}

// oversizedPNG returns a PNG that decodes and is larger than thumbnailMaxBytes, so a refusal can
// only come from the size limit. Random pixels do not compress, so an image of this size always
// passes the limit; the seed is fixed, so every run sends the same bytes.
func oversizedPNG() []byte {
	const side = 1024
	random := rand.New(rand.NewPCG(1, 2))
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = byte(random.UintN(256))
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		panic(fmt.Sprintf("encoding the oversized thumbnail: %v", err))
	}
	return encoded.Bytes()
}

// registerPublicationDraftSteps binds the steps that probe what a publication draft accepts and
// stores.
func (s *Steps) registerPublicationDraftSteps(sc stepRegistrar) {
	sc.Step(`^I try to save a publication draft of the synthetic REST API with a (displayName|version|description) of (\d+) characters$`, s.trySaveDraftWithLength)
	sc.Step(`^the publication draft of the synthetic REST API should report:$`, s.assertDraftReports)
	sc.Step(`^I save a definition of type "([^"]*)" on the publication draft of the synthetic REST API$`, s.saveDefinitionOfType)
	sc.Step(`^I save a thumbnail that is (text declared as PNG|larger than 2 MiB) on the publication draft of the synthetic REST API$`, s.saveDefectiveThumbnail)
	sc.Step(`^the publication draft of the synthetic REST API should return the definition and the thumbnail it was saved with$`, s.assertDraftContentRoundTrips)
}

// trySaveDraftWithLength saves the synthetic REST API's draft with one text field of the given
// length and publishes the answer without requiring success.
func (s *Steps) trySaveDraftWithLength(ctx context.Context, field string, length int) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	plan, err := requiredValue(ctx, keyPlanID)
	if err != nil {
		return err
	}
	body, err := draftBody(target.api, plan, map[string]string{field: strings.Repeat("x", length)})
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, target.path("/draft"), body)
	return err
}

// assertDraftReports checks fields of the synthetic REST API's draft against a table of dotted
// paths and expected values.
func (s *Steps) assertDraftReports(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	resp, err := s.readDraftPart(ctx, "/draft")
	if err != nil {
		return err
	}
	var draft any
	if err := json.Unmarshal(resp.Body, &draft); err != nil {
		return fmt.Errorf("decoding the draft: %w", err)
	}
	var differing []string
	for path, raw := range values {
		want, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", path, err)
		}
		got, present := documentPath(draft, path)
		if !present || !sameValue(got, parseConfigCell(want)) {
			differing = append(differing, fmt.Sprintf("%s is %v, want %s", path, got, want))
		}
	}
	sort.Strings(differing)
	if len(differing) > 0 {
		return fmt.Errorf("the draft does not keep what was saved: %s", strings.Join(differing, "; "))
	}
	return nil
}

// saveDefinitionOfType saves a definition under the given media type and publishes the answer
// without requiring success.
func (s *Steps) saveDefinitionOfType(ctx context.Context, mediaType string) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	body, _, err := definitionFor("valid", target.apiType, target.api)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPut, target.path("/draft/definition"), mediaType, body)
	return err
}

// saveDefectiveThumbnail saves a thumbnail the control plane must refuse, declared as a PNG, and
// publishes the answer without requiring success.
func (s *Steps) saveDefectiveThumbnail(ctx context.Context, defect string) error {
	content, ok := thumbnailDefects[defect]
	if !ok {
		return fmt.Errorf("unknown thumbnail defect %q", defect)
	}
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	body, contentType, err := thumbnailFormOf(content())
	if err != nil {
		return err
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPut, target.path("/draft/thumbnail"), contentType, body)
	return err
}

// assertDraftContentRoundTrips checks the draft returns the definition and the thumbnail that were
// saved on it.
func (s *Steps) assertDraftContentRoundTrips(ctx context.Context) error {
	target, err := targetOf(ctx, "synthetic REST API")
	if err != nil {
		return err
	}
	wantDefinition, _, err := definitionFor("valid", target.apiType, target.api)
	if err != nil {
		return err
	}
	resp, err := s.readDraftPart(ctx, "/draft/definition")
	if err != nil {
		return err
	}
	if !sameJSON(resp.Body, wantDefinition) {
		return fmt.Errorf("the draft returns a definition other than the one saved: %s", resp.Describe())
	}
	wantThumbnail, err := thumbnailPNG()
	if err != nil {
		return err
	}
	if resp, err = s.readDraftPart(ctx, "/draft/thumbnail"); err != nil {
		return err
	}
	if !bytes.Equal(resp.Body, wantThumbnail) {
		return fmt.Errorf("the draft returns a thumbnail of %d bytes, other than the %d bytes saved", len(resp.Body), len(wantThumbnail))
	}
	if resp, err = s.readDraftPart(ctx, "/draft"); err != nil {
		return err
	}
	var draft map[string]any
	if err := json.Unmarshal(resp.Body, &draft); err != nil {
		return fmt.Errorf("decoding the draft: %w", err)
	}
	if draft["hasThumbnail"] != true {
		return fmt.Errorf("the draft reports hasThumbnail %v, want true", draft["hasThumbnail"])
	}
	return nil
}
