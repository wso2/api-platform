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
 * Substrings browsers use when a dynamic `import()` cannot be fetched. Matched
 * case-insensitively because the wording differs per engine: Chrome says
 * "Failed to fetch dynamically imported module", Firefox "error loading
 * dynamically imported module", Safari "Importing a module script failed".
 */
const CHUNK_LOAD_MESSAGES = [
  'failed to fetch dynamically imported module',
  'error loading dynamically imported module',
  'importing a module script failed',
  'unable to preload css',
  'loading chunk',
  'loading css chunk',
];

/**
 * Whether a caught error is a failed code-split chunk fetch rather than a fault
 * in the page's own logic.
 *
 * Worth separating because the two need opposite recovery actions. Every page
 * in `AppRoutes` is `lazy()`, so after a deploy an already-open tab asks for a
 * chunk hash the server no longer has: the code is fine, the *bundle* the tab
 * is running is stale. Reloading fetches the new index and fixes it, whereas
 * for a genuine render fault reloading the same URL just reproduces it — which
 * is why the generic fallback offers "try again"/"go home" instead.
 */
export const isChunkLoadError = (error: unknown): boolean => {
  if (!error) return false;

  const named = (error as { name?: unknown }).name;
  if (named === 'ChunkLoadError') return true;

  const message = (error as { message?: unknown }).message;
  if (typeof message !== 'string') return false;

  const normalized = message.toLowerCase();
  return CHUNK_LOAD_MESSAGES.some((needle) => normalized.includes(needle));
};
