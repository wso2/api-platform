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

import { aRestApi } from '@/test/msw';
import type { RestApi } from '@/api/resources/restApis';
import { apiKeyAuthOf, DEFAULT_API_KEY_HEADER, requiresApiKey } from './apiKeyAuth';

/**
 * Whether the API under test requires a key, and where that key goes.
 *
 * Both halves are load-bearing: the first decides whether a real, persisted API
 * key is minted at all, and the second decides whether the credential reaches
 * the gateway in the place it is looking. Getting the second wrong produces a
 * 401 that reads as a broken key rather than a misaddressed one.
 */

const withPolicies = (policies: unknown[]): RestApi =>
  aRestApi({ policies: policies as RestApi['policies'] });

describe('apiKeyAuthOf', () => {
  it('is undefined for an API with no policies at all', () => {
    expect(apiKeyAuthOf(aRestApi())).toBeUndefined();
  });

  it('is undefined for an API whose policies do not include api-key-auth', () => {
    expect(
      apiKeyAuthOf(
        withPolicies([
          { name: 'cors', version: 'v1' },
          { name: 'set-headers', version: 'v1' },
        ]),
      ),
    ).toBeUndefined();
  });

  it('is undefined for an undefined API, so a loading page mints nothing', () => {
    expect(apiKeyAuthOf(undefined)).toBeUndefined();
  });

  it('reads the header name the policy configures', () => {
    const api = withPolicies([
      { name: 'api-key-auth', params: { in: 'header', key: 'X-API-Key' }, version: 'v1' },
    ]);

    expect(apiKeyAuthOf(api)).toEqual({ in: 'header', name: 'X-API-Key' });
  });

  it('falls back to the gateway’s own default when the policy declares no params', () => {
    // The policy's documented default. Guessing a different name here yields a
    // 401 the user would read as a bad key.
    expect(apiKeyAuthOf(withPolicies([{ name: 'api-key-auth', version: 'v1' }]))).toEqual({
      in: 'header',
      name: DEFAULT_API_KEY_HEADER,
    });
  });

  it('falls back when params exist but name the key blank', () => {
    const api = withPolicies([{ name: 'api-key-auth', params: { key: '   ' }, version: 'v1' }]);

    expect(apiKeyAuthOf(api)?.name).toBe(DEFAULT_API_KEY_HEADER);
  });

  it('honours a query-parameter placement', () => {
    const api = withPolicies([
      { name: 'api-key-auth', params: { in: 'query', key: 'apiKey' }, version: 'v1' },
    ]);

    expect(apiKeyAuthOf(api)).toEqual({ in: 'query', name: 'apiKey' });
  });

  it('treats anything other than an explicit query as a header', () => {
    // A credential in a URL is logged by every proxy on the path, so an
    // unrecognised placement must never silently move it there.
    for (const value of ['header', 'HEADER', 'cookie', 'body', '', 42, null, undefined]) {
      const api = withPolicies([{ name: 'api-key-auth', params: { in: value }, version: 'v1' }]);
      expect(apiKeyAuthOf(api)?.in).toBe('header');
    }
  });

  it('matches the placement case-insensitively', () => {
    const api = withPolicies([{ name: 'api-key-auth', params: { in: 'QUERY' }, version: 'v1' }]);

    expect(apiKeyAuthOf(api)?.in).toBe('query');
  });

  it('matches the policy name case-insensitively', () => {
    expect(apiKeyAuthOf(withPolicies([{ name: 'API-KEY-AUTH', version: 'v1' }]))).toBeDefined();
    expect(apiKeyAuthOf(withPolicies([{ name: '  api-key-auth  ', version: 'v1' }]))).toBeDefined();
  });

  it('does not match the underscored spelling, which the platform rejects', () => {
    // `API_KEY_AUTH` is a different policy name and platform-api refuses it as
    // invalid; honouring it here would mean acting on a policy that could
    // never have been saved.
    expect(apiKeyAuthOf(withPolicies([{ name: 'API_KEY_AUTH', version: 'v1' }]))).toBeUndefined();
  });

  it('finds the policy on an individual operation', () => {
    // An API can be secured per resource rather than wholesale. Keying only off
    // `api.policies` would show no key panel and every secured call would 401
    // with nothing on screen to explain it.
    const api = aRestApi({
      operations: [
        {
          name: 'listBooks',
          request: {
            method: 'GET',
            path: '/books',
            policies: [{ name: 'api-key-auth', params: { key: 'Op-Key' }, version: 'v1' }],
          },
        },
      ] as RestApi['operations'],
    });

    expect(apiKeyAuthOf(api)).toEqual({ in: 'header', name: 'Op-Key' });
  });

  it('prefers the API-level policy when both levels declare one', () => {
    const api = aRestApi({
      operations: [
        {
          name: 'listBooks',
          request: {
            method: 'GET',
            path: '/books',
            policies: [{ name: 'api-key-auth', params: { key: 'Op-Key' }, version: 'v1' }],
          },
        },
      ] as RestApi['operations'],
      policies: [
        { name: 'api-key-auth', params: { key: 'Api-Key-Level' }, version: 'v1' },
      ] as RestApi['policies'],
    });

    // Two api-key-auth policies with different params is a contradictory
    // configuration; picking the API-level one beats inventing a merge rule the
    // gateway does not have.
    expect(apiKeyAuthOf(api)?.name).toBe('Api-Key-Level');
  });

  it('tolerates malformed policy entries without throwing', () => {
    expect(
      apiKeyAuthOf(withPolicies([null, 'nonsense', 42, {}, { version: 'v1' }])),
    ).toBeUndefined();
  });

  it('tolerates a policy whose params are not an object', () => {
    const api = withPolicies([{ name: 'api-key-auth', params: 'nope', version: 'v1' }]);

    expect(apiKeyAuthOf(api)).toEqual({ in: 'header', name: DEFAULT_API_KEY_HEADER });
  });
});

describe('requiresApiKey', () => {
  it('is true only when the policy is present', () => {
    expect(requiresApiKey(withPolicies([{ name: 'api-key-auth', version: 'v1' }]))).toBe(true);
    expect(requiresApiKey(withPolicies([{ name: 'cors', version: 'v1' }]))).toBe(false);
    expect(requiresApiKey(aRestApi())).toBe(false);
    expect(requiresApiKey(undefined)).toBe(false);
  });
});
