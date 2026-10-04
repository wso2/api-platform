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

import { alpha } from '@wso2/oxygen-ui';

/**
 * Swagger UI's fixed per-verb colours are defined here so resource rows match
 * the embedded `swagger-ui-react` implementation
 * (`Components/ResourceView/ResourceRow.tsx`). Keeping these literals in one
 * place prevents inconsistent colours across the product. Only the verb
 * colours are fixed; surrounding text, surfaces and borders use theme tokens
 * to remain legible in dark mode.
 */
const METHOD_HEX: Record<string, string> = {
  DELETE: '#f93e3e',
  GET: '#61affe',
  HEAD: '#9012fe',
  OPTIONS: '#0d5aa7',
  PATCH: '#50e3c2',
  POST: '#49cc90',
  PUT: '#fca130',
};

/** Any verb outside the table — a custom or malformed method — reads as neutral. */
const UNKNOWN_METHOD_HEX = '#a0a0a0';

/** How much of the verb's colour washes the row behind it. */
const ROW_TINT = 0.14;

export type MethodPalette = {
  /** Solid fill behind the method badge's white label. */
  badge: string;
  /** The row's tinted fill. */
  bg: string;
  /** The row's 1px rule. */
  border: string;
};

/** The three shades one HTTP verb is drawn in. Case-insensitive. */
export const methodPalette = (method: string): MethodPalette => {
  const hex = METHOD_HEX[method.toUpperCase()] ?? UNKNOWN_METHOD_HEX;
  return { badge: hex, bg: alpha(hex, ROW_TINT), border: hex };
};
