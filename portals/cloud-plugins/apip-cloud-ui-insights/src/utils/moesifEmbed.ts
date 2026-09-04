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

import type { InsightsScopeLevel } from '../types';

/** postMessage types used by Moesif wrap/basic embed (`#auth=post`). */
export const MOESIF_EMBEDDED_POST_MESSAGE_TYPES = {
  SET_TOKEN: 'SET_TOKEN',
  ORG_LOAD_FINISHED: 'ORG_LOAD_FINISHED',
  SCHEMA_GEN_FINISHED: 'SCHEMA_GEN_FINISHED',
  REFRESH_TOKEN: 'REFRESH_TOKEN',
} as const;

/** Known Moesif wrap/basic hosts — reject misconfigured runtime URLs. */
export const ALLOWED_MOESIF_ORIGINS = new Set([
  'https://www.moesif.com',
  'https://web-dev.moesif.com',
]);

/** Serialized origin for postMessage targetOrigin and MessageEvent.origin checks. */
export const resolveMoesifEmbeddingOrigin = (moesifAppUrl: string): string =>
  new URL(moesifAppUrl).origin;

/** Return configuredUrl when it is HTTPS and on the Moesif allowlist. */
export const pickAllowlistedMoesifAppUrl = (
  configuredUrl: string
): string | undefined => {
  try {
    const parsed = new URL(configuredUrl);
    if (parsed.protocol !== 'https:') return undefined;
    if (!ALLOWED_MOESIF_ORIGINS.has(parsed.origin)) return undefined;
    return parsed.origin;
  } catch {
    return undefined;
  }
};

/**
 * Return a trusted Moesif app base URL (HTTPS + allowlisted origin).
 * Returns undefined when neither URL is on the allowlist — callers must not
 * silently fall back to web-dev in production.
 */
export const resolveTrustedMoesifAppUrl = (
  configuredUrl: string,
  fallbackUrl: string
): string | undefined =>
  pickAllowlistedMoesifAppUrl(configuredUrl) ??
  pickAllowlistedMoesifAppUrl(fallbackUrl);

/**
 * Org-level wrap/basic iframe.
 * Shape matches choreo-console: `{origin}/wrap/basic#auth=post`
 */
export const buildBasicIframeSrc = (embeddingOrigin: string) =>
  `${embeddingOrigin.replace(/\/$/, '')}/wrap/basic#auth=post`;

/**
 * Project-level wrap/basic iframe with Moesif `project_id` filtering.
 */
export const buildBasicProjectIframeSrc = (
  embeddingOrigin: string,
  projectId: string
) => {
  const origin = embeddingOrigin.replace(/\/$/, '');
  const cleaned = projectId.trim();
  if (!cleaned) return buildBasicIframeSrc(embeddingOrigin);
  return `${origin}/wrap/basic?project_id=${encodeURIComponent(
    cleaned
  )}#auth=post`;
};

export const resolveInsightsScopeLevel = (params: {
  projectHandle?: string;
}): InsightsScopeLevel =>
  params.projectHandle ? 'project' : 'organization';
