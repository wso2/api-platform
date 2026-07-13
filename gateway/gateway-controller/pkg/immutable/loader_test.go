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

package immutable

import (
	"os"
	"path/filepath"
	"testing"
)

const petstoreArtifact = `apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: petstore-v1
spec:
  displayName: Petstore
  version: v1.0
  context: /petstore/$version
  upstream:
    main:
      url: https://petstore3.swagger.io/api/v3
  operations:
    - method: GET
      path: /pet/{petId}
`

func TestCollectArtifacts_ConfigMapMountYieldsFileOnce(t *testing.T) {
	// Kubernetes ConfigMap mount layout:
	//   <dir>/
	//     ..2026_07_13_.../petstore.yaml        (real file in timestamped dir)
	//     ..data -> ..2026_07_13_.../            (symlink to current revision)
	//     petstore.yaml -> ..data/petstore.yaml  (per-key symlink)
	dir := t.TempDir()

	tsDir := filepath.Join(dir, "..2026_07_13_10_00_00.123456")
	if err := os.Mkdir(tsDir, 0o700); err != nil {
		t.Fatalf("setup: mkdir %s: %v", tsDir, err)
	}
	if err := os.WriteFile(filepath.Join(tsDir, "petstore.yaml"), []byte(petstoreArtifact), 0o600); err != nil {
		t.Fatalf("setup: write real file: %v", err)
	}

	dotData := filepath.Join(dir, "..data")
	if err := os.Symlink(tsDir, dotData); err != nil {
		t.Fatalf("setup: symlink ..data: %v", err)
	}

	topLevel := filepath.Join(dir, "petstore.yaml")
	if err := os.Symlink(filepath.Join(dotData, "petstore.yaml"), topLevel); err != nil {
		t.Fatalf("setup: symlink petstore.yaml: %v", err)
	}

	paths, err := collectArtifacts(dir)

	if err != nil {
		t.Fatalf("collectArtifacts: unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d path(s) %v; want exactly 1", len(paths), paths)
	}
	if paths[0] != topLevel {
		t.Errorf("got %q; want top-level symlink %q", paths[0], topLevel)
	}
}

func TestCollectArtifacts_DescendsIntoNonDotDotDirs(t *testing.T) {
	cases := []struct {
		name   string
		subdir string
	}{
		{"nested subdirectory", "rest-apis"},
		{"single-dot directory", ".hidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			sub := filepath.Join(dir, tc.subdir)
			if err := os.Mkdir(sub, 0o700); err != nil {
				t.Fatalf("setup: %v", err)
			}
			want := filepath.Join(sub, "petstore.yaml")
			if err := os.WriteFile(want, []byte(petstoreArtifact), 0o600); err != nil {
				t.Fatalf("setup: %v", err)
			}

			paths, err := collectArtifacts(dir)

			if err != nil {
				t.Fatalf("collectArtifacts: %v", err)
			}
			if len(paths) != 1 {
				t.Fatalf("got %d path(s) %v; want 1", len(paths), paths)
			}
			if paths[0] != want {
				t.Errorf("got %q; want %q", paths[0], want)
			}
		})
	}
}

