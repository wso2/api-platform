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

import { useEffect, useMemo, useRef, useState } from 'react';
import { Box, PageTitle, TextField, Typography } from '@wso2/oxygen-ui';
import { useNavigate } from 'react-router-dom';

import { useGateways } from '@/api/resources/gateways';
import { useRestApi } from '@/api/resources/restApis';
import { useDeployments } from '@/api/resources/restApis/deployments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { ScopeGate } from '@/scope/ScopeGate';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { GatewayDeployCard } from './components/GatewayDeployCard';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.title',
    defaultMessage: 'Deploy {apiName}',
    description:
      'Page heading. {apiName} is the API display name, user-supplied; do not translate it.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.subtitle',
    defaultMessage:
      'Deploy the current working copy to a gateway, and manage existing deployments.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.loading',
    defaultMessage: 'Loading deploy state',
    description: 'Shown while the API, its gateways and its deployments are being fetched.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.apiNotFound',
    defaultMessage: 'API not found',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.searchPlaceholder',
    defaultMessage: 'Search gateways',
  },
  noGatewaysMatchSearch: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.noGatewaysMatchSearch',
    defaultMessage: 'No gateways match your search',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.emptyTitle',
    defaultMessage: 'No gateway added yet',
    description: 'Empty state when the organization has no gateways to deploy to.',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.emptyDescription',
    defaultMessage: 'Add a gateway to get started with deployment.',
  },
  addGateway: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.addGateway',
    defaultMessage: 'Add Gateway',
    description: 'Empty-state action opening the gateway creation page. Verb phrase.',
  },
});

/**
 * The deploy path, ai-workspace style: one expandable card per gateway with
 * a one-click Deploy (auto-named), per-gateway status and history.
 */
export function DeployPage() {
  return (
    <ScopeGate prompt="Deployments are made for a single API." requires="api" to={routes.apiDeploy}>
      <Deploy />
    </ScopeGate>
  );
}

function Deploy() {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const orgHandle = params.orgHandle ?? '';
  const navigate = useNavigate();
  const apiQuery = useRestApi(params.apiHandler);
  const gatewaysQuery = useGateways();
  // The deployments query is gated on the handle rather than on the loaded API:
  // both are keyed by the same value, so waiting for the API detail would delay
  // this request for no benefit.
  const deploymentsQuery = useDeployments(params.apiHandler);

  const [searchQuery, setSearchQuery] = useState('');
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // Active (connected) gateways first, like ai-workspace.
  const sortedGateways = useMemo(() => {
    const gateways = gatewaysQuery.data?.list ?? [];
    return [...gateways].sort((a, b) => {
      const aActive = a.isActive === true;
      const bActive = b.isActive === true;
      if (aActive === bActive) return 0;
      return aActive ? -1 : 1;
    });
  }, [gatewaysQuery.data]);

  // Expand the first gateway once the list arrives.
  const appliedInitialExpand = useRef(false);
  useEffect(() => {
    if (appliedInitialExpand.current || sortedGateways.length === 0) return;
    appliedInitialExpand.current = true;
    setExpandedIds(new Set([sortedGateways[0].id ?? '']));
  }, [sortedGateways]);

  // `isPending`, not `isLoading`: both queries are gated on scope, and a
  // disabled query reports `isLoading: false` with no data — which would fall
  // through to the error branch while the scope is still resolving.
  if (apiQuery.isPending || gatewaysQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  const api = apiQuery.data;
  const restApiId = api.id ?? params.apiHandler ?? '';
  const deployments = deploymentsQuery.data?.list ?? [];

  const filteredGateways = searchQuery.trim()
    ? sortedGateways.filter((gateway) => {
        const query = searchQuery.toLowerCase().trim();
        // One field, not two: the domain type this replaced set its `name` from
        // `displayName`, so matching both only ever matched the same string.
        return gateway.displayName.toLowerCase().includes(query);
      })
    : sortedGateways;

  const toggleExpand = (gatewayId: string, expanded: boolean) => {
    setExpandedIds((previous) => {
      const next = new Set(previous);
      if (expanded) next.add(gatewayId);
      else next.delete(gatewayId);
      return next;
    });
  };

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ apiName: api.displayName }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      {sortedGateways.length === 0 ? (
        <EmptyState
          actionLabel={intl.formatMessage(messages.addGateway)}
          description={intl.formatMessage(messages.emptyDescription)}
          onAction={() => navigate(routes.newGateway(orgHandle))}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <>
          <Box sx={{ mb: 3, width: '100%' }}>
            <TextField
              fullWidth
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder={intl.formatMessage(messages.searchPlaceholder)}
              size="small"
              value={searchQuery}
            />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {filteredGateways.length === 0 ? (
              <Box sx={{ color: 'text.secondary', p: 6, textAlign: 'center' }}>
                <Typography>
                  <FormattedMessage {...messages.noGatewaysMatchSearch} />
                </Typography>
              </Box>
            ) : (
              filteredGateways.map((gateway) => (
                <GatewayDeployCard
                  deployments={deployments}
                  gateway={gateway}
                  isExpanded={expandedIds.has(gateway.id ?? '')}
                  key={gateway.id}
                  onRefresh={() => deploymentsQuery.refetch()}
                  onToggleExpand={(expanded) => toggleExpand(gateway.id ?? '', expanded)}
                  refreshing={deploymentsQuery.isFetching}
                  restApiId={restApiId}
                />
              ))
            )}
          </Box>
        </>
      )}
    </>
  );
}
