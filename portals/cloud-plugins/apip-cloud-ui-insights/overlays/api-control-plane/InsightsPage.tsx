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
 *
 * Cloud build overlay for API Control Plane Insights. Copy this file over
 * `portals/api-control-plane/src/pages/appShell/appShellPages/insights/InsightsPage.tsx`
 * when assembling a cloud deployment image.
 */

import { FormattedMessage } from 'react-intl';

import { ComingSoon } from '../../../../components/ComingSoon';
import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';

export function InsightsPage() {
  return (
    <ScopeGate
      prompt="Insights are reported per API."
      requires="api"
      to={routes.apiInsightsApi}
    >
      <ComingSoon
        feature={
          <FormattedMessage
            id="appShell.insightsPage.feature"
            defaultMessage="API insights"
          />
        }
      />
    </ScopeGate>
  );
}
