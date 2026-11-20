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

package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/goenv"
)

// modDownloadFailure builds the error for a failed `go mod download -json` run.
// With -json, Go writes the reason (e.g. a git authentication failure) into the
// Error field on stdout and leaves stderr empty, so stdout has to be read too.
func modDownloadFailure(gomodule string, stdout, stderr []byte, runErr error) error {
	var reasons []string

	var result struct {
		Error string `json:"Error"`
	}
	if err := json.NewDecoder(bytes.NewReader(stdout)).Decode(&result); err == nil {
		if msg := strings.TrimSpace(result.Error); msg != "" {
			reasons = append(reasons, msg)
		}
	}
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		reasons = append(reasons, msg)
	}

	reason := goenv.RedactCredentials(strings.Join(reasons, "\n"))
	if reason == "" {
		return fmt.Errorf("failed to run 'go mod download -json %s': %w", gomodule, runErr)
	}

	hint := ""
	if h := moduleAccessHint(reason); h != "" {
		hint = "\nhint: " + h
	}
	return fmt.Errorf("failed to run 'go mod download -json %s' (%w): %s%s", gomodule, runErr, reason, hint)
}

// moduleAccessHint maps the usual private-module failures to the setting that fixes them.
func moduleAccessHint(reason string) string {
	lower := strings.ToLower(reason)
	switch {
	// Checked first: when sum.golang.org cannot fetch a private module, Go quotes the
	// server's own git error ("could not read Username"), which credentials cannot fix.
	case containsAny(lower,
		"verifying module",
		"verifying go.mod",
		"sum.golang.org",
		"checksum database"):
		return "the module was checked against the public checksum database, which cannot see " +
			"private modules. Set GOPRIVATE (or GONOSUMDB) to a pattern matching the module path."
	// git reports a rejected token the same way as a missing one.
	case containsAny(lower,
		"terminal prompts disabled",
		"could not read username",
		"authentication failed",
		"permission denied (publickey)",
		"repository not found",
		"401 unauthorized",
		"403 forbidden"):
		return "the module host asked for credentials or rejected the ones given. If the module is private, " +
			"set GOPRIVATE to a pattern matching its path and provide credentials for the host " +
			"(ap gateway image build: --netrc or --git-credential). If you did, check that the token " +
			"is valid, can read the repository, and is set for this host."
	case strings.Contains(lower, "no protocol found for repository"):
		// Go reports this for a ".git"-qualified path when every scheme failed,
		// which hides an authentication failure as well as a network one.
		return "the go command could not reach the repository with any protocol. If it is private, " +
			"provide credentials for the host (ap gateway image build: --netrc or --git-credential); " +
			"otherwise check that the host is reachable from the builder container."
	}
	return ""
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
