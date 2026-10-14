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

package assets

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// SnapshotSources lists the repository files and directories outside the framework module
// that the engine reads and that the module therefore carries a copy of. Each is mirrored
// under snapshots/ at its repository-relative path.
var SnapshotSources = []string{
	"gateway/configs/config.toml",
	"gateway/configs/llm-pricing/model_prices.json",
	"gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql",
	"gateway/gateway-controller/pkg/storage/gateway-controller-db.sqlserver.sql",
}

// SyncSnapshots compares each snapshot source in repoRoot with its copy under
// moduleRoot/snapshots and returns the repository-relative files that differ or are
// missing. When write is true it copies the sources over the snapshots. A snapshot file
// whose source no longer exists is reported as well and removed when write is true.
func SyncSnapshots(repoRoot, moduleRoot string, write bool) ([]string, error) {
	var drifted []string
	for _, source := range SnapshotSources {
		sourceRoot := filepath.Join(repoRoot, filepath.FromSlash(source))
		snapshotRoot := filepath.Join(moduleRoot, snapshotDir, filepath.FromSlash(source))
		files, err := sourceFiles(sourceRoot)
		if err != nil {
			return nil, fmt.Errorf("assets: reading snapshot source %q: %w", source, err)
		}
		seen := map[string]bool{}
		for _, relative := range files {
			seen[relative] = true
			name := filepath.ToSlash(filepath.Join(source, relative))
			want, err := os.ReadFile(filepath.Join(sourceRoot, relative))
			if err != nil {
				return nil, fmt.Errorf("assets: reading %q: %w", name, err)
			}
			dest := filepath.Join(snapshotRoot, relative)
			got, err := os.ReadFile(dest)
			if err == nil && bytes.Equal(got, want) {
				continue
			}
			drifted = append(drifted, name)
			if !write {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return nil, fmt.Errorf("assets: creating %q: %w", filepath.Dir(dest), err)
			}
			if err := os.WriteFile(dest, want, 0o644); err != nil {
				return nil, fmt.Errorf("assets: writing %q: %w", dest, err)
			}
		}
		stale, err := sourceFiles(snapshotRoot)
		if err != nil {
			return nil, fmt.Errorf("assets: reading snapshot %q: %w", source, err)
		}
		for _, relative := range stale {
			if seen[relative] {
				continue
			}
			drifted = append(drifted, filepath.ToSlash(filepath.Join(source, relative)))
			if write {
				if err := os.Remove(filepath.Join(snapshotRoot, relative)); err != nil {
					return nil, fmt.Errorf("assets: removing stale snapshot %q: %w", relative, err)
				}
			}
		}
	}
	sort.Strings(drifted)
	return drifted, nil
}

// sourceFiles lists the files beneath root, as paths relative to it. A root that is itself
// a file yields one empty relative path; a missing root yields none.
func sourceFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{""}, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	return files, err
}
