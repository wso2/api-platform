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
import { Box, Button, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useDeleteDeployment, type Deployment } from '@/api/resources/restApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { GatewayDeploymentRow } from './components/GatewayDeploymentRow';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.GatewayDeploymentHistory.title',
    defaultMessage: 'API Deployment History',
    description: 'Heading over the list of past deployments of this API on one gateway.',
  },
  refreshLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.GatewayDeploymentHistory.refreshLabel',
    defaultMessage: 'Refresh deployment history',
    description: 'Accessible label for the icon button that refetches the deployment list.',
  },
  viewMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.GatewayDeploymentHistory.viewMore',
    defaultMessage: 'View More',
    description: 'Opens a drawer listing every deployment, beyond the three shown inline.',
  },
  deleted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.GatewayDeploymentHistory.deleted',
    defaultMessage: 'Deleted "{deploymentName}".',
    description:
      'Toast confirming a deployment record was removed. {deploymentName} is user-supplied; do not translate it.',
  },
});

type GatewayDeploymentHistoryProps = {
  /** Handle of the API these deployments belong to. */
  restApiId: string;
  /** Deployments on this gateway, newest first. */
  deployments: Deployment[];
  onRefresh: () => void;
  refreshing: boolean;
};

/**
 * Right panel of an expanded gateway card: the API deployment history —
 * newest three rows inline, the full list (with delete) in a drawer
 * (ai-workspace GatewayDeploymentHistory).
 */
export function GatewayDeploymentHistory({
  restApiId,
  deployments,
  onRefresh,
  refreshing,
}: GatewayDeploymentHistoryProps) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const intl = useIntl();
  const { notify } = useNotifications();
  const deleteMutation = useDeleteDeployment();

  if (deployments.length === 0) return null;

  const currentDeploymentId = deployments[0]?.deploymentId ?? null;

  const handleDelete = (deployment: Deployment) => {
    deleteMutation.mutate(
      { restApiId, deploymentId: deployment.deploymentId },
      // No `onError`: the query client's `onMutationError` already notifies.
      {
        onSuccess: () =>
          notify(
            intl.formatMessage(messages.deleted, { deploymentName: deployment.name }),
            'success',
          ),
      },
    );
  };

  return (
    <Box>
      <Box
        sx={{
          alignItems: 'center',
          borderBottom: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          pb: 2,
        }}
      >
        <Typography sx={{ flexGrow: 1, fontWeight: 500 }}>
          <FormattedMessage {...messages.title} />
        </Typography>
        <IconButton
          aria-label={intl.formatMessage(messages.refreshLabel)}
          color="primary"
          disabled={refreshing}
          onClick={onRefresh}
          size="small"
        >
          <RefreshCw size={18} />
        </IconButton>
      </Box>

      <Box>
        {deployments.slice(0, 3).map((deployment) => (
          <GatewayDeploymentRow
            deployment={deployment}
            isCurrentDeployment={deployment.deploymentId === currentDeploymentId}
            key={deployment.deploymentId}
          />
        ))}
      </Box>

      {deployments.length > 3 && (
        <Box
          sx={{
            borderColor: 'divider',
            borderTop: '1px solid',
            pt: 1,
            textAlign: 'center',
          }}
        >
          <Button color="primary" onClick={() => setDrawerOpen(true)} variant="text">
            <FormattedMessage {...messages.viewMore} />
          </Button>
        </Box>
      )}

      <Drawer
        anchor="right"
        onClose={() => setDrawerOpen(false)}
        open={drawerOpen}
        sx={{ '& .MuiDrawer-paper': { width: { md: 600, xs: '100%' } } }}
      >
        <Box sx={{ p: 3 }}>
          <Typography sx={{ mb: 2 }} variant="h6">
            <FormattedMessage {...messages.title} />
          </Typography>
          {deployments.map((deployment) => (
            <GatewayDeploymentRow
              deleteDisabled={deleteMutation.isPending}
              deployment={deployment}
              isCurrentDeployment={deployment.deploymentId === currentDeploymentId}
              key={deployment.deploymentId}
              onDelete={handleDelete}
            />
          ))}
        </Box>
      </Drawer>
    </Box>
  );
}
