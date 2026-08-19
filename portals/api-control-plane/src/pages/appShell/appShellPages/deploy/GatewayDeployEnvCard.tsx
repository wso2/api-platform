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
import {
  alpha,
  Box,
  Button,
  IconButton,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import { PackageOpen, SquarePen } from '@wso2/oxygen-ui-icons-react';

import {
  useRestoreGatewayDeployment,
  useUndeployGatewayDeployment,
} from '../../../../api/hooks/useMvpQueries';
import { useNotifications } from '../../../../components/Notifications';
import type { Api, Gateway, GatewayDeployment } from '../../../../types/domain';
import { relativeTime } from '../../../../utils/relativeTime';
import { GatewayDeploymentSelector } from './GatewayDeploymentSelector';

const STATUS_REASON_MESSAGES: Record<string, string> = {
  GATEWAY_PROCESSING_ERROR:
    'Failed to process deployment. Please check gateway logs.',
  DEPLOYMENT_TIMEOUT: 'Deployment timed out. Gateway did not respond.',
};

type GatewayDeployEnvCardProps = {
  api: Api;
  gateway: Gateway;
  /** Deployments on this gateway, newest first. */
  deployments: GatewayDeployment[];
  currentDeployment?: GatewayDeployment;
  isGatewayActive: boolean;
};

/**
 * Left panel of an expanded gateway card: deployment status bar, failure
 * reason, deployment info box, and Stop / Redeploy actions (ai-workspace
 * GatewayDeployEnvCard).
 */
export function GatewayDeployEnvCard({
  api,
  gateway,
  deployments,
  currentDeployment,
  isGatewayActive,
}: GatewayDeployEnvCardProps) {
  const theme = useTheme();
  const { notify } = useNotifications();
  const undeployMutation = useUndeployGatewayDeployment();
  const restoreMutation = useRestoreGatewayDeployment();
  const [selectorOpen, setSelectorOpen] = useState(false);

  const status = currentDeployment?.status;
  // When the gateway is offline the deployment state is unknowable — show
  // "Not Active" instead (ai-workspace behaviour).
  const effective =
    !isGatewayActive && currentDeployment ? 'NOT_ACTIVE' : status;
  const isDeployed = effective === 'DEPLOYED';
  const isUndeployed = effective === 'UNDEPLOYED';
  const isDeploying = effective === 'DEPLOYING';
  const isUndeploying = effective === 'UNDEPLOYING';
  const isFailed = effective === 'FAILED';
  const isNotActive = effective === 'NOT_ACTIVE';
  const busy = undeployMutation.isPending || restoreMutation.isPending;

  if (!currentDeployment) {
    return (
      <Box
        sx={{
          alignItems: 'center',
          color: 'text.secondary',
          display: 'flex',
          flexDirection: 'column',
          gap: 1,
          justifyContent: 'center',
          p: 3,
        }}
      >
        <PackageOpen size={40} strokeWidth={1.25} />
        <Typography color="text.secondary">Not yet deployed</Typography>
      </Box>
    );
  }

  const handleUndeploy = () => {
    undeployMutation.mutate(
      { api, deployment: currentDeployment },
      {
        onSuccess: () =>
          notify(`Stopping "${currentDeployment.name}".`, 'success'),
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Undeploy failed',
            'error'
          ),
      }
    );
  };

  const handleRedeploy = () => {
    restoreMutation.mutate(
      { api, deployment: currentDeployment },
      {
        onSuccess: () =>
          notify(`Redeploying "${currentDeployment.name}".`, 'success'),
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Redeploy failed',
            'error'
          ),
      }
    );
  };

  const created = currentDeployment.createdAt;
  const statusBarBg = isNotActive
    ? 'action.hover'
    : isDeployed
      ? alpha(theme.palette.success.main, 0.08)
      : isUndeployed
        ? 'action.hover'
        : isDeploying || isUndeploying
          ? alpha(theme.palette.warning.main, 0.08)
          : isFailed
            ? alpha(theme.palette.error.main, 0.08)
            : 'action.hover';
  const statusColor = isNotActive
    ? 'text.disabled'
    : isDeployed
      ? 'success.main'
      : isUndeployed
        ? 'text.disabled'
        : isDeploying || isUndeploying
          ? 'warning.main'
          : isFailed
            ? 'error.main'
            : 'text.secondary';
  const statusLabel = isNotActive
    ? 'Not Active'
    : isDeployed
      ? 'Active'
      : isUndeployed
        ? 'Suspended'
        : isDeploying
          ? 'Deploying'
          : isUndeploying
            ? 'Undeploying'
            : isFailed
              ? 'Failed'
              : (status ?? '');

  return (
    <Box sx={{ p: 1 }}>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'space-between',
          mb: 2,
        }}
      >
        {created && (
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
            <Typography sx={{ fontWeight: 500 }} variant="body2">
              {isDeployed ? 'Deployed' : 'Last deployed'}
            </Typography>
            <Typography color="text.secondary" variant="body2">
              ⏱ {relativeTime(created)}
            </Typography>
          </Box>
        )}
        {(isDeployed || isDeploying) && (
          <Button
            color="error"
            disabled={!isGatewayActive || busy || isDeploying}
            onClick={handleUndeploy}
            size="small"
            variant="outlined"
          >
            {undeployMutation.isPending ? 'Stopping...' : 'Stop'}
          </Button>
        )}
        {(isFailed ||
          isUndeployed ||
          isUndeploying ||
          status === 'ARCHIVED') && (
          <Button
            color="primary"
            disabled={!isGatewayActive || busy || isUndeploying}
            onClick={handleRedeploy}
            size="small"
            variant="outlined"
          >
            {restoreMutation.isPending ? 'Redeploying...' : 'Redeploy'}
          </Button>
        )}
      </Box>

      {/* Deployment status bar */}
      <Box
        sx={{
          alignItems: 'center',
          bgcolor: statusBarBg,
          borderRadius: 1,
          display: 'flex',
          justifyContent: 'space-between',
          mb: isFailed && currentDeployment.statusReason ? 0 : 2,
          px: 2,
          py: 1.5,
        }}
      >
        <Typography sx={{ fontWeight: 600 }} variant="body2">
          Deployment Status
        </Typography>
        <Typography
          sx={{ color: statusColor, fontWeight: 600 }}
          variant="body2"
        >
          {statusLabel}
        </Typography>
      </Box>

      {isFailed && currentDeployment.statusReason && (
        <Box mb={2} px={2}>
          <Typography color="error.main" variant="caption">
            {STATUS_REASON_MESSAGES[currentDeployment.statusReason] ??
              currentDeployment.statusReason}
          </Typography>
        </Box>
      )}

      {/* Deployment info box */}
      <Typography sx={{ fontWeight: 600, mb: 1 }} variant="subtitle2">
        Deployment
      </Typography>
      <Box
        sx={{
          alignItems: 'flex-start',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          display: 'flex',
          justifyContent: 'space-between',
          mb: 2,
          p: 2,
        }}
      >
        <Box>
          <Typography sx={{ fontWeight: 500 }} variant="body2">
            ID {currentDeployment.id.slice(0, 8)}
          </Typography>
          {created && (
            <Typography color="text.secondary" variant="caption">
              Deployed ⏱ {relativeTime(created)}
            </Typography>
          )}
        </Box>
        <IconButton
          aria-label={`Change deployment on ${gateway.displayName}`}
          disabled={!isGatewayActive}
          onClick={() => setSelectorOpen(true)}
          size="small"
        >
          <SquarePen size={16} />
        </IconButton>
      </Box>

      <GatewayDeploymentSelector
        api={api}
        deployments={deployments}
        onClose={() => setSelectorOpen(false)}
        open={selectorOpen}
      />
    </Box>
  );
}
