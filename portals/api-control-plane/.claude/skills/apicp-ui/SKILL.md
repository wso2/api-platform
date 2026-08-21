---
name: apicp-ui
description: Build or change UI in portals/api-control-plane — pages, components, forms, dialogs, listings, app shell/navigation, theming, and MUI→Oxygen migration. Covers the Oxygen UI (@wso2/oxygen-ui) component/theming API plus this app's own rules for data access (hooks only), i18n (react-intl), routing/scope gating, and tests. Use for any .tsx/.ts work under portals/api-control-plane/src, and whenever asked how a page, listing, form, sidebar item, or theme override should be built here.
---

# UI development — api-control-plane

One skill for all UI work in `portals/api-control-plane`. It replaces the four generated
`oxygen-*` skills and `.claude/oxygen-ui/*.md`: everything load-bearing from those is condensed
below, and the full vendor reference is still on disk (see *Where the truth lives*).

## Where the truth lives

Verify before you invent. In priority order:

| Question | Source |
| --- | --- |
| Does this component/prop/sub-component exist? | `node_modules/@wso2/oxygen-ui/dist/**/*.d.ts` — the compiled types, ground truth |
| Full Oxygen component/pattern/theming reference | `node_modules/@wso2/oxygen-ui/.ai/{components,patterns,theming,migration}.md` (shipped with the package; identical to the docs previously copied into `.claude/oxygen-ui/`) |
| Which icon names exist | `node_modules/@wso2/oxygen-ui-icons-react/dist/index.d.ts` → 8 brand icons + `export * from "lucide-react"` |
| Data-access layer rules | `src/api/README.md` |
| i18n rules | `src/i18n/README.md` |
| Test conventions | `src/test/README.md` |
| Layer bans, i18n lint rules | `eslint.config.js` — the restrictions are documented inline there |
| Online / upstream | <https://github.com/wso2/oxygen-ui> (`packages/oxygen-ui/.ai/*.md`, `src/components/**`). No hosted docs site exists as of v0.13.1 — fetch the repo, not a guessed URL. |

Vendor docs lag the package. Two known errors in them — **do not copy**:
- `<Grid item xs={12}>` → this app is on `@mui/material` v9 Grid v2: `<Grid size={{ xs: 12, md: 4 }}>`, no `item`.
- `HomeIcon`/`TrashIcon` naming → this codebase uses bare lucide names (`Home`, `Trash2`, `Plus`). Both aliases resolve; stay with the bare form.
- They also omit components that do exist (`AppBreadcrumbs`, `PageTitle.Actions`, `PageTitle.BackButton`, `ColorSchemeSVG`).

`npx @wso2/oxygen-ui init --claude` / `update --claude` regenerates `.claude/oxygen-ui/` and the
`oxygen-*` skills. If someone runs it, delete the regenerated files again — this skill is the
project's single entry point, and `node_modules/@wso2/oxygen-ui/.ai/` already carries the reference.

## Non-negotiables

1. **Every source file starts with the Apache-2.0 header** (`Copyright (c) 2026, WSO2 LLC.` block — copy it from any neighbouring file).
2. **All components come from `@wso2/oxygen-ui`**, never `@mui/material`. All icons from `@wso2/oxygen-ui-icons-react`, never `lucide-react`/`@mui/icons-material`.
3. **Backend access goes through a hook in `src/api/resources/<resource>`** — nothing else. ESLint blocks endpoints, queries, query keys, transport, generated types and legacy clients.
4. **No user-facing literal strings.** `<FormattedMessage>` for JSX, `intl.formatMessage` for string props. IDs are `apiControlPlane.<area>.<Module>.<slug>`.
5. **No colour, radius, blur, shadow or border literals.** Theme tokens (`bgcolor: 'background.paper'`, `color: 'text.secondary'`, `spacing` numbers) or a recipe from `src/theme/receipes.ts`. Local `sx` is for layout only.
6. **Scoped pages wrap their body in `ScopeGate`** (see *Routing, scope and navigation*).
7. **Never “resolve” a rule from `.claude/rules/*` with a `// TODO`.** Those rules (XSS/output encoding, dependency management, error handling) apply to this portal's `.ts`/`.tsx` too.

Verification loop, in order: `npm run typecheck` → `npm run lint` → `npm run test` → `npm run i18n`
(commit `src/i18n/messages/`, never `src/i18n/compiled/`).

## Project map

```
src/
  api/            resources/<name>/{endpoints,queries,hooks}.ts + core/ (http, scope, spec, queryKeys)
  components/     app-wide: StateViews (Loading/Empty/Error), ConfirmDialog, Notifications,
                  ErrorBoundary, AppLoader, ComingSoon, cards/*, common/*
  contexts/auth/  AuthProvider + AuthStateContext
  hooks/          cross-cutting hooks (ProductActivation)
  i18n/           I18nProvider, useLocale, useFormatters, formats.ts, messages/ (source catalogs)
  navigation/     navigationRegistry.tsx (the sidebar), navigationTypes, useNavigationItems
  pages/
    auth/         LoginPage, AuthCallbackPage
    appShell/     AppLayout, AppHeader, AppSidebar, *QuickSelector
      appShellPages/<feature>/  the actual pages (+ components/, utils/ per feature)
  routes/         paths.ts (route builders), AppRoutes.tsx, ProtectedRoute
  scope/          ConsoleScopeProvider/Context, ScopeGate, consoleRouteParams
  slots/          Slot + Hideable extension primitives
  theme/          AppThemeProvider, themes.ts, receipes.ts, emotionCache
  test/           renderWithProviders, MSW server + toolkit, mock scope/auth
  extensions.tsx  extension registration; hostPort.tsx  the value handed to extensions
```

Provider order (`App.tsx`, outermost first): `I18nProvider` → `AppThemeProvider` →
`NotificationProvider` → `AppQueryProvider` → `ApiClientProvider` → `ErrorBoundary` →
`BrowserRouter` → `AuthProvider` → `ExtensionsProvider` → `AppRoutes`. Inside a protected route:
`ConsoleScopeProvider` → `AppLayout`. Don't add a provider without a reason that names its position.

---

## Oxygen UI essentials

### Imports

```tsx
import { Box, Button, Card, PageTitle, Stack, Typography } from '@wso2/oxygen-ui';
import { Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';   // lucide names, size={18}
import { DataGrid, DatePickers, TreeView, AdapterDateFns } from '@wso2/oxygen-ui'; // MUI X namespaces
```

`@wso2/oxygen-ui` re-exports **all** of `@mui/material` plus Oxygen's own components, `styled`,
`alpha`, `useTheme`, `Theme`. MUI X ships as namespaces: `<DataGrid.DataGrid …>`,
`<DatePickers.DatePicker …>`, `<TreeView.SimpleTreeView>`. Charts live in
`@wso2/oxygen-ui-charts-react`, which is **not installed here** — adding it is a dependency change
(`.claude/rules/js-dependency-management.md`).

### Oxygen's own components (beyond MUI)

| Component | Sub-components / notes |
| --- | --- |
| `AppShell` | `.Navbar` `.Sidebar` `.Main` `.Footer` `.NotificationPanel`; props `initialCollapsed`, `collapseOnSelectOnMobile`. Wired once in `AppLayout` — pages never touch it. |
| `Header` | `.Toggle` `.Brand` `.BrandLogo` `.BrandTitle` `.Switchers` `.Actions` `.Spacer`; `minimal` hides switchers |
| `Sidebar` | `.Nav` `.Category` `.CategoryLabel` `.Item` `.ItemIcon` `.ItemLabel` `.ItemBadge` `.Footer` `.User*`; props `collapsed`, `activeItem`, `expandedMenus`, `onSelect`, `onToggleExpand`. An `Item` with nested children toggles instead of navigating. |
| `Footer` | `.Copyright` `.Version` `.Divider` (left) · `.Link` (right) |
| `UserMenu` | `.Trigger` `.Header` `.Item` `.Logout` `.Divider` |
| `NotificationPanel` | `.Header{,Icon,Title,Badge,Close}` `.Tabs` `.Actions` `.List` `.Item{,Avatar,Title,Message,Timestamp,Action}` `.EmptyState` |
| `PageTitle` | `.Header` `.SubHeader` `.Avatar` `.Link` `.Actions` `.BackButton` — the standard page heading |
| `PageContent` | padding/max-width wrapper; `fullWidth`. Already applied by `AppLayout`, so a page starts at its own content. |
| `AppBreadcrumbs` | `items: BreadcrumbItem[]`; rendered by `AppLayout` from route scope |
| `ListingTable` | `.Provider` `.Container` `.Toolbar` `.Head` `.Body` `.Footer` `.Row` `.Cell` `.SortLabel` `.RowActions` `.CellIcon` `.EmptyState` `.DensityControl`; `variant='table'\|'card'`, `density`, `striped`, `bordered`. Prefer over raw MUI `Table`. |
| `Form` | `.Section` `.Header` `.Subheader` `.Body` `.Stack` `.ElementWrapper` `.Wizard` `.CardButton` `.Card{Header,Content,Actions,Media}` |
| `ComplexSelect` | `.MenuItem` + `.MenuItem.Icon` / `.MenuItem.Text` — icon+text options inside a `Select` |
| `SearchBar`, `SearchBarWithAdvancedFilter`, `StatCard`, `CodeBlock`, `ColorSchemeToggle`, `ColorSchemeImage`, `ColorSchemeSVG`, `ThemeSwitcher`, `NotificationBanner`, `ParticleBackground`, `Layout` (`.Navbar` `.Sidebar` `.Content` `.Header`) | single-purpose; check the `.d.ts` for props |

Hooks: `useTheme`, `useThemeSwitcher`, `useThemeContent`, `useAppShell`, `useNotifications`
(Oxygen's own — not this app's `src/components/Notifications`), `useHeader`, `useSidebar`,
`useNotificationPanel`, `useListingTable` / `useListingTableRequired`.
Utils: `formatRelativeTime`, `pxToRem`, `alpha`.

### Theming

`AppThemeProvider` (`src/theme/`) wraps `OxygenUIThemeProvider` with a one-entry registry —
`AcrylicOrangeTheme` — plus the app's emotion cache and `<CssBaseline />`. Don't add a second theme
or call `OxygenUIThemeProvider` anywhere else.

Three tiers, in order of preference:

1. **Theme** — global decisions owned by Oxygen (`theme.palette.*`, `theme.typography.*`, `theme.border.*`, `theme.zIndex.*`, `theme.oxygen.*` for blur/gradient/glass/syntax).
2. **Recipes** — `src/theme/receipes.ts`: `hairline(theme)`, `glassSurfaceSx(theme)`, `interactiveCardSx`, `stickyBottomBarSx(theme)`, `overlayBarShadow`. A repeated multi-property treatment goes here, once.
3. **Local `sx`** — layout only: flex, gap, grid, min/max sizing.

```tsx
<Card sx={(theme) => ({ ...glassSurfaceSx(theme), display: 'flex', gap: 2 })} />
```

Dark mode: the theme is CSS-variable based (`--oxygen-*`, `data-color-scheme` attribute), so token
usage adapts for free. Read the mode with `useTheme().palette.mode` only when behaviour (not colour)
must branch.

---

## Page patterns

### Page skeleton

```tsx
export function ThingListPage() {
  // Gate the whole body, not just the JSX: out of scope the query stays disabled
  // and `isPending` never clears, so a loading branch would hang forever.
  return (
    <ScopeGate
      prompt="Things are created and managed at the project level."
      requires="project"
      to={routes.things}
    >
      <ThingList />
    </ScopeGate>
  );
}

function ThingList() {
  const intl = useIntl();
  const thingsQuery = useThings();
  const { notify } = useNotifications();

  // `isPending`, not `isLoading` — a disabled query has isLoading=false with no
  // data, which would flash the empty state.
  if (thingsQuery.isPending) return <LoadingState label="Loading things" />;
  if (thingsQuery.error) return <ErrorState message="Unable to load things" />;

  const things = thingsQuery.data?.list ?? [];

  return (
    <>
      <PageTitle>
        <PageTitle.Header><FormattedMessage {...messages.title} /></PageTitle.Header>
        <PageTitle.SubHeader><FormattedMessage {...messages.subtitle} /></PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button startIcon={<Plus />} variant="contained" onClick={create}>
            <FormattedMessage {...messages.create} />
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {things.length === 0 ? (
        <EmptyState
          title={intl.formatMessage(messages.emptyTitle)}
          description={intl.formatMessage(messages.emptyBody)}
          actionLabel={intl.formatMessage(messages.create)}
          onAction={create}
        />
      ) : (
        <Stack spacing={2}>{/* toolbar row, then grid/list */}</Stack>
      )}
    </>
  );
}
```

State views come from `src/components/StateViews` — `LoadingState({label, fullScreen})`,
`EmptyState({title, description, actionLabel, onAction})`, `ErrorState({title, message})`.
Never hand-roll a spinner or an error `Alert`.

Reference implementations to copy from: `apis/ApiListPage.tsx` (list + search + grid/list toggle +
delete), `apis/ApiDetailPage.tsx` (tabbed detail), `gateways/GatewayCreatePage.tsx` (create flow),
`apis/overview/ApiKeysPanel.tsx` (`ListingTable`), `projects/NewProjectDialog.tsx` (dialog form).

### Feedback and confirmation

```tsx
const { notify } = useNotifications();            // src/components/Notifications
notify('Deleted "Orders API".', 'success');       // 'success' | 'info' | 'warning' | 'error'
```

Mutation errors already surface globally (the QueryClient's `onMutationError` notifies), so a
per-call `onError` is for *better* copy, never for making the error visible at all.

Destructive actions use `ConfirmDialog` (`src/components/ConfirmDialog`), with `destructive` and —
for irreversible deletes — `confirmPhrase` + `confirmInputLabel` (type-the-name):

```tsx
<ConfirmDialog
  open={toDelete !== null}
  title="Delete API"
  message={`This permanently deletes "${toDelete?.displayName}" and all related details.`}
  confirmLabel="Delete"
  confirmPhrase={toDelete?.displayName ?? ''}
  destructive
  loading={mutation.isPending}
  onCancel={() => setToDelete(null)}
  onConfirm={confirmDelete}
/>
```

### Forms

Controlled state + `error`/`helperText`; group with `Form.Section` / `Form.Stack`; multi-step with
`Form.Wizard` (or `Stepper` where a wizard is overkill). Labels and helper text go through
`intl.formatMessage`. Field errors from the API arrive as `ApiError.fieldErrors` — map them onto the
matching field instead of dumping the message into a banner.

```tsx
<Form.Section>
  <Form.Header><FormattedMessage {...messages.detailsHeader} /></Form.Header>
  <Form.Subheader><FormattedMessage {...messages.detailsHint} /></Form.Subheader>
  <Form.Stack spacing={2}>
    <TextField
      error={Boolean(errors.name)}
      helperText={errors.name}
      label={intl.formatMessage(messages.nameLabel)}
      onChange={(event) => setName(event.target.value)}
      required
      value={name}
    />
  </Form.Stack>
</Form.Section>
```

### Accessibility and small conventions

- A `Select` needs `labelId` pointing at its `FormLabel id` — a bare `FormLabel` leaves the combobox with no accessible name (and nothing for tests to query).
- Icon-only buttons need `aria-label` (translated).
- Icon size is explicit: `<Search size={18} />`.
- Newer files sort JSX props and `sx` keys alphabetically; follow the file you're editing.
- Truncate rather than widen: `minWidth: 0` + `overflow: 'hidden'` + `noWrap`.

---

## Data access (`src/api`)

Full rules in `src/api/README.md`. What a UI author must know:

- Import **only** hooks: `useThings()`, `useThing(id)`, `useCreateThing()`, `useUpdateThing()`, `useDeleteThing()`. Endpoints/queries/keys/http/generated types are ESLint-blocked from the UI.
- Hooks resolve org/project from route scope via `useApiScope()`; pass overrides only for a genuine cross-scope read.
- Scoped queries are `enabled`-gated, so **branch on `isPending`**, not `isLoading`.
- Errors are always `ApiError` — branch on `error.code`, never HTTP status; `fieldErrors`, `details`, `trackingId` are available. Never render `error.message` as translated copy.
- Lists: read `pagination.total`, never `list.length`.
- Adding a backend call = three files (`*.endpoints.ts`, `*.queries.ts`, `*.hooks.ts`) copied from `resources/restApis/`, plus `npm run api:codegen` if the spec changed. Never hand-write a request/response type — derive from the `operationId` with `ResponseOf`/`BodyOf`/`QueryOf`/`PathOf`.
- A legacy layer (`client.ts`, `mvpApi.ts`, `*/*Client.ts`) still serves some pages. Don't extend it; port the page.

Route scope for rendering (not fetching) comes from `useConsoleScope()`:
`{ params, activeScope, organization(s), project(s), component, capabilities, isOrganizationScope, isProjectScope, isApiScope, isLoading, projectsError }`.

---

## i18n

Full rules in `src/i18n/README.md`. The short form:

```tsx
import { defineMessages, FormattedMessage, useIntl } from 'react-intl'; // direct import, always

const messages = defineMessages({           // module scope — extraction is static
  title: { id: 'apiControlPlane.pages.apis.ApiListPage.title', defaultMessage: 'APIs' },
  nameLabel: {
    id: 'apiControlPlane.pages.apis.ApiListPage.nameLabel',
    defaultMessage: 'Name',
    description: 'Label for the API name field. Noun, not a command.',
  },
});
```

- ID = `apiControlPlane.<area>.<Module>.<slug>`; `<area>` mirrors the `src/` path (`/`→`.`, filename dropped). Renaming an ID discards its translations; `defaultMessage` may change freely.
- One sentence = one message. Never concatenate; use ICU placeholders/plurals/rich-text tags.
- Never translate backend or user data — pass it through as a *value*.
- Dates/numbers via `useFormatters()` (`shortDate`, `dateTime`, `relativeTime`) or `<FormattedDate>`/`<FormattedNumber>` — never `toLocaleString()` or a module-scope `Intl.*`.
- After changing strings: `npm run i18n`, commit `src/i18n/messages/`. `npm run lint:i18n` lists remaining hardcoded JSX (the rule is `warn`, so `npm run lint --quiet` skips it). Check layout growth with `?lang=en-XA`.

---

## Routing, scope and navigation

Three tiers of scope live in the URL: `/organizations/:orgHandle`,
`.../projects/:projectHandler`, `.../apis/:apiHandler`. A page that needs deeper scope than the
current URL still mounts — at a **scope-less alias** where `select-scope` (`SELECT_SCOPE_SEGMENT`)
replaces the missing segments — and `ScopeGate` renders a picker until the handles are filled in.
That is why every sidebar item stays clickable at every scope.

Adding a page, end to end:

1. **`src/routes/paths.ts`** — add a builder using `projectPath`/`apiPath` so the alias is generated for you:
   ```ts
   thingDetail: (orgHandle = ':orgHandle', projectHandler: ScopeHandle = ':projectHandler') =>
     projectPath(orgHandle, projectHandler, 'things/detail'),
   ```
2. **`src/routes/AppRoutes.tsx`** — `lazy()`-import the page and register every path it answers on:
   ```tsx
   {scopedRoutes(projectScopedPaths(routes.thingDetail), <ThingDetailPage />)}
   ```
   (`apiScopedPaths` for API-level pages; a single `<Route>` only when the page has no alias.)
3. **The page** wraps its body in `ScopeGate` with `requires` + `to={routes.thingDetail}`.
4. **`src/navigation/navigationRegistry.tsx`** — add the sidebar entry. Build `to`/`match` from the *same* builder via the helpers, never by hand: `orgLevelTo`, `apiLevelTo`, `matchRoutes`, `submenu([...])` for a parent with children, `adaptive([...tiers])` for one item that degrades across scopes, `apiCapability(...)` for capability gating (which only applies once an API is in scope).

Never hand-write a path string or a `match` regex: `routes.*` is the single source, and
`paths.test.ts` / `navigationRegistry.test.ts` guard the pairing.

Extension points: `Slot` / `Hideable` (`src/slots/`) — a named additive slot plus a suppressible
region for built-in UI; extensions receive a plain `CloudHostPort` value (`src/hostPort.tsx`), not a
shared context. Keep `src/slots/index.tsx` free of portal-specific types; it is copied verbatim into
other hosts.

---

## Tests

Full conventions in `src/test/README.md`.

- Colocate as `*.test.ts` / `*.test.tsx`. Always render through `renderWithProviders` (never wrap providers by hand):
  ```tsx
  const { user } = renderWithProviders(<ApiListPage />, {
    route: '/organizations/api-platform-demo/projects/retail-apis/apis',
    scope: makeConsoleScope(),
    authState: authStatePresets.authenticated(),
  });
  ```
- Mock at the network boundary with MSW, never a client or a hook. The server ships **no default handlers** and fails unhandled requests, so each test declares its endpoints via `collection` / `resource` / `accepts` / `noContent` / `failure` / `recorder` and spec-typed fixtures (`aRestApi`, `manyRestApis`). Build URLs with `apiUrl('/things')`; call `resetHttpClient()` in `beforeEach`.
- `renderWithProviders` supplies `IntlProvider` with empty messages, so assert on the English `defaultMessage`.
- Prefer `userEvent`; query by role/label; await with `findBy*`. To prove *no* request fired, await a short timeout then assert `requests.count() === 0` — `waitFor` proves nothing there.
- Hook tests go one per *shape*, not per resource; endpoint tests one per resource.

---

## Migrating existing MUI code

| From | To |
| --- | --- |
| `@mui/material`, `@mui/material/styles` (`styled`, `alpha`, `useTheme`) | `@wso2/oxygen-ui` |
| `@mui/icons-material`, `lucide-react` | `@wso2/oxygen-ui-icons-react` (bare lucide names, explicit `size`) |
| `@mui/x-data-grid` / `x-date-pickers` / `x-tree-view` | `DataGrid.*` / `DatePickers.*` / `TreeView.*` from `@wso2/oxygen-ui` |
| `ThemeProvider` + `createTheme` | nothing — `AppThemeProvider` already owns this |
| `AppBar`/`Toolbar`/`Drawer` layout | `AppShell` + `Header` + `Sidebar` (already in `AppLayout`) |
| `Table`/`TableHead`/`TableRow`/`TableCell` | `ListingTable.*` |
| `<Grid item xs={12} md={4}>` | `<Grid size={{ xs: 12, md: 4 }}>` |
| `useColorScheme()` | `<ColorSchemeToggle />`, or `useTheme().palette.mode` |
| hardcoded colours/spacing | theme tokens, or a recipe in `src/theme/receipes.ts` |
| hardcoded JSX strings | `FormattedMessage` / `intl.formatMessage` (add to `defineMessages`) |
| direct client/axios calls | a hook from `src/api/resources/<resource>` |

## Review checklist

- Apache header present; imports only from `@wso2/oxygen-ui` + `@wso2/oxygen-ui-icons-react`.
- No user-facing literal; every new message has an ID in the house format (and a `description` where a translator could misread it).
- Data via hooks; `isPending` used for the loading branch; `ApiError.code` for branching; `pagination.total` for counts.
- Scoped page wrapped in `ScopeGate`; `to` is the page's own `routes.*` builder; route registered for the alias paths too; sidebar `to`/`match` derived from the same builder.
- Colours/spacing/borders via tokens or recipes; local `sx` limited to layout.
- Loading/empty/error use `StateViews`; destructive actions use `ConfirmDialog`; feedback via `useNotifications`.
- `Select` has `labelId`; icon-only buttons have a translated `aria-label`.
- `npm run typecheck && npm run lint && npm run test` clean; `npm run i18n` run and `src/i18n/messages/` committed (never `src/i18n/compiled/`).
