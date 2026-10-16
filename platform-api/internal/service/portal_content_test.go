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
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func readArchive(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("not a ZIP: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(data)
	}
	return files
}

func TestBuildContentArchive(t *testing.T) {
	t.Run("lays each document out under content/docs/<Type>/<file>", func(t *testing.T) {
		content, err := buildContentArchive([]portalDocument{
			{Handle: "guide", Folder: "HowTo", FileName: "guide.md", Content: []byte("# guide")},
			{Handle: "faq", Folder: "FAQ", FileName: "faq.md", Content: []byte("# faq")},
		}, nil, nil, defaultPublicationArchiveMaxBytes)
		if err != nil || content == nil {
			t.Fatalf("got %v, %v", content, err)
		}
		files := readArchive(t, content.Archive)
		if len(files) != 2 || files["content/docs/HowTo/guide.md"] != "# guide" || files["content/docs/FAQ/faq.md"] != "# faq" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("adds the thumbnail as content/web/api-icon with an extension from its type", func(t *testing.T) {
		for contentType, name := range map[string]string{"image/png": "content/web/api-icon.png", "image/jpeg": "content/web/api-icon.jpg"} {
			content, err := buildContentArchive(nil, &model.PublicationContent{ContentType: contentType, FileName: "logo.final.bin", Content: []byte("img")}, nil, defaultPublicationArchiveMaxBytes)
			if err != nil || content == nil {
				t.Fatalf("%s: got %v, %v", contentType, content, err)
			}
			if files := readArchive(t, content.Archive); len(files) != 1 || files[name] != "img" {
				t.Fatalf("%s: unexpected archive layout: %v", contentType, files)
			}
		}
	})

	t.Run("packs documents and the thumbnail together", func(t *testing.T) {
		content, err := buildContentArchive(
			[]portalDocument{{Handle: "guide", Folder: "HowTo", FileName: "guide.md", Content: []byte("# guide")}},
			&model.PublicationContent{ContentType: "image/png", Content: []byte("img")}, nil, defaultPublicationArchiveMaxBytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if files := readArchive(t, content.Archive); len(files) != 2 || files["content/docs/HowTo/guide.md"] == "" || files["content/web/api-icon.png"] == "" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("adds the landing page as content/web/apiContent.md", func(t *testing.T) {
		content, err := buildContentArchive(nil, nil, &model.PublicationContent{ContentType: "text/markdown", Content: []byte("# Loans")}, defaultPublicationArchiveMaxBytes)
		if err != nil || content == nil {
			t.Fatalf("got %v, %v", content, err)
		}
		if files := readArchive(t, content.Archive); len(files) != 1 || files["content/web/apiContent.md"] != "# Loans" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("packs the landing page beside the thumbnail under web/", func(t *testing.T) {
		content, err := buildContentArchive(nil,
			&model.PublicationContent{ContentType: "image/png", Content: []byte("img")},
			&model.PublicationContent{ContentType: "text/markdown", Content: []byte("# Loans")}, defaultPublicationArchiveMaxBytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if files := readArchive(t, content.Archive); len(files) != 2 || files["content/web/api-icon.png"] == "" || files["content/web/apiContent.md"] == "" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("leaves out a blank landing page", func(t *testing.T) {
		blank := &model.PublicationContent{ContentType: "text/markdown", Content: []byte(" \n\t")}
		content, err := buildContentArchive(nil, nil, blank, defaultPublicationArchiveMaxBytes)
		if err != nil || content != nil {
			t.Fatalf("a blank landing page alone: got %v, %v", content, err)
		}
		content, err = buildContentArchive(
			[]portalDocument{{Handle: "guide", Folder: "HowTo", FileName: "guide.md", Content: []byte("# guide")}}, nil, blank, defaultPublicationArchiveMaxBytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if files := readArchive(t, content.Archive); len(files) != 1 || files["content/docs/HowTo/guide.md"] == "" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("rejects a thumbnail that is not PNG or JPEG", func(t *testing.T) {
		_, err := buildContentArchive(nil, &model.PublicationContent{ContentType: "image/gif", Content: []byte("img")}, nil, defaultPublicationArchiveMaxBytes)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})

	t.Run("no documents means nothing to send", func(t *testing.T) {
		content, err := buildContentArchive(nil, nil, nil, defaultPublicationArchiveMaxBytes)
		if err != nil || content != nil {
			t.Fatalf("got %v, %v", content, err)
		}
	})

	t.Run("rejects an archive over the ceiling", func(t *testing.T) {
		noise := make([]byte, 4096)
		if _, err := rand.Read(noise); err != nil {
			t.Fatal(err)
		}
		_, err := buildContentArchive([]portalDocument{{Handle: "big", Folder: "HowTo", FileName: "big.md", Content: noise}}, nil, nil, 1024)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})

	t.Run("counts the landing page toward the ceiling", func(t *testing.T) {
		noise := make([]byte, 4096)
		if _, err := rand.Read(noise); err != nil {
			t.Fatal(err)
		}
		_, err := buildContentArchive(nil, nil, &model.PublicationContent{Content: noise}, 1024)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})

	t.Run("counts the thumbnail and landing page toward the entry limit", func(t *testing.T) {
		docs := make([]portalDocument, portalContentMaxEntries-1)
		for i := range docs {
			docs[i] = portalDocument{Handle: fmt.Sprintf("d%d", i), Folder: "HowTo", FileName: fmt.Sprintf("d%d.md", i)}
		}
		thumbnail := &model.PublicationContent{ContentType: "image/png", Content: []byte("img")}
		landingPage := &model.PublicationContent{Content: []byte("# Loans")}

		if _, err := buildContentArchive(docs, thumbnail, nil, defaultPublicationArchiveMaxBytes); err != nil {
			t.Fatalf("exactly at the limit: %v", err)
		}
		_, err := buildContentArchive(docs, thumbnail, landingPage, defaultPublicationArchiveMaxBytes)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})

	t.Run("counts the thumbnail and landing page toward the extracted total", func(t *testing.T) {
		docs := []portalDocument{
			{Handle: "a", Folder: "HowTo", FileName: "a.md", Content: make([]byte, portalContentMaxFileBytes)},
			{Handle: "b", Folder: "HowTo", FileName: "b.md", Content: make([]byte, portalContentMaxFileBytes)},
			{Handle: "c", Folder: "HowTo", FileName: "c.md", Content: make([]byte, portalContentMaxFileBytes)},
			{Handle: "d", Folder: "HowTo", FileName: "d.md", Content: make([]byte, portalContentMaxFileBytes)},
		}
		landingPage := &model.PublicationContent{Content: bytes.Repeat([]byte("a"), int(portalContentMaxFileBytes))}
		thumbnail := &model.PublicationContent{ContentType: "image/png", Content: []byte("img")}

		if _, err := buildContentArchive(docs, nil, landingPage, 1<<30); err != nil {
			t.Fatalf("at the limit: %v", err)
		}
		_, err := buildContentArchive(docs, thumbnail, landingPage, 1<<30)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})

	t.Run("rejects a landing page over the per-file limit", func(t *testing.T) {
		_, err := buildContentArchive(nil, nil, &model.PublicationContent{Content: bytes.Repeat([]byte("a"), int(portalContentMaxFileBytes)+1)}, 1<<30)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})
}
