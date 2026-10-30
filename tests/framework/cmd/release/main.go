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

// Command release computes the test framework's release version, floating-tag, and GHCR
// retention decisions for the Tests Framework Release workflow. It wraps the pure core/release
// package with the git tag listing and GHCR REST calls a real release run needs.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/release"
)

// releaseTagPrefix is the git tag namespace this module's releases live under.
const releaseTagPrefix = "tests/framework/v"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "release: missing subcommand (next-version, floating-tags, retain)")
		return 2
	}
	switch args[0] {
	case "next-version":
		return runNextVersion(args[1:], stdout, stderr)
	case "floating-tags":
		return runFloatingTags(args[1:], stdout, stderr)
	case "retain":
		return runRetain(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "release: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runNextVersion(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("next-version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "git checkout to read release tags from")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	tags, err := releaseTags(*repo)
	if err != nil {
		fmt.Fprintln(stderr, "release:", err)
		return 1
	}
	fmt.Fprintln(stdout, release.NextVersion(tags))
	return 0
}

func runFloatingTags(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("floating-tags", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "git checkout to read release tags from")
	released := flags.String("released", "", "the version just released (MAJOR.MINOR.PATCH)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	version, ok := release.ParseVersion(*released)
	if !ok {
		fmt.Fprintf(stderr, "release: -released=%q is not a MAJOR.MINOR.PATCH version\n", *released)
		return 2
	}
	tags, err := releaseTags(*repo)
	if err != nil {
		fmt.Fprintln(stderr, "release:", err)
		return 1
	}
	fmt.Fprintln(stdout, strings.Join(release.FloatingTags(tags, version), " "))
	return 0
}

// releaseTags lists this module's release tags from repo, stripped of releaseTagPrefix.
func releaseTags(repo string) ([]string, error) {
	cmd := exec.Command("git", "-C", repo, "tag", "-l", releaseTagPrefix+"*")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing release tags: %w", err)
	}
	var tags []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		tags = append(tags, strings.TrimPrefix(line, releaseTagPrefix))
	}
	return tags, scanner.Err()
}

func runRetain(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("retain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	org := flags.String("org", "", "GitHub organization that owns the package")
	pkg := flags.String("package", "", "GHCR package name")
	released := flags.String("released", "", "the version just released (MAJOR.MINOR.PATCH)")
	keep := flags.Int("keep", 3, "how many patches to keep per minor line")
	baseURL := flags.String("api-base", "https://api.github.com", "GitHub API base URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	version, ok := release.ParseVersion(*released)
	if !ok {
		fmt.Fprintf(stderr, "release: -released=%q is not a MAJOR.MINOR.PATCH version\n", *released)
		return 2
	}
	if *org == "" || *pkg == "" {
		fmt.Fprintln(stderr, "release: -org and -package are required")
		return 2
	}
	if *keep < 1 {
		fmt.Fprintf(stderr, "release: -keep=%d must be at least 1\n", *keep)
		return 2
	}

	// Retention is best-effort cleanup, never a reason to fail a release that already shipped:
	// a missing token, an insufficient scope, or any other API error becomes a warning, not a
	// non-zero exit.
	token := os.Getenv("GHCR_RETENTION_TOKEN")
	if token == "" {
		fmt.Fprintln(stdout, "::warning::GHCR_RETENTION_TOKEN is not set — skipping retention")
		return 0
	}
	client := &ghcrClient{
		baseURL:    *baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		org:        *org,
		pkg:        *pkg,
	}
	if err := retain(context.Background(), client, version, *keep, stdout); err != nil {
		fmt.Fprintf(stdout, "::warning::retention incomplete, some old testbench images may not have been pruned: %v\n", err)
	}
	return 0
}

func retain(ctx context.Context, client *ghcrClient, released release.Version, keep int, stdout io.Writer) error {
	versions, err := client.listVersions(ctx)
	if err != nil {
		return err
	}
	releasedIDs := make(map[string]bool)
	var tagged []release.TaggedVersion
	for _, gv := range versions {
		id := strconv.FormatInt(gv.ID, 10)
		for _, tag := range gv.Metadata.Container.Tags {
			if v, ok := release.ParseVersion(tag); ok && v == released {
				releasedIDs[id] = true
			}
		}
		for _, tag := range gv.Metadata.Container.Tags {
			if v, ok := release.ParseVersion(tag); ok {
				tagged = append(tagged, release.TaggedVersion{Version: v, ID: id})
				break // one release-version tag identifies the package version; co-applied floating tags don't add another
			}
		}
	}
	var toDelete []release.TaggedVersion
	for _, tv := range release.Retain(release.FilterMinor(tagged, released.Major, released.Minor), keep) {
		if tv.Version != released && !releasedIDs[tv.ID] {
			toDelete = append(toDelete, tv)
		}
	}
	var failed []error
	for _, tv := range toDelete {
		id, err := strconv.ParseInt(tv.ID, 10, 64)
		if err != nil {
			failed = append(failed, fmt.Errorf("parsing version id %q: %w", tv.ID, err))
			continue
		}
		if err := client.deleteVersion(ctx, id); err != nil {
			failed = append(failed, fmt.Errorf("deleting %s (id=%s): %w", tv.Version, tv.ID, err))
			continue
		}
		fmt.Fprintf(stdout, "retention: deleted %s (id=%s)\n", tv.Version, tv.ID)
	}
	if len(toDelete) == 0 {
		fmt.Fprintln(stdout, "retention: nothing to delete")
	}
	return errors.Join(failed...)
}
