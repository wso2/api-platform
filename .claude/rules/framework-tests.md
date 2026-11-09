# Rule: Framework Tests

## Context & Scope

Applies to every change under `tests/framework/`: feature files, step definitions, suite and
block files (`suites/*/*-suite.yaml`, `suites/it/blocks/*.yaml`), overlays and scenario lists
(`scenarios/on-prem/*.yaml`, `scenarios/cloud/*.yaml`), whether the task is writing a new test,
fixing a failing one or changing a step.

**The rules are in `tests/framework/CLAUDE.md`.** Invoke the `write-framework-tests` skill to
write a test; it walks through them in order. This file states the directives a change must
never break.

## Directives

1. **Every test starts from a scenario list.** A new scenario automates an entry in its
   feature's list: `scenarios/on-prem/<CODE>.yaml` for `suites/it` and `suites/ui`,
   `scenarios/cloud/<CODE>.yaml` for `suites/cloud`. A missing entry is added in the same
   change, with the next free ID for its prefix; never reuse or renumber a merged ID. The
   entry's status and `refs` are updated in the same change as the scenario.

2. **The right suite, file and block.** On-premises API-level behaviour goes in `suites/it`,
   portal journeys in `suites/ui`, cloud-only behaviour in `suites/cloud`. Add a scenario to the
   feature file that already owns the behaviour, and a runner to an existing block that fits
   it. A new block, a new block file (a new combination of components) or a new overlay is the
   last resort, with the reason written in a comment above it.

3. **Clean up everything a test creates.** Register each resource with the cleanup registry as
   it is created, as the actor that created it, including what a browser click creates.
   A cleanup failure in the log is a leak, never noise.

4. **Assert exactly what the entry expects.** Never widen an assertion (`401 || 403`) or assert
   the product's current behaviour in place of the expected one without the feature owner's
   decision. Pair every negative assertion
   with a positive control. A scenario the product does not pass yet is tagged `@known-issue`
   with its issue in the list entry, and the runner that hosts it excludes `~@known-issue`.

5. **Never harm shared cloud resources.** Cloud scenarios run against a shared organisation:
   create only what the scenario owns, restore any shared setting a scenario writes (the
   organisation's gateway configuration, for example), and never delete what the run did not
   create.

6. **No deferring behind a comment.** Never resolve a missing cleanup registration, a missing
   list entry or a known product failure with a `TODO`/`FIXME` comment, a skipped scenario
   without `@known-issue` and an issue, or a looser assertion.

> **Before outputting a test change:**
> * Does every new or changed scenario carry the ID of an entry in its scenario list (added in this change if new, with the next free ID), and is the entry's status and `refs` updated?
> * Is the scenario in the feature file that owns the behaviour, and its runner in an existing block that fits? If a new block, block file or overlay was added, is the reason in a comment above it?
> * Is every resource the scenario creates, including through the browser, registered for cleanup?
> * Are the assertions exact, with a positive control for each negative one, and is every `@known-issue` scenario excluded by its runner and linked to an issue?
> * For the cloud suite: does the scenario leave every shared resource as it found it?
