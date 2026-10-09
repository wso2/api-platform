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

const HTTP_METHODS = new Set(['delete', 'get', 'head', 'options', 'patch', 'post', 'put', 'trace']);

export type SpecSummary = {
  /** "OpenAPI" or "Swagger": a product name, so never translated. */
  dialect: string;
  /** Operations across every path: what the API will route. */
  routes: number;
  /** The version the document declares, e.g. "3.0.2". */
  version: string;
};

/**
 * What an accepted spec is, in the few words the success line needs:
 * "OpenAPI 3.0.2 · 6 routes found". `null` when the document doesn't say which
 * dialect it is, so the line is left out rather than guessed.
 */
export const summarizeSpec = (spec: Record<string, unknown>): SpecSummary | null => {
  const declared = [
    { dialect: 'OpenAPI', version: spec.openapi },
    { dialect: 'Swagger', version: spec.swagger },
  ].find((candidate) => typeof candidate.version === 'string' && candidate.version.trim() !== '');
  if (declared === undefined) {
    return null;
  }

  const paths =
    typeof spec.paths === 'object' && spec.paths !== null
      ? Object.values(spec.paths as Record<string, unknown>)
      : [];
  const routes = paths.reduce<number>((count, item) => {
    if (typeof item !== 'object' || item === null) return count;
    return count + Object.keys(item).filter((key) => HTTP_METHODS.has(key.toLowerCase())).length;
  }, 0);

  return { dialect: declared.dialect, routes, version: String(declared.version).trim() };
};
