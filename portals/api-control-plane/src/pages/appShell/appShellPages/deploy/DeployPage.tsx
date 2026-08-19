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
import {
  Box,
  PageContent,
  PageTitle,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { useNavigate, useParams } from 'react-router-dom';

import {
  useApi,
  useGatewayDeployments,
  useGateways,
} from '../../../../api/hooks/useMvpQueries';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../../../components/StateViews';
import { routes } from '../../../../routes/paths';
import { GatewayDeployCard } from './GatewayDeployCard';

/**
 * The deploy path, ai-workspace style: one expandable card per gateway with
 * a one-click Deploy (auto-named), per-gateway status and history.
 */
export function DeployPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const apiQuery = useApi();
  const gatewaysQuery = useGateways();
  const deploymentsQuery = useGatewayDeployments(apiQuery.data);

  const [searchQuery, setSearchQuery] = useState('');
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // Active (connected) gateways first, like ai-workspace.
  const sortedGateways = useMemo(() => {
    const gateways = gatewaysQuery.data || [];
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
    setExpandedIds(new Set([sortedGateways[0].id]));
  }, [sortedGateways]);

  if (apiQuery.isLoading || gatewaysQuery.isLoading) {
    return <LoadingState label="Loading deploy state" />;
  }
  if (!apiQuery.data) return <ErrorState title="API not found" />;

  const api = apiQuery.data;
  const deployments = deploymentsQuery.data || [];

  const filteredGateways = searchQuery.trim()
    ? sortedGateways.filter((gateway) => {
        const query = searchQuery.toLowerCase().trim();
        return (
          gateway.name.toLowerCase().includes(query) ||
          gateway.displayName.toLowerCase().includes(query)
        );
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
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Deploy {api.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>
          Deploy the current working copy to a gateway, and manage existing
          deployments.
        </PageTitle.SubHeader>
      </PageTitle>

      {sortedGateways.length === 0 ? (
        <EmptyState
          actionLabel="Add Gateway"
          description="Add a gateway to get started with deployment."
          onAction={() => navigate(routes.newGateway(orgHandle))}
          title="No gateway added yet"
        />
      ) : (
        <>
          <Box sx={{ mb: 3, width: '100%' }}>
            <TextField
              fullWidth
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Search gateways"
              size="small"
              value={searchQuery}
            />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {filteredGateways.length === 0 ? (
              <Box sx={{ color: 'text.secondary', p: 6, textAlign: 'center' }}>
                <Typography>No gateways match your search</Typography>
              </Box>
            ) : (
              filteredGateways.map((gateway) => (
                <GatewayDeployCard
                  api={api}
                  deployments={deployments}
                  gateway={gateway}
                  isExpanded={expandedIds.has(gateway.id)}
                  key={gateway.id}
                  onRefresh={() => deploymentsQuery.refetch()}
                  onToggleExpand={(expanded) =>
                    toggleExpand(gateway.id, expanded)
                  }
                  refreshing={deploymentsQuery.isFetching}
                />
              ))
            )}
          </Box>
        </>
      )}
    </PageContent>
  );
}
