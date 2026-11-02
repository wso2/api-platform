/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The API Portal's upload limits; content over any of them is refused there.
const (
	portalContentMaxFileBytes      int64 = 10 << 20 // one file
	portalContentMaxExtractedBytes int64 = 50 << 20 // all files together, uncompressed
	portalContentMaxEntries              = 500      // the portal refuses more than 500 entries
)

// defaultPublicationArchiveMaxBytes applies when the configured ZIP ceiling is <= 0.
const defaultPublicationArchiveMaxBytes int64 = 10 << 20

// portalArchiveRoot wraps everything in one folder: the portal treats a ZIP whose only
// top-level entry is a folder as a wrapper, and would refuse one holding only docs/ or web/.
const portalArchiveRoot = "content"

// portalIconName is the thumbnail's base name; the portal shows "api-icon" as the API's icon.
const portalIconName = "api-icon"

// portalLandingPageName must match exactly: the portal reads this file, not the hyphenated
// spelling its docs use, and renders it in place of its default Endpoints and Resources section.
const portalLandingPageName = "apiContent.md"

// portalIconExtensions maps a thumbnail's content type to its file extension.
var portalIconExtensions = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
}

// portalContentTooLarge covers every size limit without saying which was hit.
func portalContentTooLarge() error {
	return apperror.APIPublicationValidationFailed.New("the content selected for publishing is too large")
}

// archiveFile is one file of the content ZIP.
type archiveFile struct {
	name    string
	content []byte
}

// buildContentArchive packs the documents, thumbnail and landing page (nil for none) into the
// ZIP layout the portal reads: content/docs/<Folder>/<FileName>, content/web/api-icon.<ext>
// and content/web/apiContent.md. A blank landing page is left out, since the portal would
// show it as an empty page. It returns nil when there is nothing to send.
func buildContentArchive(docs []portalDocument, thumbnail, landingPage *model.PublicationContent, maxArchiveBytes int64) (*PortalContent, error) {
	files := make([]archiveFile, 0, len(docs)+2)
	for _, doc := range docs {
		files = append(files, archiveFile{portalArchiveRoot + "/docs/" + doc.Folder + "/" + doc.FileName, doc.Content})
	}
	if thumbnail != nil {
		ext, ok := portalIconExtensions[thumbnail.ContentType]
		if !ok {
			return nil, apperror.APIPublicationValidationFailed.New("thumbnail must be a PNG or JPEG image")
		}
		// The stored type picks the extension, so make sure the bytes agree with it.
		if http.DetectContentType(thumbnail.Content) != thumbnail.ContentType {
			return nil, apperror.APIPublicationValidationFailed.New("thumbnail content does not match its declared type")
		}
		files = append(files, archiveFile{portalArchiveRoot + "/web/" + portalIconName + "." + ext, thumbnail.Content})
	}
	if landingPage != nil && len(bytes.TrimSpace(landingPage.Content)) > 0 {
		files = append(files, archiveFile{portalArchiveRoot + "/web/" + portalLandingPageName, landingPage.Content})
	}
	if len(files) == 0 {
		return nil, nil
	}
	if err := checkPortalContentLimits(files); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, fmt.Errorf("failed to add %q to the content archive: %w", f.name, err)
		}
		if _, err := w.Write(f.content); err != nil {
			return nil, fmt.Errorf("failed to write %q to the content archive: %w", f.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize the content archive: %w", err)
	}

	if int64(buf.Len()) > maxArchiveBytes {
		return nil, portalContentTooLarge()
	}
	return &PortalContent{Archive: buf.Bytes()}, nil
}

// checkPortalContentLimits rejects a file set the portal would refuse.
func checkPortalContentLimits(files []archiveFile) error {
	if len(files) > portalContentMaxEntries {
		return portalContentTooLarge()
	}
	var total int64
	for _, f := range files {
		size := int64(len(f.content))
		total += size
		if size > portalContentMaxFileBytes || total > portalContentMaxExtractedBytes {
			return portalContentTooLarge()
		}
	}
	return nil
}
