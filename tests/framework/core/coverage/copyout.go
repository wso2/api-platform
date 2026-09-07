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

package coverage

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	mobyclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// GuestDir is the coverage directory inside instrumented containers.
const GuestDir = "/coverage"

// Extraction limits protect the host from malformed archives.
var (
	maxEntries      = 4096
	maxTotalBytes   = int64(1) << 30 // 1 GiB
	errTooMany      = fmt.Errorf("coverage: archive exceeds %d entries", maxEntries)
	errTooBig       = fmt.Errorf("coverage: archive exceeds %d bytes extracted", maxTotalBytes)
	errBadEntryType = fmt.Errorf("coverage: archive carries a non-file entry (symlink or device)")
)

// CopyDir copies a directory from a running or stopped container into dst.
func CopyDir(ctx context.Context, containerID, srcPath, dst string) error {
	if ctx == nil {
		return fmt.Errorf("coverage: copy context is required")
	}
	if strings.TrimSpace(containerID) == "" || strings.TrimSpace(srcPath) == "" || strings.TrimSpace(dst) == "" {
		return fmt.Errorf("coverage: container ID, source path, and destination are required")
	}
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return fmt.Errorf("coverage: docker client: %w", err)
	}
	defer cli.Close()

	res, err := cli.CopyFromContainer(ctx, containerID,
		mobyclient.CopyFromContainerOptions{SourcePath: srcPath})
	if err != nil {
		return fmt.Errorf("coverage: copying %s from %s: %w", srcPath, containerID, err)
	}
	defer res.Content.Close()

	if err := untarTo(dst, res.Content); err != nil {
		return fmt.Errorf("coverage: extracting %s from %s: %w", srcPath, containerID, err)
	}
	return nil
}

// untarTo extracts a coverage archive atomically into dst.
func untarTo(dst string, r io.Reader) error {
	if strings.TrimSpace(dst) == "" {
		return fmt.Errorf("coverage: extraction destination is required")
	}
	if r == nil {
		return fmt.Errorf("coverage: archive reader is required")
	}
	parent := filepath.Dir(filepath.Clean(dst))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(parent, ".coverage-extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()

	dstRoot := filepath.Clean(work) + string(filepath.Separator)

	tr := tar.NewReader(r)
	entries := 0
	var total int64
	rootPrefix := ""

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			if err := os.RemoveAll(dst); err != nil {
				return fmt.Errorf("coverage: replacing extraction destination: %w", err)
			}
			if err := os.Rename(work, dst); err != nil {
				return fmt.Errorf("coverage: publishing extracted archive: %w", err)
			}
			return nil
		}
		if err != nil {
			return err
		}

		entries++
		if entries > maxEntries {
			return errTooMany
		}

		name := hdr.Name
		if strings.ContainsRune(name, 0) || path.IsAbs(name) || hasParentSegment(name) {
			return fmt.Errorf("archive entry %q escapes the destination", name)
		}

		clean := path.Clean(name)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("archive entry %q escapes the destination", name)
		}
		if entries == 1 && hdr.Typeflag == tar.TypeDir && !strings.Contains(clean, "/") {
			rootPrefix = clean + "/"
			continue
		}
		rel := clean
		if rootPrefix != "" {
			if rel == strings.TrimSuffix(rootPrefix, "/") {
				continue
			}
			if !strings.HasPrefix(rel, rootPrefix) {
				return fmt.Errorf("archive entry %q is outside its root", name)
			}
			rel = strings.TrimPrefix(rel, rootPrefix)
		}
		if rel == "" || rel == "." {
			continue
		}

		target := filepath.Join(work, filepath.FromSlash(rel))
		if !strings.HasPrefix(filepath.Clean(target)+string(filepath.Separator), dstRoot) &&
			filepath.Clean(target) != filepath.Clean(work) {
			return fmt.Errorf("archive entry %q escapes the destination", name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			written, err := io.Copy(out, io.LimitReader(tr, maxTotalBytes-total+1))
			closeErr := out.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			total += written
			if total > maxTotalBytes {
				return errTooBig
			}
		default:
			return fmt.Errorf("%w: %q", errBadEntryType, name)
		}
	}
}

func hasParentSegment(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
