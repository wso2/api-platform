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
import { alpha, Box, Button, IconButton, Typography, useTheme } from '@wso2/oxygen-ui';
import { PackageOpen, SquarePen } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import {
  useRestoreDeployment,
  useUndeployDeployment,
  type Deployment,
} from '@/api/resources/graphqlApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';
import { GraphqlGatewayDeploymentSelector } from './GraphqlGatewayDeploymentSelector';

/**
 * Explanations for the `statusReason` codes platform-api returns on a failed
 * deployment. An unrecognised code falls through to the raw value: it is
 * backend data, so it passes to the user untranslated rather than guessed at.
 */
const statusReasonMessages = defineMessages({
  GATEWAY_PROCESSING_ERROR: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.reasonGatewayProcessingError',
    defaultMessage: 'Failed to process deployment. Please check gateway logs.',
  },
  DEPLOYMENT_TIMEOUT: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.reasonDeploymentTimeout',
    defaultMessage: 'Deployment timed out. Gateway did not respond.',
  },
});

const statusLabels = defineMessages({
  NOT_ACTIVE: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusNotActive',
    defaultMessage: 'Not Active',
    description: 'The gateway is offline, so the deployment state cannot be determined.',
  },
  DEPLOYED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusActive',
    defaultMessage: 'Active',
  },
  UNDEPLOYED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusSuspended',
    defaultMessage: 'Suspended',
  },
  DEPLOYING: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusDeploying',
    defaultMessage: 'Deploying',
  },
  UNDEPLOYING: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusUndeploying',
    defaultMessage: 'Undeploying',
  },
  FAILED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusFailed',
    defaultMessage: 'Failed',
  },
  ARCHIVED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.statusArchived',
    defaultMessage: 'Archived',
  },
});

const messages = defineMessages({
  notDeployed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.notDeployed',
    defaultMessage: 'Not yet deployed',
  },
  deployedAt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.deployedAt',
    defaultMessage: 'Deployed',
    description: 'Label before the timestamp while the deployment is live. Past tense.',
  },
  lastDeployedAt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.lastDeployedAt',
    defaultMessage: 'Last deployed',
  },
  stop: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.stop',
    defaultMessage: 'Stop',
  },
  stopping: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.stopping',
    defaultMessage: 'Stopping...',
  },
  redeploy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.redeploy',
    defaultMessage: 'Redeploy',
  },
  redeploying: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.redeploying',
    defaultMessage: 'Redeploying...',
  },
  deploymentStatus: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.deploymentStatus',
    defaultMessage: 'Deployment Status',
  },
  deploymentHeading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.deploymentHeading',
    defaultMessage: 'Deployment',
  },
  deploymentId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.deploymentId',
    defaultMessage: 'ID {shortId}',
    description:
      'Shortened deployment identifier. {shortId} is a server-generated id; do not translate it.',
  },
  deployedRelative: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.deployedRelative',
    defaultMessage: 'Deployed ⏱ {relative}',
    description:
      'Deployment age, e.g. "Deployed ⏱ 3 hours ago". {relative} is an already-formatted relative time.',
  },
  relative: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.relative',
    defaultMessage: '⏱ {relative}',
  },
  changeDeploymentLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.changeDeploymentLabel',
    defaultMessage: 'Change deployment on {gatewayName}',
    description:
      'Accessible label for the button opening the restore drawer. {gatewayName} is user-supplied; do not translate it.',
  },
  stopStarted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.stopStarted',
    defaultMessage: 'Stopping "{deploymentName}".',
    description:
      'Toast confirming an undeploy was requested. {deploymentName} is user-supplied; do not translate it.',
  },
  redeployStarted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeployEnvCard.redeployStarted',
    defaultMessage: 'Redeploying "{deploymentName}".',
    description:
      'Toast confirming a redeploy was requested. {deploymentName} is user-supplied; do not translate it.',
  },
});

type GraphqlGatewayDeployEnvCardProps = {
  graphqlApiId: string;
  gateway: Gateway;
  /** Deployments on this gateway, newest first. */
  deployments: Deployment[];
  currentDeployment?: Deployment;
  isGatewayActive: boolean;
};

/** Fork of `deploy/components/GatewayDeployEnvCard.tsx` for a GraphQL API. */
export function GraphqlGatewayDeployEnvCard({
  graphqlApiId,
  gateway,
  deployments,
  currentDeployment,
  isGatewayActive,
}: GraphqlGatewayDeployEnvCardProps) {
  const theme = useTheme();
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const { notify } = useNotifications();
  const undeployMutation = useUndeployDeployment();
  const restoreMutation = useRestoreDeployment();
  const [selectorOpen, setSelectorOpen] = useState(false);

  const status = currentDeployment?.status;
  const effective = !isGatewayActive && currentDeployment ? 'NOT_ACTIVE' : status;
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
        <Typography color="text.secondary">
          <FormattedMessage {...messages.notDeployed} />
        </Typography>
      </Box>
    );
  }

  const handleUndeploy = () => {
    undeployMutation.mutate(
      { graphqlApiId, deploymentId: currentDeployment.deploymentId },
      {
        onSuccess: () =>
          notify(
            intl.formatMessage(messages.stopStarted, {
              deploymentName: currentDeployment.name,
            }),
            'success',
          ),
      },
    );
  };

  const handleRedeploy = () => {
    restoreMutation.mutate(
      { graphqlApiId, deploymentId: currentDeployment.deploymentId },
      {
        onSuccess: () =>
          notify(
            intl.formatMessage(messages.redeployStarted, {
              deploymentName: currentDeployment.name,
            }),
            'success',
          ),
      },
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
  const statusLabel = effective ? intl.formatMessage(statusLabels[effective]) : '';

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
              <FormattedMessage {...(isDeployed ? messages.deployedAt : messages.lastDeployedAt)} />
            </Typography>
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage
                {...messages.relative}
                values={{ relative: relativeTime(created) }}
              />
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
            <FormattedMessage
              {...(undeployMutation.isPending ? messages.stopping : messages.stop)}
            />
          </Button>
        )}
        {(isFailed || isUndeployed || isUndeploying || status === 'ARCHIVED') && (
          <Button
            color="primary"
            disabled={!isGatewayActive || busy || isUndeploying}
            onClick={handleRedeploy}
            size="small"
            variant="outlined"
          >
            <FormattedMessage
              {...(restoreMutation.isPending ? messages.redeploying : messages.redeploy)}
            />
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
          <FormattedMessage {...messages.deploymentStatus} />
        </Typography>
        <Typography sx={{ color: statusColor, fontWeight: 600 }} variant="body2">
          {statusLabel}
        </Typography>
      </Box>

      {isFailed && currentDeployment.statusReason && (
        <Box mb={2} px={2}>
          <Typography color="error.main" variant="caption">
            {currentDeployment.statusReason in statusReasonMessages
              ? intl.formatMessage(
                  statusReasonMessages[
                    currentDeployment.statusReason as keyof typeof statusReasonMessages
                  ],
                )
              : currentDeployment.statusReason}
          </Typography>
        </Box>
      )}

      {/* Deployment info box */}
      <Typography sx={{ fontWeight: 600, mb: 1 }} variant="subtitle2">
        <FormattedMessage {...messages.deploymentHeading} />
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
            <FormattedMessage
              {...messages.deploymentId}
              values={{ shortId: currentDeployment.deploymentId.slice(0, 8) }}
            />
          </Typography>
          {created && (
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage
                {...messages.deployedRelative}
                values={{ relative: relativeTime(created) }}
              />
            </Typography>
          )}
        </Box>
        <IconButton
          aria-label={intl.formatMessage(messages.changeDeploymentLabel, {
            gatewayName: gateway.displayName,
          })}
          disabled={!isGatewayActive}
          onClick={() => setSelectorOpen(true)}
          size="small"
        >
          <SquarePen size={16} />
        </IconButton>
      </Box>

      <GraphqlGatewayDeploymentSelector
        deployments={deployments}
        graphqlApiId={graphqlApiId}
        onClose={() => setSelectorOpen(false)}
        open={selectorOpen}
      />
    </Box>
  );
}
