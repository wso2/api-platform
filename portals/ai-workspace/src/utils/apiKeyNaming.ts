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
 * The identifier an API key is created under, derived from what it is called.
 *
 * The identifier is constrained to lower-case words joined by hyphens, so a
 * name a person would write has to be narrowed to that before it is sent. A
 * name that narrows to nothing still needs an identifier, so it falls back
 * rather than sending an empty one.
 */
export const buildApiKeyResourceName = (displayName: string): string => {
  const normalised = displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalised || 'api-key';
};

export default buildApiKeyResourceName;
