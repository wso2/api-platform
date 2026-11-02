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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Reads the BFF-injected `runtime-config.js` global, the same way the sibling
 * `apip-cloud-ui-gateways` and `apip-cloud-ui-insights` plugins do.
 */

type WindowConfig = Partial<{
  APIP_ACP_BILLING_CONSOLE_URL: string;
  billingConsoleUrl: string;
  appBasePath: string;
}>;

const fromWindow = (): WindowConfig => {
  const globalWindow = window as Window & {
    config?: WindowConfig;
    __RUNTIME_CONFIG__?: WindowConfig;
  };
  return {
    ...(globalWindow.__RUNTIME_CONFIG__ ?? {}),
    ...(globalWindow.config ?? {}),
  };
};

/**
 * The console's serving prefix, e.g. "/console"; empty when served at the root.
 *
 * Same order and same normalization as the console's own `runtime.ts`: the
 * injected global first, then what Vite baked in at build time. The build-time
 * fallback is what carries the value today, since the server does not emit the
 * key — without it this returns empty and the fetch below goes to the origin
 * root however the console is served.
 */
export function appBasePath(): string {
  const value =
    fromWindow().appBasePath ||
    import.meta.env.VITE_APP_BASE_PATH ||
    import.meta.env.BASE_URL ||
    '';
  if (!value || value === '/') return '';
  return `/${value.replace(/^\/|\/$/g, '')}`;
}

/**
 * The organization's upgrade page in the billing console.
 *
 * `undefined` when either half is missing — a deployment with no billing
 * console (on-prem, standalone, or simply unconfigured), or before the
 * organization resolves. The banner then shows its message with no link, which
 * is the right answer: there is nowhere to send anyone.
 *
 * The `orgId` passed is this console's organization **handle**. That matches
 * what the AI Workspace plugin sends, whose `orgUuid` is likewise just the
 * platform-api organization id (`uuid: p.id`) rather than a distinct UUID.
 * Worth confirming against a real billing console before relying on the link.
 */
export function upgradeUrl(organization: string | undefined): string | undefined {
  const config = fromWindow();
  const base = (config.APIP_ACP_BILLING_CONSOLE_URL ?? config.billingConsoleUrl ?? '')
    .trim()
    .replace(/\/$/, '');
  if (!base || !organization) return undefined;
  return `${base}/cloud/bijira/upgrade?orgId=${encodeURIComponent(organization)}`;
}
