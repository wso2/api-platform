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

import { describe, expect, it } from 'vitest';

import { routes } from '../routes/paths';
import { makeConsoleScope } from '../test/mockScope';
import { findTargetAtScope, navigationRegistry } from './navigationRegistry';
import { anOrganization, aProject } from '@/test/msw/fixtures';

const definitionFor = (id: string) => {
  const item = navigationRegistry.find((entry) => entry.id === id);
  if (!item) throw new Error(`No navigation item ${id}`);
  return item;
};

const matcherFor = (id: string) => {
  const { match } = definitionFor(id);
  if (!match) throw new Error(`No match predicate for ${id}`);
  return match;
};

const ORG = `/organizations/${anOrganization().id}`;
const PROJECT = `${ORG}/projects/${aProject().id}`;
const API = `${PROJECT}/apis/api-1`;
const SELECT = `${ORG}/select-scope`;

/** Scope as the shell sees it at each depth, for exercising `to`. */
const atOrg = () =>
  makeConsoleScope({
    isApiScope: false,
    isProjectScope: false,
    organizations: [anOrganization()],
    params: { orgHandle: anOrganization().id },
    project: undefined,
  });
const atProject = () => makeConsoleScope();
const atApi = () =>
  makeConsoleScope({
    isApiScope: true,
    params: {
      apiHandler: 'api-1',
      orgHandle: anOrganization().id,
      projectHandler: aProject().id,
    },
  });
const GRAPHQL_API = `${PROJECT}/graphql-apis/graphql-api-1`;
const atGraphqlApi = () =>
  makeConsoleScope({
    isGraphQLApiScope: true,
    params: {
      graphqlApiHandler: 'graphql-api-1',
      orgHandle: anOrganization().id,
      projectHandler: aProject().id,
    },
  });

/*
 * The sidebar has one item per *concern*, not per scope: Overview is the summary
 * of wherever you are, and the API-level items follow the API you're in. These
 * tests cover the two halves that makes possible — `to` picking the right page
 * for the current scope, and `match` keeping the item highlighted on every page
 * it can reach.
 */
describe('Overview adapts to the deepest scope', () => {
  it('links to the organization home when nothing deeper is selected', () => {
    expect(definitionFor('overview').to(atOrg())).toBe(`${ORG}/home`);
  });

  it('links to the project home once a project is selected', () => {
    expect(definitionFor('overview').to(atProject())).toBe(`${PROJECT}/home`);
  });

  it('links to the API overview once an API is open', () => {
    expect(definitionFor('overview').to(atApi())).toBe(API);
  });

  // A GraphQL API sets `params.graphqlApiHandler`, never `params.apiHandler`
  // (see `graphqlApiPath`'s doc comment), so the 'api' tier needs its own
  // `graphqlTo` to resolve — without it, Overview fell through to the
  // project tier and landed on the project's own overview instead.
  it('links to the GraphQL API overview once one is open, not the project overview', () => {
    expect(definitionFor('overview').to(atGraphqlApi())).toBe(GRAPHQL_API);
  });

  // Opening a project or an API navigates into a deeper tier of this same item,
  // so Overview has to stay lit rather than handing off to another item.
  it.each([
    ['organization home', `${ORG}/home`],
    ['project home', `${PROJECT}/home`],
    ['api overview', API],
    ['graphql api overview', GRAPHQL_API],
  ])('stays active on the %s page', (_name, pathname) => {
    expect(matcherFor('overview')(pathname)).toBe(true);
  });

  it('does not light up for pages belonging to other items', () => {
    const match = matcherFor('overview');
    expect(match(`${ORG}/projects`)).toBe(false);
    expect(match(`${ORG}/gateways`)).toBe(false);
    expect(match(`${API}/deploy`)).toBe(false);
    expect(match(`${PROJECT}/apis`)).toBe(false);
  });

  // Its shallowest tier is the organization, which the shell always has — so
  // unlike the API-level items it never needs a scope picker to fall back on.
  it('never needs a scope-less alias', () => {
    for (const scope of [atOrg(), atProject(), atApi()]) {
      expect(definitionFor('overview').to(scope)).not.toContain('select-scope');
    }
  });
});

describe('scope visibility', () => {
  it('hides Projects inside a project, where Overview already covers it', () => {
    const { isVisible } = definitionFor('projects');
    expect(isVisible?.(atOrg())).toBe(true);
    expect(isVisible?.(atProject())).toBe(false);
    expect(isVisible?.(atApi())).toBe(false);
  });

  // Capability gating applies only once an API is loaded: with none, every
  // capability reads false, which would hide these items in exactly the state
  // where they are the way in.
  it.each(['develop', 'test', 'deploy'])(
    'keeps %s visible out of API scope and lets the capability decide within it',
    (id) => {
      const { isVisible } = definitionFor(id);
      expect(isVisible?.(atOrg())).toBe(true);
      expect(isVisible?.(atProject())).toBe(true);
      expect(isVisible?.(atApi())).toBe(false); // no component loaded -> unsupported
    },
  );

  // `hasUsageInsights`/`hasRuntimeLogs` are false for API_PROXY, the dominant
  // kind here, so gating on them would hide these on the APIs they are for.
  it.each(['insights', 'observability', 'publish'])('%s is not capability-gated', (id) => {
    expect(definitionFor(id).isVisible).toBeUndefined();
  });
});

describe('API-level items', () => {
  // Items that are pages in their own right, as opposed to submenu parents.
  const LEAF_ITEMS: [string, string][] = [['deploy', 'deploy']];

  it.each(LEAF_ITEMS)('%s anchors to its own scoped path', (id, suffix) => {
    const match = matcherFor(id);
    expect(match(`${API}/${suffix}`)).toBe(true);
    expect(match(`${API}/${suffix}/extra`)).toBe(false);
    expect(match(`/${suffix}`)).toBe(false);
  });

  // Clicked from a shallower scope, these link to the page's scope-less alias so
  // its `ScopeGate` can ask for what's missing — and stay highlighted there.
  it.each(LEAF_ITEMS)('%s links to, and matches, its aliases', (id, suffix) => {
    const { match, to } = definitionFor(id);
    expect(to(atOrg())).toBe(`${SELECT}/${suffix}`);
    expect(to(atProject())).toBe(`${PROJECT}/select-scope/${suffix}`);
    expect(to(atApi())).toBe(`${API}/${suffix}`);
    expect(match?.(`${SELECT}/${suffix}`)).toBe(true);
    expect(match?.(`${PROJECT}/select-scope/${suffix}`)).toBe(true);
  });

  // Dropping `apis/<handle>` as a pair is what keeps an API whose handle matches
  // a page suffix reachable: `.../apis/deploy` is that API, not the Deploy page.
  it('leaves an API handled like a page suffix reachable', () => {
    expect(matcherFor('overview')(`${PROJECT}/apis/deploy`)).toBe(true);
    expect(matcherFor('deploy')(`${PROJECT}/apis/deploy`)).toBe(false);
  });

  // Pins the fix for a real bug: Deploy had no `graphqlTo` at all, so opening
  // it from a GraphQL API resolved through the REST-only `apiLevelTo`, which
  // ignores `graphqlApiHandler` and falls back to REST's scope-less alias —
  // a page for a completely different (and unrelated) API, not this one.
  it('links to the GraphQL API deploy page, not REST’s scope-less alias', () => {
    expect(definitionFor('deploy').to(atGraphqlApi())).toBe(`${GRAPHQL_API}/deploy`);
    expect(matcherFor('deploy')(`${GRAPHQL_API}/deploy`)).toBe(true);
  });
});

// Pins the fix for a real gap: Publish had no `graphqlTo` on its 'api' tier,
// so a GraphQL API fell through to the project-level Portals page instead of
// its own — inconsistent with every other API-level item once one exists.
describe('Publish resolves to the GraphQL API-level page', () => {
  it('links to GraphqlPublishPage once a GraphQL API is open, not the project Portals page', () => {
    expect(definitionFor('publish').to(atGraphqlApi())).toBe(`${GRAPHQL_API}/publish`);
  });

  it('stays active on the GraphQL publish page', () => {
    expect(matcherFor('publish')(`${GRAPHQL_API}/publish`)).toBe(true);
  });
});

/*
 * Test and Observability have no page of their own: in API scope they
 * open a submenu, and outside it they lead to the first child's `ScopeGate`. The
 * split of responsibilities that makes that work is what these tests pin —
 * the parent owns the scope-less aliases, each child owns its scoped path, and
 * the two never claim active at the same time.
 */
describe('submenu parents', () => {
  const SUBMENUS: [string, string, [string, string][]][] = [
    [
      'develop',
      'develop',
      [
        ['develop-policies', 'policies'],
        ['develop-definition', 'definition'],
        ['develop-documents', 'documents'],
      ],
    ],
    [
      'observability',
      'observability',
      [
        ['observability-metrics', 'metrics'],
        ['observability-logs', 'logs'],
      ],
    ],
    [
      'insights',
      'insights',
      [
        ['insights-api', 'api'],
        ['insights-compliance', 'compliance'],
      ],
    ],
  ];

  it.each(SUBMENUS)('%s lists its children in order', (id, _base, children) => {
    expect(definitionFor(id).children?.map((child) => child.id)).toEqual(
      children.map(([childId]) => childId),
    );
  });

  it.each(SUBMENUS)('%s only offers them in API scope', (id) => {
    expect(definitionFor(id).requires).toBe('api');
  });

  // The parent is a link only until scope resolves; in API scope the sidebar
  // drops the link and a click expands instead, so this target stops being used.
  it.each(SUBMENUS)('%s links to its first child while out of scope', (id, base, children) => {
    const [, firstSuffix] = children[0];
    const { to } = definitionFor(id);
    expect(to(atOrg())).toBe(`${SELECT}/${base}/${firstSuffix}`);
    expect(to(atProject())).toBe(`${PROJECT}/select-scope/${base}/${firstSuffix}`);
  });

  // Highlighted while the ScopeGate is asking, and only then: once scope
  // resolves the child takes over, which is also what Oxygen expects — it leaves
  // an expanded parent unhighlighted and marks the active child instead.
  it.each(SUBMENUS)('%s matches every child alias and no scoped page', (id, base, children) => {
    const match = matcherFor(id);
    for (const [, suffix] of children) {
      expect(match(`${SELECT}/${base}/${suffix}`)).toBe(true);
      expect(match(`${PROJECT}/select-scope/${base}/${suffix}`)).toBe(true);
      expect(match(`${API}/${base}/${suffix}`)).toBe(false);
    }
  });

  it.each(SUBMENUS)('%s children own their scoped path alone', (id, base, children) => {
    const parent = definitionFor(id);
    for (const [childId, suffix] of children) {
      const child = parent.children?.find((entry) => entry.id === childId);
      expect(child, `${id} has no child ${childId}`).toBeDefined();
      expect(child?.to(atApi())).toBe(`${API}/${base}/${suffix}`);
      expect(child?.match?.(`${API}/${base}/${suffix}`)).toBe(true);
      // The parent covers the aliases; a child claiming them too would light
      // both rows up at once on the scope-gate page.
      expect(child?.match?.(`${SELECT}/${base}/${suffix}`)).toBe(false);
      expect(child?.match?.(`${API}/${base}/${suffix}/extra`)).toBe(false);
    }
  });

  it('keeps children out of the top-level list', () => {
    const topLevel = navigationRegistry.map((item) => item.id);
    for (const [, , children] of SUBMENUS) {
      for (const [childId] of children) {
        expect(topLevel).not.toContain(childId);
      }
    }
  });

  it('gives every item and sub-item a unique id', () => {
    const ids = navigationRegistry.flatMap((item) => [
      item.id,
      ...(item.children ?? []).map((child) => child.id),
    ]);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('organization-level items', () => {
  it('anchors Projects to the list, and to a project it links into', () => {
    const match = matcherFor('projects');
    expect(match(`${ORG}/projects`)).toBe(true);
    // Clicking a card navigates to the project home; the item stays lit until
    // `isVisible` drops it at project scope.
    expect(match(`${PROJECT}/home`)).toBe(true);
    expect(match('/projects')).toBe(false);
  });

  it('covers the gateway list, create and detail pages', () => {
    const match = matcherFor('gateways');
    expect(match(`${ORG}/gateways`)).toBe(true);
    expect(match(`${ORG}/gateways/new`)).toBe(true);
    expect(match(`${ORG}/gateways/gw-1`)).toBe(true);
    expect(match(`${ORG}/gateways/gw-1/extra`)).toBe(false);
  });

  /*
   * Manage replaced the Settings link: all of its pages are organization-scoped,
   * so it points at the organization's tabs at every scope and never gates.
   */
  it('points at the organization-level pages at every scope', () => {
    const { to, children } = definitionFor('manage');
    expect(to(atOrg())).toBe(`${ORG}/subscription-plans`);
    expect(to(atProject())).toBe(`${ORG}/subscription-plans`);
    expect(children?.map((child) => child.id)).toEqual([
      'manage-subscription-plans',
      'manage-meters',
      'manage-service-accounts',
    ]);
    expect(children?.[1].to(atApi())).toBe(`${ORG}/meters`);
    expect(children?.[2].to(atApi())).toBe(`${ORG}/service-accounts`);
  });

  it('lights the page entry, not the Manage parent, on either page', () => {
    const manage = definitionFor('manage');
    expect(manage.match?.(`${ORG}/subscription-plans`)).toBe(false);
    const [plans, meters] = manage.children ?? [];
    expect(plans.match?.(`${ORG}/subscription-plans`)).toBe(true);
    expect(plans.match?.(`${ORG}/meters`)).toBe(false);
    expect(meters.match?.(`${ORG}/meters`)).toBe(true);
    expect(meters.match?.(`${ORG}/settings`)).toBe(false);
  });

  it('lights Service Accounts on its own page only', () => {
    const accounts = definitionFor('manage').children?.find(
      (child) => child.id === 'manage-service-accounts',
    );
    expect(accounts?.match?.(`${ORG}/service-accounts`)).toBe(true);
    expect(accounts?.match?.(`${ORG}/subscription-plans`)).toBe(false);
    expect(accounts?.isVisible?.(atOrg()) ?? true).toBe(true);
  });
});

/*
 * The header's org/project/API switchers used to always send the user to the
 * target scope's Home page, regardless of what page they were on — picking a
 * project while on the organization's Settings tab landed on the project's
 * Overview instead of carrying Settings down with it, and clearing the
 * project back out landed on the organization's Overview rather than back on
 * its own Settings. `findTargetAtScope` re-resolves whatever page the user is
 * actually on at the target scope, in either direction.
 */
describe('findTargetAtScope', () => {
  it('keeps the user on a Manage tab when dropping a project to its organization', () => {
    const target = findTargetAtScope(`${ORG}/meters`, atProject(), {
      orgHandle: anOrganization().id,
    });
    expect(target).toBe(`${ORG}/meters`);
  });

  it('keeps the user on an API-only page when dropping its API, via the scope-less alias', () => {
    const target = findTargetAtScope(`${API}/deploy`, atApi(), {
      orgHandle: anOrganization().id,
      projectHandler: aProject().id,
    });
    expect(target).toBe(`${PROJECT}/select-scope/deploy`);
  });

  it('returns undefined for a URL no registry item claims, so callers can fall back to Home', () => {
    expect(
      findTargetAtScope('/not-a-real-page', atOrg(), { orgHandle: anOrganization().id }),
    ).toBeUndefined();
  });
});

describe('Manage menu', () => {
  it('shows Meters in the sidebar', () => {
    const meters = definitionFor('manage').children?.find((child) => child.id === 'manage-meters');
    expect(meters?.isVisible?.(atOrg()) ?? true).toBe(true);
    expect(meters?.to(atOrg())).toBe(`${ORG}/meters`);
  });

  it('shows Subscription Plans', () => {
    const plans = definitionFor('manage').children?.find(
      (child) => child.id === 'manage-subscription-plans',
    );
    expect(plans?.isVisible?.(atOrg()) ?? true).toBe(true);
  });

  it('sits in the same section as Insights and Observability, after them', () => {
    const [insights, observability, manage] = ['insights', 'observability', 'manage'].map(
      definitionFor,
    );
    expect(insights.group).toBe(manage.group);
    expect(observability.group).toBe(manage.group);
    expect(insights.order).toBeLessThan(observability.order);
    expect(observability.order).toBeLessThan(manage.order);
  });

  it('keeps Insights and Observability out of the Develop to Publish section', () => {
    const develop = definitionFor('develop');
    expect(definitionFor('insights').group).not.toBe(develop.group);
    expect(definitionFor('observability').group).not.toBe(develop.group);
    expect(definitionFor('publish').group).toBe(develop.group);
  });
});

describe('sidebar structure', () => {
  it('carries no section headings, only divider clusters', () => {
    // Clusters separate; they are never rendered as text (see
    // NavigationDefinition.group), so they are keys, not labels.
    expect(new Set(navigationRegistry.map((item) => item.group))).toEqual(
      new Set(['place', 'api', 'operate']),
    );
  });

  it('orders items so clusters come out contiguous', () => {
    const byOrder = [...navigationRegistry].sort((a, b) => a.order - b.order);
    const clusterRun = byOrder
      .map((item) => item.group)
      .filter((group, index, all) => group !== all[index - 1]);

    expect(clusterRun).toEqual(['place', 'api', 'operate']);
  });

  it('leaves `level` to extensions, which need it for path building', () => {
    expect(navigationRegistry.every((item) => item.level === undefined)).toBe(true);
  });

  it('gives every item a route builder and a matcher', () => {
    for (const item of navigationRegistry) {
      expect(item.match, `${item.id} has no match`).toBeTypeOf('function');
      expect(item.to(atApi()), `${item.id} has no target`).toBeTruthy();
    }
  });
});

describe('every page routes.* builds is anchored, not a bare suffix', () => {
  it('requires the org prefix everywhere', () => {
    for (const item of navigationRegistry) {
      const target = item.to(atApi());
      expect(target?.startsWith('/organizations/')).toBe(true);
      // A matcher must not fire on the same path without its org/project prefix.
      expect(item.match?.(target!.replace(ORG, ''))).toBe(false);
    }
  });

  // The create page sits in the `:apiHandler` slot (`.../apis/new`), so
  // Overview's API tier claims it — which reads correctly in the sidebar, since
  // creating an API belongs to the API area and lands on its overview. Worth
  // pinning: it is a consequence of the URL shape, not a decision, and the same
  // overlap makes every API-level item treat `new` as a handle.
  it('leaves the new-api page under Overview', () => {
    const newApi = routes.newApi(anOrganization().id, aProject().id);
    const owners = navigationRegistry.filter((item) => item.match?.(newApi)).map((item) => item.id);

    expect(owners).toEqual(['overview']);
  });
});
