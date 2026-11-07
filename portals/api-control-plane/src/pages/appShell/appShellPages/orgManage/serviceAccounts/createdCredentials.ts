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

import type { ServiceAccountCredentials } from '@/api/resources/serviceAccounts';

/**
 * Hands a freshly issued secret from the create page to the "created" page.
 *
 * It lives in module memory on purpose: router state is written to the
 * browser's history, and the secret must not outlive the tab's page view. A
 * reload therefore loses it, which is the same as navigating away.
 */
let held: ServiceAccountCredentials | null = null;

export const holdCredentials = (credentials: ServiceAccountCredentials) => {
  held = credentials;
};

export const peekCredentials = () => held;

export const clearCredentials = () => {
  held = null;
};
