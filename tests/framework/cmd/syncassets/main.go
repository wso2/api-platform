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

// Command syncassets generates the framework's embedded snapshots from the repository files
// they copy. The snapshots are not committed: the release workflow runs it before tagging.
// It lists the files that differ and, unless -check is given, rewrites them.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wso2/api-platform/tests/framework/core/assets"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("syncassets", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", "../..", "api-platform repository root")
	module := flags.String("module", ".", "framework module root")
	check := flags.Bool("check", false, "report drift and exit non-zero without writing")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	repoRoot, err := filepath.Abs(*repo)
	if err != nil {
		fmt.Fprintln(stderr, "syncassets:", err)
		return 2
	}
	moduleRoot, err := filepath.Abs(*module)
	if err != nil {
		fmt.Fprintln(stderr, "syncassets:", err)
		return 2
	}
	drifted, err := assets.SyncSnapshots(repoRoot, moduleRoot, !*check)
	if err != nil {
		fmt.Fprintln(stderr, "syncassets:", err)
		return 2
	}
	for _, name := range drifted {
		fmt.Fprintln(stdout, name)
	}
	if *check && len(drifted) > 0 {
		fmt.Fprintf(stderr, "syncassets: %d snapshot file(s) differ from the repository; run: go run ./cmd/syncassets\n", len(drifted))
		return 1
	}
	return 0
}
