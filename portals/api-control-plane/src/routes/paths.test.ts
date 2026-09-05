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

import { getRouteParamsFromPathname } from '../scope/consoleRouteParams';
import { routes, type ApiPathBuilder, type ProjectPathBuilder } from './paths';

const ORG = 'acme';
const PROJECT = 'orders';
const API = 'api-1';

// Only the pages whose scope-less aliases `AppRoutes` actually registers, i.e.
// the ones a sidebar item can link to before its scope is known. Overview's own
// tiers (`projectHome`, `api`) are absent deliberately: that item degrades to a
// shallower tier instead of linking un-scoped, so those aliases don't exist.
const PROJECT_LEVEL: [string, ProjectPathBuilder][] = [
  ['apis', routes.apis],
];

const API_LEVEL: [string, ApiPathBuilder][] = [
  ['apiDevelopPolicies', routes.apiDevelopPolicies],
  ['apiDevelopRouting', routes.apiDevelopRouting],
  ['apiDevelopDocuments', routes.apiDevelopDocuments],
  ['apiTestConsole', routes.apiTestConsole],
  ['apiTestCurl', routes.apiTestCurl],
  ['apiTestChat', routes.apiTestChat],
  ['apiDeploy', routes.apiDeploy],
  ['apiInsightsApi', routes.apiInsightsApi],
  ['apiInsightsCompliance', routes.apiInsightsCompliance],
  ['apiObservabilityAlerts', routes.apiObservabilityAlerts],
  ['apiObservabilityMetrics', routes.apiObservabilityMetrics],
  ['apiObservabilityLogs', routes.apiObservabilityLogs],
  ['apiManageMonetize', routes.apiManageMonetize],
  ['apiManageLifecycle', routes.apiManageLifecycle],
  ['apiAdmin', routes.apiAdmin],
];

/*
 * `ConsoleScopeProvider` reads scope back out of the pathname, and the sidebar's
 * scope-less aliases are URLs it has to read as "no project/API selected". If an
 * alias left a `projects` or `apis` segment with the page's suffix behind it, the
 * suffix would come back as a handle, `ScopeGate` would think scope was complete,
 * and the page would query a project that doesn't exist. These tests hold the two
 * halves — builder and parser — to that contract.
 */
describe('scope-less aliases round-trip through the scope parser', () => {
  it.each(PROJECT_LEVEL)('%s: a project-less alias has no handles', (_id, build) => {
    const params = getRouteParamsFromPathname(build(ORG, null));

    expect(params.orgHandle).toBe(ORG);
    expect(params.projectHandler).toBeUndefined();
    expect(params.apiHandler).toBeUndefined();
  });

  it.each(PROJECT_LEVEL)('%s: the scoped path still yields its project', (_id, build) => {
    expect(getRouteParamsFromPathname(build(ORG, PROJECT))).toMatchObject({
      orgHandle: ORG,
      projectHandler: PROJECT,
    });
  });

  it.each(API_LEVEL)('%s: an api-less alias keeps the project only', (_id, build) => {
    const params = getRouteParamsFromPathname(build(ORG, PROJECT, null));

    expect(params.projectHandler).toBe(PROJECT);
    expect(params.apiHandler).toBeUndefined();
  });

  it.each(API_LEVEL)('%s: a fully scope-less alias has no handles', (_id, build) => {
    const params = getRouteParamsFromPathname(build(ORG, null, null));

    expect(params.projectHandler).toBeUndefined();
    expect(params.apiHandler).toBeUndefined();
  });

  it.each(API_LEVEL)('%s: the scoped path still yields both handles', (_id, build) => {
    expect(
      getRouteParamsFromPathname(build(ORG, PROJECT, API))
    ).toMatchObject({ apiHandler: API, orgHandle: ORG, projectHandler: PROJECT });
  });

  /*
   * `newApi` is the one page whose suffix sits in a handle slot (`.../apis/new`),
   * so the alias convention above cannot protect it — the parser's reserved-segment
   * list does. Read back as a handle, `new` would name a phantom API in the header
   * switcher and breadcrumbs, turn `isApiScope` on before the API exists, and fire
   * a detail request for it.
   */
  it('newApi: the create page is not an API called "new"', () => {
    const params = getRouteParamsFromPathname(routes.newApi(ORG, PROJECT));

    expect(params.orgHandle).toBe(ORG);
    expect(params.projectHandler).toBe(PROJECT);
    expect(params.apiHandler).toBeUndefined();
  });

  it('an API legitimately handled "new" is unreachable, by design', () => {
    // Documents the trade-off rather than asserting a wish: the reserved segment
    // wins, so the backend must never mint `new` as an API handle.
    expect(
      getRouteParamsFromPathname(routes.api(ORG, PROJECT, 'new')).apiHandler
    ).toBeUndefined();
  });
});

describe('path builders', () => {
  it('keeps every alias distinct from every other page at the same scope', () => {
    const aliases = [
      ...PROJECT_LEVEL.map(([, build]) => build(ORG, null)),
      ...API_LEVEL.flatMap(([, build]) => [
        build(ORG, PROJECT, null),
        build(ORG, null, null),
      ]),
      // Pages that keep a fully-scoped path of their own.
      routes.organizationHome(ORG),
      routes.projects(ORG),
      routes.gateways(ORG),
      routes.settings(ORG),
      routes.projectSettings(ORG, PROJECT),
      routes.projectHome(ORG, PROJECT),
      routes.api(ORG, PROJECT, API),
      routes.apiEdit(ORG, PROJECT, API),
      ...PROJECT_LEVEL.map(([, build]) => build(ORG, PROJECT)),
      ...API_LEVEL.map(([, build]) => build(ORG, PROJECT, API)),
      routes.newApi(ORG, PROJECT),
    ];

    expect(new Set(aliases).size).toBe(aliases.length);
  });

  it('does not put a page suffix where an API handle is read', () => {
    // `.../apis/deploy` must stay the detail path of an API handled `deploy`,
    // never the Deploy page awaiting an API.
    expect(routes.apiDeploy(ORG, PROJECT, null)).not.toContain('/apis/');
    expect(routes.api(ORG, PROJECT, 'deploy')).toBe(
      `/organizations/${ORG}/projects/${PROJECT}/apis/deploy`
    );
  });
});
