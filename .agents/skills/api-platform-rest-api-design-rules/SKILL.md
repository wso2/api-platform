---
name: api-platform-rest-api-design-rules
description: WSO2 API Platform-specific REST API design rules, layered on top of the generic api-design skill. Use when assessing, validating, reviewing, or fixing a WSO2 API Platform OpenAPI spec (e.g. platform-api/src/resources/openapi.yaml) and you want the standard api-design checks PLUS api-platform house rules — consistent camelCase parameter names (flagging PascalCase like ProjectID/GatewayID), consistent collection response envelopes ({ list, pagination }), and ap: OAuth2 scope naming. Trigger when the user says "validate/assess/check this spec against the api-platform rules", "apply the api-platform design rules", or names this skill directly. Extends api-platform:api-design.
allowed-tools: Bash, Read, Edit, Grep, Glob, Write, Skill
---

# WSO2 API Platform REST API Design Rules

This skill is an **extension of the generic `api-platform:api-design` skill**. It does not replace it — it adds a layer of WSO2 API Platform house rules that the generic assessor does not check.

Use it on WSO2 API Platform OpenAPI specs (for example `platform-api/src/resources/openapi.yaml`). The generic skill handles AI Agent Readiness, Security Readiness, and WSO2 REST Design Guidelines. This skill adds platform-specific conventions that the platform's own APIs are expected to follow consistently.

## Usage

```
/api-platform-rest-api-design-rules <path-to-openapi-spec>
```

`<path-to-openapi-spec>` — path to the OpenAPI YAML/JSON file. If omitted, ask the user for it.

## Prerequisites

- **The base plugin skill `api-platform:api-design` must be installed and enabled.** This skill depends on it for the generic assessment (Step 3) and reuses its `assets/report_template.html` for the HTML render (Step 5b). It ships in the WSO2 **agent-skills** plugin — install it from **https://github.com/wso2/agent-skills**. Verify it in Step 0 before doing any work; if it is missing, stop and ask the user to install it rather than continuing with a partial run.
  - This skill is shared across coding tools, and each tool enables/stores plugins differently, so the check is tool-specific: **Claude Code** records enabled plugins in `.claude/settings.json` → `enabledPlugins` (and caches files under `~/.claude/plugins/`); **Codex** manages them via `codex /plugins`, tracked in `~/.codex/config.toml`.
- The skill `rest-api-oauth-scopes` (in `.agents/skills/`) is the source of truth for scope-naming rules and is consulted by APR-003 below.
- For the generic assessment leg, the base skill needs Spectral on PATH (`spectral --version`). The platform rules in this skill (APR-001..003) are evaluated by reading the spec directly and do not require Spectral.

## Workflow

### Step 0 — Preflight: verify the base plugin is enabled

The base plugin skill `api-platform:api-design` is required (Step 3 and Step 5b). Verify it from the coding tool's **marketplace/plugin config** — do **not** probe for skill files on disk (install paths and versions vary by tool and are not a reliable signal):

- **Claude Code** — the `enabledPlugins` map in `.claude/settings.json` (project) or `~/.claude/settings.json` (user) must contain `api-platform@wso2-agent-skills: true`:

  ```bash
  grep -REl '"api-platform@wso2-agent-skills"[[:space:]]*:[[:space:]]*true' \
    .claude/settings.json ~/.claude/settings.json 2>/dev/null
  ```

- **Codex** — it must be enabled in `~/.codex/config.toml` (or via `codex /plugins`).

If the config confirms the plugin is enabled, continue. Otherwise **stop and ask the user to install it** — do **not** try to fix it yourself (do not edit `.claude/settings.json`/`~/.codex/config.toml`, run install commands, clone the repo, or otherwise enable the plugin on the user's behalf). Tell the user:

> The `api-platform:api-design` plugin is not enabled, and this in-house designer skill depends on it. Please install and enable the WSO2 agent-skills plugin by following the steps in https://github.com/wso2/agent-skills, then re-run this skill.

Then **end the run immediately**. This is a hard prerequisite for the whole skill: if the base plugin is not installed, do **not** proceed with anything — not even the in-house APR rules (option 3), the `api-platform-rules.json` output, or the HTML render. There is no partial-run fallback; the only correct action is to ask the user to install the plugin and stop.

### Step 1 — Resolve the spec

If the user did not provide a spec path, ask for one. Confirm the file exists before continuing.

### Step 2 — Choose what to run

Ask the user which assessment to run (default to option 1):

> **What would you like to run?**
> 1. **Both — generic api-design + api-platform rules** (recommended)
> 2. **Generic api-design only** — the standard 3-dimension assessment
> 3. **Api-platform rules only** — just the house rules in this skill

Map the answer:

- **Option 1 (both):** Run Step 3, then Step 4, then Step 5 (merged summary), then Step 5b (render the merged HTML).
- **Option 2 (generic only):** Run Step 3 only, then stop. This is exactly the base skill's behaviour — hand off entirely to `api-platform:api-design`. (The base skill already renders its own HTML.)
- **Option 3 (platform rules only):** Skip Step 3. Run Step 4, then Step 5 (platform-only summary), then Step 5b (render the standalone HTML).

### Step 3 — Run the generic assessment (base skill)

Invoke the `api-platform:api-design` skill against the spec to produce the standard assessment (AI Agent Readiness, Security Readiness, WSO2 REST Design Guidelines). Let it write its report to `./api-reports/` as usual. Note the report paths it prints — Step 5 references them.

Do not re-implement any base-skill checks here. If the user only wants the generic assessment (option 2), this step is the whole job.

### Step 4 — Run the api-platform house rules

Read the spec in full and evaluate each rule below. For every violation, record an object with the same shape the base skill uses so the findings merge cleanly:

- `id` — `apr-NNN` (e.g. `apr-001`), numbered per rule, incrementing per finding.
- `severity` — as stated in the rule.
- `rule` — the rule code (e.g. `APR-001`).
- `path` — JSON path to the offending element (e.g. `components.parameters.ProjectID`).
- `issue` — what is wrong.
- `description` — why it matters for this platform.
- `fixSuggestion` — concrete, minimal change.
- `autoFixable` — `true` only for safe, non-breaking edits; `false` for anything that renames a wire-level parameter, path, or response key (breaking change).

The rule definitions live in **`references/api-platform-house-rules.md`** (next to this skill). Read that file in full and evaluate every rule it lists against the spec:

- **APR-001** — Parameter names must be consistent lowerCamelCase, allowing the `-Q`/`-Q-Opt` query-variant key suffix (MEDIUM).
- **APR-002** — Collection responses must use the standard `{ list, pagination }` list envelope (MEDIUM).
- **APR-003** — OAuth2 scope names must follow the `ap:` convention (HIGH); defers to the `rest-api-oauth-scopes` skill as the source of truth.
- **APR-004** — Errors must use the standard error shape `{ status, code, message, errors[] }` with a string `code` from the error catalog, via shared `components/responses` (HIGH).
- **APR-005** — Create operations (`POST` → `201`) must return a `Location` header (MEDIUM).
- **APR-006** — Non-CRUD actions use `POST` on a kebab-case verb sub-path (LOW).
- **APR-007** — Collection list items should use a lightweight `*ListItem`/`*Info` schema, not the full resource (MEDIUM).
- **APR-008** — Collection GETs use the standard pagination/query parameters (`limit`/`offset`/`sortBy`/`sortOrder`/`query`) consistently (MEDIUM).

Each rule entry in the reference gives its severity, what to flag, how to detect it, the concrete fix, and whether the fix is `autoFixable`. Use those to populate the finding fields above. Do not rely on the summaries here — open the reference file so new or updated rules are picked up automatically. The reference is the single source of truth; this list is only an index, so if it drifts from the file, trust the file.

### Step 5 — Report

Produce a single summary. When both legs ran (option 1), present the base skill's three-dimension scores first, then an **API Platform Rules** section listing APR findings grouped by rule (APR-001, APR-002, APR-003) with counts by severity. When only platform rules ran (option 3), present just the API Platform Rules section.

Write the platform findings to `./api-reports/api-platform-rules.json` (create `./api-reports/` if needed) so they can be fed into a later fix pass and into the HTML render in Step 5b. The file shape is:

```json
{
  "meta": { "spec": "<spec path>", "assessedAt": "<ISO-8601>", "rules": ["APR-001","APR-002","APR-003"] },
  "findings": [ { "id": "apr-001", "severity": "MEDIUM", "rule": "APR-001", "path": "...", "issue": "...", "description": "...", "fixSuggestion": "...", "autoFixable": true } ]
}
```

If a rule passed cleanly, you may record a single `"severity": "INFO"` marker for it (the HTML renderer drops INFO rows but still credits the rule as passed). If the base report exists, mention both paths.

### Step 5b — Render the final HTML report

Produce a single self-contained HTML report. This **reuses the base `api-design` skill's report generation** — the same scoring formula and the same `assets/report_template.html` — via this skill's `scripts/merge-apr-report.js`. The base template is treated as **read-only**: the script injects the extra "API Platform House Rules" section into an in-memory copy and never edits the template on disk.

First locate the base template (the version segment changes between releases):

```bash
TEMPLATE=$(find ~/.claude/plugins/cache -path '*api-design/assets/report_template.html' | head -1)
```

Then run the renderer. It auto-detects the mode:

- **Option 1 (both ran)** — pass `--report` so the APR section is **merged into** the base report produced by Step 3 (`./api-reports/<spec-stem>-api-readiness-report.json`). The final HTML shows all four base dimensions **plus** API Platform Rules, and overwrites that same `*-api-readiness-report.html`:

  ```bash
  node .agents/skills/api-platform-rest-api-design-rules/scripts/merge-apr-report.js \
    --apr      ./api-reports/api-platform-rules.json \
    --report   ./api-reports/<spec-stem>-api-readiness-report.json \
    --template "$TEMPLATE" \
    --write-json
  ```

- **Option 3 (platform rules only)** — omit `--report`. The script builds a minimal report from the APR file's own `meta` and renders an HTML showing only the API Platform Rules section (the template skips dimensions with no data). Output defaults to `./api-reports/api-platform-rules-report.html`:

  ```bash
  node .agents/skills/api-platform-rest-api-design-rules/scripts/merge-apr-report.js \
    --apr      ./api-reports/api-platform-rules.json \
    --template "$TEMPLATE"
  ```

Flags: `--apr` and `--template` are required; `--report` switches on merge mode; `--out` overrides the output path; `--write-json` also refreshes the report JSON so it stays consistent with the HTML. If the script reports that template anchors were not found, the base template changed shape — update the anchor strings near the top of `scripts/merge-apr-report.js`. Print the final HTML path to the user.

### Step 6 — Offer fixes

Ask whether to apply fixes. Follow the same discipline as the base skill's Fix Workflow:
- **Safe / non-breaking edits** (autoFixable: true — e.g. renaming a `components.parameters` key and its `$ref`s) may be applied directly.
- **Breaking edits** (autoFixable: false — renaming a wire-level parameter `name`, a response body key, or an OAuth2 scope) must be listed and confirmed before applying, because they change the API contract for existing clients.

Apply confirmed fixes with targeted edits to the spec file in place, then report what was applied, skipped, and what still needs manual action.
