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

// portalIconExtensions maps a thumbnail's content type to its file extension.
var portalIconExtensions = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
}

// portalContentTooLarge covers every size limit without saying which was hit.
func portalContentTooLarge() error {
	return apperror.APIPublicationValidationFailed.New("selected documents are too large to publish")
}

// buildContentArchive packs the documents and the thumbnail (nil for none) into the
// ZIP the portal reads: each document at content/docs/<Folder>/<FileName>, the thumbnail
// at content/web/api-icon.<ext>. It returns nil when there is nothing to send. A ZIP over
// maxArchiveBytes is rejected.
func buildContentArchive(docs []portalDocument, thumbnail *model.PublicationContent, maxArchiveBytes int64) (*PortalContent, error) {
	if len(docs) == 0 && thumbnail == nil {
		return nil, nil
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, doc := range docs {
		w, err := zw.Create(portalArchiveRoot + "/docs/" + doc.Folder + "/" + doc.FileName)
		if err != nil {
			return nil, fmt.Errorf("failed to add document %q to the content archive: %w", doc.Handle, err)
		}
		if _, err := w.Write(doc.Content); err != nil {
			return nil, fmt.Errorf("failed to write document %q to the content archive: %w", doc.Handle, err)
		}
	}
	if thumbnail != nil {
		ext, ok := portalIconExtensions[thumbnail.ContentType]
		if !ok {
			return nil, apperror.APIPublicationValidationFailed.New("thumbnail must be a PNG or JPEG image")
		}
		w, err := zw.Create(portalArchiveRoot + "/web/" + portalIconName + "." + ext)
		if err != nil {
			return nil, fmt.Errorf("failed to add the thumbnail to the content archive: %w", err)
		}
		if _, err := w.Write(thumbnail.Content); err != nil {
			return nil, fmt.Errorf("failed to write the thumbnail to the content archive: %w", err)
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
