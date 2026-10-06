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

/** Placeholder backend URL used to detect an untouched endpoint. */
export const PLACEHOLDER_UPSTREAM_URL = 'https://example.com';

/** Summary on each catch-all operation: what it does, in the user's terms. */
const FORWARDED = 'Any path, forwarded as-is';

/**
 * The definition an API created from an endpoint starts with: one catch-all
 * path forwarding the five methods a pass-through proxy needs, so a client's
 * PUT reaches the backend as readily as its GET. The text is content rather
 * than UI copy, so it deliberately does not go through `react-intl`, the same
 * way a code sample or a backend payload doesn't.
 *
 * It has no description: the old one ("…or ask AI to refine them") landed in
 * the form as if the user had written it, and pointed at an AI this flow
 * doesn't have.
 */
export const DEFAULT_API_SKELETON: Record<string, unknown> = {
  openapi: '3.0.3',
  info: {
    title: 'Untitled API',
    version: '1.0.0',
  },
  servers: [{ url: PLACEHOLDER_UPSTREAM_URL }],
  paths: {
    '/*': {
      get: { summary: FORWARDED, responses: { '200': { description: 'OK' } } },
      post: { summary: FORWARDED, responses: { '200': { description: 'OK' } } },
      put: { summary: FORWARDED, responses: { '200': { description: 'OK' } } },
      patch: { summary: FORWARDED, responses: { '200': { description: 'OK' } } },
      delete: { summary: FORWARDED, responses: { '200': { description: 'OK' } } },
    },
  },
};

/**
 * The skeleton as it should be stored for this API: its info and server taken
 * from what the user entered on the details step rather than the skeleton's
 * placeholders. Otherwise the stored definition goes on saying "Untitled API"
 * and example.com whatever the API is called and wherever it points.
 */
export const skeletonFor = ({
  description,
  displayName,
  upstreamUrl,
  version,
}: {
  description?: string;
  displayName: string;
  upstreamUrl?: string;
  version: string;
}): Record<string, unknown> => ({
  ...DEFAULT_API_SKELETON,
  info: {
    title: displayName,
    version,
    ...(description ? { description } : {}),
  },
  servers: [{ url: upstreamUrl || PLACEHOLDER_UPSTREAM_URL }],
});
