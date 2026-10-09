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
 * Carrying the user back to where they were before a detour into gateway
 * setup.
 *
 * Adding a gateway from an API's Deploy page used to strand the user: after
 * provisioning and connecting it, the only way back was the gateway list. The
 * page they came from now rides along as a `returnTo` query parameter through
 * the create and detail pages.
 */

const PARAM = 'returnTo';

/**
 * Only same-app paths are honoured. Anything else (a full URL, or `//host`,
 * which browsers read as one) is dropped, so the parameter can't be used to
 * send someone off-site.
 */
const isInternalPath = (value: string): boolean =>
  value.startsWith('/') && !value.startsWith('//') && !value.startsWith('/\\');

/** The return path in `search`, if there is a safe one. */
export const readReturnTo = (search: string): string | undefined => {
  const value = new URLSearchParams(search).get(PARAM);
  return value && isInternalPath(value) ? value : undefined;
};

/** `path` with the return path attached (or unchanged when there is none). */
export const withReturnTo = (path: string, returnTo: string | undefined): string =>
  returnTo && isInternalPath(returnTo)
    ? `${path}${path.includes('?') ? '&' : '?'}${PARAM}=${encodeURIComponent(returnTo)}`
    : path;
