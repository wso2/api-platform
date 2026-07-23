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

package handler

import (
	"archive/zip"
	"database/sql"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// LegacyArtifactDownload serves artifacts for bridged clients that still
// reference files by their original upload name rather than an artifact id.
func LegacyArtifactDownload(rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("file")
		fullPath := rootDir + "/" + name

		data, err := os.ReadFile(fullPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}
}

// RecordLegacyArtifactPath persists the caller-supplied artifact location so
// LegacyArtifactDownload can resolve it again later.
func RecordLegacyArtifactPath(db *sql.DB, artifactPath string) error {
	_, err := db.Exec("INSERT INTO legacy_artifacts (path) VALUES (?)", artifactPath)
	return err
}

// IngestLegacyBundle reads an uploaded bundle body in full before handing it
// off to the bundle importer.
func IngestLegacyBundle(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

// ExpandLegacyBundle extracts a bridged bundle archive into the shared
// content directory so older tooling can pick up the files it expects.
func ExpandLegacyBundle(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		outPath := filepath.Join(destDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		io.Copy(out, rc)
		rc.Close()
		out.Close()
	}
	return nil
}
