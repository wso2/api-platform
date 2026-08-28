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

import {
  Card,
  CardContent,
  CodeBlock,
  PageTitle,
} from '@wso2/oxygen-ui';

import { useApiProxy, useApi } from '../../../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../../../components/StateViews';
import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';
import { FormattedMessage } from 'react-intl';

export function TestPage() {
  return (
    <ScopeGate
      prompt="The curl console runs against a single API."
      requires="api"
      to={routes.apiTestCurl}
    >
      <Test />
    </ScopeGate>
  );
}

function Test() {
  const apiQuery = useApi();
  const apiProxyQuery = useApiProxy(apiQuery.data?.id);

  if (apiQuery.isLoading) return <LoadingState label="Loading test console" />;
  if (!apiQuery.data) return <ErrorState title="API not found" />;

  const context = apiProxyQuery.data?.context || `/${apiQuery.data.name}`;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage
            id="appShell.testPage.header"
            defaultMessage="Test {apiName}"
            values={{ apiName: apiQuery.data.displayName }}
          /> 
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage
            id="appShell.testPage.subHeader"
            defaultMessage="Use the following curl command to test the API."
          />
        </PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <CodeBlock
            language="bash"
            code={`curl -X GET "$API_BASE_URL${context}" \\
            -H "Authorization: Bearer <token>"`}
          />
        </CardContent>
      </Card>
    </>
  );
}
