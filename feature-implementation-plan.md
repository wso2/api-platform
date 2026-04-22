# Minor-Only Versioning + Go-Style Short URL for Python Policies

## Overview

Two features:

1. **Minor-only version support**: `@v1.1` resolves to the latest `v1.1.x` patch — matching Go's `go mod download` behavior for both VCS and PyPI Python sources.
2. **Go-style short URL format**: Allow `pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1` in `build.yaml` — identical to `gomodule` format. The builder transparently expands this to a real pip VCS spec.

---

## Feature 1: Minor-Only Version Support

### Current State

Go supports three version granularities in `gomodule`:
- `@v1` → latest `v1.x.y` (major-only) ✅ Go works, ✅ Python works
- `@v1.1` → latest `v1.1.y` (minor-only) ✅ Go works, ❌ **Python doesn't support this**
- `@v1.0.3` → exact ✅ Go works, ✅ Python works

### Gaps to Fix

#### VCS path
- `majorOnlyRefPattern` (`v(\d+)$`) only matches `v1`, not `v1.1`
- `resolveVCSMajorVersion` only supports major-only; needs to also handle minor-only
- Need to also match `v1.1` (two components) and filter tags to `v1.1.*`

#### Indexed (PyPI) path
- `majorOnlyPackageVersionPattern` (`^\d+\.0$`) only matches `1.0`
- The `~=` operator in pip already handles minor-only natively: `~=1.1` means `>=1.1, ==1.*` which is NOT the same as "latest 1.1.x"
- For true minor-only (`pkg~=1.1.0` means `>=1.1.0, ==1.1.*`), we need `~=N.M.0` format

### Proposed Changes

#### Version reference classification

| Format | Example VCS ref | Example PyPI | Resolution |
|--------|----------------|--------------|------------|
| Major-only | `policies/foo/v1` | `pkg~=1.0` | Latest `v1.x.y` |
| Minor-only | `policies/foo/v1.1` | `pkg~=1.1.0` | Latest `v1.1.y` |
| Exact | `policies/foo/v1.0.3` | `pkg==1.0.3` | Exact `v1.0.3` |

#### VCS ref detection

Replace the single `majorOnlyRefPattern` with a generalized approach:

```
v1       → major-only   (1 component after v)
v1.1     → minor-only   (2 components after v)
v1.1.3   → exact         (3 components after v)
```

Regex patterns:
- Major-only: `v(\d+)$` — current, works ✓
- Minor-only: `v(\d+)\.(\d+)$` — **NEW** (matches `v1.1` but NOT `v1.1.3`)
- Exact: `v(\d+)\.(\d+)\.(\d+)` — anything with 3+ components

Detection logic: check exact first (has 3 segments), then minor-only (has 2 segments), then major-only (has 1 segment).

#### PyPI spec detection

| User writes | Format | Is range query? |
|-------------|--------|-----------------|
| `pkg~=1.0` | PEP 440 compatible release, `>=1.0, ==1.*` | Major-only ✓ |
| `pkg~=1.1.0` | PEP 440 compatible release, `>=1.1.0, ==1.1.*` | **Minor-only** ✓ (NEW) |
| `pkg==1.0.3` | Exact | Exact ✓ |

> [!NOTE]
> PEP 440 `~=` semantics: `~=X.Y` clips the **last** component. So `~=1.0` → `==1.*`, `~=1.1.0` → `==1.1.*`. This is exactly the behavior we want. pip handles all resolution natively.

---

## Feature 2: Go-Style Short URL Format

### Problem

Go policies in `build.yaml`:
```yaml
gomodule: github.com/wso2/gateway-controllers/policies/url-guardrail@v1
```

Python policies currently:
```yaml
pipPackage: "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor"
```

These are inconsistent. For WSO2 native Python policies hosted in the same repo (gateway-controllers), the format should be identical.

### Design

Support **both** formats in `pipPackage`:

#### Format 1: Go-style short URL (NEW)
```yaml
pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
```

The builder detects this is a short URL (not a `git+` VCS spec, not a `pkg==ver` indexed spec) and expands it to a full VCS pip spec using these rules:

**Expansion logic** — identical to Go's convention where the module path equals the subdirectory:

Given: `github.com/<org>/<repo>/policies/<name>@<version>`

1. **Host + repo**: `github.com/<org>/<repo>` → repo URL: `https://github.com/<org>/<repo>.git`
2. **Subdirectory**: `policies/<name>` (the path after the repo, before `@`)
3. **Git tag**: `policies/<name>/<version>` (subdirectory + version — matching Go's tag convention where the tag path = module subdirectory path)
4. **Full VCS spec**: `git+https://github.com/<org>/<repo>.git@policies/<name>/<version>#subdirectory=policies/<name>`

Example:
```
Input:    github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
Expands:  git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor
```

After expansion, the existing VCS flow (major-only/minor-only/exact detection + resolution) handles it.

#### Format 2: Full VCS URL (unchanged)
```yaml
pipPackage: "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor"
```

Customers who host policies on non-GitHub hosts, or use different URL patterns, can continue using full VCS specs.

#### Format 3: Indexed packages (unchanged)
```yaml
pipPackage: "prompt-compressor==1.0.3"
pipPackage: "prompt-compressor~=1.0"
pipPackage: "prompt-compressor~=1.1.0"
```

### Detection Logic

The builder classifies a `pipPackage` value as:

1. **Full VCS spec**: starts with `git+` (or contains ` @ git+`) → existing path
2. **Indexed package**: contains `==` or `~=` → existing path
3. **Go-style short URL**: contains `@` but doesn't match (1) or (2) → **NEW: expand first, then VCS path**

```go
func classifyPipPackage(spec string) string {
    if isDirectPipSpec(spec) { return "vcs" }              // git+...
    if strings.Contains(spec, "==") || strings.Contains(spec, "~=") { return "indexed" }
    if strings.Contains(spec, "@") { return "shorturl" }    // NEW
    return "unknown"
}
```

### Short URL Expansion

```go
// expandShortURL expands a Go-style module path to a full VCS pip spec.
// Input:  "github.com/wso2/gateway-controllers/policies/prompt-compressor@v1"
// Output: "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor"
func expandShortURL(spec string) (string, error) {
    // Split on '@' to get path and version
    atIdx := strings.LastIndex(spec, "@")
    if atIdx < 0 { return "", error }

    modulePath := spec[:atIdx]   // "github.com/wso2/gateway-controllers/policies/prompt-compressor"
    version := spec[atIdx+1:]    // "v1"

    // Split module path into host/org/repo and subdirectory
    // Convention: first 3 segments = host/org/repo, rest = subdirectory
    segments := strings.SplitN(modulePath, "/", 4)
    if len(segments) < 4 {
        return "", fmt.Errorf("short URL must have at least 4 path segments: host/org/repo/subdir")
    }

    host := segments[0]                           // "github.com"
    orgRepo := segments[1] + "/" + segments[2]     // "wso2/gateway-controllers"
    subdirectory := segments[3]                    // "policies/prompt-compressor"

    repoURL := fmt.Sprintf("https://%s/%s.git", host, orgRepo)
    gitRef := subdirectory + "/" + version          // "policies/prompt-compressor/v1"

    return fmt.Sprintf("git+%s@%s#subdirectory=%s", repoURL, gitRef, subdirectory), nil
}
```

> [!IMPORTANT]
> **Tag convention for Python policies**: Just like Go, where the Go module tag path equals the module's subdirectory path (e.g., `policies/url-guardrail/v1.0.3` tag for `policies/url-guardrail` module), Python policies in the same repo must follow the same tag naming: `policies/prompt-compressor/v1.0.0`. This is already what the user implements via GitHub Actions.

### build-manifest.yaml Behavior

The `build-manifest.yaml` stores the **resolved exact full VCS spec** (not the short URL), because the manifest must be self-contained and reproducible.

Input (`build.yaml`):
```yaml
- name: prompt-compressor
  pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
```

Output (`build-manifest.yaml`):
```yaml
- name: prompt-compressor
  version: v1.0.0
  pipPackage: git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1.0.0#subdirectory=policies/prompt-compressor
```

---

## Detailed Code Changes

### [python_module.go](file:///Users/sehan/Documents/GitHub/api-platform/gateway/gateway-builder/internal/discovery/python_module.go) — MODIFY

#### 1. Update regex patterns (line 39-42)

```diff
 var (
-	majorOnlyPackageVersionPattern = regexp.MustCompile(`^\d+\.0$`)
-	majorOnlyRefPattern            = regexp.MustCompile(`v(\d+)$`)
+	// Indexed package version patterns for ~= operator:
+	//   "1.0"   → major-only (latest 1.x.y)
+	//   "1.1.0" → minor-only (latest 1.1.y)
+	majorOnlyPkgVersionPattern = regexp.MustCompile(`^\d+\.0$`)
+	minorOnlyPkgVersionPattern = regexp.MustCompile(`^\d+\.\d+\.0$`)
+
+	// VCS ref patterns — applied to the version suffix after the last "v" in the ref:
+	//   "v1"     → major-only
+	//   "v1.1"   → minor-only
+	//   "v1.1.3" → exact
+	majorOnlyRefPattern = regexp.MustCompile(`v(\d+)$`)
+	minorOnlyRefPattern = regexp.MustCompile(`v(\d+)\.(\d+)$`)
+	exactRefPattern     = regexp.MustCompile(`v\d+\.\d+\.\d+`)
 )
```

#### 2. Update `FetchPipPackage` — Add short URL expansion (line 156)

```go
func FetchPipPackage(pipPackage string) (*PipPackageInfo, error) {
    // 1. Full VCS spec (git+...)
    if isDirectPipSpec(pipPackage) {
        return fetchVCSPipPackage(pipPackage)
    }

    // 2. Indexed package (== or ~=)
    if strings.Contains(pipPackage, "==") || strings.Contains(pipPackage, "~=") {
        return fetchIndexedPipPackage(pipPackage)
    }

    // 3. Go-style short URL (host/org/repo/path@version)
    if strings.Contains(pipPackage, "@") {
        expanded, err := expandShortURL(pipPackage)
        if err != nil {
            return nil, fmt.Errorf("failed to expand short URL %q: %w", pipPackage, err)
        }
        slog.Info("Expanded short URL to VCS spec",
            "input", pipPackage,
            "expanded", sanitizePipSpec(expanded),
            "phase", "discovery")
        return fetchVCSPipPackage(expanded)
    }

    return nil, fmt.Errorf("unrecognized pipPackage format: %q", pipPackage)
}
```

#### 3. Add `expandShortURL` function (new)

```go
// expandShortURL expands a Go-style module path to a full VCS pip spec.
// Input:  "github.com/wso2/gateway-controllers/policies/prompt-compressor@v1"
// Output: "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor"
//
// Convention: the first 3 path segments (host/org/repo) form the Git repository URL.
// The remaining segments form the subdirectory within the repo. The Git tag is
// constructed as subdirectory/version, matching Go's module tag convention.
func expandShortURL(spec string) (string, error) {
    spec = strings.TrimSpace(spec)

    atIdx := strings.LastIndex(spec, "@")
    if atIdx < 0 || atIdx == len(spec)-1 {
        return "", fmt.Errorf("short URL must contain '@version': %s", spec)
    }

    modulePath := spec[:atIdx]
    version := spec[atIdx+1:]

    segments := strings.Split(modulePath, "/")
    if len(segments) < 4 {
        return "", fmt.Errorf("short URL must have at least 4 path segments (host/org/repo/subdir), got %d: %s", len(segments), spec)
    }

    host := segments[0]
    org := segments[1]
    repo := segments[2]
    subdirectory := strings.Join(segments[3:], "/")

    repoURL := fmt.Sprintf("https://%s/%s/%s.git", host, org, repo)
    gitRef := subdirectory + "/" + version

    return fmt.Sprintf("git+%s@%s#subdirectory=%s", repoURL, gitRef, subdirectory), nil
}
```

#### 4. Add minor-only VCS detection and refactor resolution (lines 444-556)

Replace `isMajorOnlyVCSRef` / `extractMajorFromRef` / `resolveVCSMajorVersion` with a generalized approach:

```go
// vcsVersionType classifies a git ref's version granularity.
type vcsVersionType int

const (
    vcsVersionExact     vcsVersionType = iota // v1.0.3 — 3 components
    vcsVersionMinorOnly                       // v1.1   — 2 components
    vcsVersionMajorOnly                       // v1     — 1 component
    vcsVersionNone                            // not a version ref (e.g., "main")
)

// classifyVCSRef determines the version type of a git ref.
func classifyVCSRef(ref string) vcsVersionType {
    if exactRefPattern.MatchString(ref) {
        return vcsVersionExact
    }
    if minorOnlyRefPattern.MatchString(ref) {
        return vcsVersionMinorOnly
    }
    if majorOnlyRefPattern.MatchString(ref) {
        return vcsVersionMajorOnly
    }
    return vcsVersionNone
}

// resolveVCSVersion resolves a major-only or minor-only VCS ref to the highest
// exact matching tag using git ls-remote.
//
// For major-only (e.g., "policies/foo/v1"):
//   - Matches tags like "policies/foo/v1.0.0", "policies/foo/v1.2.3"
//   - Returns highest by semver
//
// For minor-only (e.g., "policies/foo/v1.1"):
//   - Matches tags like "policies/foo/v1.1.0", "policies/foo/v1.1.5"
//   - Returns highest patch version
func resolveVCSVersion(repoURL string, partialRef string, versionType vcsVersionType) (string, error) {
    sanitizedURL := sanitizeURL(repoURL)

    slog.Info("Resolving VCS partial version ref",
        "repo", sanitizedURL,
        "ref", partialRef,
        "type", versionType,
        "phase", "discovery")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", repoURL)
    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    out, err := cmd.Output()
    if err != nil {
        sanitizedStderr := strings.ReplaceAll(stderr.String(), repoURL, sanitizedURL)
        if ctx.Err() == context.DeadlineExceeded {
            return "", fmt.Errorf("git ls-remote timed out for %s", sanitizedURL)
        }
        return "", fmt.Errorf("git ls-remote failed for %s: %w; stderr: %s", sanitizedURL, err, sanitizedStderr)
    }

    // Build expected tag prefix: "policies/foo/v1." for major-only, "policies/foo/v1.1." for minor-only
    tagPrefix := partialRef + "."

    type candidate struct {
        tag   string
        major int
        minor int
        patch int
    }

    var candidates []candidate
    for _, line := range strings.Split(string(out), "\n") {
        if line == "" { continue }

        parts := strings.SplitN(line, "\t", 2)
        if len(parts) != 2 { continue }

        refPath := parts[1]
        if !strings.HasPrefix(refPath, "refs/tags/") || strings.HasSuffix(refPath, "^{}") { continue }

        tagName := strings.TrimPrefix(refPath, "refs/tags/")
        if !strings.HasPrefix(tagName, tagPrefix) { continue }

        // Extract remaining version components after the prefix
        suffix := strings.TrimPrefix(tagName, tagPrefix)

        switch versionType {
        case vcsVersionMajorOnly:
            // suffix should be "minor.patch" (e.g., "1.3")
            segs := strings.Split(suffix, ".")
            if len(segs) != 2 { continue }
            minor, err := strconv.Atoi(segs[0])
            if err != nil { continue }
            patch, err := strconv.Atoi(segs[1])
            if err != nil { continue }
            candidates = append(candidates, candidate{tag: tagName, minor: minor, patch: patch})

        case vcsVersionMinorOnly:
            // suffix should be just "patch" (e.g., "3")
            patch, err := strconv.Atoi(suffix)
            if err != nil { continue }
            candidates = append(candidates, candidate{tag: tagName, patch: patch})
        }
    }

    if len(candidates) == 0 {
        return "", fmt.Errorf("no tags found matching %q in %s", partialRef, sanitizedURL)
    }

    // Sort descending: by minor (for major-only), then by patch
    sort.Slice(candidates, func(i, j int) bool {
        if candidates[i].minor != candidates[j].minor {
            return candidates[i].minor > candidates[j].minor
        }
        return candidates[i].patch > candidates[j].patch
    })

    best := candidates[0].tag
    slog.Info("Resolved VCS partial version ref",
        "repo", sanitizedURL,
        "ref", partialRef,
        "resolvedTag", best,
        "candidateCount", len(candidates),
        "phase", "discovery")

    return best, nil
}
```

#### 5. Update `fetchVCSPipPackage` to use the new classifier

```go
func fetchVCSPipPackage(pipPackage string) (*PipPackageInfo, error) {
    vcs, err := parseVCSPipSpec(pipPackage)
    if err != nil {
        return nil, fmt.Errorf("failed to parse VCS pip spec: %w", err)
    }

    resolvedSpec := vcs.FullSpec
    versionType := classifyVCSRef(vcs.GitRef)

    if versionType == vcsVersionMajorOnly || versionType == vcsVersionMinorOnly {
        exactRef, err := resolveVCSVersion(vcs.RepoURL, vcs.GitRef, versionType)
        if err != nil {
            return nil, err
        }

        resolvedSpec = rebuildVCSPipSpec(vcs, exactRef)
        slog.Info("Resolved VCS partial version",
            "original", sanitizePipSpec(vcs.FullSpec),
            "resolved", sanitizePipSpec(resolvedSpec),
            "phase", "discovery")
    }

    // ... rest of function unchanged (pip wheel, extract, etc.) ...
}
```

#### 6. Update `ParsePipPackageRef` — support minor-only `~=N.M.0`

```go
func ParsePipPackageRef(ref string) (*PipPackageRef, error) {
    ref = strings.TrimSpace(ref)
    if ref == "" {
        return nil, fmt.Errorf(...)
    }

    if idx := strings.Index(ref, "~="); idx > 0 {
        pkgName := strings.TrimSpace(ref[:idx])
        versionPart := strings.TrimSpace(ref[idx+2:])
        version, indexURL := parseIndexURL(versionPart)

        // Validate: must be "N.0" (major-only) or "N.M.0" (minor-only)
        isMajor := majorOnlyPkgVersionPattern.MatchString(version)
        isMinor := minorOnlyPkgVersionPattern.MatchString(version)
        if pkgName == "" || version == "" || (!isMajor && !isMinor) {
            return nil, fmt.Errorf(...)
        }

        return &PipPackageRef{
            PackageName: pkgName,
            Version:     version,
            IndexURL:    indexURL,
            IsMajorOnly: true,  // "range query" flag — covers both major and minor
        }, nil
    }

    // ... exact == path unchanged ...
}
```

> [!NOTE]
> The `IsMajorOnly` field name is slightly misleading now since it covers minor-only too. It really means "is a range query" vs exact. The actual PEP 440 `~=` semantics handle both cases correctly: `~=1.0` for major-only, `~=1.1.0` for minor-only. pip resolves correctly in both cases.

#### 7. Remove old functions

- Remove `isMajorOnlyVCSRef` — replaced by `classifyVCSRef`
- Remove `extractMajorFromRef` — no longer needed (resolution is generalized)
- Remove `resolveVCSMajorVersion` — replaced by `resolveVCSVersion`

---

### [generator.go](file:///Users/sehan/Documents/GitHub/api-platform/gateway/gateway-builder/internal/buildfile/generator.go) — NO CHANGE

The lock entry already uses `found.PipSpec` (the resolved exact spec) for pip policies (line 208-210). Short URLs get expanded before entering the VCS path, and `PipSpec` in the `DiscoveredPolicy` contains the fully-resolved VCS spec. No changes needed.

---

### [manifest.go](file:///Users/sehan/Documents/GitHub/api-platform/gateway/gateway-builder/internal/discovery/manifest.go) — NO CHANGE

`discoverPipPolicy` calls `FetchPipPackage(entry.PipPackage)`. Since `FetchPipPackage` now handles short URL expansion internally, `manifest.go` doesn't need changes. The `OriginalPipSpec` field will correctly store the original `build.yaml` value (whether short URL or full VCS), and `PipSpec` will have the resolved exact spec.

---

### [build.yaml](file:///Users/sehan/Documents/GitHub/api-platform/gateway/build.yaml) — UPDATE

After implementation, the native Python policy entries change to Go-style:

```diff
   - name: prompt-compressor
-    pipPackage: "git+https://github.com/sehan-dissanayake/api-platform.git@policies/prompt-compressor/v1#subdirectory=gateway/sample-policies/prompt-compressor"
+    pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
   - name: prompt-compressor
-    pipPackage: "git+https://github.com/sehan-dissanayake/api-platform.git@policies/prompt-compressor/v2#subdirectory=gateway/sample-policies/prompt-compressor"
+    pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v2
```

---

## End-to-End Examples

### Example 1: Go-style short URL with major-only

**build.yaml**:
```yaml
- name: prompt-compressor
  pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
```

**Resolution flow**:
```
1. Classify: short URL (has @ but no git+, no ==, no ~=)
2. Expand: git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor
3. Parse VCS: ref = "policies/prompt-compressor/v1"
4. Classify ref: major-only (v1, 1 component)
5. git ls-remote → find tags matching "policies/prompt-compressor/v1.*"
6. Highest: policies/prompt-compressor/v1.0.0
7. Rewrite: git+...@policies/prompt-compressor/v1.0.0#subdirectory=...
8. pip wheel → extract → policy-definition.yaml → version: v1.0.0
```

**build-manifest.yaml**:
```yaml
- name: prompt-compressor
  version: v1.0.0
  pipPackage: git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1.0.0#subdirectory=policies/prompt-compressor
```

### Example 2: Minor-only VCS

**build.yaml**:
```yaml
- name: prompt-compressor
  pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1.1
```

**Resolution**: `@v1.1` → find `policies/prompt-compressor/v1.1.*` tags → `v1.1.5` → exact spec

### Example 3: Customer full URL

**build.yaml**:
```yaml
- name: my-custom-policy
  pipPackage: "git+https://gitlab.internal.company.com/team/policies.git@my-custom-policy/v2#subdirectory=my-custom-policy"
```

Customers use full VCS URLs — works unchanged. No expansion needed.

### Example 4: PyPI minor-only

**build.yaml**:
```yaml
- name: some-policy
  pipPackage: "some-policy~=1.2.0"
```

pip resolves `~=1.2.0` → latest `1.2.y` → e.g., `1.2.7`.

---

## Verification Plan

### Unit Tests

| Test | Description |
|------|-------------|
| `TestExpandShortURL` | `github.com/wso2/gateway-controllers/policies/prompt-compressor@v1` → correct VCS spec |
| `TestExpandShortURL_ThreeSegmentsError` | `github.com/wso2/repo@v1` → error (need 4+ segments) |
| `TestClassifyVCSRef_MajorOnly` | `"policies/foo/v1"` → `vcsVersionMajorOnly` |
| `TestClassifyVCSRef_MinorOnly` | `"policies/foo/v1.1"` → `vcsVersionMinorOnly` |
| `TestClassifyVCSRef_Exact` | `"policies/foo/v1.0.3"` → `vcsVersionExact` |
| `TestClassifyVCSRef_NotVersion` | `"main"` → `vcsVersionNone` |
| `TestParsePipPackageRef_MinorOnly` | `"pkg~=1.2.0"` → IsMajorOnly=true, Version="1.2.0" |
| `TestFetchPipPackage_ShortURL` | Integration: short URL → expanded → VCS path |

### Build Verification

```bash
cd gateway/gateway-builder && go test ./internal/discovery/ -v
cd gateway/gateway-builder && go test ./...
```

### Manual Integration Test

1. Update `build.yaml` with Go-style short URL for prompt-compressor
2. Run full build
3. Verify `build-manifest.yaml` has exact resolved VCS spec
4. Verify runtime works correctly
