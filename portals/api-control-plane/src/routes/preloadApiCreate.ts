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

const importApiCreationWizard = () =>
  import('../pages/appShell/appShellPages/apis/create/ApiCreationWizard');

/**
 * Starts fetching the Create API wizard's chunk ahead of the click, so opening
 * it is not a cold lazy-load (a transform-per-module wait in dev, a chunk
 * download in production). Safe to call repeatedly: the module is cached, and a
 * failed fetch is retried by the route's own `lazy()` import.
 */
export const preloadApiCreationWizard = () => {
  importApiCreationWizard().catch(() => undefined);
};
