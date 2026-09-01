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

import { resolveTrustedMoesifAppUrl } from '../utils/moesifEmbed';

export type InsightsRuntimeConfig = {
  /** Same-origin BFF proxy prefix, e.g. "/proxy". */
  platformApiBaseUrl: string;
  platformApiVersion: string;
  /** Moesif wrap host (iframe postMessage origin). */
  moesifAppUrl: string;
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
  environmentName: string;
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

const isProductionEnvironment = () => {
  const env =
    fromWindow().environmentName ??
    import.meta.env.VITE_ENVIRONMENT_NAME ??
    'local';
  return env === 'production' || env === 'stage';
};

const defaultMoesifAppUrl = () =>
  isProductionEnvironment()
    ? 'https://www.moesif.com'
    : 'https://web-dev.moesif.com';

const configuredMoesifAppUrl = () =>
  fromWindow().MOESIF_APP_URL ||
  fromWindow().MOESIF_BASIC_INSIGHTS_URL ||
  fromWindow().moesifAppUrl ||
  fromWindow().moesifBasicInsightsUrl ||
  import.meta.env.VITE_MOESIF_APP_URL ||
  import.meta.env.VITE_MOESIF_BASIC_INSIGHTS_URL ||
  '';

export const insightsRuntimeConfig: InsightsRuntimeConfig = {
  platformApiBaseUrl:
    fromWindow().PLATFORM_API_BASE_URL ||
    fromWindow().platformApiBaseUrl ||
    import.meta.env.VITE_PLATFORM_API_BASE_URL ||
    '/proxy',
  platformApiVersion:
    fromWindow().PLATFORM_API_VERSION ||
    fromWindow().platformApiVersion ||
    import.meta.env.VITE_PLATFORM_API_VERSION ||
    'v0.9',
  moesifAppUrl: resolveTrustedMoesifAppUrl(
    configuredMoesifAppUrl() || defaultMoesifAppUrl(),
    defaultMoesifAppUrl()
  ),
};

export const platformApiRoot = () => {
  const base = insightsRuntimeConfig.platformApiBaseUrl.replace(/\/$/, '');
  const version = insightsRuntimeConfig.platformApiVersion.replace(/^\//, '');
  return `${base}/api/${version}`;
};
