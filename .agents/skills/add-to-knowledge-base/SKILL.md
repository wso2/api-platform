---
name: add-to-knowledge-base
description: Add or update a living feature document in the repository knowledge base under kb/. Use when someone asks to "add to knowledge base", "add-to-knowledge-base", "document this feature in kb", "create a kb entry", or "write the feature doc for <feature>". Produces one Open Knowledge Format (OKF) concept file per feature, shaped for product managers and developers at once, with a sources list that the docs-sync check enforces against code changes.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob, Skill
---

# Add to knowledge base

Create or update one feature document under `kb/<component>/<feature>.md`. The file is an OKF concept, so it starts with YAML frontmatter and is validated as part of the `kb/` bundle. Its body follows the template in [TEMPLATE.md](TEMPLATE.md). A reference example is `kb/platform-api/openapi-spec-management.md`.

## Steps

Prerequisite: the `okf-open-knowledge-format` skill is not committed. If it is missing, run `make install-skills` from the repo root first.

1. **Create the initial document with the OKF skill.** Invoke `okf-open-knowledge-format` to produce the concept file, the directory `index.md`, and a `log.md` entry, and to validate the bundle. Bundle root is `kb/`, which already declares `okf_version: "0.2"`. Use `type: Feature`. Set `generated.by` to the requesting user as `human:<github-handle>` and `status: draft` unless told otherwise. Never add `verified` unless a human has actually compared the document against the code.

2. **Apply the rules below and rewrite whatever violates them.** Do this before shaping the body, because the OKF skill's default output does not know these rules.

3. **Apply the template.** Replace the body with [TEMPLATE.md](TEMPLATE.md), filled in. Keep exactly its sections and order. Do not add sections. If a section has nothing to say, write "None" under it rather than deleting it, so a reader can tell "considered" from "forgotten".

4. **Finish.** Add the file to `kb/<component>/index.md` with its description, add a dated entry to `kb/log.md` (newest first), re-run the OKF validation, and run `scripts/check-feature-docs.sh` if the branch has code changes. Report which `sources` paths do not exist yet.

## Rules

These override anything the OKF skill or the template example suggests.

1. **Files only, never functions or methods.** Refer to code by file path. Do not name a Go function, method, struct, test function, or SQL statement. A test is referenced as "a test in `<file>`", never by its name. This applies to prose, tables, and shell commands (no `go test -run <Name>`).
2. **Never reference spec-kit artifacts.** Nothing under `specs/`, `.specify/`, or any `artifacts/` folder, and no mention of `spec.md`, `plan.md`, `research.md`, `data-model.md`, `tasks.md` or `quickstart.md`. Those files are removed from the repository. Carry the substance across in your own words; the Decisions table has a "Why" column instead of a citation column for this reason.
3. **Two audiences, one file.** The body is a shared header, then a Product view, then a Developer view. Nothing in the Product view names a file. Nothing in the Developer view re-explains a use case. Decisions are written in user terms ("uploading never changes how traffic is routed"), not implementation terms.
4. **Decisions versus open questions.** A choice that was made goes in Decisions with its reason. A choice not yet made goes in Open questions with options, owner and needed-by. When a question is answered, move it.
5. **Limitations are rows with tracking.** Each row links an issue or says "Not yet filed". Delete the row when fixed. Never leave a stale row.
6. **Every Entry points path is in `sources`, and the reverse.** The `sources` list is what `scripts/check-feature-docs.sh` uses to fail a PR that changes documented code without updating the document. Planned files that do not exist yet are allowed and are reported as warnings.
7. **Minimal.** One sentence per idea. No history, no dates in the body, no planning narrative, no session notes, no branch names. No marketing words.
8. **Preserve what exists.** When updating a document, change only what the code or decision change invalidates. Do not restructure or re-flow unrelated sections. Bump `stale_after` only together with a real doc-versus-code comparison.

## Checklist before finishing

- Frontmatter has `type`, `title`, `description`, `resource`, `status`, `generated`, `stale_after`, `sources`.
- `grep -nE "specs/|\.specify|artifacts/|spec\.md|plan\.md|research\.md|tasks\.md|data-model\.md|quickstart\.md"` on the file returns nothing.
- No function, method, struct or test name appears anywhere in the file.
- Section list is exactly: Use cases, Decisions, Scope, Current limitations, Open questions, Entry points, Examples.
- `kb/<component>/index.md` and `kb/log.md` are updated.
- The OKF validation reports the bundle conformant.
