---
name: feature-usage-tracking
description: Decide whether a new or changed UI control in portals/api-control-plane is a feature to track, add it to the API Platform feature taxonomy (src/featureUsage/api-platform-taxonomy.yaml), regenerate, and wire the one trackFeature call so its usage reaches Moesif. Use whenever a button, confirm dialog, wizard step, copy action, page or dialog that commits something is added or changed in the console, and when asked to "track", "instrument", "add feature usage" or "add to the taxonomy".
---

# Feature-usage tracking — api-control-plane

Run this skill alongside `apicp-ui` whenever UI work adds or changes a control. It ends in one of
two outcomes, both reported back to the user:

- **Not a feature:** say which controls were evaluated and why each is not tracked. No files change.
- **A feature:** the taxonomy entry is added, `taxonomy.generated.ts` is regenerated, the control
  calls `trackFeature`, and every check passes.

The tracking machinery (`collector.ts`, `actions.ts`, `featureMetadata`) never changes for a new
feature. Read `src/featureUsage/README.md` for the full background; this file is the procedure.

## 1. List the controls the change adds

From the diff (or the component being written), list every control a user can act on: buttons, menu
items, confirm dialogs, wizard Continue/Create steps, copy buttons, toggles, links, tabs, and pages
or dialogs that open.

## 2. Decide, per control, whether it is a feature

Check the existing taxonomy first: `grep -n '<words from the action>' src/featureUsage/api-platform-taxonomy.yaml`.

| The control… | Decision |
| --- | --- |
| Commits work: create, save, update, delete, deploy, publish, attach, revoke, regenerate, enable/disable | **Track** — one feature per action (`adoption`/`engagement`/`lifecycle`) |
| Opens a feature's main flow: a wizard, a create page, a catalogue whose use matters | **Track** as `discovery`, once per open |
| Moves past a step of a multi-step flow, with a choice the funnel needs | **Track** as `engagement`, with the choice as a dimension |
| Copies a setup artifact that signals activation (an install command, a cURL command) | **Track** as `engagement` |
| Does the **same action** as an existing feature from another place | **Don't add a feature.** Reuse the code; if the place matters, add a value to its `entry_point` dimension |
| Is a **variant** of an existing feature (REST vs GraphQL, another gateway type) | **Don't add a feature.** Reuse the code; add an enum value to the dimension |
| Navigates (links, breadcrumbs, sidebar, opening a list item), searches, filters, sorts, pages, toggles a view, types into a field | **Don't track** |
| Cancels or closes without committing, or runs automatically on page load | **Don't track** |
| Copies a URL, a key, an ID, or opens documentation | **Don't track** |

A control the taxonomy has no UI category for, or one that belongs to the API Portal or AI
Workspace, is not this console's feature: stop and say so.

## 3. Write the taxonomy entry

Edit `src/featureUsage/api-platform-taxonomy.yaml` (the WSO2 feature-taxonomy template, schema v1).
Put the entry next to its category's other features. Copy the closest existing entry and change it.

```yaml
  - code: "apip.<category>.<feature-action>"
    name: "<Title Case Name>"
    description: "<What the user does, in one sentence.>"
    category: "<category>"
    tier: "core"
    status: "stable"
    availability:
      introduced_in: "<product.version at the top of the file>"
    tracking:
      event_name: "apip.<category>.<feature-action>"   # identical to code
      event_category: "<discovery|engagement|adoption|abandonment|lifecycle>"
      platforms: ["web"]
      dimensions:
        - name: "portal"                                 # always first; always present
          type: "enum"
          values: ["api-control-plane"]
          required: true                                 # false only when platforms is ["web", "api"]
          description: "The API Platform UI the action was taken in."
        - name: "<dimension>"                            # only what the UI knows, as an enum
          type: "enum"
          values: ["<value>", "<value>"]
          required: <true|false>
      aggregations:
        - type: "count"
          label: "<Title Case Name>"
```

Rules:

- **Code** is `apip.<category>.<feature-action>`, feature first, lower-case and hyphenated. The
  second segment is the category id from the file's `categories:` section (`manage-apis`, `gateway`,
  `policies`, `security`, `lifecycle`, `monetization`, `admin`, …). It must not exist already.
- **`-click` suffix:** if the click calls a Platform API endpoint that changes something, the UI
  feature is `<endpoint-feature>-click` (`api-delete-click`); the bare name (`api-delete`) is the
  endpoint feature, reported by Platform API, `platforms: ["api"]`. Don't add or change endpoint
  features here; if the endpoint has none yet, tell the user it may need one.
- **`portal`** is what makes it a console feature: the console tracks exactly the features whose
  `portal` values include `api-control-plane`. Never add it to an API-only feature.
- **Dimensions are enums only**, with values the UI itself produces (a tab, a mode, a step) — never
  user input, names, IDs, URLs or server data. Lower-case, hyphenated values. A dimension describes a
  variant of the same action; a different action is a different feature.
- **Codes and enum values are permanent** once shipped: add new ones, never rename.

## 4. Regenerate

```bash
npm run taxonomy:codegen
```

This rewrites `src/featureUsage/taxonomy.generated.ts` (never edit it by hand). If it refuses — a
console dimension that isn't an enum, a `portal` on a non-web feature, an unknown `portal` value —
fix the YAML and rerun.

## 5. Wire the call

In the component, at the moment the action happens:

```tsx
import { useTrackFeature } from '@/featureUsage';

const trackFeature = useTrackFeature();
// …
trackFeature('apip.<category>.<feature-action>', { <dimension>: '<value>' });
```

| Step | Fire |
| --- | --- |
| Discovery (open) | Once, when the page or dialog opens: a `useEffect` guarded by a `useRef` (see `openReported` in `apis/create/ApiCreationWizard.tsx`) |
| Engagement (a choice) | When the user moves past the step, with what they chose, including an untouched default |
| Commit (the click) | On the click that commits the work, after client-side validation, before the request |
| Copy | After the clipboard write succeeds |

Never fire in render, in a query, or in a mutation's `onSuccess`, and never pass `portal`, a user or
an organization: the hook and `featureMetadata` add them. If several places open the same feature,
pass `entry_point` through router state (see `apis/create/entryPoint.ts`).

**Policies:** attaching a policy is reported by `trackPolicyAttachment` (`src/featureUsage/policies.ts`),
mapped from the policy's name. A new policy only needs its name added to the capability feature's
`policy_name` values. A new capability is a new `apip.policies.<capability>-attach` feature with a
`policy_name` dimension; only a capability with a single policy also needs an entry in
`SINGLE_POLICY_FEATURES` in `taxonomy.ts`.

## 6. Verify

```bash
npm run typecheck
npx vitest run src/featureUsage
npm run lint
```

`src/featureUsage` tests check that:

- `taxonomy.generated.ts` matches the YAML (`taxonomy.generated.test.ts`) — rerun step 4 if not;
- `portal` is used correctly;
- every console feature has a call site (`coverage.test.ts`) — a feature added in step 3 without
  step 5 fails here.

Don't add page tests that assert tracking; that belongs in the UI suite (`tests/framework/suites/ui`).

## 7. Document and report

- Add a row to "What the console tracks" in `src/featureUsage/README.md`.
- If `platform-api/internal/featureusage/taxonomy/api-platform-taxonomy.yaml` exists in this
  checkout, copy the updated YAML there too: Platform API embeds the same taxonomy.
- Report to the user: each control from step 1 with its decision and reason, the code added, its
  dimensions, where it fires, and the check results.
