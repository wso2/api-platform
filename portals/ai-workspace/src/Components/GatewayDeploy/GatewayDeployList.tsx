/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Box, Button, TextField, Typography } from '@wso2/oxygen-ui';
import { useNavigate } from 'react-router-dom';
import type { Gateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
import { useAppAuth } from '../../contexts/AppAuthContext';
import { SCOPES } from '../../auth/permissions';
import { useAppShell } from '../../contexts/AppShellContext';
import { buildOrgPath } from '../../utils/projectRouting';
import GatewayDeployCard from './GatewayDeployCard';
import NoDataImage from '../../assets/images/NoData.svg';

interface GatewayDeployListProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  initialExpandedIds?: string[];
  onConfigureAndDeploy?: (gateway: Gateway) => void;
}

/**
 * List of hybrid gateways with search and expandable cards.
 * Duplicated from choreo-console HybridGatewayList, adapted for oxygen-ui.
 */
export default function GatewayDeployList({
  searchQuery,
  onSearchChange,
  initialExpandedIds = [],
  onConfigureAndDeploy,
}: GatewayDeployListProps) {
  const { gateways } = useGatewayDeploy();
  const { hasPermission } = useAppAuth();
  const { currentOrganization } = useAppShell();
  const navigate = useNavigate();

  const [expandedGatewayIds, setExpandedGatewayIds] = useState<Set<string>>(
    () => new Set(initialExpandedIds)
  );

  const hasAppliedInitialRef = React.useRef(false);
  useEffect(() => {
    if (hasAppliedInitialRef.current || initialExpandedIds.length === 0) return;
    hasAppliedInitialRef.current = true;
    setExpandedGatewayIds(new Set(initialExpandedIds));
  }, [initialExpandedIds]);

  const aiGateways = useMemo(() => {
    return gateways.filter((gw) => gw.functionalityType === 'ai');
  }, [gateways]);

  const filteredGateways = useMemo(() => {
    if (!searchQuery.trim()) return aiGateways;
    const query = searchQuery.toLowerCase().trim();
    return aiGateways.filter(
      (g) =>
        (g.name || '').toLowerCase().includes(query) ||
        (g.displayName || '').toLowerCase().includes(query)
    );
  }, [aiGateways, searchQuery]);

  // Sort: active (connected) gateways first
  const sortedGateways = useMemo(
    () =>
      [...filteredGateways].sort((a, b) => {
        const aActive = a.status === 'connected';
        const bActive = b.status === 'connected';
        if (aActive === bActive) return 0;
        return aActive ? -1 : 1;
      }),
    [filteredGateways]
  );

  const handleToggleExpand = useCallback(
    (gatewayId: string, expanded: boolean) => {
      setExpandedGatewayIds((prev) => {
        const next = new Set(prev);
        if (expanded) {
          next.add(gatewayId);
        } else {
          next.delete(gatewayId);
        }
        return next;
      });
    },
    []
  );

  if (aiGateways.length === 0) {
    if (!hasPermission(SCOPES.GATEWAY_MANAGE)) {
      return (
        <Alert severity="warning" sx={{ mt: 2 }}>
          An AI Gateway is required to deploy. Please contact your administrator
          to add one.
        </Alert>
      );
    }

    return (
      <Box sx={{ textAlign: 'center', p: 6 }}>
        <Box
          component="img"
          src={NoDataImage}
          alt="No data"
          sx={{ width: 50, mb: 1 }}
        />
        <Typography variant="body1" gutterBottom>
          No AI Gateway Added Yet
        </Typography>
        <Typography variant="body2" sx={{ mb: 3 }}>
          Add an AI Gateway to get started with deployment.
        </Typography>
        <Button
          variant="contained"
          onClick={() =>
            navigate(buildOrgPath(currentOrganization, '/gateways/new'))
          }
        >
          Add Gateway
        </Button>
      </Box>
    );
  }

  return (
    <>
      <Box sx={{ width: '100%', mb: 3 }}>
        <TextField
          placeholder="Search gateways"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          size="small"
          fullWidth
        />
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {sortedGateways.length === 0 ? (
          <Box sx={{ textAlign: 'center', p: 6, color: 'text.secondary' }}>
            <Typography>No gateways match your search</Typography>
          </Box>
        ) : (
          sortedGateways.map((gateway) => (
            <GatewayDeployCard
              key={gateway.id}
              gateway={gateway}
              isExpanded={expandedGatewayIds.has(gateway.id)}
              onToggleExpand={(expanded: any) =>
                handleToggleExpand(gateway.id, expanded)
              }
              onConfigureAndDeploy={onConfigureAndDeploy}
            />
          ))
        )}
      </Box>
    </>
  );
}
