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

// The gateway-builder container fetches gomodule policies with the go command,
// so a policy in a private repository needs Go module settings (GOPRIVATE, ...)
// and credentials for its host inside that container. This file decides which
// settings may be passed and turns the credential flags into a read-only netrc
// mount. Nothing here is written into the build workspace, whose output can be
// copied into the gateway images.

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// netrcMountTarget is where git and the go command look for credentials in the
// builder image, which runs as root.
const netrcMountTarget = "/root/.netrc"

// Sources reported for an effective Go setting.
const (
	GoEnvSourceFlag        = "--go-env"
	GoEnvSourceEnvironment = "environment"
)

const maxGoEnvValueLen = 4096

type goEnvRule struct {
	autoForward bool // copied from the caller's environment without a flag
	mask        bool // value may carry a token, so it is never printed
	validate    func(key, value string) error
}

// allowedGoEnv is the complete list of variables that may reach the builder. It is
// an allowlist on purpose: Go adds variables every release, and several existing
// ones (GOFLAGS, GOENV, GOCACHEPROG, ...) can run commands or override the
// builder's own GOOS/GOARCH/GOTOOLCHAIN handling.
var allowedGoEnv = map[string]goEnvRule{
	"GOPRIVATE":  {autoForward: true, validate: validatePatternList},
	"GONOPROXY":  {autoForward: true, validate: validatePatternList},
	"GONOSUMDB":  {autoForward: true, validate: validatePatternList},
	"GOPROXY":    {validate: validateProxyList},
	"GOSUMDB":    {validate: validateSumDB},
	"GOVCS":      {validate: validatePlain},
	"GOINSECURE": {validate: validatePatternList},
	"GOAUTH":     {mask: true, validate: validatePlain},
}

// SupportedGoEnvKeys returns the allowlisted variable names in a stable order.
func SupportedGoEnvKeys() []string {
	keys := make([]string, 0, len(allowedGoEnv))
	for k := range allowedGoEnv {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// GoEnvSetting is one Go setting passed to the builder and where it came from.
type GoEnvSetting struct {
	Key    string
	Value  string
	Source string
	masked bool
}

// DisplayValue returns the value to show the user; masked settings print as "(set)".
func (s GoEnvSetting) DisplayValue() string {
	if s.masked {
		return "(set)"
	}
	return s.Value
}

// ResolveGoEnv combines --go-env values (KEY=VALUE, or KEY to copy it from the
// caller's environment) with the auto-forwarded variables. --go-env wins, and an
// explicit empty value keeps an auto-forwarded variable out of the builder.
func ResolveGoEnv(flagValues []string, lookup func(string) (string, bool)) ([]GoEnvSetting, error) {
	explicit := make(map[string]string, len(flagValues))
	for _, fv := range flagValues {
		key, value, hasValue := strings.Cut(fv, "=")
		rule, ok := allowedGoEnv[key]
		if !ok {
			return nil, unsupportedGoEnvError(key)
		}
		if _, dup := explicit[key]; dup {
			return nil, fmt.Errorf("--go-env %s is given more than once", key)
		}
		if !hasValue {
			v, ok := lookup(key)
			if !ok || v == "" {
				return nil, fmt.Errorf("--go-env %s: %s is not set in your environment; pass --go-env %s=<value> instead", key, key, key)
			}
			value = v
		}
		if value != "" {
			if err := rule.validate(key, value); err != nil {
				return nil, fmt.Errorf("--go-env %s: %w", key, err)
			}
		}
		explicit[key] = value
	}

	var settings []GoEnvSetting
	for _, key := range SupportedGoEnvKeys() {
		rule := allowedGoEnv[key]
		if value, ok := explicit[key]; ok {
			if value != "" {
				settings = append(settings, GoEnvSetting{Key: key, Value: value, Source: GoEnvSourceFlag, masked: rule.mask})
			}
			continue
		}
		if !rule.autoForward {
			continue
		}
		if value, ok := lookup(key); ok && value != "" {
			if err := rule.validate(key, value); err != nil {
				return nil, fmt.Errorf("%s from your environment: %w; fix it or override it with --go-env %s=<value>", key, err, key)
			}
			settings = append(settings, GoEnvSetting{Key: key, Value: value, Source: GoEnvSourceEnvironment, masked: rule.mask})
		}
	}
	return settings, nil
}

// unsupportedGoEnvError names the key only when it looks like a Go setting, so a
// token passed to the wrong flag is not echoed into terminal or CI logs.
func unsupportedGoEnvError(key string) error {
	supported := strings.Join(SupportedGoEnvKeys(), ", ")
	if strings.HasPrefix(key, "GO") && envVarName.MatchString(key) {
		return fmt.Errorf("--go-env %s is not supported; supported variables: %s", key, supported)
	}
	return fmt.Errorf("--go-env accepts only Go module settings: %s", supported)
}

func validatePlain(key, value string) error {
	if len(value) > maxGoEnvValueLen {
		return fmt.Errorf("value is longer than %d characters", maxGoEnvValueLen)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("value contains a control character")
		}
	}
	return nil
}

func validatePatternList(key, value string) error {
	if err := validatePlain(key, value); err != nil {
		return err
	}
	if strings.Contains(value, "://") {
		return fmt.Errorf("takes module path patterns such as github.com/acme/*, not URLs")
	}
	return nil
}

func validateProxyList(key, value string) error {
	if err := validatePlain(key, value); err != nil {
		return err
	}
	for _, entry := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' }) {
		entry = strings.TrimSpace(entry)
		if entry == "direct" || entry == "off" {
			continue
		}
		if err := validateServiceURL(entry); err != nil {
			return err
		}
	}
	return nil
}

func validateSumDB(key, value string) error {
	if err := validatePlain(key, value); err != nil {
		return err
	}
	fields := strings.Fields(value)
	if len(fields) == 0 || len(fields) > 2 {
		return fmt.Errorf("expected \"off\", a database name, or a name followed by its URL")
	}
	// The first field is a name or name+key, never a URL. Refusing URL shapes keeps a
	// credential out of the printed settings.
	if strings.Contains(fields[0], "://") || strings.ContainsAny(fields[0], "@?#") {
		return fmt.Errorf("the database name must be a name or name+key, not a URL; put credentials in a --netrc file")
	}
	if len(fields) == 2 {
		return validateServiceURL(fields[1])
	}
	return nil
}

// validateServiceURL accepts an http(s) URL with no credentials in its userinfo,
// query string or fragment. Credentials belong in the netrc file, which is not
// printed or logged.
func validateServiceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("expected an http(s) URL, \"direct\" or \"off\"")
	}
	if u.User != nil {
		return fmt.Errorf("URLs must not contain credentials; put them in a --netrc file or use --git-credential")
	}
	if strings.ContainsAny(raw, "?#") {
		return fmt.Errorf("URLs must not contain a query string or fragment; put credentials in a --netrc file or use --git-credential")
	}
	return nil
}

var (
	envVarName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	netrcHost   = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)
	netrcSafeRe = regexp.MustCompile(`^[^\s"]+$`)
)

// GitCredential is a username and token for one host, from --git-credential.
type GitCredential struct {
	Host     string
	Username string
	token    string
}

// ParseGitCredentials parses --git-credential values of the form
// <host>=<username>:<ENV_VAR>. The token is read from ENV_VAR so it never appears
// on the command line. Errors never repeat the value after "=", which may be a
// token pasted in by mistake.
func ParseGitCredentials(specs []string, lookup func(string) (string, bool)) ([]GitCredential, error) {
	const format = "expected <host>=<username>:<ENV_VAR>, where ENV_VAR names an environment variable holding the token"
	creds := make([]GitCredential, 0, len(specs))
	seen := make(map[string]bool, len(specs))
	for i, spec := range specs {
		hostPart, rest, ok := strings.Cut(spec, "=")
		host := strings.ToLower(strings.TrimSpace(hostPart))
		if !ok || host == "" {
			return nil, fmt.Errorf("--git-credential #%d: %s", i+1, format)
		}
		if strings.Contains(host, "://") || strings.Contains(host, "/") {
			return nil, fmt.Errorf("--git-credential #%d: use the host name only (for example github.com), not a URL", i+1)
		}
		if strings.Contains(host, ":") {
			return nil, fmt.Errorf("--git-credential #%d: use the host name without a port", i+1)
		}
		if !netrcHost.MatchString(host) {
			return nil, fmt.Errorf("--git-credential #%d: %q is not a valid host name", i+1, host)
		}
		if seen[host] {
			return nil, fmt.Errorf("--git-credential for %s is given more than once", host)
		}
		seen[host] = true

		username, envName, ok := strings.Cut(rest, ":")
		if !ok || username == "" || !envVarName.MatchString(envName) {
			return nil, fmt.Errorf("--git-credential for %s: %s", host, format)
		}
		if !netrcSafeRe.MatchString(username) {
			return nil, fmt.Errorf("--git-credential for %s: the username must not contain spaces or quotes", host)
		}
		token, ok := lookup(envName)
		if !ok || token == "" {
			return nil, fmt.Errorf("--git-credential for %s: the environment variable named for the token is not set or is empty", host)
		}
		if !netrcSafeRe.MatchString(token) {
			return nil, fmt.Errorf("--git-credential for %s: the token must not contain spaces or quotes", host)
		}
		creds = append(creds, GitCredential{Host: host, Username: username, token: token})
	}
	return creds, nil
}

// ValidateNetrcFile checks a --netrc path and returns it as an absolute path. A
// netrc that other users can read is refused, the way ssh refuses an open key.
func ValidateNetrcFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("--netrc: failed to resolve %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("--netrc: file not found: %s", abs)
		}
		return "", fmt.Errorf("--netrc: cannot access %s: %w", abs, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("--netrc: %s is not a regular file", abs)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("--netrc: %s is empty", abs)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("--netrc: %s can be accessed by other users; restrict it with: chmod 600 %s", abs, abs)
	}
	return abs, nil
}

// BuilderAccess holds the Go settings and credentials for the gateway-builder container.
type BuilderAccess struct {
	GoEnv          []GoEnvSetting
	NetrcFile      string          // absolute path from --netrc
	GitCredentials []GitCredential // from --git-credential
}

// HasCredentials reports whether any credentials will be mounted into the builder.
func (a BuilderAccess) HasCredentials() bool {
	return a.NetrcFile != "" || len(a.GitCredentials) > 0
}

// HasGoEnv reports whether the named setting will be passed to the builder.
func (a BuilderAccess) HasGoEnv(key string) bool {
	return slices.ContainsFunc(a.GoEnv, func(s GoEnvSetting) bool { return s.Key == key })
}

// GoEnvValue returns the value of a setting passed to the builder, or "".
func (a BuilderAccess) GoEnvValue(key string) string {
	for _, s := range a.GoEnv {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// prepare returns the extra `docker run` arguments, the environment for the docker
// process (nil to inherit), and a cleanup func that is always safe to call.
//
// Settings are passed as name-only -e flags with their values in the docker
// process's environment, so values never appear on the command line or in the
// build log. --git-credential tokens are written to a netrc in a new private
// directory under scratchParent, which must not be the build workspace.
func (a BuilderAccess) prepare(scratchParent string, baseEnv []string) (args []string, env []string, cleanup func(), err error) {
	cleanup = func() {}

	if len(a.GoEnv) > 0 {
		overrides := make(map[string]string, len(a.GoEnv))
		for _, s := range a.GoEnv {
			args = append(args, "-e", s.Key)
			overrides[s.Key] = s.Value
		}
		env = mergeEnv(baseEnv, overrides)
	}

	netrc := a.NetrcFile
	if len(a.GitCredentials) > 0 {
		// The owner's process ID in the name lets a later run tell a live build's folder
		// from a leftover one (sweepStaleCredentialDirs).
		dir, err := os.MkdirTemp(scratchParent, fmt.Sprintf("%spid%d-*", credentialDirPrefix, os.Getpid())) // created 0700
		if err != nil {
			return nil, nil, cleanup, fmt.Errorf("failed to create a private directory for builder credentials: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(dir) }
		netrc, err = writeNetrc(dir, a.GitCredentials)
		if err != nil {
			cleanup()
			return nil, nil, func() {}, err
		}
	}
	if netrc != "" {
		mount, err := bindMountSpec(netrc, netrcMountTarget)
		if err != nil {
			cleanup()
			return nil, nil, func() {}, err
		}
		args = append(args, "--mount", mount)
	}
	return args, env, cleanup, nil
}

const (
	credentialDirPrefix = "builder-credentials-"
	// staleCredentialDirAge is how old a credentials folder must be before it is removed
	// even though its owner looks alive (process ID reuse) or cannot be identified.
	staleCredentialDirAge = 24 * time.Hour
)

// sweepStaleCredentialDirs removes credentials folders left in parent by ap runs that
// ended before they could delete them (a hard kill, a crash). A folder whose owning
// process is still running belongs to a concurrent build and is kept.
func sweepStaleCredentialDirs(parent string, alive func(pid int) bool, now time.Time) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry.Name(), credentialDirPrefix)
		if !ok || !entry.IsDir() { // IsDir is false for symlinks, which are never followed
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		old := now.Sub(info.ModTime()) > staleCredentialDirAge
		pid := 0
		if digits, ok := strings.CutPrefix(rest, "pid"); ok {
			digits, _, _ = strings.Cut(digits, "-")
			pid, _ = strconv.Atoi(digits)
		}
		if old || (pid > 0 && !alive(pid)) {
			_ = os.RemoveAll(filepath.Join(parent, entry.Name()))
		}
	}
}

// writeNetrc writes the credentials to a new 0600 file in dir and returns its path.
func writeNetrc(dir string, creds []GitCredential) (string, error) {
	f, err := os.CreateTemp(dir, "netrc-*") // created 0600
	if err != nil {
		return "", fmt.Errorf("failed to create the builder credentials file: %w", err)
	}
	var b strings.Builder
	for _, c := range creds {
		fmt.Fprintf(&b, "machine %s\nlogin %s\npassword %s\n\n", c.Host, c.Username, c.token)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return "", fmt.Errorf("failed to write the builder credentials file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to write the builder credentials file: %w", err)
	}
	return f.Name(), nil
}

// bindMountSpec builds a read-only --mount value. --mount (unlike -v) fails when
// the source is missing instead of creating an empty directory there, and its
// value is CSV, so the source path is quoted when it contains a comma or quote.
func bindMountSpec(src, dst string) (string, error) {
	if strings.ContainsAny(src, "\r\n") {
		return "", fmt.Errorf("the credentials file path must not contain a line break")
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"type=bind", "src=" + src, "dst=" + dst, "readonly"}); err != nil {
		return "", err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// mergeEnv returns base with every key in overrides set to its value, replacing
// existing entries rather than appending duplicates.
func mergeEnv(base []string, overrides map[string]string) []string {
	env := make([]string, 0, len(base)+len(overrides))
	for _, e := range base {
		key, _, _ := strings.Cut(e, "=")
		if _, replaced := overrides[key]; replaced {
			continue
		}
		env = append(env, e)
	}
	for _, key := range SupportedGoEnvKeys() {
		if v, ok := overrides[key]; ok {
			env = append(env, key+"="+v)
		}
	}
	return env
}
