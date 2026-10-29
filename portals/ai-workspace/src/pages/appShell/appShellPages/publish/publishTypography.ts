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

import type { Theme } from '@wso2/oxygen-ui';

/**
 * The control plane's type scale, one step smaller in its Oxygen UI version (body 14px, not 16px),
 * so the Publish pages look the same in both portals.
 */
const CONTROL_PLANE_SCALE = {
  h1: { fontSize: '1.75rem', fontWeight: 400, lineHeight: 1.167 },
  h2: { fontSize: '1.5rem', fontWeight: 400, lineHeight: 1.2 },
  h3: { fontSize: '1.375rem', fontWeight: 400, lineHeight: 1.167 },
  h4: { fontSize: '1.25rem', fontWeight: 400, lineHeight: 1.235 },
  h5: { fontSize: '1rem', fontWeight: 400, lineHeight: 1.334 },
  h6: { fontSize: '0.875rem', fontWeight: 500, lineHeight: 1.6 },
  subtitle1: { fontSize: '1.125rem', fontWeight: 400, lineHeight: 1.75 },
  subtitle2: { fontSize: '0.875rem', fontWeight: 400, lineHeight: 1.57 },
  body1: { fontSize: '0.875rem', fontWeight: 400, lineHeight: 1.5 },
  body2: { fontSize: '0.8125rem', fontWeight: 400, lineHeight: 1.43 },
  caption: { fontSize: '0.6875rem', fontWeight: 400, lineHeight: 1.66 },
} as const;

/** The surrounding theme, with each variant's size, weight and line height replaced. */
export const withControlPlaneTypography = (outer: Theme): Theme => {
  const typography = { ...outer.typography } as Record<string, unknown>;
  for (const [variant, scale] of Object.entries(CONTROL_PLANE_SCALE)) {
    typography[variant] = { ...(outer.typography as unknown as Record<string, object>)[variant], ...scale };
  }
  return { ...outer, typography } as unknown as Theme;
};
