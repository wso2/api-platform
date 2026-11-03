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

// Command release computes the test framework's release version and floating-tag decisions for
// the Tests Framework Release workflow. It wraps the pure core/release package with the git tag
// listing a real release run needs.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/release"
)

// releaseTagPrefix is the git tag namespace this module's releases live under.
const releaseTagPrefix = "tests/framework/v"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "release: missing subcommand (next-version, latest-version, floating-tags)")
		return 2
	}
	switch args[0] {
	case "next-version":
		return runNextVersion(args[1:], stdout, stderr)
	case "latest-version":
		return runLatestVersion(args[1:], stdout, stderr)
	case "floating-tags":
		return runFloatingTags(args[1:], stdout, stderr)
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
	next, err := release.NextVersion(tags)
	if err != nil {
		fmt.Fprintln(stderr, "release:", err)
		return 1
	}
	fmt.Fprintln(stdout, next)
	return 0
}

func runLatestVersion(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("latest-version", flag.ContinueOnError)
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
	latest, ok := release.Highest(release.ParseVersions(tags))
	if !ok {
		fmt.Fprintln(stderr, "release: no release exists yet")
		return 1
	}
	fmt.Fprintln(stdout, latest)
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
