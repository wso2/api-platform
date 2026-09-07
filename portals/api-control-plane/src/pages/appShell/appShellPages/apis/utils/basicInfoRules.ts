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
 * The field rules for an API's basic information; the same five fields the
 * create wizard collects (`displayName`, `id`, `version`, `context`,
 * `upstream.main.url`) and the edit page later changes.
 *
 * They live here rather than in either form because both forms validate the
 * same platform constraints: a second copy would let the create form accept a
 * context the edit form rejects, or the reverse.
 */

/** Longest handle the platform accepts as an API identifier. */
export const HANDLE_MAX_LENGTH = 64;

/** Lowercase words joined by single hyphens — what a URL segment may hold. */
export const HANDLE_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/** A version has to survive being pasted into a path segment. */
export const VERSION_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

/** A context is an absolute path: leading slash, then path-safe characters. */
export const CONTEXT_PATTERN = /^\/[A-Za-z0-9\-._~/]*$/;

/** Whether `value` parses as an absolute `http(s)` URL. */
export const isHttpUrl = (value: string): boolean => {
  try {
    const { protocol } = new URL(value);
    return protocol === 'http:' || protocol === 'https:';
  } catch {
    return false;
  }
};
