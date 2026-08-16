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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sync"
)

type tarEntry struct {
	name     string
	typeflag byte
	body     []byte
}

func buildTar(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typeflag, Mode: 0o644, Size: int64(len(e.body))}
		if e.typeflag == tar.TypeSymlink {
			hdr.Linkname = "/etc/passwd"
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if len(e.body) > 0 {
			_, err := tw.Write(e.body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	return &buf
}

// TestUntarToExtractsDockerShapedStream verifies Docker archive extraction.
func TestUntarToExtractsDockerShapedStream(t *testing.T) {
	binary := []byte{0x00, 0x01, 0xff, 0xfe, 'c', 'o', 'v'}
	stream := buildTar(t, []tarEntry{
		{name: "coverage/", typeflag: tar.TypeDir},
		{name: "coverage/covmeta.abc123", typeflag: tar.TypeReg, body: binary},
		{name: "coverage/covcounters.abc123.1.42", typeflag: tar.TypeReg, body: []byte("counters")},
	})

	dst := t.TempDir()
	require.NoError(t, untarTo(dst, stream))

	got, err := os.ReadFile(filepath.Join(dst, "covmeta.abc123"))
	require.NoError(t, err)
	require.Equal(t, binary, got, "binary content must survive extraction byte-for-byte")

	got, err = os.ReadFile(filepath.Join(dst, "covcounters.abc123.1.42"))
	require.NoError(t, err)
	require.Equal(t, []byte("counters"), got)
}

func TestUntarToPreservesNestedFlatArchivePaths(t *testing.T) {
	dst := t.TempDir()
	require.NoError(t, untarTo(dst, buildTar(t, []tarEntry{
		{name: "service-a/covmeta", typeflag: tar.TypeReg, body: []byte("a")},
		{name: "service-b/covmeta", typeflag: tar.TypeReg, body: []byte("b")},
	})))

	first, err := os.ReadFile(filepath.Join(dst, "service-a", "covmeta"))
	require.NoError(t, err)
	second, err := os.ReadFile(filepath.Join(dst, "service-b", "covmeta"))
	require.NoError(t, err)
	require.Equal(t, []byte("a"), first)
	require.Equal(t, []byte("b"), second)
}

func TestUntarToDoesNotPublishPartialOutput(t *testing.T) {
	dst := t.TempDir()
	sentinel := filepath.Join(dst, "existing")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o644))

	err := untarTo(dst, buildTar(t, []tarEntry{
		{name: "new", typeflag: tar.TypeReg, body: []byte("new")},
		{name: "link", typeflag: tar.TypeSymlink},
	}))
	require.Error(t, err)
	got, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	require.Equal(t, []byte("keep"), got)
	_, readErr = os.Stat(filepath.Join(dst, "new"))
	require.ErrorIs(t, readErr, os.ErrNotExist)
}

func TestUntarToRejectsTraversal(t *testing.T) {
	for _, name := range []string{
		"coverage/../evil",
		"../evil",
		"/etc/passwd",
		"coverage/a/../../evil",
	} {
		stream := buildTar(t, []tarEntry{{name: name, typeflag: tar.TypeReg, body: []byte("x")}})
		err := untarTo(t.TempDir(), stream)
		require.Error(t, err, "entry %q must be rejected", name)
	}
}

// TestUntarToRejectsNonFileEntries rejects unsafe archive entries.
func TestUntarToRejectsNonFileEntries(t *testing.T) {
	stream := buildTar(t, []tarEntry{
		{name: "coverage/", typeflag: tar.TypeDir},
		{name: "coverage/link", typeflag: tar.TypeSymlink},
	})
	require.ErrorIs(t, untarTo(t.TempDir(), stream), errBadEntryType)
}

func TestUntarToBoundsEntryCount(t *testing.T) {
	old := maxEntries
	maxEntries = 3
	t.Cleanup(func() { maxEntries = old })

	entries := []tarEntry{{name: "coverage/", typeflag: tar.TypeDir}}
	for _, n := range []string{"a", "b", "c"} {
		entries = append(entries, tarEntry{name: "coverage/" + n, typeflag: tar.TypeReg, body: []byte("x")})
	}
	require.ErrorIs(t, untarTo(t.TempDir(), buildTar(t, entries)), errTooMany)
}

func TestUntarToBoundsTotalBytes(t *testing.T) {
	old := maxTotalBytes
	maxTotalBytes = 8
	t.Cleanup(func() { maxTotalBytes = old })

	stream := buildTar(t, []tarEntry{
		{name: "coverage/", typeflag: tar.TypeDir},
		{name: "coverage/big", typeflag: tar.TypeReg, body: bytes.Repeat([]byte("x"), 9)},
	})
	require.ErrorIs(t, untarTo(t.TempDir(), stream), errTooBig)
}

func TestCopyDirValidatesInputs(t *testing.T) {
	require.Error(t, CopyDir(nil, "container", "/coverage", t.TempDir()))
	require.Error(t, CopyDir(context.Background(), "", "/coverage", t.TempDir()))
}

// TestNewSinkWipesPriorRun ensures runs do not share stale counters.
func TestNewSinkWipesPriorRun(t *testing.T) {
	root := filepath.Join(t.TempDir(), "coverage-out")
	stale := filepath.Join(root, "gateway-core-sqlite", "gateway-runtime")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stale, "covcounters.old.1.1"), []byte("stale"), 0o644))

	sink, err := NewSink(root)
	require.NoError(t, err)

	entries, err := os.ReadDir(sink.Root())
	require.NoError(t, err)
	require.Empty(t, entries, "prior run's counters survived the per-run wipe")
}

func TestNewSinkRefusesDangerousRoots(t *testing.T) {
	_, err := NewSink("")
	require.Error(t, err)
	_, err = NewSink(string(filepath.Separator))
	require.Error(t, err)
	home, herr := os.UserHomeDir()
	require.NoError(t, herr)
	_, err = NewSink(home)
	require.Error(t, err)
}

// TestDirFlattensVariantNames verifies block names become single path elements.
func TestDirFlattensVariantNames(t *testing.T) {
	sink, err := NewSink(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)

	dir, err := sink.Dir("gateway-core/sqlite", "gateway-runtime")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(sink.Root(), sanitize("gateway-core/sqlite"), "gateway-runtime"), dir)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	rel, err := filepath.Rel(sink.Root(), dir)
	require.NoError(t, err)
	require.Equal(t, 2, len(splitPath(rel)), "want exactly <block>/<service> under the root")
}

func TestSinkRejectsNilReceiverAndPreservesDistinctNames(t *testing.T) {
	var sink *Sink
	require.Empty(t, sink.Root())
	_, err := sink.Dir("block", "service")
	require.Error(t, err)

	root := filepath.Join(t.TempDir(), "out")
	actual, err := NewSink(root)
	require.NoError(t, err)
	first, err := actual.Dir("a/b", "service")
	require.NoError(t, err)
	second, err := actual.Dir("a-b", "service")
	require.NoError(t, err)
	require.NotEqual(t, first, second)
}

func TestDirRejectsEmptyNames(t *testing.T) {
	sink, err := NewSink(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)
	_, err = sink.Dir("", "gateway-runtime")
	require.Error(t, err)
	_, err = sink.Dir("gateway-core/sqlite", " ")
	require.Error(t, err)
}

// TestDirIsConcurrencySafe verifies concurrent directory creation.
func TestDirIsConcurrencySafe(t *testing.T) {
	sink, err := NewSink(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			block := "gateway-core/sqlite"
			if i%2 == 0 {
				block = "gateway-core/postgres"
			}
			if _, err := sink.Dir(block, "gateway-runtime"); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func splitPath(p string) []string {
	var parts []string
	for dir := p; dir != "." && dir != ""; dir = filepath.Dir(dir) {
		parts = append([]string{filepath.Base(dir)}, parts...)
	}
	return parts
}
