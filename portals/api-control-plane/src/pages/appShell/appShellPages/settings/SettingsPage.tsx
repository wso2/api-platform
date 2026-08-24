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

import { Card, CardContent, PageTitle, Typography } from '@wso2/oxygen-ui';

import { useConsoleScope } from '../../../../scope/ConsoleScopeProvider';

// No `ScopeGate`: Settings is the one page with no scope requirement. The sidebar
// links to the organization-level path while browsing the org and to the project's
// once one is selected, and a project card's gear deep-links the same page — so it
// renders at whatever scope it is reached in.
export function SettingsPage() {
  const { organization, params, project } = useConsoleScope();
  // Whichever scope the page was reached in, named rather than handled: the
  // heading reads "…for Retail APIs", not "…for retail-apis". Falls back to the
  // handle, which the route always carries, so the heading still says what it is
  // about while the display name is still loading.
  const subject =
    project?.displayName ??
    organization?.displayName ??
    params.projectHandler ??
    params.orgHandle;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>Settings</PageTitle.Header>
        <PageTitle.SubHeader>Minimal settings overview for {subject}.</PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <Typography>
            Advanced organization admin settings, governance, marketplace, and
            developer portal configuration are intentionally excluded from the
            MVP replacement app.
          </Typography>
        </CardContent>
      </Card>
    </>
  );
}
