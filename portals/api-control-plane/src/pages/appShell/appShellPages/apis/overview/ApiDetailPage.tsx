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

import { useMemo } from 'react';
import { AppPage } from '@/components/AppPage';
import { REST_API_TYPE } from '@/api/resources/apiPublications';
import { useRestApi } from '@/api/resources/restApis';
import type { Gateway } from '@/api/resources/gateways';
import { useDeployments } from '@/api/resources/restApis/deployments';
import { useRestApiGateways } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiOverviewHeader } from './ApiOverviewHeader';
import { OverviewTab } from './OverviewTab';
import { ProgressBanner } from './ProgressBanner';
import { useCan } from '@/permissions/useCan';

// No `ScopeGate`: this page is the API tier of the sidebar's Overview item, which
// degrades to a shallower tier rather than linking here without an API.
export function ApiDetailPage() {
  return (
    <AppPage>
      <ApiDetailPageContent />
    </AppPage>
  );
}

function ApiDetailPageContent() {
  const { params } = useConsoleScope();
  const canEdit = useCan('UpdateRESTAPI');
  const apiQuery = useRestApi(params.apiHandler);
  const apiGatewaysQuery = useRestApiGateways(apiQuery.data?.id);
  const deploymentsQuery = useDeployments(apiQuery.data?.id);
  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = apiGatewaysQuery.data?.list ?? [];
    const deployments = deploymentsQuery.data?.list ?? [];
    const latestByGateway = new Map<string, number>();
    deployments
      .filter((deployment) => deployment.status === 'DEPLOYED')
      .forEach((deployment) => {
        const time = new Date(deployment.createdAt || 0).getTime();
        const current = latestByGateway.get(deployment.gatewayId);
        if (current === undefined || time > current)
          latestByGateway.set(deployment.gatewayId, time);
      });
    return gateways
      .filter((gateway) => gateway.isDeployed || latestByGateway.has(gateway.id ?? ''))
      .sort(
        (a, b) => (latestByGateway.get(b.id ?? '') || 0) - (latestByGateway.get(a.id ?? '') || 0),
      );
  }, [apiGatewaysQuery.data, deploymentsQuery.data]);

  if (apiQuery.isLoading) return <LoadingState label="Loading API" />;
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title="API not found" />;
  }

  const api = apiQuery.data;
  const restApiId = api.id ?? params.apiHandler ?? '';

  // The page is the API tier of the sidebar's Overview item, so it only ever
  // mounts with all three handles in the URL; `apiPath` degrades to the
  // scope-less alias for anything still missing.
  const deployPath = routes.apiDeploy(
    params.orgHandle ?? '',
    params.projectHandler ?? null,
    params.apiHandler ?? null,
  );

  // Same reasoning: the page only mounts fully scoped, so the edit page's path
  // is always the fully-scoped one.
  const editPath = routes.apiEdit(
    params.orgHandle ?? '',
    params.projectHandler ?? '',
    params.apiHandler ?? '',
  );

  return (
    <>
      <ApiOverviewHeader
        api={api}
        apiId={restApiId}
        apiType={REST_API_TYPE}
        banner={<ProgressBanner api={api} deployed={deployedGateways.length > 0} />}
        canEdit={canEdit}
        deployOperation="DeployAPI"
        deployPath={deployPath}
        editPath={editPath}
      />

      <OverviewTab
        api={api}
        deployedGateways={deployedGateways}
        deployments={deploymentsQuery.data?.list ?? []}
      />
    </>
  );
}
