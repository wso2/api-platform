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
import { Alert, Box, PageTitle, TextField, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useGateways } from '@/api/resources/gateways';
import type { Deployment } from '@/api/resources/restApis/deployments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { GatewayDeployCard, type UseDeployMutationHook } from './GatewayDeployCard';
import type { UseDeploymentMutationHook } from './GatewayDeploymentSelector';

// IDs keep the page they were written for, so existing translations carry over.
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
  gatewayManaged: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.DeployPage.gatewayManaged',
    defaultMessage:
      'This API was created from a gateway. You can view its deployments, but deploy, redeploy, restore and undeploy actions are managed by the gateway and are unavailable in the API Control Plane.',
    description:
      'Info banner on the Deploy page of a read-only API, i.e. one that a gateway pushed to the control plane.',
  },
});

/** The deploy and deployment-lifecycle mutations, adapted per API kind. */
export type DeployMutationHooks = {
  useDelete: UseDeploymentMutationHook;
  useDeploy: UseDeployMutationHook;
  useRestore: UseDeploymentMutationHook;
  useUndeploy: UseDeploymentMutationHook;
};

export type ApiDeployViewProps = {
  /** The API being deployed; `undefined` once loaded means it was not found. */
  api: { displayName: string } | undefined;
  /** True while the API itself is still loading. */
  apiPending: boolean;
  /** The id the deploy mutations address the API by. */
  apiId: string;
  /** Title for the not-found state; defaults to the REST page's "API not found". */
  apiNotFoundTitle?: string;
  deployments: Deployment[];
  deploymentsFetching: boolean;
  onRefreshDeployments: () => void;
  mutations: DeployMutationHooks;
  orgHandle: string;
  /**
   * A gateway-created API. Its deployments are shown, but the gateway owns them,
   * so every action that would change them is disabled.
   */
  readOnly?: boolean;
};

/**
 * The deploy path, ai-workspace style: one expandable card per gateway with
 * a one-click Deploy (auto-named), per-gateway status and history. Shared by
 * every API kind's Deploy page — each supplies its own API, deployments and
 * mutations, so the pages cannot drift apart.
 */
export function ApiDeployView({
  api,
  apiId,
  apiNotFoundTitle,
  apiPending,
  deployments,
  deploymentsFetching,
  mutations,
  onRefreshDeployments,
  orgHandle,
  readOnly = false,
}: ApiDeployViewProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const gatewaysQuery = useGateways();

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
  if (apiPending || gatewaysQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!api) {
    return <ErrorState title={apiNotFoundTitle ?? intl.formatMessage(messages.apiNotFound)} />;
  }

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

      {readOnly && (
        // Padding, not margin: AppPage's Stack resets its children's margins,
        // which would silently drop an `mt`/`mb` on the Alert itself.
        <Box sx={{ pb: 1, pt: 3 }}>
          <Alert severity="info">
            <FormattedMessage {...messages.gatewayManaged} />
          </Alert>
        </Box>
      )}

      {sortedGateways.length === 0 ? (
        <EmptyState
          actionLabel={intl.formatMessage(messages.addGateway)}
          description={intl.formatMessage(messages.emptyDescription)}
          onAction={() => navigate(routes.newGateway(orgHandle))}
          title={intl.formatMessage(messages.emptyTitle)}
          operationId="CreateGateway"
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
                  apiId={apiId}
                  deployments={deployments}
                  gateway={gateway}
                  isExpanded={expandedIds.has(gateway.id ?? '')}
                  key={gateway.id}
                  onRefresh={onRefreshDeployments}
                  onToggleExpand={(expanded) => toggleExpand(gateway.id ?? '', expanded)}
                  readOnly={readOnly}
                  refreshing={deploymentsFetching}
                  useDelete={mutations.useDelete}
                  useDeploy={mutations.useDeploy}
                  useRestore={mutations.useRestore}
                  useUndeploy={mutations.useUndeploy}
                />
              ))
            )}
          </Box>
        </>
      )}
    </>
  );
}
