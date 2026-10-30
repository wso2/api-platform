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

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// gitRepoWithTags creates a throwaway git checkout with one commit and the given
// "tests/framework/v..." tags, so next-version/floating-tags can be exercised against a real
// (but hermetic, local-only) git repository rather than a mocked tag list.
func gitRepoWithTags(t *testing.T, tags ...string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	require.NoError(t, os.WriteFile(dir+"/f", []byte("x"), 0o644))
	run("add", "f")
	run("commit", "-q", "-m", "init")
	for _, tag := range tags {
		run("tag", tag)
	}
	return dir
}

func TestRunNextVersionBumpsThePatchOfTheHighestExistingRelease(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.1.0", "tests/framework/v0.4.0",
		"tests/framework/v0.4.2", "tests/framework/v0.3.9")
	var stdout, stderr bytes.Buffer

	code := run([]string{"next-version", "-repo", dir}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.4.3\n", stdout.String())
}

func TestRunNextVersionBootstrapsWhenNoReleaseExists(t *testing.T) {
	dir := gitRepoWithTags(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"next-version", "-repo", dir}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.0.1\n", stdout.String())
}

func TestRunFloatingTagsForTheHighestOverallRelease(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.0")
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "0.4.1"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.4 0 latest\n", stdout.String())
}

func TestRunFloatingTagsForABackportGetsOnlyItsMinorLine(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.2")
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "0.3.5"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.3\n", stdout.String())
}

func TestRunFloatingTagsRejectsAMalformedReleasedVersion(t *testing.T) {
	dir := gitRepoWithTags(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "not-a-version"}, &stdout, &stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "not a MAJOR.MINOR.PATCH version")
}

func TestRunRejectsAnUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"bogus"}, &stdout, &stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "unknown subcommand")
}

func TestRunRetainSkipsWithoutATokenAndStillSucceeds(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "")
	var stdout, stderr bytes.Buffer

	code := run([]string{"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Contains(t, stdout.String(), "::warning::")
}

func TestRunRetainDeletesTheOldestVersionsBeyondTheKeepLimit(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	var versions []ghcrVersion
	for i, tags := range [][]string{{"0.4.0"}, {"0.4.1"}, {"0.4.2"}, {"latest", "0.4.3", "0.4", "0"}, {"latest"}, {"0.3.9"}} {
		v := ghcrVersion{ID: int64(i + 1)}
		v.Metadata.Container.Tags = tags
		versions = append(versions, v)
	}
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/wso2/packages/container/testbench/versions", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer a-token", r.Header.Get("Authorization"))
		require.NoError(t, json.NewEncoder(w).Encode(versions))
	})
	mux.HandleFunc("DELETE /orgs/wso2/packages/container/testbench/versions/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3",
		"-keep", "3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, []string{"1"}, deleted) // only 0.4.0 (id=1) falls outside the newest 3: .1, .2, .3
	require.Contains(t, stdout.String(), "deleted 0.4.0")
}

func untaggedVersions(firstID int64, count int) []ghcrVersion {
	versions := make([]ghcrVersion, count)
	for i := range versions {
		versions[i].ID = firstID + int64(i)
	}
	return versions
}

func TestRunRetainFindsVersionsBeyondTheFirstPage(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	pages := map[string][]ghcrVersion{"1": untaggedVersions(1, versionsPageSize)}
	for i, tag := range []string{"0.4.0", "0.4.1", "0.4.2", "0.4.3"} {
		v := ghcrVersion{ID: int64(1000 + i)}
		v.Metadata.Container.Tags = []string{tag}
		pages["2"] = append(pages["2"], v)
	}
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/wso2/packages/container/testbench/versions", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(pages[r.URL.Query().Get("page")]))
	})
	mux.HandleFunc("DELETE /orgs/wso2/packages/container/testbench/versions/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3",
		"-keep", "3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, []string{"1000"}, deleted) // 0.4.0, on page 2, is the only one beyond the newest 3
}

func TestRunRetainWarnsInsteadOfPruningOnATruncatedListing(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/wso2/packages/container/testbench/versions", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(untaggedVersions(1, versionsPageSize)))
	})
	mux.HandleFunc("DELETE /orgs/wso2/packages/container/testbench/versions/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Empty(t, deleted)
	require.Contains(t, stdout.String(), "::warning::")
	require.Contains(t, stdout.String(), "pages")
}

func taggedVersion(id int64, tags ...string) ghcrVersion {
	v := ghcrVersion{ID: id}
	v.Metadata.Container.Tags = tags
	return v
}

func retentionServer(t *testing.T, list func(page string) []ghcrVersion, status func(id string) int) (*httptest.Server, *[]string) {
	t.Helper()
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/wso2/packages/container/testbench/versions", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(list(r.URL.Query().Get("page"))))
	})
	mux.HandleFunc("DELETE /orgs/wso2/packages/container/testbench/versions/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.PathValue("id"))
		w.WriteHeader(status(r.PathValue("id")))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &deleted
}

func TestRunRetainRejectsAKeepBelowOne(t *testing.T) {
	for _, keep := range []string{"0", "-1"} {
		var stdout, stderr bytes.Buffer

		code := run([]string{"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3", "-keep", keep}, &stdout, &stderr)

		require.Equal(t, 2, code, keep)
		require.Contains(t, stderr.String(), "must be at least 1", keep)
	}
}

func TestRunRetainNeverDeletesTheVersionJustReleased(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	versions := []ghcrVersion{
		taggedVersion(1, "0.4.0"), taggedVersion(2, "0.4.1"), taggedVersion(3, "0.4.2"),
		taggedVersion(4, "0.4.3"), taggedVersion(5, "0.4.4"), taggedVersion(6, "0.4.5"),
	}
	server, deleted := retentionServer(t, func(string) []ghcrVersion { return versions },
		func(string) int { return http.StatusNoContent })
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.1",
		"-keep", "3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.ElementsMatch(t, []string{"1", "3"}, *deleted) // 0.4.0 and 0.4.2; 0.4.1 is the released version
}

func TestRunRetainNeverDeletesAPackageVersionThatAlsoCarriesTheReleasedTag(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	versions := []ghcrVersion{
		taggedVersion(1, "0.4.2", "0.4.3"), taggedVersion(2, "0.4.4"),
		taggedVersion(3, "0.4.5"), taggedVersion(4, "0.4.6"),
	}
	server, deleted := retentionServer(t, func(string) []ghcrVersion { return versions },
		func(string) int { return http.StatusNoContent })
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3",
		"-keep", "3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Empty(t, *deleted)
}

func TestRunRetainKeepsDeletingAfterOneDeletionFails(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	versions := []ghcrVersion{taggedVersion(1, "0.4.0"), taggedVersion(2, "0.4.1"), taggedVersion(3, "0.4.2"), taggedVersion(4, "0.4.3"), taggedVersion(5, "0.4.4")}
	server, deleted := retentionServer(t, func(string) []ghcrVersion { return versions }, func(id string) int {
		if id == "1" {
			return http.StatusInternalServerError
		}
		return http.StatusNoContent
	})
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.4",
		"-keep", "3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, []string{"1", "2"}, *deleted) // the failure on 0.4.0 doesn't stop 0.4.1
	require.Contains(t, stdout.String(), "deleted 0.4.1")
	require.Contains(t, stdout.String(), "::warning::")
}

func TestRunRetainAcceptsAListingOfExactlyTheMaximumPages(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	server, deleted := retentionServer(t, func(page string) []ghcrVersion {
		if page == strconv.Itoa(maxVersionPages+1) {
			return nil
		}
		return untaggedVersions(1, versionsPageSize)
	}, func(string) int { return http.StatusNoContent })
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Empty(t, *deleted)
	require.NotContains(t, stdout.String(), "::warning::")
	require.Contains(t, stdout.String(), "nothing to delete")
}

func TestRunRetainWarnsWithoutFailingWhenTheAPIRejectsTheRequest(t *testing.T) {
	t.Setenv("GHCR_RETENTION_TOKEN", "a-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"retain", "-org", "wso2", "-package", "testbench", "-released", "0.4.3", "-api-base", server.URL,
	}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Contains(t, stdout.String(), "::warning::")
}
