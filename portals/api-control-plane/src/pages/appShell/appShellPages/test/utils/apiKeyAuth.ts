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

import type { Policy, RestApi } from '@/api/resources/restApis';

/**
 * Determines whether the API requires an API key and where to send it.
 *
 * The gateway enforces API-key authentication only when the `api-key-auth`
 * policy is attached. The policy defines `params.key` and `params.in`, which
 * specify the credential's header or query-parameter name and location.
 * `RESTAPI` has no `security` field, so this information is read from its
 * policies. The result controls both the test-key panel and request headers.
 */

/** Name of the policy that makes an API key mandatory. Lowercase is canonical. */
export const API_KEY_AUTH_POLICY = 'api-key-auth';

/**
 * Header the policy checks when it declares no `params.key`.
 *
 * The gateway's own documented default for `api-key-auth` — see
 * `tests/integration-e2e/steps_secured_test.go`, which pins it as the header a
 * secured API is invoked with. Guessing anything else here produces a 401 that
 * reads as a broken key rather than a misaddressed one.
 */
export const DEFAULT_API_KEY_HEADER = 'API-Key';

/** Where a credential travels. */
export type ApiKeyLocation = 'header' | 'query';

export type ApiKeyAuth = {
  /** Name of the header or query parameter carrying the key. */
  name: string;
  in: ApiKeyLocation;
};

/** A trimmed string, or `undefined` when nothing usable is left. */
const text = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const next = value.trim();
  return next === '' ? undefined : next;
};

/**
 * Every policy attached anywhere on the API — its own list, plus each
 * operation's.
 *
 * Operation-level policies count because an API can be secured per resource
 * rather than wholesale: keying only off `api.policies` would show no key panel
 * for such an API and every secured call would 401 with nothing on screen to
 * explain it.
 */
const allPolicies = (api: RestApi): Policy[] => [
  ...(api.policies ?? []),
  ...(api.operations ?? []).flatMap((operation) => operation.request?.policies ?? []),
];

/** Reads the API's first case-insensitive `api-key-auth` policy, if any. */
export const apiKeyAuthOf = (api: RestApi | undefined): ApiKeyAuth | undefined => {
  if (!api) return undefined;

  const policy = allPolicies(api).find(
    (candidate) => text(candidate?.name)?.toLowerCase() === API_KEY_AUTH_POLICY,
  );
  if (!policy) return undefined;

  const params = (policy.params ?? {}) as Record<string, unknown>;

  return {
    // Only an explicit `query` uses the URL; all other values use the policy's
    // default header to avoid exposing credentials in proxy logs.
    in: text(params.in)?.toLowerCase() === 'query' ? 'query' : 'header',
    name: text(params.key) ?? DEFAULT_API_KEY_HEADER,
  };
};

/** Whether the API requires a key at all. */
export const requiresApiKey = (api: RestApi | undefined): boolean =>
  apiKeyAuthOf(api) !== undefined;
