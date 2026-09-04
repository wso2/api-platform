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

/// <reference path="./vite-env.d.ts" />

import { pickAllowlistedMoesifAppUrl } from '../utils/moesifEmbed';

export type InsightsRuntimeConfig = {
  /** Same-origin BFF proxy prefix, e.g. "/proxy". */
  platformApiBaseUrl: string;
  platformApiVersion: string;
  /** Moesif wrap host (iframe postMessage origin). Undefined when not configured. */
  moesifAppUrl?: string;
};

type LegacyWindowConfig = Partial<{
  PLATFORM_API_BASE_URL: string;
  platformApiBaseUrl: string;
  PLATFORM_API_VERSION: string;
  platformApiVersion: string;
  MOESIF_APP_URL: string;
  MOESIF_BASIC_INSIGHTS_URL: string;
  moesifAppUrl: string;
  moesifBasicInsightsUrl: string;
  /** AI Workspace BFF runtime key (`moesif_web_url` → APIP_AIW_MOESIF_WEB_URL). */
  APIP_AIW_MOESIF_WEB_URL: string;
}>;

const fromWindow = (): LegacyWindowConfig => {
  const globalWindow = window as Window & {
    config?: LegacyWindowConfig;
    __RUNTIME_CONFIG__?: LegacyWindowConfig;
  };
  return {
    ...(globalWindow.__RUNTIME_CONFIG__ ?? {}),
    ...(globalWindow.config ?? {}),
  };
};

const configuredMoesifAppUrl = () =>
  fromWindow().MOESIF_APP_URL ||
  fromWindow().MOESIF_BASIC_INSIGHTS_URL ||
  fromWindow().moesifAppUrl ||
  fromWindow().moesifBasicInsightsUrl ||
  fromWindow().APIP_AIW_MOESIF_WEB_URL ||
  import.meta.env.VITE_MOESIF_APP_URL ||
  import.meta.env.VITE_MOESIF_BASIC_INSIGHTS_URL ||
  import.meta.env.APIP_AIW_MOESIF_WEB_URL ||
  '';

const resolveInsightsMoesifAppUrl = (): string | undefined => {
  const configured = configuredMoesifAppUrl().trim();
  if (!configured) return undefined;
  return pickAllowlistedMoesifAppUrl(configured);
};

/** True when a trusted Moesif wrap origin is available (runtime or Vite env). */
export const isInsightsMoesifConfigured = (): boolean =>
  Boolean(resolveInsightsMoesifAppUrl());

/** Same-origin BFF proxy prefix from Vite `base` (`/` → `/proxy`, `/ai-workspace/` → `/ai-workspace/proxy`). */
export const defaultPlatformApiBaseUrl = (
  viteBase = import.meta.env.BASE_URL
): string => {
  const base = String(viteBase ?? '/').replace(/\/$/, '');
  return `${base}/proxy`;
};

export const insightsRuntimeConfig: InsightsRuntimeConfig = {
  platformApiBaseUrl:
    fromWindow().PLATFORM_API_BASE_URL ||
    fromWindow().platformApiBaseUrl ||
    import.meta.env.VITE_PLATFORM_API_BASE_URL ||
    defaultPlatformApiBaseUrl(),
  platformApiVersion:
    fromWindow().PLATFORM_API_VERSION ||
    fromWindow().platformApiVersion ||
    import.meta.env.VITE_PLATFORM_API_VERSION ||
    'v0.9',
  moesifAppUrl: resolveInsightsMoesifAppUrl(),
};

export const platformApiRoot = () => {
  const base = insightsRuntimeConfig.platformApiBaseUrl.replace(/\/$/, '');
  const version = insightsRuntimeConfig.platformApiVersion.replace(/^\//, '');
  return `${base}/api/${version}`;
};
