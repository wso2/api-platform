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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

const module = "github.com/acme/policies/header-transform@v1"

var errExit = errors.New("exit status 1")

func TestModDownloadFailure_ReportsJSONErrorField(t *testing.T) {
	// Captured from `go mod download -json` against a private GitHub repo with no credentials:
	// the reason is on stdout and stderr is empty.
	stdout := []byte(`{
	"Path": "github.com/acme/policies/header-transform",
	"Version": "v1",
	"Error": "github.com/acme/policies/header-transform@v1: invalid version: git ls-remote -q --end-of-options https://github.com/acme/policies in /go/pkg/mod/cache/vcs/abc: exit status 128:\n\tfatal: could not read Username for 'https://github.com': terminal prompts disabled"
}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.ErrorIs(t, err, errExit)
	assert.Contains(t, err.Error(), "could not read Username for 'https://github.com': terminal prompts disabled")
	assert.Contains(t, err.Error(), "hint: the module host asked for credentials")
	assert.Contains(t, err.Error(), "GOPRIVATE")
}

func TestModDownloadFailure_ChecksumDatabaseHint(t *testing.T) {
	stdout := []byte(`{"Path":"github.com/acme/p","Error":"github.com/acme/p@v1.0.0: verifying module: github.com/acme/p@v1.0.0: reading https://sum.golang.org/lookup/github.com/acme/p@v1.0.0: 404 Not Found"}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "verifying module")
	assert.Contains(t, err.Error(), "hint: the module was checked against the public checksum database")
}

func TestModDownloadFailure_ChecksumHintWinsOverQuotedGitError(t *testing.T) {
	// Captured from a real build with valid credentials but no GOPRIVATE: sum.golang.org
	// could not fetch the private repo, and Go quotes the server's own git error.
	stdout := []byte(`{"Path":"github.com/O-sura/custom-policy-hub/logger","Version":"v1.0.0","Error":"github.com/O-sura/custom-policy-hub/logger@v1.0.0: verifying go.mod: github.com/O-sura/custom-policy-hub/logger@v1.0.0/go.mod: reading https://sum.golang.org/lookup/github.com/!o-sura/custom-policy-hub/logger@v1.0.0: 404 Not Found\n\tserver response:\n\tnot found: github.com/O-sura/custom-policy-hub/logger@v1.0.0: invalid version: git ls-remote -q --end-of-options https://github.com/O-sura/custom-policy-hub in /tmp/gopath/pkg/mod/cache/vcs/35ff8b: exit status 128:\n\t\tfatal: could not read Username for 'https://github.com': terminal prompts disabled\n\tConfirm the import path was entered correctly."}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "hint: the module was checked against the public checksum database")
	assert.NotContains(t, err.Error(), "hint: the module host asked for credentials")
}

func TestModDownloadFailure_ChecksumMismatchIsNeverToldToBypass(t *testing.T) {
	// Go 1.26.10's wording (cmd/go/internal/modfetch/fetch.go): a mismatch against the
	// checksum database, and a mismatch against go.sum.
	cases := map[string]string{
		"checksum database": "github.com/acme/p@v1.0.0: verifying module: checksum mismatch\n\tdownloaded: h1:AAAA=\n\tsum.golang.org: h1:BBBB=\n\nSECURITY ERROR\nThis download does NOT match the one reported by the checksum server.\nThe bits may have been replaced on the origin server, or an attacker may\nhave intercepted the download attempt.\n\nFor more information, see 'go help module-auth'.",
		"go.sum":            "verifying github.com/acme/p@v1.0.0: checksum mismatch\n\tdownloaded: h1:AAAA=\n\tgo.sum:     h1:BBBB=\n\nSECURITY ERROR\nThis download does NOT match an earlier download recorded in go.sum.",
	}
	for name, goErr := range cases {
		t.Run(name, func(t *testing.T) {
			stdout, _ := json.Marshal(map[string]string{"Error": goErr})

			err := modDownloadFailure(module, stdout, nil, errExit)

			assert.Contains(t, err.Error(), "hint: the downloaded module does not match the checksum recorded")
			assert.Contains(t, err.Error(), "Do not set GOPRIVATE, GONOSUMDB or GOSUMDB=off")
			assert.NotContains(t, err.Error(), "Set GOPRIVATE (or GONOSUMDB)")
		})
	}
}

func TestModDownloadFailure_UnreachableChecksumDatabaseIsNotTreatedAsPrivate(t *testing.T) {
	stdout, _ := json.Marshal(map[string]string{"Error": `github.com/acme/p@v1.0.0: verifying module: github.com/acme/p@v1.0.0: Get "https://sum.golang.org/lookup/github.com/acme/p@v1.0.0": dial tcp: lookup sum.golang.org: no such host`})

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "hint: the builder could not reach the checksum database")
	assert.NotContains(t, err.Error(), "Set GOPRIVATE (or GONOSUMDB)")
}

func TestModDownloadFailure_RejectedTokenHint(t *testing.T) {
	// Captured from a real build with a wrong token: git reports it like a missing one.
	stdout := []byte(`{"Error":"github.com/O-sura/custom-policy-hub/logger@v1.0.0: invalid version: git ls-remote -q --end-of-options https://github.com/O-sura/custom-policy-hub in /go/pkg/mod/cache/vcs/35ff8b: exit status 128:\n\tfatal: could not read Username for 'https://github.com': terminal prompts disabled"}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "rejected the ones given")
	assert.Contains(t, err.Error(), "check that the token is valid, can read the repository")
}

func TestModDownloadFailure_NoProtocolHint(t *testing.T) {
	// What Go reports for a ".git"-qualified private repository when authentication fails.
	stdout := []byte(`{"Path":"git.acme.com/policies.git","Error":"git.acme.com/policies.git@v1.0.0: no protocol found for repository"}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "hint: the go command could not reach the repository with any protocol")
}

func TestModDownloadFailure_RedactsCredentials(t *testing.T) {
	stdout := []byte(`{"Error":"reading https://bot:s3cret@proxy.example.com/github.com/acme/p/@v/v1.0.0.info: 403 Forbidden"}`)
	stderr := []byte("go: proxy https://bot:s3cret@proxy.example.com unreachable")

	err := modDownloadFailure(module, stdout, stderr, errExit)

	assert.NotContains(t, err.Error(), "s3cret")
	assert.Contains(t, err.Error(), "https://<redacted-credentials>@proxy.example.com")
	assert.Contains(t, err.Error(), "unreachable", "stderr should still be reported")
}

func TestModDownloadFailure_KeepsPercentSigns(t *testing.T) {
	stdout := []byte(`{"Error":"reading https://proxy.example.com/github.com/%21acme/p/@v/list: 410 Gone"}`)

	err := modDownloadFailure(module, stdout, nil, errExit)

	assert.Contains(t, err.Error(), "github.com/%21acme/p")
	assert.NotContains(t, err.Error(), "%!")
}

func TestModDownloadFailure_NoOutputFallsBackToExitError(t *testing.T) {
	err := modDownloadFailure(module, []byte("not json"), nil, errExit)

	assert.ErrorIs(t, err, errExit)
	assert.Equal(t, "failed to run 'go mod download -json "+module+"': exit status 1", err.Error())
}

func TestModuleAccessHint_NoHintForUnrelatedFailure(t *testing.T) {
	assert.Empty(t, moduleAccessHint("github.com/acme/p@v9.9.9: invalid version: unknown revision v9.9.9"))
}
