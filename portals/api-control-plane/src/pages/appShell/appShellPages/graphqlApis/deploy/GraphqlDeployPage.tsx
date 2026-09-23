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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { useGateways } from '@/api/resources/gateways';
import { useGraphQLApi } from '@/api/resources/graphqlApis';
import {
  useDeleteDeployment,
  useDeployApi,
  useDeployments,
  useRestoreDeployment,
  useUndeployDeployment,
} from '@/api/resources/graphqlApis/deployments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import {
  GatewayDeployCard,
  type UseDeployMutationHook,
} from '../../deploy/components/GatewayDeployCard';
import type { UseDeploymentMutationHook } from '../../deploy/components/GatewayDeploymentSelector';

/** Adapts GraphQL's `{ graphqlApiId, ... }` mutations to the shared deploy-card
 * subtree's uniform `{ apiId, ... }` shape — see `GatewayDeployCard` for why
 * this lives at the caller rather than in the shared component. */
const useDeployForCard: UseDeployMutationHook = () => {
  const mutation = useDeployApi();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, body }, options) => mutation.mutate({ graphqlApiId: apiId, body }, options),
  };
};

const useUndeployForCard: UseDeploymentMutationHook = () => {
  const mutation = useUndeployDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const useRestoreForCard: UseDeploymentMutationHook = () => {
  const mutation = useRestoreDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const useDeleteForCard: UseDeploymentMutationHook = () => {
  const mutation = useDeleteDeployment();
  return {
    isPending: mutation.isPending,
    mutate: ({ apiId, deploymentId }, options) =>
      mutation.mutate({ graphqlApiId: apiId, deploymentId }, options),
  };
};

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.title',
    defaultMessage: 'Deploy {apiName}',
    description: 'Page heading. {apiName} is the API display name, user-supplied; do not translate it.',
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
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.GraphqlDeployPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
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
 * Fork of `deploy/DeployPage.tsx` for a GraphQL API. No `ScopeGate`: this
 * route lives outside `ConsoleScopeProvider`'s REST-only api-scope matching
 * (see `graphqlApiPath`), so it guards on its own route param instead.
 */
export function GraphqlDeployPage() {
  const intl = useIntl();
  const { orgHandle = '', graphqlApiHandler } = useParams();
  const navigate = useNavigate();
  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const gatewaysQuery = useGateways();
  const deploymentsQuery = useDeployments(graphqlApiHandler);

  const [searchQuery, setSearchQuery] = useState('');
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  const sortedGateways = useMemo(() => {
    const gateways = gatewaysQuery.data?.list ?? [];
    return [...gateways].sort((a, b) => {
      const aActive = a.isActive === true;
      const bActive = b.isActive === true;
      if (aActive === bActive) return 0;
      return aActive ? -1 : 1;
    });
  }, [gatewaysQuery.data]);

  const appliedInitialExpand = useRef(false);
  useEffect(() => {
    if (appliedInitialExpand.current || sortedGateways.length === 0) return;
    appliedInitialExpand.current = true;
    setExpandedIds(new Set([sortedGateways[0].id ?? '']));
  }, [sortedGateways]);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending || gatewaysQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  const api = apiQuery.data;
  const deployments = deploymentsQuery.data?.list ?? [];

  const filteredGateways = searchQuery.trim()
    ? sortedGateways.filter((gateway) => {
        const query = searchQuery.toLowerCase().trim();
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
                  apiId={graphqlApiHandler}
                  deployments={deployments}
                  gateway={gateway}
                  isExpanded={expandedIds.has(gateway.id ?? '')}
                  key={gateway.id}
                  onRefresh={() => deploymentsQuery.refetch()}
                  onToggleExpand={(expanded) => toggleExpand(gateway.id ?? '', expanded)}
                  refreshing={deploymentsQuery.isFetching}
                  useDelete={useDeleteForCard}
                  useDeploy={useDeployForCard}
                  useRestore={useRestoreForCard}
                  useUndeploy={useUndeployForCard}
                />
              ))
            )}
          </Box>
        </>
      )}
    </>
  );
}
