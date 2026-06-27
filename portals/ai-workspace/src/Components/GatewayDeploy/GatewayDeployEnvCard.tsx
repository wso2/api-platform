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
import { Box, Button, IconButton, Typography } from '@wso2/oxygen-ui';
import { SquarePen } from 'lucide-react';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
import GatewayDeploymentSelector from './GatewayDeploymentSelector';
import { HybridGateway } from '../../apis/gateway/types';
import NoData from '../../assets/images/NoData.svg';

const STATUS_REASON_MESSAGES: Record<string, string> = {
  GATEWAY_PROCESSING_ERROR: 'Failed to process deployment. Please check gateway logs.',
  DEPLOYMENT_TIMEOUT: 'Deployment timed out. Gateway did not respond.',
};

interface GatewayDeployEnvCardProps {
  gateway: HybridGateway;
  isGatewayActive: boolean;
}

/**
 * Environment card for a specific gateway showing deploy status and actions.
 */
export default function GatewayDeployEnvCard({
  gateway,
  isGatewayActive,
}: GatewayDeployEnvCardProps) {
  const {
    deployments,
    deployToGateway,
    undeployDeployment,
    redeployDeployment,
    deployingGatewayId,
    isDeployingToGateway,
    isPollingGateway,
  } = useGatewayDeploy();

  const isThisGatewayDeploying = deployingGatewayId === gateway.handle;
  const isPolling = isPollingGateway(gateway.handle);
  const [selectorOpen, setSelectorOpen] = useState(false);

  // Compute current deployment for this gateway
  const { currentDeployment, deploymentStatus } = useMemo(() => {
    if (!deployments?.list) {
      return {
        currentDeployment: null,
        deploymentStatus: 'NOT_DEPLOYED' as const,
      };
    }

    const gatewayDeployments = deployments.list
      .filter((d) => d.gatewayHandle === gateway.handle)
      .sort((a, b) => {
        const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return timeB - timeA;
      });

    if (gatewayDeployments.length === 0) {
      return {
        currentDeployment: null,
        deploymentStatus: 'NOT_DEPLOYED' as const,
      };
    }

    const deployed = gatewayDeployments.find((d) => d.status === 'DEPLOYED');
    const latest = gatewayDeployments[0];
    const current = deployed ?? latest;
    return {
      currentDeployment: current,
      deploymentStatus: current.status as string,
    };
  }, [deployments, gateway.handle]);

  // When gateway is not active, override status to "Not Active"
  const effectiveStatus = (!isGatewayActive && currentDeployment) ? 'NOT_ACTIVE' : deploymentStatus;

  const isDeployed = effectiveStatus === 'DEPLOYED';
  const isUndeployed = effectiveStatus === 'UNDEPLOYED';
  const isDeploying = effectiveStatus === 'DEPLOYING';
  const isUndeploying = effectiveStatus === 'UNDEPLOYING';
  const isFailed = effectiveStatus === 'FAILED';
  const isNotActive = effectiveStatus === 'NOT_ACTIVE';
  const hasDeployment = currentDeployment !== null;

  const handleUndeploy = async () => {
    if (!currentDeployment?.deploymentId) return;
    await undeployDeployment(currentDeployment.deploymentId, gateway.handle);
  };

  const handleRedeploy = async () => {
    if (!currentDeployment?.deploymentId) return;
    await redeployDeployment(currentDeployment.deploymentId, gateway.handle);
  };

  const formattedDate = currentDeployment?.createdAt
    ? new Date(currentDeployment.createdAt).toLocaleString()
    : null;

  const getRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = Date.now();
    const diffMs = now - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'just now';
    if (diffMins < 60)
      return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24)
      return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
  };

  const relativeTime = currentDeployment?.createdAt
    ? getRelativeTime(currentDeployment.createdAt)
    : null;

  if (!hasDeployment) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          gap: 1,
          p: 1,
        }}
      >
        <Box
          component="img"
          src={NoData}
          alt="No deployment data"
          sx={{ width: 80, maxWidth: '100%', height: 'auto' }}
        />
        <Typography color="text.secondary">Not yet deployed</Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ p: 1 }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          mb: 2,
        }}
      >
        {relativeTime && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 500 }}>
              {isDeployed ? 'Deployed' : 'Last deployed'}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              ⏱ {relativeTime}
            </Typography>
          </Box>
        )}
        {isDeployed && (
          <Button
            variant="outlined"
            color="error"
            size="small"
            onClick={handleUndeploy}
            disabled={!isGatewayActive || isDeployingToGateway || isPolling}
          >
            {isThisGatewayDeploying ? 'Stopping...' : 'Stop'}
          </Button>
        )}
        {isDeploying && (
          <Button
            variant="outlined"
            color="error"
            size="small"
            disabled
          >
            Stop
          </Button>
        )}
        {isUndeploying && (
          <Button
            variant="outlined"
            color="primary"
            size="small"
            disabled
          >
            Redeploy
          </Button>
        )}
        {isFailed && (
          <Button
            variant="outlined"
            color="primary"
            size="small"
            onClick={handleRedeploy}
            disabled={!isGatewayActive || isDeployingToGateway || isPolling}
          >
            Re-deploy
          </Button>
        )}
        {(isUndeployed || deploymentStatus === 'ARCHIVED') && (
          <Button
            variant="outlined"
            color="primary"
            size="small"
            onClick={handleRedeploy}
            disabled={!isGatewayActive || isDeployingToGateway || isPolling}
          >
            {isThisGatewayDeploying ? 'Redeploying...' : 'Redeploy'}
          </Button>
        )}
      </Box>

      {/* Deployment Status bar */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 2,
          py: 1.5,
          mb: (isFailed && currentDeployment?.statusReason) ? 0 : 2,
          borderRadius: 1,
          bgcolor: isNotActive
            ? 'grey.100'
            : isDeployed
              ? 'rgba(46, 125, 50, 0.08)'
              : isUndeployed
                ? 'grey.100'
                : (isDeploying || isUndeploying)
                  ? 'rgba(237, 108, 2, 0.08)'
                  : isFailed
                    ? 'rgba(211, 47, 47, 0.08)'
                    : 'action.hover',
        }}
      >
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Deployment Status
        </Typography>
        <Typography
          variant="body2"
          sx={{
            fontWeight: 600,
            color: isNotActive
              ? '#7E7E7E'
              : isDeployed
                ? 'success.main'
                : isUndeployed
                  ? '#7E7E7E'
                  : (isDeploying || isUndeploying)
                    ? 'warning.main'
                    : isFailed
                      ? 'error.main'
                      : 'text.secondary',
          }}
        >
          {isNotActive ? 'Not Active' : isDeployed ? 'Active' : isUndeployed ? 'Suspended' : isDeploying ? 'Deploying' : isUndeploying ? 'Undeploying' : isFailed ? 'Failed' : deploymentStatus}
        </Typography>
      </Box>

      {isFailed && currentDeployment?.statusReason && (
        <Box px={2} mb={2}>
          <Typography
            variant="caption"
            style={{ color: '#FD6B6B' }}
            data-cyid="deployment-status-reason"
          >
            {STATUS_REASON_MESSAGES[currentDeployment.statusReason] ?? currentDeployment.statusReason}
          </Typography>
        </Box>
      )}

      {/* Deployment info box */}
      <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
        Deployment
      </Typography>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          p: 2,
          mb: 2,
        }}
      >
        <Box>
          {currentDeployment?.deploymentId && (
            <Typography variant="body2" sx={{ fontWeight: 500 }}>
              ID {currentDeployment.deploymentId.slice(0, 8)}
            </Typography>
          )}
          {relativeTime && (
            <Typography variant="caption" color="text.secondary">
              Deployed ⏱ {relativeTime}
            </Typography>
          )}
        </Box>
        <IconButton
          size="small"
          onClick={() => setSelectorOpen(true)}
          title="Change deployment"
          disabled={!isGatewayActive}
        >
          <SquarePen size={16} />
        </IconButton>
      </Box>

      {/* Deployment selector / restore drawer */}
      <GatewayDeploymentSelector
        gatewayHandle={gateway.handle}
        open={selectorOpen}
        onClose={() => setSelectorOpen(false)}
      />
    </Box>
  );
}
