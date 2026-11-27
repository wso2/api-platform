//go:build !windows

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

package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeManifestEngine is an engine that records its arguments and answers
// manifest inspect and images like podman does after a two-platform build: one
// image of the list is dangling, the other has a name and so is not listed.
const fakeManifestEngine = `#!/bin/sh
echo "$*" >> "$CALLS"
case "$1 $2" in
  "manifest inspect") echo '{"manifests":[{"digest":"sha256:aaa"},{"digest":"sha256:bbb"}]}' ;;
  "images --filter") printf 'id-aaa sha256:aaa\nid-zzz sha256:zzz\n' ;;
esac
`

// manifest rm alone leaves the list's images behind, so they are removed too:
// only the dangling ones, and only after the list (podman reports an image a
// list refers to as not dangling).
func TestRemoveManifestListRemovesItsImages(t *testing.T) {
	dir := t.TempDir()
	engine := filepath.Join(dir, "engine")
	if err := os.WriteFile(engine, []byte(fakeManifestEngine), 0o700); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls")
	t.Setenv("CALLS", calls)
	logFile, err := os.CreateTemp(dir, "log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	removeManifestList(engine, "localhost/list", logFile)

	b, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := "manifest inspect localhost/list\n" +
		"manifest rm localhost/list\n" +
		"images --filter dangling=true --digests --format {{.ID}} {{.Digest}}\n" +
		"rmi id-aaa\n"
	if string(b) != want {
		t.Fatalf("calls:\n%s\nwant:\n%s", b, want)
	}
}

// When the list was never created (the build failed first), only manifest rm is tried.
func TestRemoveManifestListWithoutAList(t *testing.T) {
	dir := t.TempDir()
	engine := filepath.Join(dir, "engine")
	script := "#!/bin/sh\necho \"$*\" >> \"$CALLS\"\n[ \"$1\" = manifest ] && [ \"$2\" = inspect ] && exit 125\nexit 0\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls")
	t.Setenv("CALLS", calls)
	logFile, err := os.CreateTemp(dir, "log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	removeManifestList(engine, "localhost/list", logFile)

	b, _ := os.ReadFile(calls)
	if got := strings.Split(strings.TrimSpace(string(b)), "\n"); len(got) != 2 || got[1] != "manifest rm localhost/list" {
		t.Fatalf("calls = %q, want manifest inspect then manifest rm only", got)
	}
}
