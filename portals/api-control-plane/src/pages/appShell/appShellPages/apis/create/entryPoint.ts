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

import { API_PLATFORM_FEATURES, type FeatureDimensions } from '@/featureUsage';

/**
 * Which control opened the API creation wizard, reported with the wizard's
 * feature-usage action. In its own small module so the pages linking to the
 * wizard do not pull in the lazily loaded wizard itself.
 */
export type ApiCreationEntryPoint =
  FeatureDimensions<'apip.manage-apis.api-creation-wizard'>['entry_point'];

const STATE_KEY = 'apiCreationEntryPoint';

const ENTRY_POINTS: readonly string[] =
  API_PLATFORM_FEATURES['apip.manage-apis.api-creation-wizard'].dimensions.entry_point;

/**
 * Navigation options for a control that opens the wizard:
 * `navigate(routes.newApi(org, project), apiCreationEntry('api-list'))`.
 * Router state rather than a query parameter, so the URL stays clean.
 */
export const apiCreationEntry = (entryPoint: Exclude<ApiCreationEntryPoint, 'direct'>) => ({
  state: { [STATE_KEY]: entryPoint },
});

/** The entry point a navigation carried; `direct` for a typed URL, a reload, or anything else. */
export function readApiCreationEntryPoint(state: unknown): ApiCreationEntryPoint {
  if (typeof state === 'object' && state !== null && STATE_KEY in state) {
    const value = (state as Record<string, unknown>)[STATE_KEY];
    if (typeof value === 'string' && ENTRY_POINTS.includes(value)) {
      return value as ApiCreationEntryPoint;
    }
  }
  return 'direct';
}
