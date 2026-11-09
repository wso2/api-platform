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
 * Where the WSO2 Cloud console serves its organization pages, which this
 * console links to rather than re-implementing.
 *
 * Deployments share one origin with the WSO2 Cloud console, so the default is
 * this page's own origin. A deployment that serves it elsewhere (local
 * development, say) passes `consoleUrl` instead. These are paths on that
 * origin, outside this console's own base path, so a link to one must be a
 * plain anchor that loads the page, never a router link.
 */
export const ORGANIZATIONS_PATH = "/organizations";

const trimTrailingSlashes = (value: string): string =>
  value.replace(/\/+$/, "");

/** The configured origin when it is an http(s) URL, else null — never a `javascript:` or other scheme. */
const safeOrigin = (consoleUrl?: string): string | null => {
  const trimmed = consoleUrl?.trim();
  if (!trimmed) return null;
  try {
    const { protocol } = new URL(trimmed);
    return protocol === "https:" || protocol === "http:"
      ? trimTrailingSlashes(trimmed)
      : null;
  } catch {
    return null;
  }
};

/** The WSO2 Cloud console's organization pages. */
export function organizationsUrl(consoleUrl?: string): string {
  return `${safeOrigin(consoleUrl) ?? window.location.origin}${ORGANIZATIONS_PATH}`;
}
