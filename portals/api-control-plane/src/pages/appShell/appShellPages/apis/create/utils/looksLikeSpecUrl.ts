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

/** File names a spec is published under, matched on the last path segment. */
const SPEC_FILE_EXTENSION = /\.(json|ya?ml)$/i;

/** Path segments spec endpoints are conventionally served from. */
const SPEC_PATH_SEGMENT = /^(openapi|swagger|api-docs)(\.[a-z]+)?$/i;

/** Hosts that only ever serve files, never a running API. */
const FILE_HOSTS = new Set([
  'bitbucket.org',
  'gist.githubusercontent.com',
  'github.com',
  'gitlab.com',
  'raw.githubusercontent.com',
]);

/**
 * Whether a URL reads as the address of a spec document rather than of the
 * running API it describes.
 *
 * Both create tabs ask this one question: the spec tab offers "use it as an
 * endpoint" only when the answer is no, and the endpoint tab offers "import it
 * as a spec" only when it is yes. Sharing the predicate is what keeps the two
 * offers from ever sending a user back and forth between the tabs.
 *
 * A heuristic over the address alone: nothing is fetched to decide.
 */
export const looksLikeSpecUrl = (value: string): boolean => {
  let url: URL;
  try {
    url = new URL(value.trim());
  } catch {
    return false;
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return false;
  }
  if (FILE_HOSTS.has(url.hostname.toLowerCase())) {
    return true;
  }
  const segments = url.pathname.split('/').filter((segment) => segment !== '');
  const last = segments.at(-1) ?? '';
  if (SPEC_FILE_EXTENSION.test(last)) {
    return true;
  }
  return segments.some((segment) => SPEC_PATH_SEGMENT.test(segment));
};
