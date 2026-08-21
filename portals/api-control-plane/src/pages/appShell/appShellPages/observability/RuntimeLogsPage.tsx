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

import { Card, CardContent, CodeBlock, PageTitle } from '@wso2/oxygen-ui';
import { useParams } from 'react-router-dom';

import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';
import { FormattedMessage } from 'react-intl';

export function RuntimeLogsPage() {
  return (
    <ScopeGate
      prompt="Runtime logs are streamed per API."
      requires="api"
      to={routes.apiObservabilityLogs}
    >
      <RuntimeLogs />
    </ScopeGate>
  );
}

function RuntimeLogs() {
  const { apiHandler } = useParams();

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage
            id="appShell.runtimeLogsPage.header"
            defaultMessage="Observability"
          />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage
            id="appShell.runtimeLogsPage.subHeader"
            defaultMessage="Runtime logs for {apiHandler}."
            values={{ apiHandler }}
          />
        </PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <CodeBlock
            language="bash"
            code={`[info] Runtime log streaming integration point
            [info] Advanced filters and live tail are deferred from the MVP`}
          />
        </CardContent>
      </Card>
    </>
  );
}
