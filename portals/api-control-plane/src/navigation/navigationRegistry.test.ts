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

import { organizations, projects } from '../api/mocks/data';
import { routes } from '../routes/paths';
import { makeConsoleScope } from '../test/mockScope';
import { navigationRegistry } from './navigationRegistry';

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

const ORG = `/organizations/${organizations[0].id}`;
const PROJECT = `${ORG}/projects/${projects[0].id}`;
const API = `${PROJECT}/apis/api-1`;
const SELECT = `${ORG}/select-scope`;

/** Scope as the shell sees it at each depth, for exercising `to`. */
const atOrg = () =>
  makeConsoleScope({
    isApiScope: false,
    isProjectScope: false,
    params: { orgHandle: organizations[0].id },
    project: undefined,
  });
const atProject = () => makeConsoleScope();
const atApi = () =>
  makeConsoleScope({
    isApiScope: true,
    params: {
      apiHandler: 'api-1',
      orgHandle: organizations[0].id,
      projectHandler: projects[0].id,
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

  // Opening a project or an API navigates into a deeper tier of this same item,
  // so Overview has to stay lit rather than handing off to another item.
  it.each([
    ['organization home', `${ORG}/home`],
    ['project home', `${PROJECT}/home`],
    ['api overview', API],
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
  it.each(['test', 'deploy', 'manage'])(
    'keeps %s visible out of API scope and lets the capability decide within it',
    (id) => {
      const { isVisible } = definitionFor(id);
      expect(isVisible?.(atOrg())).toBe(true);
      expect(isVisible?.(atProject())).toBe(true);
      expect(isVisible?.(atApi())).toBe(false); // no component loaded -> unsupported
    }
  );

  // `hasUsageInsights`/`hasRuntimeLogs` are false for API_PROXY, the dominant
  // kind here, so gating on them would hide these on the APIs they are for.
  it.each(['insights', 'observability', 'admin'])(
    '%s is not capability-gated',
    (id) => {
      expect(definitionFor(id).isVisible).toBeUndefined();
    }
  );
});

describe('API-level items', () => {
  // Items that are pages in their own right, as opposed to submenu parents.
  const LEAF_ITEMS: [string, string][] = [
    ['deploy', 'deploy'],
    ['admin', 'admin'],
  ];

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
});

/*
 * Test, Observability and Manage have no page of their own: in API scope they
 * open a submenu, and outside it they lead to the first child's `ScopeGate`. The
 * split of responsibilities that makes that work is what these tests pin —
 * the parent owns the scope-less aliases, each child owns its scoped path, and
 * the two never claim active at the same time.
 */
describe('submenu parents', () => {
  const SUBMENUS: [string, string, [string, string][]][] = [
    [
      'test',
      'test',
      [
        ['test-console', 'console'],
        ['test-curl', 'curl'],
        ['test-chat', 'chat'],
      ],
    ],
    [
      'observability',
      'observability',
      [
        ['observability-alerts', 'alerts'],
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
    [
      'manage',
      'manage',
      [
        ['manage-monetize', 'monetize'],
        ['manage-lifecycle', 'lifecycle'],
      ],
    ],
  ];

  it.each(SUBMENUS)('%s lists its children in order', (id, _base, children) => {
    expect(definitionFor(id).children?.map((child) => child.id)).toEqual(
      children.map(([childId]) => childId)
    );
  });

  it.each(SUBMENUS)('%s only offers them in API scope', (id) => {
    expect(definitionFor(id).requires).toBe('api');
  });

  // The parent is a link only until scope resolves; in API scope the sidebar
  // drops the link and a click expands instead, so this target stops being used.
  it.each(SUBMENUS)(
    '%s links to its first child while out of scope',
    (id, base, children) => {
      const [, firstSuffix] = children[0];
      const { to } = definitionFor(id);
      expect(to(atOrg())).toBe(`${SELECT}/${base}/${firstSuffix}`);
      expect(to(atProject())).toBe(
        `${PROJECT}/select-scope/${base}/${firstSuffix}`
      );
    }
  );

  // Highlighted while the ScopeGate is asking, and only then: once scope
  // resolves the child takes over, which is also what Oxygen expects — it leaves
  // an expanded parent unhighlighted and marks the active child instead.
  it.each(SUBMENUS)(
    '%s matches every child alias and no scoped page',
    (id, base, children) => {
      const match = matcherFor(id);
      for (const [, suffix] of children) {
        expect(match(`${SELECT}/${base}/${suffix}`)).toBe(true);
        expect(match(`${PROJECT}/select-scope/${base}/${suffix}`)).toBe(true);
        expect(match(`${API}/${base}/${suffix}`)).toBe(false);
      }
    }
  );

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

  // Settings is the one page with no scope requirement: the sidebar links to the
  // org-level path, and a project card's gear deep-links the same page.
  it('lights Settings up at either of its entry points, whatever the scope', () => {
    const { match, to } = definitionFor('settings');
    expect(to(atOrg())).toBe(`${ORG}/settings`);
    expect(to(atApi())).toBe(`${ORG}/settings`);
    expect(match?.(`${ORG}/settings`)).toBe(true);
    expect(match?.(`${PROJECT}/settings`)).toBe(true);
    expect(match?.(`${API}/settings`)).toBe(false);
  });
});

describe('sidebar structure', () => {
  it('carries no section headings, only divider clusters', () => {
    // Clusters separate; they are never rendered as text (see
    // NavigationDefinition.group), so they are keys, not labels.
    expect(new Set(navigationRegistry.map((item) => item.group))).toEqual(
      new Set(['place', 'api', 'global'])
    );
  });

  it('orders items so clusters come out contiguous', () => {
    const byOrder = [...navigationRegistry].sort((a, b) => a.order - b.order);
    const clusterRun = byOrder
      .map((item) => item.group)
      .filter((group, index, all) => group !== all[index - 1]);

    expect(clusterRun).toEqual(['place', 'api', 'global']);
  });

  it('leaves `level` to extensions, which need it for path building', () => {
    expect(navigationRegistry.every((item) => item.level === undefined)).toBe(
      true
    );
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
    const newApi = routes.newApi(organizations[0].id, projects[0].id);
    const owners = navigationRegistry
      .filter((item) => item.match?.(newApi))
      .map((item) => item.id);

    expect(owners).toEqual(['overview']);
  });
});
