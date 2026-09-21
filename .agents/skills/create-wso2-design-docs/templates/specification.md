# **\<Feature Name\> - Specification**

<!--
This document carries the feature specification forward into the WSO2 design doc set — it does
not reinvent it. Where the feature already has a written specification, transfer it across with
light adaptation rather than rewriting it, keeping the section order below.

Keep the identifiers stable: FR-nnn and SC-nnn are carried over verbatim and are referenced by
test-scenarios.md, so renumbering them breaks traceability across the set.

THIS DOCUMENT IS PASTED INTO A GOOGLE DOC AND MUST STAND ALONE. Never name the source artifacts
it was derived from, never cite an id that only exists in them (a task id, a research id, a gate
id), and never link to a sibling document — refer to it by title and section in prose. State
every decision and every outstanding item in full, so no sentence depends on a file the reader
cannot open. Committed repository paths (source files, config keys, .claude/rules/*.md) are fine.

Two sections exist for the WSO2 review rather than being carried over: Scope (§1), because a
reviewer asks the boundary question first, and Open Questions (§7), because anything the inputs
leave unestablished is recorded rather than guessed.

Deliberately NOT here — each already has a home, don't duplicate it:
- Flow-by-flow behaviour and error mapping -> implementation.md §3.4, §3.5
- Configuration keys -> implementation.md §4
- Backward compatibility / upgrade path -> db-schema.md §5, rest-apis.md §6, implementation.md §8
- Per-endpoint privilege and scopes -> rest-apis.md, security-impact-review.md
-->

# 1. Scope

<!--
The boundary, stated explicitly. "Out of scope" carries more weight than "in scope" — list the
things a reader would reasonably assume are included but are not, and say where each was
decided (an assumption, a clarification, an FR that excludes it) — without naming the file it
was decided in.
-->

**In scope**

* \<...\>

**Out of scope**

* \<what a reader might assume is included, and why it is not\>

# 2. Scenarios

<!--
One sub-section per user story, in priority
order, keeping the story's priority. Acceptance scenarios stay in Given/When/Then form — they
are what test-scenarios.md §3 turns into concrete cases, so keep them verifiable.
-->

## 2.1 \<User story title\> (Priority: P\<n\>)

\<Who the actor is, what they are trying to do, and what they get.\>

**Why this priority**: \<what this story alone delivers, and what depends on it.\>

**Independent test**: \<how this story can be verified on its own.\>

**Acceptance scenarios**

1. **Given** \<precondition\>, **When** \<action\>, **Then** \<observable outcome\>.
2. **Given** \<...\>, **When** \<...\>, **Then** \<...\>.

## 2.2 \<Next user story\> (Priority: P\<n\>)

\<...\>

## Edge Cases

<!--
The awkward inputs and states the happy path does not cover. Where an artifact does not
establish the behaviour, say so here and raise it in §6 — never invent the answer.
-->

* \<edge case, and the defined behaviour — or a pointer to the Open Question that covers it.\>

# 3. Requirements

<!--
One numbered requirement per behaviour, phrased so it can be verified. IDs are carried over and
must not be renumbered — test-scenarios.md references them.
-->

| ID | Requirement | Priority | Notes |
| :---- | :---- | :---- | :---- |
| FR-001 | \<The system MUST ...\> | Must / Should / May | \<...\> |
| FR-002 | \<...\> | | |

# 4. Key Entities

<!--
The things the feature introduces or touches, and what each one is. Fold in any overloaded term
the rest of the document relies on, so a reader never has to guess what a word means here.
-->

| Entity / Term | Definition |
| :---- | :---- |
| \<entity\> | \<what it represents, its cardinality, and what owns it\> |
| \<term\> | \<definition\> |

# 5. Success Criteria

<!--
The measurable half of the specification. Technology-agnostic and observable — a target with no way to measure it is not one.
Keep the SC-nnn ids; test-scenarios.md traces to them.
-->

| ID | Outcome | Target |
| :---- | :---- | :---- |
| SC-001 | \<observable outcome\> | \<measurable target\> |
| SC-002 | \<...\> | \<...\> |

# 6. Assumptions

<!-- State what breaks if the assumption does not hold — an assumption with no consequence is a note, not an assumption. -->

* \<assumption\> - \<what breaks if it does not hold.\>
* \<dependency\> - \<component/version required, and why.\>

# 7. Open Questions

<!--
Everything the feature's inputs and the code do not establish. A guessed answer reads as
verified to a reviewer, so an unknown belongs here instead. Blocked work should say what it
blocks.
-->

| Question | Owner | Target date | Status |
| :---- | :---- | :---- | :---- |
| \<question, and what it blocks\> | \<name\> | \<YYYY-MM-DD\> | Open / Resolved |
