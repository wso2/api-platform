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

/** The WSO2 Cloud platforms the switcher knows, in display order. */
export const PLATFORM_CATALOG = [
  { id: "agent-manager", name: "Agent Manager" },
  { id: "api-platform", name: "API Platform" },
  { id: "integration-platform", name: "Integration Platform" },
  { id: "identity-platform", name: "Identity Platform" },
  { id: "web-app-hosting", name: "Web App Hosting" },
  { id: "api-analytics", name: "API Analytics" },
] as const;

/** The platform this console is. Its card is marked current. */
export const CURRENT_PLATFORM_ID = "api-platform";

/** `cloudShell` from the deployment's runtime config. Every field is optional. */
export type CloudShellConfig = {
  platformUrls?: Record<string, string | undefined>;
  /** Comma-separated platform ids shown as unavailable ("Coming soon"). */
  comingSoonPlatforms?: string;
};

export type PlatformEntry = {
  id: string;
  name: string;
  url?: string;
  disabled: boolean;
};

export type ManageLink = { key: string; name: string; url: string };

/** An http(s) URL with trailing slashes removed, or "" for anything else (never `javascript:`). */
export function safeUrl(value?: string): string {
  const trimmed = value?.trim();
  if (!trimmed) return "";
  try {
    const { protocol } = new URL(trimmed);
    return protocol === "https:" || protocol === "http:"
      ? trimmed.replace(/\/+$/, "")
      : "";
  } catch {
    return "";
  }
}

/**
 * The platforms to list. A platform with no URL is left out, unless it is
 * listed in `comingSoonPlatforms` (shown disabled) or is this console itself,
 * which is always shown and falls back to `currentUrl`.
 */
export function resolvePlatforms(
  shell: CloudShellConfig | undefined,
  currentUrl: string,
): PlatformEntry[] {
  const comingSoon = new Set(
    (shell?.comingSoonPlatforms ?? "")
      .split(",")
      .map((id) => id.trim())
      .filter(Boolean),
  );
  const entries: PlatformEntry[] = [];
  for (const { id, name } of PLATFORM_CATALOG) {
    const url = safeUrl(shell?.platformUrls?.[id]);
    if (id === CURRENT_PLATFORM_ID) {
      entries.push({ id, name, url: url || currentUrl, disabled: false });
    } else if (comingSoon.has(id)) {
      entries.push({ id, name, disabled: true });
    } else if (url) {
      entries.push({ id, name, url, disabled: false });
    }
  }
  return entries;
}

/**
 * The Manage links: the WSO2 Cloud console's home, and its organization
 * settings (the same page the header's gear icon opens). None when the
 * console's origin is unknown.
 */
export function resolveManageLinks(cloudConsoleUrl?: string): ManageLink[] {
  const origin = safeUrl(cloudConsoleUrl);
  if (!origin) return [];
  return [
    { key: "home", name: "Cloud home", url: origin },
    { key: "settings", name: "Settings", url: `${origin}/organizations` },
  ];
}
