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

/** Cloud Console's subscriptions page, where a trial is upgraded. */
export const SUBSCRIPTIONS_PATH = '/billing?tab=subscriptions';

/**
 * The subscriptions page on Cloud Console's origin: `consoleUrl` when it is an
 * http(s) URL, else this page's own origin, which deployments share with Cloud
 * Console. Never a `javascript:` or other scheme.
 */
export function subscriptionsUrl(consoleUrl?: string): string {
  const trimmed = consoleUrl?.trim();
  let origin = window.location.origin;
  if (trimmed) {
    try {
      const { protocol } = new URL(trimmed);
      if (protocol === 'https:' || protocol === 'http:') origin = trimmed.replace(/\/+$/, '');
    } catch {
      // Not a URL: keep this page's origin.
    }
  }
  return `${origin}${SUBSCRIPTIONS_PATH}`;
}
