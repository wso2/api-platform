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
import { FormattedMessage } from 'react-intl';

/**
 * Body of the runtime-logs page — identical for REST and GraphQL APIs (an
 * MVP placeholder that only echoes the API handle in the subheader), so it is
 * shared directly rather than forked. Only the outer scope guard differs per
 * kind: `RuntimeLogsPage` wraps this in `ScopeGate`, `GraphqlObservabilityLogsPage`
 * in `GraphqlApiPageGuard`.
 */
export function RuntimeLogsContent({ apiHandler }: { apiHandler: string | undefined }) {
  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage id="appShell.runtimeLogsPage.header" defaultMessage="Observability" />
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
