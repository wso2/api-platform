---
name: create-wso2-design-docs
description: Generate the WSO2 design document set for a Spec Kit feature. Use after running Spec Kit (/speckit.specify, /speckit.plan, optionally /speckit.tasks or /speckit.implement) when someone asks to "create the WSO2 design docs", "apply the wso2-design-doc templates", "generate the design document for this feature", or "fill the WSO2 design doc templates from the spec". Produces one filled document per template under <spec-kit feature folder>/wso2.
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, AskUserQuestion
---

# Create WSO2 design docs from a Spec Kit feature

Takes the Spec Kit artifacts a developer has already produced for a feature (`spec.md`, `plan.md`,
and whatever else exists — `research.md`, `data-model.md`, `contracts/`, `tasks.md`, `quickstart.md`)
and writes the WSO2 design document set into `<feature folder>/wso2/`.

The templates live in **`templates/`** next to this file. They are the copy this skill fills —
use them, not `guidelines/wso2-design-doc/template/`. Worked examples are still in
`guidelines/wso2-design-doc/examples/` (`cicd-ai-workspace`, `dp-to-cp-for-gateway-ai-workspace`);
read one when unsure how much detail a section wants.

## Usage

```text
/create-wso2-design-docs [feature-folder | feature-name]
```

---

## Step 1 — Locate the feature folder

A Spec Kit feature folder is a directory under a `specs/` directory that sits beside a `.specify/`
directory, e.g. `openapi-support/specs/001-openapi-spec-management/`.

```bash
find . -type d -name specs -not -path "*/node_modules/*" -not -path "*/.git/*" \
  -exec sh -c 'test -d "$(dirname "$1")/.specify" && ls -d "$1"/*/' _ {} \;
```

- An argument that is a path → use it.
- An argument that is a name/number → match it against the listing.
- No argument and exactly one candidate → use it.
- No argument and several candidates → ask the user which one with `AskUserQuestion`.

Fail with a clear message if the folder has no `spec.md` — this skill runs *after* Spec Kit, never
instead of it.

## Step 2 — Read every Spec Kit artifact before writing anything

Read all of these that exist, in full:

| Artifact | What it feeds |
| :---- | :---- |
| `spec.md` | `overview.md`, `specification.md`, `feature-capabilities.md` |
| `plan.md` | `overview.md` (solution shape), `implementation.md` |
| `research.md` | `overview.md` (challenges), `feature-capabilities.md` (decisions + sources) |
| `data-model.md` | `db-schema.md`, `implementation.md` §3.3 |
| `contracts/*.yaml`, `*.json` | `rest-apis.md` |
| `tasks.md` | `implementation.md` §7 rollout/phasing, `test-scenarios.md` §9 |
| `quickstart.md` | `test-scenarios.md` §2 preconditions |
| `checklists/` | `test-scenarios.md` |

Also read the feature's constitution/memory (`<spec-kit root>/.specify/memory/`) if present — it
carries project-level constraints the design doc must not contradict.

## Step 3 — Decide which documents apply

Always write: `overview.md`, `feature-capabilities.md`, `specification.md`, `implementation.md`,
`test-scenarios.md`, `security-impact-review.md`, `rnd-task-completion-questionnarie.md`.

Conditionally write:

| Document | Write it when |
| :---- | :---- |
| `rest-apis.md` | The feature adds or changes an endpoint (a `contracts/` dir, an OpenAPI diff, or any REST surface in the spec/plan) |
| `db-schema.md` | The feature touches a table, column, index, or constraint |
| `uis.md` | The feature has a portal/UI surface |

Skip a conditional document entirely rather than writing a file of placeholders. Say in the final
report which ones were skipped and why.

## Step 4 — Fill each document

Copy `templates/<file>` to `<feature folder>/wso2/<file>` and fill it in. Rules:

1. **Every `<placeholder>` is replaced, and every `<!-- guidance -->` comment is removed.** A
   delivered document must contain no `\<...\>` angle-bracket placeholders and no guidance
   comments. Grep for both before finishing.
2. **Only what the Spec Kit artifacts and the code actually say.** Never invent a decision, a
   status code, a scope name, a config default, a DDL column, or a metric. Where the artifacts are
   silent on something the template asks for, write `NOT ANSWERED` (questionnaires) or add a row to
   the document's Open Questions table — never a plausible guess. A guessed answer reads as
   verified to a reviewer.
3. **Never leave a questionnaire row blank.** `security-impact-review.md` and
   `rnd-task-completion-questionnarie.md` get `N/A` with a one-line reason where a section does not
   apply, `NOT ANSWERED` where the artifacts do not establish the answer. A `Yes` says *how* it is
   enforced and *where*, not just "yes".
4. **Delete sections that genuinely do not apply**, rather than filling them with `N/A` prose —
   except in the two questionnaires, whose rows are fixed and must all survive.
5. **THE DELIVERED DOCUMENTS MUST STAND ALONE.** Each file is copied and pasted into a Google
   Doc. Nothing in the feature folder — not the source artifacts, not the sibling documents —
   exists for that reader, so a reference to one is a dead end. In the delivered text:
   * **Never name a source artifact.** No `spec.md`, `plan.md`, `research.md`, `data-model.md`,
     `tasks.md`, `quickstart.md`, `contracts/…`, `checklists/…`, `.specify/…`, and no
     `specs/<feature>/…` path. Never write "Spec Kit" (or the name of any tool that produced the
     inputs) anywhere in a document.
   * **Never cite an id that only exists in those artifacts** — a task id (`T007`), a research id
     (`R8`), a plan gate id (`G2`), a checklist item. State the decision, the finding, or the
     outstanding work *itself*, in full, so the sentence survives without the source.
   * **Never link to a sibling document.** Refer to it by title and section in prose — "see the
     REST APIs document, §6" — never `[rest-apis.md](./rest-apis.md)`.
   * **Committed repository paths are fine and wanted**: `platform-api/internal/service/api.go`,
     `.claude/rules/db-schema-changes.md`, a config key, a function name. A reader with the repo
     can open those; they are not what this rule is about.
   Grep the output for every one of these before finishing.
6. **Cross-reference, don't duplicate.** `implementation.md` §3.1 points at the REST APIs
   document; §3.3 at the DB schema document; `test-scenarios.md` traces each row back to an
   `FR-nnn`/`SC-nnn` in the specification document. Keep the FR/SC ids stable across the set —
   those ids are the traceability, since the documents cannot link to each other (rule 5).
7. **`specification.md` carries the feature specification forward — it does not reinvent it.**
   Its section order is Scope, Scenarios (user stories with priority, acceptance scenarios, edge
   cases), Requirements, Key Entities, Success Criteria, Assumptions, Open Questions. Carry the
   `FR-nnn` and `SC-nnn` numbering over verbatim rather than renumbering to a local scheme. Do
   not restate flow-by-flow behaviour, error mapping, configuration keys, or the upgrade path
   there — those live in `implementation.md` §3.4/§3.5/§4/§8, `db-schema.md` §5 and
   `rest-apis.md` §6.
8. **Header blocks.** `overview.md`'s table: author from `git config user.name`/`user.email`,
   `Status: Draft`, `Last updated` = today, GitHub issue from the spec if it names one, else
   `NOT ANSWERED`. The "Related docs" row names the other documents in this set by title, never as
   file links. Same for the questionnaires' "Submitted by"/"Submitted date"; leave
   reviewer/approval rows `NOT ANSWERED`.
9. **Decision sources are real, citable outside the repo, or absent.** In
   `feature-capabilities.md`, the Decision Source column names a forum the reader can actually
   reach — a meeting title + date, a GitHub issue/PR link, a proposal doc the feature itself
   carries — or, where the artifacts establish only *that* a decision was taken during design
   and not where, `Feature design - <YYYY-MM-DD>`. Never cite a source artifact filename
   (rule 5), and never fabricate a meeting title or date.
10. **Diagrams.** Keep image references as the templates have them (`![][image1]` plus the trailing
   comment-form link definition) and note in the final report that they need attaching. Do not
   invent an image path.

## Step 5 — Apply the repo's rules to the design, not just the prose

The design docs are reviewed against the repo's `.claude/rules/`. While filling in:

- **`db-schema.md`** — these are GA products. Only additive changes to a shipped table (new
  nullable/defaulted column, new index, new table). A column added to an existing table needs its
  per-dialect `ALTER TABLE` recorded in §2 and §5, and every dialect listed in §3. If the change is
  a retype/rename/added-NOT NULL/added-UNIQUE, say so explicitly — that is a migration, not a
  schema edit. Follow `.claude/rules/db-schema-changes.md`; invoke the `designing-db-schemas` skill
  if the schema design itself is still open.
- **`rest-apis.md`** — every endpoint carries authentication, required scope(s), and how the
  organization is resolved (always from verified claims, never from the payload). Error bodies stay
  generic. Follow `.claude/rules/authentication_authorization.md` and `error-handling.md`.
- **`test-scenarios.md`** — §5 (authorization and tenant isolation) is never skipped for anything
  reachable over an API.
- **`implementation.md`** §3.2 validation table and `test-scenarios.md` §4 must agree row for row.

If an artifact describes something that violates one of those rules, write what the artifacts say
and record the conflict as an Open Question row — do not silently "fix" the design here, and do not
paper over it with a `TODO` comment.

## Step 6 — Trim `overview.md` against the finished set

`overview.md` is written first, before the documents that carry the detail, so it always ends up
holding material that a later document now owns. Once every other file is written, **re-read
`overview.md` in full against them and cut it back.**

It is the document a reviewer reads first and may be the only one they read. It has to answer
*what is the problem, why does it matter, what is the shape of the solution, and what did we
decide not to do* — nothing else. A reviewer who wants the endpoint list, the DDL, the validation
table or the test matrix goes to the document that owns it.

Cut, in this order:

1. **Anything another document now owns in more detail.** Endpoint definitions belong to
   `rest-apis.md`; DDL and column notes to `db-schema.md`; validation rules, error mapping,
   rollout and observability to `implementation.md`; requirements, scenarios and the full
   out-of-scope list to `specification.md`; test cases to `test-scenarios.md`. Keep at most a
   one-line orientation ("four endpoints on an `openapi` sub-resource") and name the document
   that has the rest — in prose, never as a link (rule 5).
2. **Restatements of the requirements.** A Key Behaviours bullet that just re-words an `FR` adds
   nothing. Keep only the observable rules a reviewer would otherwise be surprised by.
3. **Challenge rows that are really design principles.** A challenge earns its row when the
   solution was non-obvious. If the "solution" is a principle already stated above it, drop it.
4. **Repetition between sections.** The same fact commonly appears in the problem statement, the
   solution, a sub-section and a challenge row. Keep the clearest one.

Do not cut the header table, the problem framing, the benefits, the solution shape, the diagram
reference, or the fact that something is out of scope — a shortened Out of Scope list that names
the document holding the full one is right; deleting the section is not. Never drop an open
decision or a stated risk to save space.

As a rough calibration, a trimmed `overview.md` for a single-service feature is well under half
the length of `implementation.md`. If it is longer than the document it introduces, it is still
carrying someone else's detail.

## Step 7 — Report

Print:

- the output folder path and every file written,
- what Step 6 cut from `overview.md`, in one line (e.g. "dropped the storage-shape and
  presence sub-sections — the DB schema and REST APIs documents own them"),
- the conditional documents skipped, with the reason,
- every `NOT ANSWERED` row and every Open Question raised, grouped by file — this is the developer's
  to-do list,
- the diagrams that still need attaching.

## Checklist before finishing

```bash
cd <feature folder>/wso2
 grep -rn '\<[a-z]' . | grep -v '^Binary'   # no leftover \<placeholder\>
grep -rn '<!--' .                            # no leftover guidance comments
grep -rn 'TODO\|FIXME' .                     # never defer behind a comment

# Rule 5 — the documents must stand alone once pasted into a Google Doc:
grep -rnE 'spec\.md|plan\.md|research\.md|data-model\.md|tasks\.md|quickstart\.md' .
grep -rnE 'contracts/|checklists/|\.specify|specs/[0-9]|Spec Kit|spec.kit' .
grep -rnE '\b(T[0-9]{3}|R[0-9]{1,2}|G[0-9]{1,2})\b' .   # task / research / gate ids
#   ^ `R1`-`R10` also matches the designing-db-schemas rule numbers, which ARE legitimate:
#     they name a committed reference a reader can open. Check each hit, don't blanket-strip.
grep -rn '](\./\|](\.\./' .                              # links to sibling or source files
```

- Output is under `<feature folder>/wso2/`, not `guidelines/`.
- `overview.md` has been through the Step 6 trim and carries no detail another document owns.
- No document names a source artifact, a task/research/gate id, or links to a sibling document
  (rule 5) — each one has to make sense pasted into a Google Doc on its own.
- `FR-nnn`/`SC-nnn` ids in `specification.md` match `spec.md` and are referenced by `test-scenarios.md`.
- Both questionnaires have every row filled with a real answer, `N/A` + reason, or `NOT ANSWERED`.
- Nothing in the set asserts a fact the Spec Kit artifacts or the code do not support.
