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

import { runtimeConfig } from '../../config/runtime';

/**
 * Absolute base every platform-api handler is registered against.
 *
 * Derived from the same runtime config the transport uses, rather than
 * hard-coded, so a change to the version segment does not silently leave every
 * handler matching a URL nothing requests any more. Resolved against jsdom's
 * origin because the transport's `baseURL` is relative (`/api/v0.9`) and MSW
 * matches on absolute URLs.
 */
export const API_BASE = new URL(
  `${runtimeConfig.platformApiBaseUrl}/api/${runtimeConfig.platformApiVersion}`,
  window.location.origin
).toString();

/**
 * Absolute URL for a platform-api path.
 *
 * Accepts MSW path parameters, so `apiUrl('/rest-apis/:restApiId')` matches any
 * handle — useful when the id under test is incidental.
 */
export const apiUrl = (path: string): string =>
  `${API_BASE.replace(/\/$/, '')}${path}`;
