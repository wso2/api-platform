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
 * Displays an API/policy version prefixed with "v" (e.g. "v1.0") without
 * doubling up when the stored value already carries one — `VERSION_PATTERN`
 * (`apis/utils/basicInfoRules.ts`) accepts a leading "v"/"V" since it's just
 * another alphanumeric character, so a version typed or imported as "v1.0"
 * would otherwise render as "vv1.0" wherever a caller prepends "v" itself.
 */
export const versionLabel = (version?: string): string => {
  if (!version) return '';
  return /^v/i.test(version) ? version : `v${version}`;
};
