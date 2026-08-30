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

/**
 * Whether a user has closed the setup banner on a given gateway.
 *
 * Kept in `localStorage` rather than component state so the dismissal outlives
 * a reload: the banner is onboarding, and one that returns every time the page
 * is refreshed stops being dismissible in any meaningful sense.
 *
 * Per gateway, not per user: closing the walkthrough on one gateway says
 * nothing about the next one someone provisions. Reads and writes share this
 * module so the key is spelled once, and both treat a storage failure (private
 * mode, a full quota) as "not dismissed".
 */

const KEY_PREFIX = 'apicp.gateways.setupBanner.dismissed';

const storageKey = (gatewayId: string): string => `${KEY_PREFIX}.${gatewayId}`;

export function isSetupBannerDismissed(gatewayId: string): boolean {
  if (typeof window === 'undefined' || !gatewayId) return false;
  try {
    return window.localStorage.getItem(storageKey(gatewayId)) === 'true';
  } catch {
    return false;
  }
}

export function dismissSetupBanner(gatewayId: string): void {
  if (typeof window === 'undefined' || !gatewayId) return;
  try {
    window.localStorage.setItem(storageKey(gatewayId), 'true');
  } catch {
    // Ignore persistence failures: the banner still closes for this visit.
  }
}
