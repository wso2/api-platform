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

import { runtimeConfig } from './runtime';

/**
 * The features a deployment can hide with `disableFeatures` (the BFF's
 * `disable_features`). Each name hides the feature's sidebar entry and makes its
 * page answer "not found". Names a deployment lists that are not here are
 * ignored, so a config written for a newer console does not break an older one.
 */
export const FEATURES = [
  'metrics',
  'logs',
  'observability',
  'insights',
  'api-insights',
  'compliance',
  'subscription-plans',
  'meters',
  'service-accounts',
  'paidplan',
] as const;

export type FeatureId = (typeof FEATURES)[number];

/**
 * Sidebar-extension pages (the cloud build's) by extension id, and the features
 * that hide each: the router answers "not found" for them, since an extension
 * that stands down in the sidebar still has its URL registered.
 */
export const EXTENSION_FEATURES: Record<string, FeatureId[]> = {
  observability: ['observability'],
  'observability-logs': ['observability', 'logs'],
  'observability-metrics': ['observability', 'metrics'],
  'organization-insights': ['insights'],
  'project-insights': ['insights'],
};

/** Whether the deployment has switched this feature off. Everything is on by default. */
export const isFeatureDisabled = (feature: FeatureId): boolean =>
  runtimeConfig.disableFeatures.includes(feature);
