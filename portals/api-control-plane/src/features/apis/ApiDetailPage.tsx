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

import { useState } from 'react';
import { PageContent, PageTitle, Tab, Tabs } from '@wso2/oxygen-ui';

import { useApiDetail } from '../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { DocumentsTab } from './develop/DocumentsTab';
import { PolicyTab } from './develop/PolicyTab';
import { RoutingTab } from './develop/RoutingTab';
import { OverviewTab } from './overview/OverviewTab';

export function ApiDetailPage() {
  const detailQuery = useApiDetail();
  const [tab, setTab] = useState(0);

  if (detailQuery.isLoading) return <LoadingState label="Loading API" />;
  if (detailQuery.error || !detailQuery.data) {
    return <ErrorState title="API not found" />;
  }

  const detail = detailQuery.data;

  // Develop tab set mirrors the product: Overview, then Policy/Routing/Documents.
  const tabs = ['Overview', 'Policy', 'Routing', 'Documents'] as const;
  const active = tabs[tab] ?? 'Overview';

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>{detail.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>{detail.description}</PageTitle.SubHeader>
      </PageTitle>

      <Tabs
        onChange={(_event, value) => setTab(value)}
        sx={{ mb: 3 }}
        value={tab}
      >
        {tabs.map((label) => (
          <Tab key={label} label={label} />
        ))}
      </Tabs>

      {active === 'Overview' && <OverviewTab detail={detail} />}
      {active === 'Policy' && <PolicyTab detail={detail} />}
      {active === 'Routing' && <RoutingTab detail={detail} />}
      {active === 'Documents' && <DocumentsTab />}
    </PageContent>
  );
}
