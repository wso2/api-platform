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

import { runtimeConfig } from '@/config/runtime';
import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { InsightsPageContent } from './InsightsPageContent';

export function InsightsPage() {
  // Cloud-proxy insights is not API-scoped yet (see InsightsPageContent), so
  // only that branch needs the scope prompt; the on-prem panel below is a
  // static external link that needs no API context to render.
  if (runtimeConfig.cloudProxyEnabled) {
    return (
      <ScopeGate
        graphqlTo={routes.graphqlApiInsightsApi}
        prompt="Insights are reported per API."
        requires="api"
        to={routes.apiInsightsApi}
      >
        <InsightsPageContent />
      </ScopeGate>
    );
  }

  return <InsightsPageContent />;
}
