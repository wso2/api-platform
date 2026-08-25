/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { ReactNode } from 'react';
import {
  Activity,
  BellRing,
  ChartColumn,
  ChartLine,
  CircleDollarSign,
  Code,
  ClipboardList,
  FileCheck,
  FileText,
  Gauge,
  GitBranch,
  Home,
  Layers,
  MessagesSquare,
  Network,
  Rocket,
  Route,
  ScrollText,
  Settings,
  ShieldCheck,
  SquareTerminal,
  Terminal,
} from '@wso2/oxygen-ui-icons-react';

import type { ApiCapabilities } from '../pages/appShell/appShellPages/apis/utils/apiCapabilities';
import {
  apiScopeSelectPaths,
  apiScopedPaths,
  routes,
  type ApiPathBuilder,
  type ScopedPathBuilder,
} from '../routes/paths';
import type { ConsoleRouteParams } from '../scope/ConsoleScopeContext';
import type { NavigationDefinition, NavigationLevel } from './navigationTypes';

/**
 * Divider-separated clusters, in sidebar order. Keys are never displayed — the
 * sidebar renders no headings (see `NavigationDefinition.group`).
 */
const CLUSTER = {
  /** Where you are and what's alongside it: Overview, Projects, Gateways. */
  place: 'place',
  /** What you can do to the API you're in. */
  api: 'api',
  /** Reachable at any scope. */
  global: 'global',
} as const;

/**
 * Turns a route pattern into an anchored full-path regex: regex metacharacters
 * are escaped, then each `:param` becomes a single-segment wildcard. So
 * `/organizations/:orgHandle/projects/:projectHandler/settings` yields
 * `^/organizations/[^/]+/projects/[^/]+/settings$`.
 */
const toRouteRegex = (pattern: string): RegExp =>
  new RegExp(
    `^${pattern
      .replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      .replace(/:[A-Za-z][A-Za-z0-9]*/g, '[^/]+')}$`
  );

/**
 * Builds a `match` predicate from the same `routes.*` builders an item links to,
 * called with their `:param` defaults.
 *
 * Hand-writing `match` alongside `to` means maintaining the same path twice, and
 * the two had already drifted: `settings` matched a bare `/\/settings$/`, so any
 * future org- or api-level settings page would light up the *project* Settings
 * item, and `runtime-logs` matched `/\/observe\/runtimelogs$/` with no
 * org/project segments at all. Deriving both from one builder makes that class
 * of drift impossible — a renamed route updates the highlight for free.
 */
const matchRoutes = (...patterns: string[]) => {
  const regexes = patterns.map(toRouteRegex);
  return (pathname: string) => regexes.some((regex) => regex.test(pathname));
};

/** `to` for an org-level item — always linkable inside the app shell. */
const orgLevelTo =
  (build: (orgHandle: string) => string): NavigationDefinition['to'] =>
  ({ params }) =>
    params.orgHandle ? build(params.orgHandle) : undefined;

/**
 * `to` for an API-level item, falling back to the page's scope-less alias when
 * the project or API is missing.
 *
 * The item stays clickable at every scope: the alias mounts the same page, whose
 * `ScopeGate` prompts for the missing handles and then navigates to the fully
 * scoped URL. Returning `undefined` — the old behaviour, paired with a filter
 * that hid the item — meant an org-level page offered no route into any
 * API-level feature at all.
 */
const apiLevelTo =
  (build: ApiPathBuilder): NavigationDefinition['to'] =>
  ({ params }) =>
    params.orgHandle
      ? build(
          params.orgHandle,
          params.projectHandler ?? null,
          params.apiHandler ?? null
        )
      : undefined;

/** One entry in a submenu: its own id, label, icon and page. */
type SubItem = {
  icon: ReactNode;
  id: string;
  label: string;
  to: ApiPathBuilder;
};

/**
 * A child of a submenu parent: an ordinary API-level item, one nesting level down.
 *
 * `match` is the fully-scoped path only. The parent owns the scope-less aliases
 * (see `submenu` below), so exactly one of the two is ever active.
 */
const subItem = ({ icon, id, label, to }: SubItem): NavigationDefinition => ({
  icon,
  id,
  label,
  // Children render in the order their parent lists them; `order` only sorts
  // top-level items, so it plays no part here.
  order: 0,
  to: apiLevelTo(to),
  match: matchRoutes(to(':orgHandle', ':projectHandler', ':apiHandler')),
});

/**
 * The `to`/`match`/`children` of a submenu parent — an item that opens rather
 * than navigates.
 *
 * A parent has no page of its own, so:
 *
 * - `to` is its **first child's** target. In API scope the sidebar drops the link
 *   entirely and a click expands instead (Oxygen's `Sidebar.Item` switches to
 *   `onToggleExpand` as soon as it has nested children), so this only ever
 *   resolves out of scope — where it points at that child's scope-less alias and
 *   the child page's `ScopeGate` asks for an API.
 * - `match` covers every child's aliases and nothing else, so the parent stays
 *   highlighted on the scope-gate page and hands the highlight to the child once
 *   scope resolves. Oxygen leaves an expanded parent unhighlighted by design.
 */
const submenu = (
  items: SubItem[]
): Pick<NavigationDefinition, 'children' | 'match' | 'requires' | 'to'> => ({
  children: items.map(subItem),
  match: matchRoutes(...items.flatMap((item) => apiScopeSelectPaths(item.to))),
  requires: 'api',
  to: apiLevelTo(items[0].to),
});

/** One page an adaptive item points at, plus the scope that page needs. */
type ScopeTier = { level: NavigationLevel; to: ScopedPathBuilder };

const LEVEL_DEPTH: Record<NavigationLevel, number> = {
  organization: 0,
  project: 1,
  api: 2,
};

const isLevelInScope = (level: NavigationLevel, params: ConsoleRouteParams) => {
  if (level === 'api') return Boolean(params.projectHandler && params.apiHandler);
  if (level === 'project') return Boolean(params.projectHandler);
  return Boolean(params.orgHandle);
};

/** The tier's route pattern, calling its builder with the handles its level takes. */
const tierPattern = ({ level, to }: ScopeTier): string => {
  if (level === 'api') return to(':orgHandle', ':projectHandler', ':apiHandler');
  if (level === 'project') return to(':orgHandle', ':projectHandler');
  return to(':orgHandle');
};

/**
 * One sidebar item pointing at a different page per scope: the deepest tier the
 * route can satisfy wins.
 *
 * This is what lets a single **Overview** item mean "the summary of wherever you
 * are" — the organization at org scope, the project once you pick one, the API
 * once you open one. Clicking a project card or an API card navigates into the
 * deeper tier, and because `match` covers every tier, Overview simply stays
 * highlighted rather than handing off to a different item.
 *
 * Note what it does *not* do: an adaptive item whose shallowest tier is
 * `organization` never needs a `ScopeGate`, because there is always some tier it
 * can satisfy. It degrades instead of prompting. Tiers are only ever called with
 * handles the route already has, which is why `ScopedPathBuilder` takes no
 * `null`.
 *
 * Returns both `to` and `match` so a spread wires them together and they can't
 * drift apart:
 *
 * ```tsx
 * { id: 'overview', ...adaptive([{ level: 'api', to: routes.api }, ...]) }
 * ```
 */
const adaptive = (
  tiers: ScopeTier[]
): Pick<NavigationDefinition, 'match' | 'to'> => {
  const deepestFirst = [...tiers].sort(
    (left, right) => LEVEL_DEPTH[right.level] - LEVEL_DEPTH[left.level]
  );

  return {
    match: matchRoutes(...tiers.map(tierPattern)),
    to: ({ params }) => {
      if (!params.orgHandle) return undefined;
      const tier = deepestFirst.find((candidate) =>
        isLevelInScope(candidate.level, params)
      );
      return tier?.to(
        params.orgHandle,
        params.projectHandler,
        params.apiHandler
      );
    },
  };
};


/**
 * Capability gating for an API-level item, applied only once an API is actually
 * in scope.
 *
 * With no API loaded every capability is `false` (see `getApiCapabilities`), so
 * a bare `({ capabilities }) => capabilities.canDeploy` would hide Deploy/Test/
 * Manage from the sidebar in exactly the state where the user needs them as a
 * way in. Out of API scope the item shows and leads to the scope picker; in API
 * scope the capability still decides, so an API that can't be deployed has no
 * Deploy item.
 */
const apiCapability =
  (
    isSupported: (capabilities: ApiCapabilities) => boolean
  ): NonNullable<NavigationDefinition['isVisible']> =>
  ({ capabilities, isApiScope }) =>
    !isApiScope || isSupported(capabilities);

export const navigationRegistry: NavigationDefinition[] = [
  {
    id: 'overview',
    label: 'Overview',
    group: CLUSTER.place,
    order: 10,
    icon: <Home />,
    // The summary of wherever you are. Opening a project or an API navigates
    // into a deeper tier of this same item rather than to a different one.
    ...adaptive([
      { level: 'api', to: routes.api },
      { level: 'project', to: routes.projectHome },
      { level: 'organization', to: routes.organizationHome },
    ]),
  },
  {
    id: 'projects',
    label: 'Projects',
    group: CLUSTER.place,
    order: 20,
    icon: <Layers />,
    // Inside a project this is redundant with Overview, and switching projects
    // is the header switcher's job.
    isVisible: ({ isProjectScope }) => !isProjectScope,
    to: orgLevelTo(routes.projects),
    match: matchRoutes(routes.projects(), routes.projectHome()),
  },
  {
    id: 'gateways',
    label: 'Gateways',
    group: CLUSTER.place,
    order: 30,
    icon: <Network />,
    to: orgLevelTo(routes.gateways),
    match: matchRoutes(routes.gateways(), routes.newGateway(), routes.gateway()),
  },
  {
    id: 'develop',
    label: 'Develop',
    group: CLUSTER.api,
    order: 35,
    icon: <Code />,
    isVisible: apiCapability(({ canDevelop }) => canDevelop),
    ...submenu([
      {
        icon: <ShieldCheck />,
        id: 'develop-policies',
        label: 'Policies',
        to: routes.apiDevelopPolicies,
      },
      {
        icon: <Route />,
        id: 'develop-routing',
        label: 'Routing',
        to: routes.apiDevelopRouting,
      },
      {
        icon: <FileText />,
        id: 'develop-documents',
        label: 'Documents',
        to: routes.apiDevelopDocuments,
      },
    ]),
  },
  {
    id: 'test',
    label: 'Test',
    group: CLUSTER.api,
    order: 40,
    icon: <Terminal />,
    isVisible: apiCapability(({ canTest }) => canTest),
    ...submenu([
      {
        icon: <SquareTerminal />,
        id: 'test-console',
        label: 'API Console',
        to: routes.apiTestConsole,
      },
      {
        icon: <Terminal />,
        id: 'test-curl',
        label: 'Curl',
        to: routes.apiTestCurl,
      },
      {
        icon: <MessagesSquare />,
        id: 'test-chat',
        label: 'API Chat',
        to: routes.apiTestChat,
      },
    ]),
  },
  {
    id: 'deploy',
    label: 'Deploy',
    group: CLUSTER.api,
    order: 50,
    icon: <Rocket />,
    isVisible: apiCapability(({ canDeploy }) => canDeploy),
    to: apiLevelTo(routes.apiDeploy),
    match: matchRoutes(...apiScopedPaths(routes.apiDeploy)),
  },
  {
    // No capability gate, unlike its neighbours: `hasUsageInsights` is false for
    // kinds this console shows by default, so gating on it would hide Insights on
    // exactly the APIs it is meant for. Same for Observability below.
    id: 'insights',
    label: 'Insights',
    group: CLUSTER.api,
    order: 60,
    icon: <ChartColumn />,
    ...submenu([
      {
        icon: <ChartLine />,
        id: 'insights-api',
        label: 'API Insights',
        to: routes.apiInsightsApi,
      },
      {
        icon: <FileCheck />,
        id: 'insights-compliance',
        label: 'Compliance',
        to: routes.apiInsightsCompliance,
      },
    ]),
  },
  {
    id: 'observability',
    label: 'Observability',
    group: CLUSTER.api,
    order: 70,
    icon: <Activity />,
    ...submenu([
      {
        icon: <ScrollText />,
        id: 'observability-logs',
        label: 'Logs',
        to: routes.apiObservabilityLogs,
      },
      {
        icon: <BellRing />,
        id: 'observability-alerts',
        label: 'Alert',
        to: routes.apiObservabilityAlerts,
      },
      {
        icon: <Gauge />,
        id: 'observability-metrics',
        label: 'Metrics',
        to: routes.apiObservabilityMetrics,
      },
    ]),
  },
  {
    id: 'manage',
    label: 'Manage',
    group: CLUSTER.api,
    order: 80,
    icon: <ClipboardList />,
    isVisible: apiCapability(({ canManage }) => canManage),
    ...submenu([
      {
        icon: <CircleDollarSign />,
        id: 'manage-monetize',
        label: 'Monetize',
        to: routes.apiManageMonetize,
      },
      {
        icon: <GitBranch />,
        id: 'manage-lifecycle',
        label: 'LifeCycle',
        to: routes.apiManageLifecycle,
      },
    ]),
  },
  {
    id: 'admin',
    label: 'Admin',
    group: CLUSTER.api,
    order: 90,
    icon: <ShieldCheck />,
    to: apiLevelTo(routes.apiAdmin),
    match: matchRoutes(...apiScopedPaths(routes.apiAdmin)),
  },
  {
    // The one page with no scope requirement at all, hence its own cluster.
    id: 'settings',
    label: 'Settings',
    group: CLUSTER.global,
    order: 100,
    icon: <Settings />,
    // Follows you down one level: the organization's settings while browsing the
    // org, that project's settings once you are inside one — one pinned link at a
    // time, never both. Same page either way; only the scope it reads differs.
    ...adaptive([
      { level: 'project', to: routes.projectSettings },
      { level: 'organization', to: routes.settings },
    ]),
  },
];
