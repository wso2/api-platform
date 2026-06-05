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

import { useMemo, useState } from 'react';
import { Box, Button, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import type { DeploymentResponse } from '../../utils/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
import GatewayDeploymentRow from './GatewayDeploymentRow';

interface GatewayDeploymentHistoryProps {
  gatewayId: string;
}

/**
 * Deployment history list for a specific gateway.
 */
export default function GatewayDeploymentHistory({
  gatewayId,
}: GatewayDeploymentHistoryProps) {
  const [isDrawerOpen, setDrawerOpen] = useState(false);
  const {
    deployments,
    isLoadingDeployments,
    deploymentsError,
    refetchDeployments,
    deleteDeployment,
  } = useGatewayDeploy();

  const gatewayDeployments =
    useMemo((): DeploymentResponse[] => {
      if (!deployments?.list) return [];

      const filtered = deployments.list.filter(
        (d) => d.gatewayId === gatewayId
      );

      return [...filtered].sort((a, b) => {
        const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return timeB - timeA;
      });
    }, [deployments, gatewayId]);

  const currentDeploymentId = useMemo(() => {
    if (gatewayDeployments.length === 0) return null;
    return gatewayDeployments[0]?.deploymentId ?? null;
  }, [gatewayDeployments]);

  const handleRefetch = () => {
    refetchDeployments();
  };

  const handleDeleteDeployment = async (deploymentId: string) => {
    await deleteDeployment(deploymentId);
  };

  if (deploymentsError) {
    return (
      <Box sx={{ mt: 1 }}>
        <Typography color="error">
          Failed to fetch the API deployment history.
        </Typography>
        <Button
          size="small"
          color="error"
          onClick={handleRefetch}
          sx={{ mt: 1 }}
        >
          Retry
        </Button>
      </Box>
    );
  }

  if (gatewayDeployments.length === 0) {
    return null;
  }

  return (
    <Box>
      {/* Heading */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          pb: 2,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Typography sx={{ fontWeight: 500, flexGrow: 1 }}>
          API Deployment History
        </Typography>
        <IconButton
          color="primary"
          onClick={handleRefetch}
          size="small"
          disabled={isLoadingDeployments}
        >
          <RefreshCw />
        </IconButton>
      </Box>

      {/* First 3 deployments */}
      <Box>
        {gatewayDeployments.slice(0, 3).map((deployment, index) => (
          <GatewayDeploymentRow
            key={deployment.deploymentId}
            deployment={deployment}
            index={index}
            totalCount={gatewayDeployments.length}
            isCurrentDeployment={
              deployment.deploymentId === currentDeploymentId
            }
            isInDrawer={false}
          />
        ))}
      </Box>

      {/* View More */}
      {gatewayDeployments.length > 3 && (
        <Box
          sx={{
            textAlign: 'center',
            pt: 1,
            borderTop: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Button
            variant="text"
            color="primary"
            onClick={() => setDrawerOpen(true)}
          >
            View More
          </Button>
        </Box>
      )}

      {/* Full history drawer */}
      <Drawer
        anchor="right"
        open={isDrawerOpen}
        onClose={() => setDrawerOpen(false)}
        sx={{ '& .MuiDrawer-paper': { width: { xs: '100%', md: 600 } } }}
      >
        <Box sx={{ p: 3 }}>
          <Typography variant="h6" sx={{ mb: 2 }}>
            API Deployment History
          </Typography>
          {gatewayDeployments.map((deployment, index) => (
            <GatewayDeploymentRow
              key={deployment.deploymentId}
              deployment={deployment}
              index={index}
              totalCount={gatewayDeployments.length}
              isCurrentDeployment={
                deployment.deploymentId === currentDeploymentId
              }
              isInDrawer
              onDelete={handleDeleteDeployment}
            />
          ))}
        </Box>
      </Drawer>
    </Box>
  );
}
