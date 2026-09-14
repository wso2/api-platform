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

/**
 * SwaggerHub's public registry, read straight from the browser.
 *
 * Not a `src/api/resources` hook on purpose: that layer is this product's own
 * API behind the BFF, whereas this is a third-party registry the user names at
 * runtime — the same client-side read the "from URL" source already performs.
 * `api.swaggerhub.com` answers anonymous reads of public definitions with
 * `Access-Control-Allow-Origin: *`, so no proxy stands in between.
 */

const REGISTRY_BASE_URL = 'https://api.swaggerhub.com/apis';

/**
 * How many of an organization's APIs one listing carries. A larger org is
 * paged by the registry; only this first page is offered, and
 * `SwaggerHubOrganization.total` says whether anything was left behind.
 */
const LISTING_LIMIT = 100;

/** One API in an organization, with every version the registry lists for it. */
export type SwaggerHubApi = {
  /** Display name, e.g. "SwaggerHub Registry API". */
  name: string;
  /** Registry slug used to address it, e.g. "registry-api". */
  slug: string;
  /** Newest first, as the registry orders them. */
  versions: string[];
};

export type SwaggerHubOrganization = {
  apis: SwaggerHubApi[];
  /** How many APIs the organization holds, which may exceed `apis.length`. */
  total: number;
};

export type SwaggerHubLookupResult =
  | { organization: SwaggerHubOrganization; status: 'found' }
  /** The registry has no such organization (or it holds nothing public). */
  | { status: 'notFound' }
  /** The registry could not be reached, or answered with something else. */
  | { status: 'failed' };

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;

/** Reads one `properties` entry — the registry's key/value bag per API. */
const propertyValue = (properties: unknown, type: string): string | undefined => {
  if (!Array.isArray(properties)) {
    return undefined;
  }
  for (const entry of properties) {
    const property = asRecord(entry);
    if (property?.type === type && typeof property.value === 'string') {
      return property.value;
    }
  }
  return undefined;
};

/**
 * The versions an API declares.
 *
 * `X-Versions` is a comma-separated list where a `*` marks a published
 * version ("*1.0.0,*1.1.0"); the star is display noise here, since an
 * unpublished version simply isn't readable anonymously. `X-Version` (the
 * default one) is the fallback when the list is absent.
 */
const readVersions = (properties: unknown): string[] => {
  const list = propertyValue(properties, 'X-Versions');
  if (list !== undefined && list !== '') {
    return list
      .split(',')
      .map((version) => version.replace(/^\*/, '').trim())
      .filter((version) => version !== '');
  }
  const single = propertyValue(properties, 'X-Version');
  return single === undefined ? [] : [single];
};

/** Everything in the response that names a usable API; anything else is skipped. */
const readApis = (payload: unknown): SwaggerHubApi[] => {
  const apis = asRecord(payload)?.apis;
  if (!Array.isArray(apis)) {
    return [];
  }

  return apis.flatMap((entry): SwaggerHubApi[] => {
    const api = asRecord(entry);
    if (api === null) {
      return [];
    }
    const slug = propertyValue(api.properties, 'X-Name');
    if (slug === undefined || slug === '') {
      return [];
    }
    const versions = readVersions(api.properties);
    if (versions.length === 0) {
      return [];
    }
    return [
      {
        name: typeof api.name === 'string' && api.name !== '' ? api.name : slug,
        slug,
        versions,
      },
    ];
  });
};

/**
 * Looks an organization up in the registry, returning its APIs and their
 * versions — one request covers both, because the listing carries each API's
 * version list with it.
 *
 * `signal` lets a caller drop a lookup that a later keystroke has replaced; an
 * aborted request reports `failed`, which the caller is expected to ignore.
 */
export const lookupSwaggerHubOrganization = async (
  organization: string,
  signal?: AbortSignal,
): Promise<SwaggerHubLookupResult> => {
  const url = `${REGISTRY_BASE_URL}/${encodeURIComponent(
    organization,
  )}?limit=${LISTING_LIMIT}&page=0`;

  let payload: unknown;
  let total = 0;
  try {
    const response = await fetch(url, {
      headers: { Accept: 'application/json' },
      signal,
    });
    if (response.status === 404) {
      return { status: 'notFound' };
    }
    if (!response.ok) {
      return { status: 'failed' };
    }
    payload = await response.json();
    const count = asRecord(payload)?.totalCount;
    total = typeof count === 'number' ? count : 0;
  } catch {
    // Network failure, an abort, or a body that wasn't JSON. The reason is
    // developer-facing and stays out of the message the user reads.
    return { status: 'failed' };
  }

  const apis = readApis(payload);
  // An organization that exists but publishes nothing readable is, for this
  // step's purposes, nothing to choose from.
  if (apis.length === 0) {
    return { status: 'notFound' };
  }
  return {
    organization: { apis, total: Math.max(total, apis.length) },
    status: 'found',
  };
};

/** Where one published definition lives. */
export const swaggerHubSpecUrl = (organization: string, slug: string, version: string): string =>
  `${REGISTRY_BASE_URL}/${encodeURIComponent(organization)}/${encodeURIComponent(
    slug,
  )}/${encodeURIComponent(version)}`;
