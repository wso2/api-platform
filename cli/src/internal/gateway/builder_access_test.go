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

import (
	"encoding/csv"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func settingsMap(settings []GoEnvSetting) map[string]GoEnvSetting {
	m := make(map[string]GoEnvSetting, len(settings))
	for _, s := range settings {
		m[s.Key] = s
	}
	return m
}

func TestResolveGoEnv_AutoForwardsOnlyPrivatePatternVariables(t *testing.T) {
	env := map[string]string{
		"GOPRIVATE": "github.com/acme/*",
		"GONOPROXY": "gitlab.acme.com",
		"GONOSUMDB": "github.com/acme/*,gitlab.acme.com",
		"GOPROXY":   "http://localhost:3000", // host-only value, must not leak in
		"GOFLAGS":   "-mod=vendor",
		"GOAUTH":    "netrc",
	}

	settings, err := ResolveGoEnv(nil, lookupFrom(env))
	if err != nil {
		t.Fatalf("ResolveGoEnv() error = %v", err)
	}

	got := settingsMap(settings)
	for _, key := range []string{"GOPRIVATE", "GONOPROXY", "GONOSUMDB"} {
		s, ok := got[key]
		if !ok || s.Value != env[key] || s.Source != GoEnvSourceEnvironment {
			t.Errorf("%s = %+v, want value %q from environment", key, s, env[key])
		}
	}
	for _, key := range []string{"GOPROXY", "GOFLAGS", "GOAUTH"} {
		if _, ok := got[key]; ok {
			t.Errorf("%s was forwarded without --go-env", key)
		}
	}
}

func TestResolveGoEnv_FlagValues(t *testing.T) {
	env := map[string]string{
		"GOPRIVATE": "github.com/from-env/*",
		"GOPROXY":   "https://goproxy.acme.com,direct",
		"GONOSUMDB": "github.com/from-env/*",
	}

	settings, err := ResolveGoEnv([]string{
		"GOPRIVATE=github.com/acme/*,gitlab.acme.com", // overrides the environment
		"GOPROXY",    // copied from the environment
		"GONOSUMDB=", // keeps the environment value out
		"GOAUTH=git /workspace",
	}, lookupFrom(env))
	if err != nil {
		t.Fatalf("ResolveGoEnv() error = %v", err)
	}

	got := settingsMap(settings)
	if s := got["GOPRIVATE"]; s.Value != "github.com/acme/*,gitlab.acme.com" || s.Source != GoEnvSourceFlag {
		t.Errorf("GOPRIVATE = %+v, want the --go-env value", s)
	}
	if s := got["GOPROXY"]; s.Value != env["GOPROXY"] || s.Source != GoEnvSourceFlag {
		t.Errorf("GOPROXY = %+v, want the environment value via --go-env", s)
	}
	if _, ok := got["GONOSUMDB"]; ok {
		t.Errorf("GONOSUMDB was forwarded although --go-env GONOSUMDB= cleared it")
	}
	if s := got["GOAUTH"]; s.Value != "git /workspace" || s.DisplayValue() != "(set)" {
		t.Errorf("GOAUTH = %+v (display %q), want masked value", s, s.DisplayValue())
	}

	keys := make([]string, 0, len(settings))
	for _, s := range settings {
		keys = append(keys, s.Key)
	}
	if !slices.IsSorted(keys) {
		t.Errorf("settings are not in a stable order: %v", keys)
	}
}

func TestResolveGoEnv_Rejects(t *testing.T) {
	tests := []struct {
		name      string
		flags     []string
		env       map[string]string
		wantErr   string
		mustNotIn string
	}{
		{name: "not allowlisted", flags: []string{"GOFLAGS=-toolexec=/tmp/x"}, wantErr: "GOFLAGS is not supported"},
		{name: "builder-owned", flags: []string{"GOTOOLCHAIN=local"}, wantErr: "GOTOOLCHAIN is not supported"},
		{name: "non-Go name is not echoed", flags: []string{"ghp_SECRET123"}, wantErr: "accepts only Go module settings", mustNotIn: "ghp_SECRET123"},
		{name: "duplicate", flags: []string{"GOPRIVATE=a", "GOPRIVATE=b"}, wantErr: "more than once"},
		{name: "bare name not set", flags: []string{"GOPROXY"}, wantErr: "GOPROXY is not set in your environment"},
		{name: "proxy credentials", flags: []string{"GOPROXY=https://bot:s3cret@goproxy.acme.com,direct"}, wantErr: "must not contain credentials", mustNotIn: "s3cret"},
		{name: "proxy scheme", flags: []string{"GOPROXY=file:///srv/goproxy"}, wantErr: "expected an http(s) URL"},
		{name: "sumdb credentials", flags: []string{"GOSUMDB=sum.acme.com https://bot:s3cret@sum.acme.com"}, wantErr: "must not contain credentials", mustNotIn: "s3cret"},
		{name: "sumdb shape", flags: []string{"GOSUMDB=a b c"}, wantErr: "expected \"off\""},
		{name: "pattern given as URL", flags: []string{"GOPRIVATE=https://github.com/acme"}, wantErr: "not URLs"},
		{name: "control character", flags: []string{"GOVCS=private:git\nGOFLAGS=x"}, wantErr: "control character"},
		{name: "invalid value in environment", env: map[string]string{"GOPRIVATE": "https://github.com/acme"}, wantErr: "GOPRIVATE from your environment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveGoEnv(tt.flags, lookupFrom(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ResolveGoEnv() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if tt.mustNotIn != "" && strings.Contains(err.Error(), tt.mustNotIn) {
				t.Fatalf("error %q repeats %q", err, tt.mustNotIn)
			}
		})
	}
}

func TestResolveGoEnv_AcceptsValidValues(t *testing.T) {
	flags := []string{
		"GOPROXY=https://goproxy.acme.com|https://proxy.golang.org,direct",
		"GOSUMDB=sum.acme.com+abc123 https://sum.acme.com",
		"GOINSECURE=git.internal.acme.com",
		"GOVCS=private:git,public:git|hg",
	}
	if _, err := ResolveGoEnv(flags, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv() error = %v", err)
	}
	if _, err := ResolveGoEnv([]string{"GOSUMDB=off", "GOPROXY=off"}, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv(off) error = %v", err)
	}
}

func TestParseGitCredentials(t *testing.T) {
	env := map[string]string{"GH_TOKEN": "github_pat_abc", "GL_TOKEN": "glpat-xyz"}

	creds, err := ParseGitCredentials([]string{
		"GitHub.com=x-access-token:GH_TOKEN",
		"gitlab.acme.com=ci-bot:GL_TOKEN",
	}, lookupFrom(env))
	if err != nil {
		t.Fatalf("ParseGitCredentials() error = %v", err)
	}
	want := []GitCredential{
		{Host: "github.com", Username: "x-access-token", token: "github_pat_abc"},
		{Host: "gitlab.acme.com", Username: "ci-bot", token: "glpat-xyz"},
	}
	if !slices.Equal(creds, want) {
		t.Fatalf("ParseGitCredentials() = %+v, want %+v", creds, want)
	}
}

func TestParseGitCredentials_Rejects(t *testing.T) {
	env := map[string]string{"GH_TOKEN": "github_pat_abc", "SPACED": "a b", "EMPTY": ""}
	tests := []struct {
		name    string
		specs   []string
		wantErr string
	}{
		{name: "no equals sign", specs: []string{"github.com:x:GH_TOKEN"}, wantErr: "expected <host>=<username>:<ENV_VAR>"},
		{name: "no username", specs: []string{"github.com=GH_TOKEN"}, wantErr: "expected <host>=<username>:<ENV_VAR>"},
		{name: "URL instead of host", specs: []string{"https://github.com=x:GH_TOKEN"}, wantErr: "not a URL"},
		{name: "port", specs: []string{"git.acme.com:8443=x:GH_TOKEN"}, wantErr: "without a port"},
		{name: "duplicate host", specs: []string{"github.com=a:GH_TOKEN", "GITHUB.COM=b:GH_TOKEN"}, wantErr: "more than once"},
		{name: "unset variable", specs: []string{"github.com=x:NOT_SET"}, wantErr: "not set or is empty"},
		{name: "empty variable", specs: []string{"github.com=x:EMPTY"}, wantErr: "not set or is empty"},
		{name: "token with space", specs: []string{"github.com=x:SPACED"}, wantErr: "token must not contain spaces"},
		{name: "username with space", specs: []string{"github.com=a b:GH_TOKEN"}, wantErr: "username must not contain spaces"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGitCredentials(tt.specs, lookupFrom(env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseGitCredentials() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseGitCredentials_NeverEchoesWhatFollowsTheEqualsSign(t *testing.T) {
	// A token pasted in place of the variable name must not reach terminal or CI logs.
	for _, spec := range []string{
		"github.com=x-access-token:ghp_PastedToken123", // looks like a variable name, not set
		"github.com=x-access-token:glpat-Pasted-Token", // not a variable name
		"github.com=ghp_PastedToken123",                // no username
	} {
		_, err := ParseGitCredentials([]string{spec}, lookupFrom(nil))
		if err == nil {
			t.Fatalf("ParseGitCredentials(%q) succeeded", spec)
		}
		for _, secret := range []string{"ghp_PastedToken123", "glpat-Pasted-Token"} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error %q repeats the token", err)
			}
		}
	}
}

func TestValidateNetrcFile(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "netrc")
	if err := os.WriteFile(good, []byte("machine github.com login x password y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ValidateNetrcFile(good)
	if err != nil || got != good {
		t.Fatalf("ValidateNetrcFile(good) = %q, %v", got, err)
	}

	t.Chdir(dir)
	if got, err := ValidateNetrcFile("netrc"); err != nil || !filepath.IsAbs(got) {
		t.Fatalf("ValidateNetrcFile(relative) = %q, %v; want an absolute path", got, err)
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct{ path, want string }{
		"missing":   {filepath.Join(dir, "nope"), "file not found"},
		"directory": {dir, "not a regular file"},
		"empty":     {empty, "is empty"},
	} {
		if _, err := ValidateNetrcFile(tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to contain %q", name, err, tc.want)
		}
	}
}

func TestValidateNetrcFile_RefusesFilesOthersCanRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not checked on Windows")
	}
	open := filepath.Join(t.TempDir(), "netrc")
	if err := os.WriteFile(open, []byte("machine github.com login x password y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateNetrcFile(open); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("ValidateNetrcFile(0644) error = %v, want a chmod 600 hint", err)
	}
}

func TestPrepare_NothingConfigured(t *testing.T) {
	args, env, cleanup, err := BuilderAccess{}.prepare(t.TempDir(), []string{"PATH=/bin"})
	defer cleanup()
	if err != nil || len(args) != 0 || env != nil {
		t.Fatalf("prepare() = %v, %v, %v; want no args and an inherited environment", args, env, err)
	}
}

func TestPrepare_GoEnvTravelsByNameOnly(t *testing.T) {
	access := BuilderAccess{GoEnv: []GoEnvSetting{
		{Key: "GOPRIVATE", Value: "github.com/acme/*"},
		{Key: "GOPROXY", Value: "https://goproxy.acme.com,direct"},
	}}
	base := []string{"PATH=/bin", "GOPRIVATE=github.com/old/*"}

	args, env, cleanup, err := access.prepare(t.TempDir(), base)
	defer cleanup()
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}

	wantArgs := []string{"-e", "GOPRIVATE", "-e", "GOPROXY"}
	if !slices.Equal(args, wantArgs) {
		t.Fatalf("args = %v, want %v (values must not be on the command line)", args, wantArgs)
	}
	wantEnv := []string{"PATH=/bin", "GOPRIVATE=github.com/acme/*", "GOPROXY=https://goproxy.acme.com,direct"}
	if !slices.Equal(env, wantEnv) {
		t.Fatalf("env = %v, want %v", env, wantEnv)
	}
}

func TestPrepare_NetrcFileIsMountedReadOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "with,comma")
	netrc := filepath.Join(dir, "netrc")

	args, _, cleanup, err := BuilderAccess{NetrcFile: netrc}.prepare(t.TempDir(), nil)
	defer cleanup()
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if len(args) != 2 || args[0] != "--mount" {
		t.Fatalf("args = %v, want a single --mount", args)
	}

	// Docker parses --mount as one CSV record.
	fields, err := csv.NewReader(strings.NewReader(args[1])).Read()
	if err != nil {
		t.Fatalf("mount spec %q is not valid CSV: %v", args[1], err)
	}
	want := []string{"type=bind", "src=" + netrc, "dst=/root/.netrc", "readonly"}
	if !slices.Equal(fields, want) {
		t.Fatalf("mount fields = %v, want %v", fields, want)
	}
}

func TestPrepare_GitCredentialsBecomeAPrivateNetrc(t *testing.T) {
	scratch := t.TempDir()
	access := BuilderAccess{GitCredentials: []GitCredential{
		{Host: "github.com", Username: "x-access-token", token: "github_pat_abc"},
		{Host: "gitlab.acme.com", Username: "ci-bot", token: "glpat-xyz"},
	}}

	args, _, cleanup, err := access.prepare(scratch, nil)
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if strings.Contains(strings.Join(args, " "), "github_pat_abc") {
		t.Fatalf("token appears in docker arguments: %v", args)
	}

	fields, err := csv.NewReader(strings.NewReader(args[1])).Read()
	if err != nil {
		t.Fatal(err)
	}
	netrc := strings.TrimPrefix(fields[1], "src=")
	if filepath.Dir(filepath.Dir(netrc)) != scratch {
		t.Fatalf("netrc %s was not created in a new directory under %s", netrc, scratch)
	}

	data, err := os.ReadFile(netrc)
	if err != nil {
		t.Fatal(err)
	}
	wantContent := "machine github.com\nlogin x-access-token\npassword github_pat_abc\n\n" +
		"machine gitlab.acme.com\nlogin ci-bot\npassword glpat-xyz\n\n"
	if string(data) != wantContent {
		t.Fatalf("netrc content = %q, want %q", data, wantContent)
	}

	if runtime.GOOS != "windows" {
		fileInfo, _ := os.Stat(netrc)
		dirInfo, _ := os.Stat(filepath.Dir(netrc))
		if fileInfo.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("permissions file=%o dir=%o, want 600 and 700", fileInfo.Mode().Perm(), dirInfo.Mode().Perm())
		}
	}

	cleanup()
	if _, err := os.Stat(filepath.Dir(netrc)); !os.IsNotExist(err) {
		t.Fatalf("credentials directory still exists after cleanup: %v", err)
	}
}

func TestNetrcMountHint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh to produce exit codes")
	}
	exitWith := func(code string) error { return exec.Command("sh", "-c", "exit "+code).Run() }
	withNetrc := BuilderAccess{NetrcFile: "/elsewhere/netrc"}

	if hint := netrcMountHint(exitWith("125"), withNetrc); !strings.Contains(hint, "home directory") {
		t.Errorf("exit 125 with --netrc: hint = %q, want the shared-directory hint", hint)
	}
	if hint := netrcMountHint(exitWith("1"), withNetrc); hint != "" {
		t.Errorf("builder failure (exit 1): hint = %q, want none", hint)
	}
	if hint := netrcMountHint(exitWith("125"), BuilderAccess{}); hint != "" {
		t.Errorf("exit 125 without --netrc: hint = %q, want none", hint)
	}
	if hint := netrcMountHint(errors.New("not an exit error"), withNetrc); hint != "" {
		t.Errorf("non-exit error: hint = %q, want none", hint)
	}
}
