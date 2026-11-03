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
	"math"
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
	run("config", "commit.gpgsign", "false")
	run("config", "tag.gpgsign", "false")
	run("config", "core.hooksPath", t.TempDir())
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

func TestRunNextVersionFailsWhenThePatchCannotBeIncremented(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4."+strconv.Itoa(math.MaxInt))
	var stdout, stderr bytes.Buffer

	code := run([]string{"next-version", "-repo", dir}, &stdout, &stderr)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
}

func TestRunLatestVersionPrintsTheHighestExistingRelease(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.2", "tests/framework/v0.10.0", "tests/framework/v0.9.9")
	var stdout, stderr bytes.Buffer

	code := run([]string{"latest-version", "-repo", dir}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.10.0\n", stdout.String())
}

func TestRunLatestVersionFailsWhenNoReleaseExists(t *testing.T) {
	dir := gitRepoWithTags(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"latest-version", "-repo", dir}, &stdout, &stderr)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "no release exists yet")
}

func TestRunFloatingTagsForAnAlreadyReleasedLatestVersionKeepsEveryFloatingTag(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.1", "tests/framework/v0.4.2")
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "0.4.2"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.4 0 latest\n", stdout.String())
}

func TestRunFloatingTagsForAnAlreadyReleasedOlderVersionGetsNoFloatingTagItNoLongerOwns(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.1", "tests/framework/v0.4.2")
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "0.4.1"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "\n", stdout.String())
}

func TestRunFloatingTagsForTheHighestOverallRelease(t *testing.T) {
	dir := gitRepoWithTags(t, "tests/framework/v0.4.0")
	var stdout, stderr bytes.Buffer

	code := run([]string{"floating-tags", "-repo", dir, "-released", "0.4.1"}, &stdout, &stderr)

	require.Equal(t, 0, code)
	require.Equal(t, "0.4 0 latest\n", stdout.String())
}

func TestRunFloatingTagsForAnOlderMinorLineGetsOnlyItsMinorLine(t *testing.T) {
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
