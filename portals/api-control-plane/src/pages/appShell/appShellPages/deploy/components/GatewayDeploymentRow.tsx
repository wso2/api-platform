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

import { Box, Chip, IconButton, Typography } from '@wso2/oxygen-ui';
import { XCircle } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import type { Deployment, DeploymentStatus } from '@/api/resources/restApis/deployments';
import { useFormatters } from '@/i18n/useFormatters';

/**
 * One descriptor per `DeploymentStatus`, keyed by the status itself so the map
 * is exhaustive at compile time — a status added to the spec fails here rather
 * than rendering a raw enum value.
 */
const statusMessages = defineMessages({
  DEPLOYED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusDeployed',
    defaultMessage: 'Deployed',
    description: 'Deployment state: live on the gateway and serving traffic.',
  },
  DEPLOYING: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusDeploying',
    defaultMessage: 'Deploying',
    description: 'Deployment state: in progress, awaiting the gateway acknowledgement.',
  },
  UNDEPLOYING: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusUndeploying',
    defaultMessage: 'Undeploying',
    description: 'Deployment state: being taken out of service.',
  },
  UNDEPLOYED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusUndeployed',
    defaultMessage: 'Undeployed',
    description: 'Deployment state: out of service, but its record is kept.',
  },
  FAILED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusFailed',
    defaultMessage: 'Failed',
    description: 'Deployment state: the gateway rejected it or could not complete it.',
  },
  ARCHIVED: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.statusArchived',
    defaultMessage: 'Archived',
    description: 'Deployment state: superseded by a newer deployment.',
  },
});

const messages = defineMessages({
  latest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.latest',
    defaultMessage: 'Latest',
    description: 'Badge marking the most recent deployment on a gateway.',
  },
  deleteLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentRow.deleteLabel',
    defaultMessage: 'Delete {deploymentName}',
    description:
      'Accessible label for the delete button on one deployment row. {deploymentName} is user-supplied; do not translate it.',
  },
});

const STATUS_COLOR: Record<DeploymentStatus, 'success' | 'warning' | 'error' | 'default'> = {
  DEPLOYED: 'success',
  DEPLOYING: 'warning',
  UNDEPLOYING: 'warning',
  UNDEPLOYED: 'warning',
  FAILED: 'error',
  ARCHIVED: 'default',
};

export function DeploymentStatusChip({ status }: { status: DeploymentStatus }) {
  const intl = useIntl();
  return (
    <Chip
      color={STATUS_COLOR[status]}
      label={intl.formatMessage(statusMessages[status])}
      size="small"
      variant="outlined"
    />
  );
}

type GatewayDeploymentRowProps = {
  deployment: Deployment;
  isCurrentDeployment: boolean;
  /** Delete is only offered inside the full-history drawer. */
  onDelete?: (deployment: Deployment) => void;
  deleteDisabled?: boolean;
};

/** Single deployment row in the deployment history (ai-workspace layout). */
export function GatewayDeploymentRow({
  deployment,
  isCurrentDeployment,
  onDelete,
  deleteDisabled,
}: GatewayDeploymentRowProps) {
  const intl = useIntl();
  // `useFormatters`, not the module-scope `Intl.*` in `utils/relativeTime`:
  // that one freezes its locale at import, so it never follows a locale switch.
  const { dateTime, relativeTime } = useFormatters();
  const isSettled =
    deployment.status !== 'DEPLOYED' &&
    deployment.status !== 'DEPLOYING' &&
    deployment.status !== 'UNDEPLOYING';
  const created = deployment.createdAt;

  return (
    <Box
      sx={{
        borderBottom: '1px solid',
        borderColor: 'divider',
        py: 1.5,
        '&:last-child': { borderBottom: 'none' },
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'space-between',
        }}
      >
        <Box
          sx={{
            alignItems: 'flex-start',
            display: 'flex',
            flex: 1,
            minWidth: 0,
          }}
        >
          {/* Decorative marker. Drawn rather than typed: as the "●" character it
              was untranslatable JSX text, and screen readers announced it. */}
          <Box
            component="span"
            sx={{
              bgcolor: 'success.main',
              borderRadius: '50%',
              flexShrink: 0,
              height: 8,
              mr: 1,
              mt: 0.75,
              width: 8,
            }}
          />
          <Box sx={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                flexWrap: 'wrap',
                gap: 1,
              }}
            >
              <Typography component="span" sx={{ fontSize: '0.875rem', fontWeight: 500 }}>
                {deployment.name}
              </Typography>
              {created && (
                <Typography color="text.secondary" component="span" variant="caption">
                  {relativeTime(created)}
                </Typography>
              )}
            </Box>
            {created && (
              <Typography
                color="text.secondary"
                component="span"
                sx={{ mt: 0.5 }}
                variant="caption"
              >
                {dateTime(created)}
              </Typography>
            )}
          </Box>
        </Box>

        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            flexShrink: 0,
            gap: 1,
            ml: 2,
          }}
        >
          {isCurrentDeployment && (
            <Chip label={intl.formatMessage(messages.latest)} size="small" variant="outlined" />
          )}
          <DeploymentStatusChip status={deployment.status} />
          {onDelete && isSettled && (
            <IconButton
              aria-label={intl.formatMessage(messages.deleteLabel, {
                deploymentName: deployment.name,
              })}
              color="error"
              disabled={deleteDisabled}
              onClick={() => onDelete(deployment)}
              size="small"
            >
              <XCircle size={16} />
            </IconButton>
          )}
        </Box>
      </Box>
    </Box>
  );
}
