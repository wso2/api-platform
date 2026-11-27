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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
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
		{name: "sumdb name is a URL with credentials", flags: []string{"GOSUMDB=https://bot:s3cret@sum.acme.com"}, wantErr: "not a URL", mustNotIn: "s3cret"},
		{name: "sumdb name with userinfo", flags: []string{"GOSUMDB=bot:s3cret@sum.acme.com"}, wantErr: "not a URL", mustNotIn: "s3cret"},
		{name: "proxy query token", flags: []string{"GOPROXY=https://goproxy.acme.com?access_token=s3cret,direct"}, wantErr: "query string or fragment", mustNotIn: "s3cret"},
		{name: "proxy query token copied by bare --go-env", flags: []string{"GOPROXY"}, env: map[string]string{"GOPROXY": "https://goproxy.acme.com/?access_token=s3cret"}, wantErr: "query string or fragment", mustNotIn: "s3cret"},
		{name: "proxy fragment", flags: []string{"GOPROXY=https://goproxy.acme.com#s3cret"}, wantErr: "query string or fragment", mustNotIn: "s3cret"},
		{name: "sumdb URL query token", flags: []string{"GOSUMDB=sum.acme.com+k https://sum.acme.com?token=s3cret"}, wantErr: "query string or fragment", mustNotIn: "s3cret"},
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
		"GONOPROXY=none", // documented way to send GOPRIVATE modules through GOPROXY
	}
	if _, err := ResolveGoEnv(flags, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv() error = %v", err)
	}
	if _, err := ResolveGoEnv([]string{"GOSUMDB=off", "GOPROXY=off"}, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv(off) error = %v", err)
	}
	// Verifier keys are base64 and may contain "+", "/" and "=".
	if _, err := ResolveGoEnv([]string{"GOSUMDB=sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"}, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv(sumdb key) error = %v", err)
	}
	if _, err := ResolveGoEnv([]string{"GOSUMDB=sum.acme.com+abc/d= https://sum.acme.com/"}, lookupFrom(nil)); err != nil {
		t.Fatalf("ResolveGoEnv(sumdb key with slash) error = %v", err)
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
	args, env, cleanup, err := BuilderAccess{}.prepare(t.TempDir(), []string{"PATH=/bin"}, bindMount(false))
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

	args, env, cleanup, err := access.prepare(t.TempDir(), base, bindMount(false))
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

// writeTestNetrc writes a 0600 netrc file and returns its path.
func writeTestNetrc(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "netrc")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// mountSource returns the src= field of a --mount argument.
func mountSource(t *testing.T, args []string) string {
	t.Helper()
	if len(args) != 2 || args[0] != "--mount" {
		t.Fatalf("args = %v, want a single --mount", args)
	}
	// Docker parses --mount as one CSV record.
	fields, err := csv.NewReader(strings.NewReader(args[1])).Read()
	if err != nil {
		t.Fatalf("mount spec %q is not valid CSV: %v", args[1], err)
	}
	return strings.TrimPrefix(fields[1], "src=")
}

// The --netrc file is never mounted itself: a private copy is, so podman and
// buildah relabel only the copy, and the copy is deleted after the build.
func TestPrepare_NetrcFileIsCopiedAndTheCopyMounted(t *testing.T) {
	const content = "machine git.acme.com\nlogin ci\npassword secret\n"
	original := writeTestNetrc(t, t.TempDir(), content)
	scratch := filepath.Join(t.TempDir(), "with,comma")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}

	args, _, cleanup, err := BuilderAccess{NetrcFile: original}.prepare(scratch, nil, bindMount(false))
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	copied := mountSource(t, args)
	fields, _ := csv.NewReader(strings.NewReader(args[1])).Read()
	if want := []string{"type=bind", "src=" + copied, "dst=/root/.netrc", "readonly"}; !slices.Equal(fields, want) {
		t.Fatalf("mount fields = %v, want %v", fields, want)
	}
	if copied == original || filepath.Dir(filepath.Dir(copied)) != scratch ||
		!strings.HasPrefix(filepath.Base(filepath.Dir(copied)), credentialDirPrefix) {
		t.Fatalf("mounted %s, want a copy in a new credentials folder under %s", copied, scratch)
	}
	if data, err := os.ReadFile(copied); err != nil || string(data) != content {
		t.Fatalf("copy = %q, %v; want the --netrc contents", data, err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(copied); info.Mode().Perm() != 0o600 {
			t.Fatalf("copy permissions = %o, want 600", info.Mode().Perm())
		}
	}

	cleanup()
	if _, err := os.Stat(filepath.Dir(copied)); !os.IsNotExist(err) {
		t.Fatalf("credentials folder still exists after cleanup: %v", err)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != content {
		t.Fatalf("the --netrc file changed: %q, %v", data, err)
	}
}

func TestPrepare_NetrcMountRelabel(t *testing.T) {
	original := writeTestNetrc(t, t.TempDir(), "machine h\nlogin u\npassword p\n")

	args, _, cleanup, err := BuilderAccess{NetrcFile: original}.prepare(t.TempDir(), nil, bindMount(true))
	defer cleanup()
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	copied := mountSource(t, args)
	want := []string{"--mount", "type=bind,src=" + copied + ",dst=/root/.netrc,readonly,relabel=private"}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestPrepare_NetrcFileErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	if _, _, _, err := (BuilderAccess{NetrcFile: missing}).prepare(t.TempDir(), nil, bindMount(false)); err == nil {
		t.Fatal("expected an error for a missing --netrc file")
	}

	large := writeTestNetrc(t, t.TempDir(), strings.Repeat("x", maxNetrcBytes+1))
	scratch := t.TempDir()
	_, _, _, err := BuilderAccess{NetrcFile: large}.prepare(scratch, nil, bindMount(false))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v, want a too-large error", err)
	}
	if entries, _ := os.ReadDir(scratch); len(entries) != 0 {
		t.Fatalf("a failed copy left %v behind", entries)
	}
}

func TestVolumeMount(t *testing.T) {
	// buildah: a comma in the path is fine with --volume, unlike a quoted --mount value.
	got, err := volumeMount(true)("/home/u/with,comma/netrc", "/root/.netrc")
	if err != nil {
		t.Fatalf("volumeMount() error = %v", err)
	}
	if want := []string{"--volume", "/home/u/with,comma/netrc:/root/.netrc:ro,Z"}; !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	got, err = volumeMount(false)("/home/u/netrc", "/root/.netrc")
	if err != nil {
		t.Fatalf("volumeMount() error = %v", err)
	}
	if want := []string{"--volume", "/home/u/netrc:/root/.netrc:ro"}; !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	if _, err := volumeMount(false)("/home/u/a:b/netrc", "/root/.netrc"); err == nil || !strings.Contains(err.Error(), "colon") {
		t.Fatalf("expected a colon error, got %v", err)
	}
	if _, err := volumeMount(false)("/home/u/a\nb", "/root/.netrc"); err == nil {
		t.Fatal("expected an error for a line break")
	}
}

func TestPrepare_GeneratedNetrcUsesEngineMount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildah does not run on Windows, and volumeMount rejects the drive colon in a Windows path")
	}
	access := BuilderAccess{GitCredentials: []GitCredential{{Host: "example.com", Username: "ci", token: "t"}}}
	args, _, cleanup, err := access.prepare(t.TempDir(), nil, volumeMount(true))
	defer cleanup()
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if len(args) != 2 || args[0] != "--volume" || !strings.HasSuffix(args[1], ":/root/.netrc:ro,Z") {
		t.Fatalf("args = %v, want a buildah --volume mount", args)
	}
}

func TestPrepare_GitCredentialsBecomeAPrivateNetrc(t *testing.T) {
	scratch := t.TempDir()
	access := BuilderAccess{GitCredentials: []GitCredential{
		{Host: "github.com", Username: "x-access-token", token: "github_pat_abc"},
		{Host: "gitlab.acme.com", Username: "ci-bot", token: "glpat-xyz"},
	}}

	args, _, cleanup, err := access.prepare(scratch, nil, bindMount(false))
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

	if want := fmt.Sprintf("builder-credentials-pid%d-", os.Getpid()); !strings.HasPrefix(filepath.Base(filepath.Dir(netrc)), want) {
		t.Fatalf("credentials folder %s should start with %s", filepath.Dir(netrc), want)
	}
	// A sweep by a concurrent build must keep this live build's folder.
	sweepStaleCredentialDirs(scratch, processAlive, time.Now())
	if _, err := os.Stat(netrc); err != nil {
		t.Fatalf("sweep removed a live build's credentials: %v", err)
	}

	cleanup()
	if _, err := os.Stat(filepath.Dir(netrc)); !os.IsNotExist(err) {
		t.Fatalf("credentials directory still exists after cleanup: %v", err)
	}
}

func TestSweepStaleCredentialDirs(t *testing.T) {
	parent := t.TempDir()
	now := time.Now()
	mkdir := func(name string, age time.Duration) string {
		t.Helper()
		dir := filepath.Join(parent, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "netrc-1"), []byte("machine h login u password p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		at := now.Add(-age)
		if err := os.Chtimes(dir, at, at); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	const livePID = 1111
	alive := func(pid int) bool { return pid == livePID }

	removed := []string{
		mkdir("builder-credentials-pid2222-a1", time.Minute),  // owner no longer running
		mkdir("builder-credentials-pid1111-b2", 25*time.Hour), // owner's ID reused long after
		mkdir("builder-credentials-123456", 25*time.Hour),     // older name format, abandoned
		mkdir("builder-credentials-pidjunk-c3", 25*time.Hour), // unparseable, abandoned
	}
	kept := []string{
		mkdir("builder-credentials-pid1111-d4", time.Minute), // a concurrent build
		mkdir("builder-credentials-654321", time.Minute),     // older name format, recent
		mkdir("gateway-image-build-42", 48*time.Hour),        // not a credentials folder
	}
	plainFile := filepath.Join(parent, "builder-credentials-pid2222-file")
	if err := os.WriteFile(plainFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	kept = append(kept, plainFile)

	sweepStaleCredentialDirs(parent, alive, now)

	for _, p := range removed {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed (err=%v)", filepath.Base(p), err)
		}
	}
	for _, p := range kept {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should have been kept: %v", filepath.Base(p), err)
		}
	}
}

func TestSweepStaleCredentialDirsNeverFollowsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}
	parent, outside := t.TempDir(), t.TempDir()
	precious := filepath.Join(outside, "keep-me")
	if err := os.WriteFile(precious, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(parent, "builder-credentials-pid2222-link")); err != nil {
		t.Fatal(err)
	}

	sweepStaleCredentialDirs(parent, func(int) bool { return false }, time.Now().Add(48*time.Hour))

	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("sweep followed a symlink and removed %s: %v", precious, err)
	}
}

func TestProcessAliveForThisProcess(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("processAlive(own pid) = false")
	}
}

func TestBuilderRunArgs(t *testing.T) {
	config := DockerBuildConfig{
		TempDir:                    "/home/u/.wso2ap/.tmp/gateway-image-build-1",
		GatewayBuilder:             "ghcr.io/wso2/api-platform/gateway-builder:1.2.0",
		GatewayControllerBaseImage: "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		GatewayRuntimeBaseImage:    "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
	}
	base := []string{
		"run", "--rm", "-v", "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace",
		"-e", "GOPRIVATE",
		"ghcr.io/wso2/api-platform/gateway-builder:1.2.0",
		"-gateway-controller-base-image", "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		"-gateway-runtime-base-image", "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
	}

	// Not set: no -fetch-timeout, so builder images that predate the flag keep working.
	if got := builderRunArgs(config, false, []string{"-e", "GOPRIVATE"}); !slices.Equal(got, base) {
		t.Fatalf("args = %v, want %v", got, base)
	}

	// Set: passed after the image, where the builder reads its own flags.
	config.FetchTimeout = 10 * time.Minute
	want := append(slices.Clone(base), "-fetch-timeout", "10m0s")
	if got := builderRunArgs(config, false, []string{"-e", "GOPRIVATE"}); !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}
