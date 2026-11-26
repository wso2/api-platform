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

// Command affectedblocks selects the integration-suite blocks a change can affect, for the
// Platform Tests workflow. It reads the changed files, relative to the repository root, one
// per line on stdin, and writes two GitHub Actions outputs to stdout:
//
//	blocks=<the -blocks value, empty for every block>
//	label=<what the integration jobs' titles show>
//	complete=<,product,...,: the products whose every block runs, for the coverage uploads>
//
// The rules, for each changed file:
//   - a feature file runs the blocks that list it;
//   - a block file runs the blocks it declares;
//   - a resource runs the blocks of the features that name it;
//   - a product step package (steps/<product>/) runs the blocks whose features use its steps,
//     or a step of a package that imports it, plus the first block of each block file: every
//     block registers every package, so a fault in a package's hooks would fail those too;
//   - product source (gateway/, platform-api/, portals/api-portal/ and their catalog
//     definitions) runs every block that starts the product;
//   - anything else, shared step code included, runs every block.
//
// Run it from tests/framework.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// main parses the flags and prints the selection.
func main() {
	suite := flag.String("suite", "suites/it/it-suite.yaml", "the integration suite file")
	suiteDir := flag.String("suite-dir", "tests/framework/suites/it",
		"the suite directory relative to the repository root, as changed files name it")
	flag.Parse()

	if err := run(*suite, *suiteDir); err != nil {
		fmt.Fprintln(os.Stderr, "affectedblocks:", err)
		os.Exit(1)
	}
}

// run loads the suite, reads the changed files from stdin and prints the outputs.
func run(suite, suiteDir string) error {
	registry, err := catalog.Registry()
	if err != nil {
		return err
	}
	resolved, err := topology.LoadFile(suite, registry)
	if err != nil {
		return err
	}
	idx, err := buildIndex(resolved, filepath.Dir(suite), suiteDir)
	if err != nil {
		return err
	}

	var changed []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		changed = append(changed, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	sel := idx.affected(changed)
	fmt.Fprintf(os.Stderr, "selected: %s\n", orAll(sel.blocks()))
	fmt.Printf("blocks=%s\n", sel.blocks())
	fmt.Printf("label=%s\n", idx.label(sel))
	fmt.Printf("complete=%s\n", idx.complete(sel))
	return nil
}

// orAll names an empty selection for the log.
func orAll(blocks string) string {
	if blocks == "" {
		return "every block"
	}
	return blocks
}
