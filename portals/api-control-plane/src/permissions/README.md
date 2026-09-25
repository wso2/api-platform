# Permissions (`src/permissions`)

What the signed-in user may be **offered**. Every action in the console is keyed
on its OpenAPI `operationId`, and the accepted `ap:*` scopes for that operation
are generated from `platform-api/resources/openapi.yaml`.

> **This is UX, not security.** The Platform API's scope enforcement decides
> whether an operation actually happens. Everything here shapes affordances, and
> is written on the assumption it can be wrong: uncertain cases fail _open_, and
> the 403 path must always work. Never treat a check here as a control.

## Vocabulary

Two different things in this codebase are called "scope". Keep them apart:

| Term           | Means                                     | Lives in                   |
| -------------- | ----------------------------------------- | -------------------------- |
| **permission** | an `ap:*` OAuth2 scope: may you _do_ this | `src/permissions`          |
| **scope**      | the org / project / API tier in the URL   | `src/scope`, `useApiScope` |

`ScopeGate` asks "which project?"; `Can` asks "may you?". Both can sit on the
same page and they are not related.

## How it resolves

```
openapi.yaml ──generated──► operationScopes.ts     Record<operationId, ApScope[]>  (any-of)
                                    │
GET /api/session ─► user.scopes ─► PermissionProvider ─► can('CreateProject')
   (BFF decodes the token: scope                         │
    claim, or roles expanded via                         ▼
    the grant table in role mode)           useCan · Can · useActionPermission
```

Nothing is fetched. The scopes arrive with the session, already resolved by the
BFF, so there is no request to make and no cache to invalidate.

## Using it

Ask for the **operation**, never for a scope string:

```tsx
useCan('CreateProject'); // ✅ generated any-of list, includes ap:project:manage
useHasScope('ap:project:create'); // ❌ locks out an org admin who holds only :manage
```

Three call shapes cover almost everything:

```tsx
// Hide — a control pointing at something the user cannot see anyway.
<Can do="CreateProject"><Button …>New project</Button></Can>

// Disable + tooltip — an action on a row already on screen.
<Can do="DeleteProject" denied="disable"><Button …>Delete</Button></Can>

// Substitute — a whole page body.
<Can do="ListProjects" fallback={<ForbiddenState />}><Body /></Can>

// Any-of — a menu trigger, hidden when nothing inside it is reachable.
<Can do={['DeployAPI', 'UndeployDeployment']}><IconButton …/></Can>
```

`denied="disable"` clones the child with `disabled` and wraps it in a tooltip, so
it works for any control taking that prop (Button, IconButton, MenuItem, Switch).
The child must be a single element; anything else throws rather than render a
control the user is not entitled to use.

There is deliberately no `PermissionButton` / `PermissionMenuItem` /
`PermissionGate`. Those differ only in which control they wrap, and that list
never ends. What genuinely differs is how a denial should _read_, so that is the
prop.

### Hooks

| Hook                      | Use for                                                                             |
| ------------------------- | ----------------------------------------------------------------------------------- |
| `useCan(op)`              | the common boolean                                                                  |
| `useCanAny([op, …])`      | is anything in this group reachable                                                 |
| `useActionPermission(op)` | `{ allowed, reason, required, tooltip }` for a bespoke control                      |
| `useHasScope(scope)`      | an override scope that maps to no single operation                                  |
| `usePermissions()`        | `mode`, `isLoading`, and callable predicates for use inside a `useMemo` over a list |

`usePermissions().can` exists because `useNavigationItems` maps over the
navigation registry inside a `useMemo`, where a hook per item is impossible.

## Hide, disable, or substitute

This is a product decision, not a per-page judgement. The rule:

| Surface                                             | Treatment                                  |
| --------------------------------------------------- | ------------------------------------------ |
| Action on a visible resource (Edit, Delete, Deploy) | **disable + tooltip**                      |
| Row overflow menu item                              | **disable + tooltip**                      |
| Overflow menu with no allowed item                  | **hide the trigger** (`useCanAny`)         |
| Create entry point (New API, New gateway)           | **hide**                                   |
| Page body with no read scope                        | **`ForbiddenState`**, never an empty table |
| Sidebar item / settings tab with no read scope      | **hide**                                   |
| Deep link to a forbidden page                       | **substitute, don't redirect**             |
| Anything still loading                              | **treat as allowed**                       |

Why disable rather than hide for a visible row: the user can see the thing, so a
button that silently vanishes reads as a broken console, while a disabled one
teaches the permission model and gives support something to act on. Why
`ForbiddenState` rather than an empty list: "no gateways exist" and "you may not
see gateways" are different facts, and rendering them identically teaches users
to distrust every empty table.

Never hide the only route to a page the user _can_ read.

## Modes, and the `ui-permissions` flag

| `mode`                 | Behaviour when the token carries **no scope claim** |
| ---------------------- | --------------------------------------------------- |
| `permissive` (default) | offer everything, let the server decide             |
| `enforce`              | offer nothing                                       |

`enforce` is switched on by adding `ui-permissions` to `FEATURE_FLAGS`. Absent
it, the console runs permissive, which is the safe rollout posture and the
correct steady state for a backend with authorization disabled.

A backend in **role mode** needs no special handling here. Set the BFF's
`[api_control_plane.auth.authorization]` to `mode = "role"` with the same
`role_to_scope_mapping` file the Platform API reads. The BFF then expands the
token's roles into `ap:*` scopes before `/api/session` returns them, so
`user.scopes` holds the same effective scopes the Platform API enforces against.
Leave the BFF in scope mode against a role-based IdP and `user.scopes` holds only
the OIDC scopes, so every gated action shows as denied.

Note the distinction the whole layer rests on: `scopes: undefined` means "no
scope claim" and defers to `mode`; `scopes: []` means "nothing granted" and
denies. Collapsing them blanks the console on a role-mode deployment.

## When the server disagrees

The transport publishes every 403 on `onForbidden` with the operation that drew
it. Two things consume that:

- The toast a user sees uses `permissionMessages.denied`, the same sentence as
  the tooltip, so a predicted denial and a server denial read identically.
- `PermissionProvider` checks whether it had predicted that operation was
  _allowed_. If so, the generated map and the deployed backend have diverged, and
  it logs a dev-only warning naming the operation. **If you see that warning, run
  `npm run api:codegen` against the matching spec version.**

A denial the console predicted is not reported: that is the system working.

## Regenerating the map

```bash
npm run api:codegen        # platform.d.ts + operationScopes.ts, together
```

Both are committed and CI-checked. The map is
`satisfies Record<keyof operations, readonly ApScope[]>`, so an operation added to
the spec but missing from the map fails `tsc`. The generator also refuses to emit
when the spec breaks an assumption the evaluator depends on: more than one
security requirement object per operation, an undeclared scope, or a narrow scope
listed without its sibling `:manage` where one exists.

## What does not belong here

**Row-level ownership.** API keys are creator-scoped: the server compares
`created_by` against the caller unless they hold `ap:api_key:all:manage`. No scope
set can answer "is this row mine" — that needs the row. Put those predicates in
the owning resource module and have them _consume_ `useHasScope`; do not push
them into `useCan`, which answers a different question.

## Testing

`evaluate.ts` is pure, so prefer testing it directly. For components, mount
`PermissionProvider` with an explicit mode over an auth state carrying the scopes
under test:

```tsx
renderWithProviders(<PermissionProvider mode="enforce">{ui}</PermissionProvider>, {
  authState: makeAuthState({ user: { name: 'T', email: 't@e.com', scopes: ['ap:project:read'] } }),
});
```

Gate a control on the scope it actually needs and assert both directions; a test
that only covers the allowed path will not notice a control that is never
reachable. `resetPermissionWarnings()` clears the warn-once memo between specs.

## Not yet wired

Navigation and settings tabs do not consult permissions yet: `requiresOperation`
on `NavigationDefinition`, the filter in `useNavigationItems`, and the equivalent
for sidebar/tab extensions are still to come. Until then a sidebar item stays
visible regardless of scope, and the page it opens is responsible for its own
`Can` gate.
