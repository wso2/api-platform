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

// This file holds the rules that keep remote policy references reproducible:
// every remote policy in a build file must name one immutable artifact, and the
// builder must be able to prove that the artifact it fetched is that artifact.
//
//   - gomodule entries need an exact module version (a full semantic version
//     tag or a pseudo-version) and a `checksum:` holding the module's go.sum
//     "h1:" hash. The hash is compared with the Sum reported by
//     'go mod download -json' and the build fails on any difference.
//   - pipPackage entries that point at a git repository need a full commit
//     SHA as the git ref. Git object names are content hashes, so the commit
//     SHA pins the exact source tree; a tag or branch can be moved.
//   - pipPackage entries that point at a package index need an exact '=='
//     version and a `checksum:` holding the sha256 of the wheel. pip verifies
//     it with --require-hashes.

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/goenv"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const (
	goChecksumPrefix  = "h1:"
	pipChecksumPrefix = "sha256:"
)

// gitCommitSHAPattern matches a full SHA-1 (40 hex) or SHA-256 (64 hex) git
// object name. Abbreviated SHAs are rejected: they are not guaranteed to stay
// unambiguous as the repository grows.
var gitCommitSHAPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// IsGitCommitSHA reports whether ref is a full git commit SHA.
func IsGitCommitSHA(ref string) bool {
	return gitCommitSHAPattern.MatchString(ref)
}

// ParsePinnedGoModule splits a gomodule reference of the form
// "<module path>@<version>" and checks that the version names exactly one
// module version: a canonical semantic version such as v1.4.0, or a
// pseudo-version such as v0.0.0-20260101000000-0123456789ab. Major-only or
// minor-only versions (v1, v1.4), version queries (latest, upgrade, >=v1.0.0)
// and branch names are rejected because they resolve to whatever is newest
// when the build runs.
func ParsePinnedGoModule(gomodule string) (path, version string, err error) {
	gomodule = strings.TrimSpace(gomodule)
	at := strings.LastIndex(gomodule, "@")
	if at <= 0 || at == len(gomodule)-1 {
		return "", "", fmt.Errorf(
			"gomodule %q must include an exact version, e.g. <module>@v1.2.3", gomodule)
	}
	path, version = gomodule[:at], gomodule[at+1:]

	if !isExactGoVersion(version) {
		return "", "", fmt.Errorf(
			"gomodule %q does not pin an exact version: %q is a version query that can resolve to a "+
				"different release over time; use a full version such as v1.2.3 or a pseudo-version",
			gomodule, version)
	}

	if err := module.Check(path, version); err != nil {
		return "", "", fmt.Errorf("gomodule %q is not a valid module version: %w", gomodule, err)
	}

	return path, version, nil
}

// isExactGoVersion reports whether v is a canonical semantic version (which
// includes pseudo-versions and +incompatible versions). semver.Canonical drops
// build metadata, so "+incompatible" is removed before comparing.
func isExactGoVersion(v string) bool {
	if !semver.IsValid(v) {
		return false
	}
	return semver.Canonical(v) == strings.TrimSuffix(v, "+incompatible")
}

// ValidateGoChecksum checks that sum has the go.sum "h1:" form: the prefix
// followed by the base64 encoding of a 32-byte SHA-256 digest.
func ValidateGoChecksum(sum string) error {
	if !strings.HasPrefix(sum, goChecksumPrefix) {
		return fmt.Errorf("checksum %q must start with %q (the module hash recorded in go.sum)", sum, goChecksumPrefix)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sum, goChecksumPrefix))
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("checksum %q is not a valid h1: hash (expected base64 of a 32-byte SHA-256 digest)", sum)
	}
	return nil
}

// ValidatePipChecksum checks that sum has the form "sha256:<64 lowercase hex>".
func ValidatePipChecksum(sum string) error {
	if !strings.HasPrefix(sum, pipChecksumPrefix) {
		return fmt.Errorf("checksum %q must start with %q (the sha256 of the wheel file)", sum, pipChecksumPrefix)
	}
	digest := strings.TrimPrefix(sum, pipChecksumPrefix)
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return fmt.Errorf("checksum %q must be sha256: followed by 64 lowercase hex characters", sum)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("checksum %q is not valid hex: %w", sum, err)
	}
	return nil
}

// VerifyGoModuleSum compares the checksum declared in the build file with the
// Sum that 'go mod download -json' reported for the downloaded module.
func VerifyGoModuleSum(gomodule, expected, actual string) error {
	if actual == "" {
		return fmt.Errorf("'go mod download' did not report a checksum for %s; refusing to use an unverified module", gomodule)
	}
	if expected != actual {
		return fmt.Errorf(
			"checksum mismatch for %s: build file declares %s but the downloaded module hashes to %s; "+
				"the module content differs from the pinned release",
			gomodule, expected, actual)
	}
	return nil
}

// validatePolicyPin applies the pinning rules to one build file entry.
func validatePolicyPin(index int, entry types.BuildEntry) error {
	prefix := fmt.Sprintf("policy entry %d (%s)", index, entry.Name)

	switch {
	case entry.Gomodule != "":
		if _, _, err := ParsePinnedGoModule(entry.Gomodule); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		if entry.Checksum == "" {
			return fmt.Errorf(
				"%s: checksum is required for gomodule entries; set it to the module's h1: hash "+
					"(the Sum field of 'go mod download -json %s')", prefix, entry.Gomodule)
		}
		if err := ValidateGoChecksum(entry.Checksum); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}

	case entry.PipPackage != "":
		return validatePipPin(prefix, entry)

	default:
		if entry.Checksum != "" {
			return fmt.Errorf("%s: checksum is only used with gomodule and indexed pipPackage entries", prefix)
		}
	}
	return nil
}

func validatePipPin(prefix string, entry types.BuildEntry) error {
	spec := strings.TrimSpace(entry.PipPackage)

	// Git references: direct "git+..." specs and the short
	// "<host>/<org>/<repo>/<subdir>@<ref>" form.
	if isDirectPipSpec(spec) || (!strings.Contains(spec, "==") && !strings.Contains(spec, "~=") && strings.Contains(spec, "@")) {
		ref, err := pipGitRef(spec)
		if err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		if !IsGitCommitSHA(ref) {
			return fmt.Errorf(
				"%s: pipPackage %q must pin a full git commit SHA (40 hex characters), got ref %q; "+
					"tags and branches can be moved to different code",
				prefix, SanitizePipSpec(spec), ref)
		}
		if entry.Checksum != "" {
			return fmt.Errorf(
				"%s: checksum is not used for git references; the commit SHA in pipPackage already pins the content", prefix)
		}
		return nil
	}

	// Indexed packages: "<pkg>==<version>[@<index-url>]".
	ref, err := ParsePipPackageRef(spec)
	if err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	if ref.IsVersionRange {
		return fmt.Errorf(
			"%s: pipPackage %q uses a version range; use an exact version such as %s==1.2.3",
			prefix, SanitizePipSpec(spec), ref.PackageName)
	}
	if entry.Checksum == "" {
		return fmt.Errorf(
			"%s: checksum is required for indexed pipPackage entries; set it to sha256:<hex> of the wheel file", prefix)
	}
	if err := ValidatePipChecksum(entry.Checksum); err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return nil
}

// pipGitRef returns the git ref of a direct VCS spec or a short URL.
func pipGitRef(spec string) (string, error) {
	if isDirectPipSpec(spec) {
		vcs, err := parseVCSPipSpec(spec)
		if err != nil {
			return "", err
		}
		return vcs.GitRef, nil
	}
	at := strings.LastIndex(spec, "@")
	if at < 0 || at == len(spec)-1 {
		return "", fmt.Errorf("pipPackage %q must end with @<commit-sha>", spec)
	}
	return spec[at+1:], nil
}

// readGoEnv is a seam over 'go env -json' so tests can supply the effective Go
// environment. It returns the values of the requested variables, taking both
// the process environment and 'go env -w' settings into account.
var readGoEnv = func(keys ...string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", append([]string{"env", "-json"}, keys...)...)
	cmd.Env = goenv.WithToolchain(cmd.Environ())
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run 'go env -json': %w", err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(out, &values); err != nil {
		return nil, fmt.Errorf("failed to parse 'go env -json' output: %w", err)
	}
	return values, nil
}

// checkGoChecksumEnv refuses to fetch remote policy modules when the Go
// toolchain has been told to skip checksum verification. The checksum database
// is what verifies the policies' own dependencies, so it must stay on.
// Excluding the policy modules from the checksum database (GONOSUMDB or
// GOPRIVATE) is allowed, because their pinned h1: checksums are still verified,
// but it is logged as a warning.
func checkGoChecksumEnv(modulePaths []string) error {
	if len(modulePaths) == 0 {
		return nil
	}

	env, err := readGoEnv("GOSUMDB", "GOFLAGS", "GONOSUMDB", "GOPRIVATE")
	if err != nil {
		return err
	}

	if strings.TrimSpace(env["GOSUMDB"]) == "off" {
		return fmt.Errorf(
			"GOSUMDB=off disables Go checksum verification; remote policy modules and their " +
				"dependencies must be verified against the checksum database")
	}
	for _, flag := range strings.Fields(env["GOFLAGS"]) {
		if flag == "-insecure" || strings.HasPrefix(flag, "-insecure=") {
			return fmt.Errorf("GOFLAGS contains %s; remote policy modules must be fetched with checksum verification", flag)
		}
	}

	for _, key := range []string{"GONOSUMDB", "GOPRIVATE"} {
		patterns := env[key]
		if patterns == "" {
			continue
		}
		for _, p := range modulePaths {
			if module.MatchPrefixPatterns(patterns, p) {
				slog.Warn("Policy module is excluded from the Go checksum database; relying on the pinned checksum only",
					"module", p, "variable", key, "phase", "discovery")
			}
		}
	}
	return nil
}
