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

import { http, HttpResponse, type HttpHandler } from 'msw';
import { vi } from 'vitest';

import { runtimeConfig } from '@/config/runtime';

/** Where the Policy Hub is, as the app under test sees it. */
export const policyHubUrl = runtimeConfig.policyHubBaseUrl;

/** The hub is a different origin from the page, so jsdom enforces CORS on its replies. */
export const hubCors = { 'Access-Control-Allow-Origin': '*' };

/**
 * jsdom's AbortSignal is not the one undici's fetch accepts, so the signal React Query
 * hands a hub request is dropped; MSW still answers the request. Call in `beforeEach`
 * and undo with `vi.restoreAllMocks()` in `afterEach`.
 */
export const stubHubFetch = (): void => {
  const mswFetch = globalThis.fetch;
  vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) =>
    mswFetch(input, { ...init, signal: undefined }),
  );
};

export type HubPolicy = {
  name: string;
  displayName: string;
  /** Published versions, e.g. `['1.2', '1.10']`. */
  versions: string[];
  /** The YAML served for any of its versions. */
  definition?: string;
  categories?: string[];
};

/**
 * A small Policy Hub: the catalog list (with `search`/`categories`/paging), the category
 * list, each policy's versions and its definition.
 */
export const policyHubHandlers = (catalog: HubPolicy[]): HttpHandler[] => {
  const summary = (policy: HubPolicy, version: string) => ({
    categories: policy.categories ?? [],
    displayName: policy.displayName,
    isLatest: version === policy.versions[policy.versions.length - 1],
    name: policy.name,
    tags: [],
    version,
  });
  const byName = (name: string) => catalog.find((policy) => policy.name === name);

  return [
    http.get(`${policyHubUrl}/policies/categories`, () =>
      HttpResponse.json(
        { data: [...new Set(catalog.flatMap((policy) => policy.categories ?? []))] },
        { headers: hubCors },
      ),
    ),
    http.get(`${policyHubUrl}/policies/:name/versions/:version/definition`, ({ params }) => {
      const policy = byName(String(params.name));
      return policy?.definition
        ? HttpResponse.text(policy.definition, { headers: hubCors })
        : new HttpResponse(null, { headers: hubCors, status: 404 });
    }),
    http.get(`${policyHubUrl}/policies/:name/versions`, ({ params }) => {
      const policy = byName(String(params.name));
      return policy
        ? HttpResponse.json(
            policy.versions.map((version) => summary(policy, version)),
            { headers: hubCors },
          )
        : new HttpResponse(null, { headers: hubCors, status: 404 });
    }),
    http.get(`${policyHubUrl}/policies`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      const term = (query.get('search') ?? '').toLowerCase();
      const categories = (query.get('categories') ?? '').split(',').filter(Boolean);
      const matches = catalog
        .filter((policy) => !term || policy.displayName.toLowerCase().includes(term))
        .filter(
          (policy) =>
            categories.length === 0 ||
            (policy.categories ?? []).some((category) => categories.includes(category)),
        )
        .map((policy) => summary(policy, policy.versions[policy.versions.length - 1]));
      const offset = Number(query.get('offset') ?? 0);
      const limit = Number(query.get('limit') ?? 20);
      return HttpResponse.json(
        {
          data: matches.slice(offset, offset + limit),
          pagination: { limit, offset, total: matches.length },
        },
        { headers: hubCors },
      );
    }),
  ];
};
