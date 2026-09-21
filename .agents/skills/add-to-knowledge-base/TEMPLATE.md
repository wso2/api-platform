---
type: Feature
title: <Feature name>
description: <One sentence: what a user can now do that they could not before.>
resource: /<component>
tags: [<component>, <area>, <area>]
status: draft | stable | deprecated
generated: { by: human:<github-handle>, at: <YYYY-MM-DDT00:00:00Z> }
stale_after: <YYYY-MM-DDT00:00:00Z, about six months out>
sources:
  - id: <short-id>
    resource: /<component>/<path-to-file>
    title: <What this file is>
  # One entry per file named in Entry points. Files only, never functions.
---

# <Feature name>

**Status**: <Planned | In progress | Shipped in <release>>.

<One paragraph: the situation before, what the feature adds, and the one boundary a reader must not misunderstand. Link related concepts inline with bundle-relative links, for example [portal publishing](/<component>/<concept>.md).>

The document has two parts. **Product view** answers what the feature does, why it is shaped this way and what it does not do. **Developer view** answers where the code is and how to try it. Nothing in the product view names a file; nothing in the developer view re-explains a use case.

---

## Product view

### Use cases

| # | Actor | Wants to | So that | Status |
|---|---|---|---|---|
| UC1 | <role> | <action> | <outcome> | Planned / In progress / Done |

### Decisions

<!-- Product and design choices where a reasonable alternative existed. One row per choice, in user terms. If a test enforces the choice, say so in the Choice cell as "Enforced by a test that ...". No file names, no function names. -->

| Decision | Choice | Why |
|---|---|---|
| <question> | <what was chosen> | <one line> |

### Scope

In scope:

- <capability>

Out of scope:

- <capability>

### Current limitations

<!-- Things in scope that do not fully work yet. Delete the row when the issue closes. -->

| Limitation | Impact | Tracking |
|---|---|---|
| <what> | <who notices, how> | <issue link, or "Not yet filed"> |

### Open questions

<!-- Decisions not yet taken. Move a row into Decisions once answered. -->

| Question | Options | Owner | Needed by |
|---|---|---|---|
| <question> | <candidates, with the current recommendation> | <role or person> | <milestone> |

---

## Developer view

### Entry points

Ordered the way a reader should explore. Every path also appears in `sources` so the sync check covers it.

| Start here | Path | What you will find |
|---|---|---|
| Contract | `<component>/resources/openapi.yaml` | <endpoints, schemas, scopes> |
| HTTP | `<component>/internal/handler/<file>.go` | <what this layer owns> |
| Logic | `<component>/internal/service/<file>.go` | <what this layer owns> |
| Storage | `<component>/internal/repository/<file>.go` | <what this layer owns> |
| Schema | `<component>/internal/database/schema.*.sql` | <table> |
| Config | `<component>/config/config.go` | <key> |
| Tests | `<component>/internal/handler/<file>_test.go` | Executable specification of the behaviour |

<Optional: one or two sentences on a helper to reuse or a place not to duplicate, naming the file only.>

### Examples

```bash
# The two to five commands that exercise the feature end to end on a running instance.
```
