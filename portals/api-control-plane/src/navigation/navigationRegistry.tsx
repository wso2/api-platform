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
  Braces,
  ChartColumn,
  ChartLine,
  Code,
  FileCheck,
  FileText,
  Gauge,
  Home,
  Layers,
  Network,
  PanelTop,
  Rocket,
  ScrollText,
  Settings,
  ShieldCheck,
  FlaskConical,
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
      .replace(/:[A-Za-z][A-Za-z0-9]*/g, '[^/]+')}$`,
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
 *
 * `graphqlBuild`, when given, is resolved instead while a GraphQL API is in
 * scope — mirrors `subItem`'s own `graphqlTo` branch. Omit it for a REST-only
 * item with no GraphQL sibling page.
 */
const apiLevelTo =
  (build: ApiPathBuilder, graphqlBuild?: ApiPathBuilder): NavigationDefinition['to'] =>
  ({ params }) => {
    if (!params.orgHandle) return undefined;
    if (graphqlBuild && params.graphqlApiHandler) {
      return graphqlBuild(params.orgHandle, params.projectHandler ?? null, params.graphqlApiHandler);
    }
    return build(params.orgHandle, params.projectHandler ?? null, params.apiHandler ?? null);
  };

/** One entry in a submenu: its own id, label, icon and page. */
type SubItem = {
  icon: ReactNode;
  id: string;
  label: string;
  to: ApiPathBuilder;
  /**
   * This child's GraphQL sibling route, resolved instead of `to` whenever a
   * GraphQL API is in scope (`params.graphqlApiHandler` set). Omit for a
   * REST-only concept with no GraphQL equivalent (e.g. per-operation
   * Resources) — pair that with `hideInGraphqlScope` so the item disappears
   * rather than linking to a REST page while browsing a GraphQL API.
   */
  graphqlTo?: ApiPathBuilder;
  /** Hide this child entirely while a GraphQL API is in scope. */
  hideInGraphqlScope?: boolean;
};

/**
 * A child of a submenu parent: an ordinary API-level item, one nesting level down.
 *
 * `match` is the fully-scoped path only (both the REST and, when given, the
 * GraphQL one). The parent owns the scope-less aliases (see `submenu` below),
 * so exactly one of the two is ever active.
 */
const subItem = ({ icon, id, label, to, graphqlTo, hideInGraphqlScope }: SubItem): NavigationDefinition => ({
  icon,
  id,
  label,
  // Children render in the order their parent lists them; `order` only sorts
  // top-level items, so it plays no part here.
  order: 0,
  isVisible: hideInGraphqlScope ? ({ isGraphQLApiScope }) => !isGraphQLApiScope : undefined,
  to: ({ params }) => {
    if (!params.orgHandle) return undefined;
    if (graphqlTo && params.graphqlApiHandler) {
      return graphqlTo(params.orgHandle, params.projectHandler ?? null, params.graphqlApiHandler);
    }
    return to(params.orgHandle, params.projectHandler ?? null, params.apiHandler ?? null);
  },
  match: matchRoutes(
    to(':orgHandle', ':projectHandler', ':apiHandler'),
    ...(graphqlTo ? [graphqlTo(':orgHandle', ':projectHandler', ':graphqlApiHandler')] : []),
  ),
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
  items: SubItem[],
): Pick<
  NavigationDefinition,
  'children' | 'match' | 'requires' | 'revealsForGraphqlApi' | 'to'
> => ({
  children: items.map(subItem),
  // Scope-less REST aliases only — never a fully-scoped path, GraphQL's
  // included: once scope resolves (REST or GraphQL) the matching child claims
  // the highlight instead (see `subItem`'s own `match`), same as the REST
  // parent/child split this mirrors. GraphQL has no scope-less alias at all
  // (see `graphqlApiPath`), so there is nothing further to add here for it.
  match: matchRoutes(...items.flatMap((item) => apiScopeSelectPaths(item.to))),
  requires: 'api',
  revealsForGraphqlApi: items.some((item) => item.graphqlTo),
  // In GraphQL scope with no page of its own (mirroring the REST branch
  // below), the parent points at its first GraphQL-capable child — not
  // necessarily items[0], which may be a REST-only entry with no
  // `graphqlTo` (e.g. Develop's "Resources"). Out of any API scope this is
  // moot: `apiLevelTo` degrades to items[0]'s scope-less alias regardless.
  to: (scope) => {
    const { params } = scope;
    if (params.graphqlApiHandler && params.orgHandle) {
      const firstGraphqlCapable = items.find((item) => item.graphqlTo);
      if (firstGraphqlCapable?.graphqlTo) {
        return firstGraphqlCapable.graphqlTo(
          params.orgHandle,
          params.projectHandler ?? null,
          params.graphqlApiHandler,
        );
      }
    }
    return apiLevelTo(items[0].to)(scope);
  },
});

/** One page an adaptive item points at, plus the scope that page needs. */
type ScopeTier = {
  level: NavigationLevel;
  to: ScopedPathBuilder;
  /**
   * This tier's GraphQL sibling route, used instead of `to` whenever a
   * GraphQL API is in scope (`params.graphqlApiHandler` set, no
   * `params.apiHandler` — see `graphqlApiHandler`'s doc comment on
   * `ConsoleRouteParams`) — only meaningful on the `'api'` tier. Omit when
   * the item has no GraphQL-side page at that tier (e.g. Portals): a GraphQL
   * API then falls through to the next-shallowest tier, exactly as it
   * already does for a page with no `'api'` tier at all.
   */
  graphqlTo?: ApiPathBuilder;
};

const LEVEL_DEPTH: Record<NavigationLevel, number> = {
  organization: 0,
  project: 1,
  api: 2,
};

const isLevelInScope = (tier: ScopeTier, params: ConsoleRouteParams) => {
  if (tier.level === 'api') {
    return Boolean(
      params.projectHandler &&
        (params.apiHandler || (tier.graphqlTo && params.graphqlApiHandler)),
    );
  }
  if (tier.level === 'project') return Boolean(params.projectHandler);
  return Boolean(params.orgHandle);
};

/** The tier's route pattern(s) — both REST and, when given, GraphQL. */
const tierPatterns = (tier: ScopeTier): string[] => {
  if (tier.level === 'api') {
    const patterns = [tier.to(':orgHandle', ':projectHandler', ':apiHandler')];
    if (tier.graphqlTo) {
      patterns.push(tier.graphqlTo(':orgHandle', ':projectHandler', ':graphqlApiHandler'));
    }
    return patterns;
  }
  if (tier.level === 'project') return [tier.to(':orgHandle', ':projectHandler')];
  return [tier.to(':orgHandle')];
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
const adaptive = (tiers: ScopeTier[]): Pick<NavigationDefinition, 'match' | 'to'> => {
  const deepestFirst = [...tiers].sort(
    (left, right) => LEVEL_DEPTH[right.level] - LEVEL_DEPTH[left.level],
  );

  return {
    match: matchRoutes(...tiers.flatMap(tierPatterns)),
    to: ({ params }) => {
      if (!params.orgHandle) return undefined;
      const tier = deepestFirst.find((candidate) => isLevelInScope(candidate, params));
      if (!tier) return undefined;
      if (tier.graphqlTo && params.graphqlApiHandler && !params.apiHandler) {
        return tier.graphqlTo(params.orgHandle, params.projectHandler ?? null, params.graphqlApiHandler);
      }
      return tier.to(params.orgHandle, params.projectHandler, params.apiHandler);
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
    isSupported: (capabilities: ApiCapabilities) => boolean,
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
    // `graphqlTo` lets the 'api' tier resolve for a GraphQL API too — see
    // `ScopeTier`'s doc comment.
    ...adaptive([
      { level: 'api', to: routes.api, graphqlTo: routes.graphqlApi },
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
        graphqlTo: routes.graphqlApiDevelopPolicies,
      },
      {
        icon: <Braces />,
        id: 'develop-definition',
        label: 'Definition',
        to: routes.apiDevelopDefinition,
      },
      {
        // No GraphQL analog: a GraphQL API has one endpoint, not a
        // per-operation resource list.
        icon: <List />,
        id: 'develop-routing',
        label: 'Resources',
        to: routes.apiDevelopRouting,
        hideInGraphqlScope: true,
      },
      {
        icon: <FileText />,
        id: 'develop-documents',
        label: 'Documents',
        to: routes.apiDevelopDocuments,
        graphqlTo: routes.graphqlApiDevelopDocuments,
      },
    ]),
  },
  {
    id: 'test',
    label: 'Test',
    group: CLUSTER.api,
    order: 40,
    icon: <FlaskConical />,
    isVisible: apiCapability(({ canTest }) => canTest),
    // A leaf, not a parent: the console, the cURL builder and the response all
    // live on one page, so there is nothing to disclose beneath it.
    // `graphqlTo` sends a GraphQL API to its own test console page instead.
    ...adaptive([{ level: 'api', to: routes.apiTest, graphqlTo: routes.graphqlApiTestConsole }]),
  },
  {
    id: 'deploy',
    label: 'Deploy',
    group: CLUSTER.api,
    order: 50,
    icon: <Rocket />,
    isVisible: apiCapability(({ canDeploy }) => canDeploy),
    to: apiLevelTo(routes.apiDeploy, routes.graphqlApiDeploy),
    match: matchRoutes(
      ...apiScopedPaths(routes.apiDeploy),
      routes.graphqlApiDeploy(':orgHandle', ':projectHandler', ':graphqlApiHandler'),
    ),
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
        graphqlTo: routes.graphqlApiInsightsApi,
      },
      {
        icon: <FileCheck />,
        id: 'insights-compliance',
        label: 'Compliance',
        to: routes.apiInsightsCompliance,
        graphqlTo: routes.graphqlApiInsightsCompliance,
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
        icon: <Gauge />,
        id: 'observability-metrics',
        label: 'Metrics',
        to: routes.apiObservabilityMetrics,
        graphqlTo: routes.graphqlApiObservabilityMetrics,
      },
      {
        icon: <ScrollText />,
        id: 'observability-logs',
        label: 'Logs',
        to: routes.apiObservabilityLogs,
        graphqlTo: routes.graphqlApiObservabilityLogs,
      },
    ]),
  },
  {
    // "Publish this API to a portal": the API-level counterpart of the org-level
    // portal registry, which lives in the cloud-plugin sidebar as "Portals".
    // Shows at every scope. Out of API scope it links to the scope-less alias so
    // `PortalsPage`'s ScopeGate can walk the user down to an API.
    id: 'publish',
    label: 'Publish',
    group: CLUSTER.api,
    order: 80,
    icon: <PanelTop />,
    // The GraphQL API-level page is `GraphqlPublishPage`, not a new "Portals"
    // page of its own: `PortalsPage` (REST's `apiPortals` target) is a bare,
    // contextless "coming soon" with no per-API content, and
    // `GraphqlPublishPage` is that same placeholder for a GraphQL API (see
    // its own doc comment) — reusing it is the more faithful match.
    ...adaptive([
      { level: 'api', to: routes.apiPortals, graphqlTo: routes.graphqlApiPublish },
      { level: 'project', to: routes.projectPortals },
      { level: 'organization', to: routes.organizationPortals },
    ]),
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
