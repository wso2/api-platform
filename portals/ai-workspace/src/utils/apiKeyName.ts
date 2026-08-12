/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Must match apiKeyNameMinLength/apiKeyNameMaxLength in platform-api's apikey.go,
// which itself mirrors the gateway's ValidateAPIKeyName rule — keep all three in sync.
export const API_KEY_NAME_MIN_LENGTH = 1;
export const API_KEY_NAME_MAX_LENGTH = 100;

/**
 * Slugifies a display name into a candidate API key id. Does NOT pad short results —
 * callers must validate the result and reject with a message rather than padding,
 * since the user typed this value directly into the field it derives from.
 */
export function slugifyApiKeyName(displayName: string): string {
  return displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/** Validates a candidate id against the same rule the gateway enforces. Returns null when valid. */
export function validateApiKeyName(name: string): string | null {
  if (!name) return 'API key name is required.';
  if (name.length < API_KEY_NAME_MIN_LENGTH) {
    return `Must be at least ${API_KEY_NAME_MIN_LENGTH} character(s).`;
  }
  if (name.length > API_KEY_NAME_MAX_LENGTH) {
    return `Must be at most ${API_KEY_NAME_MAX_LENGTH} character(s).`;
  }
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(name)) {
    return 'API key name must be lowercase alphanumeric with hyphens only.';
  }
  return null;
}
