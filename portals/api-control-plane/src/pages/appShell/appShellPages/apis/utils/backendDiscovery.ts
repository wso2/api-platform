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

import yaml from 'js-yaml';

import { HTTP_METHODS } from './developEdit';

/** A method+path resource exposed by the backend service. */
export type BackendResource = { method: string; path: string };

const HTTP_METHOD_SET = new Set(HTTP_METHODS.map((m) => m.toLowerCase()));

/**
 * Conventional OpenAPI/Swagger document URLs to probe for a backend base URL.
 * If the URL already points at a contract document (`.json`/`.yaml`), it is
 * used directly. Otherwise we try the usual locations relative to the base,
 * plus the host root when the base has a sub-path.
 */
export const contractCandidates = (backendUrl: string): string[] => {
  const trimmed = backendUrl.trim().replace(/\/+$/, '');
  if (!trimmed) return [];
  if (/\.(json|ya?ml)$/i.test(trimmed)) return [trimmed];

  const candidates = [
    `${trimmed}/openapi.json`,
    `${trimmed}/openapi.yaml`,
    `${trimmed}/swagger.json`,
    `${trimmed}/v3/api-docs`,
    `${trimmed}/api-docs`,
  ];
  try {
    const url = new URL(trimmed);
    if (url.pathname && url.pathname !== '/') {
      candidates.push(`${url.origin}/openapi.json`);
    }
  } catch {
    // Not a parseable URL — the base-relative candidates above are best effort.
  }
  return candidates;
};

/**
 * Parses an OpenAPI v3 or Swagger v2 document (JSON or YAML) into the list of
 * backend resources it declares. Path-item keys that are not HTTP methods
 * (`parameters`, `$ref`, `servers`, …) are ignored.
 */
export const parseContractResources = (text: string): BackendResource[] => {
  let doc: unknown;
  try {
    doc = JSON.parse(text);
  } catch {
    try {
      doc = yaml.load(text);
    } catch {
      return [];
    }
  }

  const paths = doc && typeof doc === 'object' ? (doc as Record<string, unknown>).paths : undefined;
  if (!paths || typeof paths !== 'object') return [];

  const resources: BackendResource[] = [];
  const seen = new Set<string>();
  for (const [path, item] of Object.entries(paths as Record<string, unknown>)) {
    if (!item || typeof item !== 'object') continue;
    for (const methodKey of Object.keys(item as Record<string, unknown>)) {
      if (!HTTP_METHOD_SET.has(methodKey.toLowerCase())) continue;
      const method = methodKey.toUpperCase();
      const key = `${method} ${path}`;
      if (seen.has(key)) continue;
      seen.add(key);
      resources.push({ method, path });
    }
  }
  return resources;
};

/**
 * Fetches the backend's OpenAPI/Swagger contract (trying the conventional
 * locations) and returns the resources it declares. The first candidate that
 * responds OK and parses to at least one resource wins.
 *
 * This is a direct browser fetch, so it is subject to the backend's CORS
 * policy — a backend that does not allow the console origin will fail here and
 * the caller should fall back to manual mapping.
 *
 * `signal` is used only to short-circuit between candidates and let the caller
 * discard a superseded run — it is intentionally NOT passed to `fetch` (jsdom
 * and undici disagree on the AbortSignal realm, which throws), so an in-flight
 * request completes but its result is ignored once aborted.
 */
export const discoverBackendResources = async (
  backendUrl: string,
  signal?: AbortSignal,
): Promise<BackendResource[]> => {
  const candidates = contractCandidates(backendUrl);
  let lastError: Error | undefined;
  for (const url of candidates) {
    if (signal?.aborted) return [];
    try {
      const response = await fetch(url, {
        headers: { Accept: 'application/json, application/yaml, text/yaml, */*' },
      });
      if (!response.ok) continue;
      const resources = parseContractResources(await response.text());
      if (resources.length > 0) return resources;
    } catch (error) {
      lastError = error instanceof Error ? error : new Error('fetch failed');
    }
  }
  if (lastError) throw lastError;
  return [];
};
