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

import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { ApiList } from './ApiList';

export function ApiListPage() {
  // Gating the whole body, not just the JSX: out of project scope `useRestApis`
  // stays disabled and `isPending` never clears, so the loading branch below
  // would sit there forever instead of the scope prompt showing.
  return (
    <ScopeGate
      prompt="APIs are created and managed at the project level."
      requires="project"
      to={routes.apis}
    >
      <ApiList />
    </ScopeGate>
  );
}
