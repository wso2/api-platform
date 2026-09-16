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
  Box,
  Button,
  CircularProgress,
  Drawer,
  FormControlLabel,
  IconButton,
  Radio,
  RadioGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useRestoreDeployment, type Deployment } from '@/api/resources/graphqlApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';
import { DeploymentStatusChip } from '../../../deploy/components/GatewayDeploymentRow';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.title',
    defaultMessage: 'Select Deployment to Restore',
    description: 'Heading of the drawer for picking an earlier deployment to put back in service.',
  },
  closeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.closeLabel',
    defaultMessage: 'Close',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.empty',
    defaultMessage: 'No deployments available',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.cancel',
    defaultMessage: 'Cancel',
  },
  restore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.restore',
    defaultMessage: 'Restore',
  },
  restoring: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.restoring',
    defaultMessage: 'Restoring...',
  },
  restoreStarted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.deploy.components.GraphqlGatewayDeploymentSelector.restoreStarted',
    defaultMessage: 'Restoring "{deploymentName}".',
    description:
      'Toast confirming a restore was requested. {deploymentName} is user-supplied; do not translate it.',
  },
});

type GraphqlGatewayDeploymentSelectorProps = {
  graphqlApiId: string;
  /** Deployments on this gateway, newest first. */
  deployments: Deployment[];
  open: boolean;
  onClose: () => void;
};

/**
 * Fork of `deploy/components/GatewayDeploymentSelector.tsx` for a GraphQL API
 * — only the deployments-module import differs. Reuses the REST file's
 * `DeploymentStatusChip`, which takes only a status string and imports no hook.
 */
export function GraphqlGatewayDeploymentSelector({
  graphqlApiId,
  deployments,
  open,
  onClose,
}: GraphqlGatewayDeploymentSelectorProps) {
  const intl = useIntl();
  const { dateTime, relativeTime } = useFormatters();
  const { notify } = useNotifications();
  const restoreMutation = useRestoreDeployment();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const currentDeployedId =
    deployments.find((item) => item.status === 'DEPLOYED')?.deploymentId ?? null;

  const handleRestore = () => {
    const deployment = deployments.find((item) => item.deploymentId === selectedId);
    if (!deployment) return;
    restoreMutation.mutate(
      { graphqlApiId, deploymentId: deployment.deploymentId },
      {
        onSuccess: () => {
          notify(
            intl.formatMessage(messages.restoreStarted, {
              deploymentName: deployment.name,
            }),
            'success',
          );
          setSelectedId(null);
          onClose();
        },
      },
    );
  };

  const canRestore =
    selectedId !== null && selectedId !== currentDeployedId && !restoreMutation.isPending;

  return (
    <Drawer
      anchor="right"
      onClose={onClose}
      open={open}
      sx={{ '& .MuiDrawer-paper': { width: { md: 560, xs: '100%' } } }}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box
          sx={{
            alignItems: 'center',
            borderBottom: '1px solid',
            borderColor: 'divider',
            display: 'flex',
            gap: 1,
            p: 2,
          }}
        >
          <IconButton
            aria-label={intl.formatMessage(messages.closeLabel)}
            onClick={onClose}
            size="small"
          >
            <ChevronLeft size={20} />
          </IconButton>
          <Typography sx={{ flexGrow: 1 }} variant="h6">
            <FormattedMessage {...messages.title} />
          </Typography>
        </Box>

        <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
          {deployments.length === 0 ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <Typography color="text.secondary">
                <FormattedMessage {...messages.empty} />
              </Typography>
            </Box>
          ) : (
            <RadioGroup
              onChange={(event) => setSelectedId(event.target.value)}
              value={selectedId ?? ''}
            >
              {deployments.map((deployment) => (
                <Box
                  key={deployment.deploymentId}
                  sx={{
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                    borderRadius: 1,
                    '&:hover': { bgcolor: 'action.hover' },
                    '&:last-child': { borderBottom: 'none' },
                  }}
                >
                  <FormControlLabel
                    control={<Radio size="small" />}
                    label={
                      <Box
                        sx={{
                          alignItems: 'center',
                          display: 'flex',
                          justifyContent: 'space-between',
                          width: '100%',
                        }}
                      >
                        <Box>
                          <Box
                            sx={{
                              alignItems: 'center',
                              display: 'flex',
                              gap: 1,
                            }}
                          >
                            <Typography sx={{ fontSize: '0.875rem', fontWeight: 500 }}>
                              {deployment.name}
                            </Typography>
                            {deployment.createdAt && (
                              <Typography color="text.secondary" variant="caption">
                                {relativeTime(deployment.createdAt)}
                              </Typography>
                            )}
                          </Box>
                          {deployment.createdAt && (
                            <Typography
                              color="text.secondary"
                              sx={{ display: 'block', mt: 0.25 }}
                              variant="caption"
                            >
                              {dateTime(deployment.createdAt)}
                            </Typography>
                          )}
                        </Box>
                        <Box sx={{ flexShrink: 0, ml: 2 }}>
                          <DeploymentStatusChip status={deployment.status} />
                        </Box>
                      </Box>
                    }
                    sx={{
                      alignItems: 'flex-start',
                      m: 0,
                      px: 1,
                      py: 1.5,
                      width: '100%',
                      '& .MuiFormControlLabel-label': { flex: 1 },
                    }}
                    value={deployment.deploymentId}
                  />
                </Box>
              ))}
            </RadioGroup>
          )}
        </Box>

        <Box
          sx={{
            borderColor: 'divider',
            borderTop: '1px solid',
            display: 'flex',
            gap: 1,
            justifyContent: 'flex-end',
            p: 2,
          }}
        >
          <Button
            color="secondary"
            disabled={restoreMutation.isPending}
            onClick={onClose}
            variant="outlined"
          >
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button
            disabled={!canRestore}
            onClick={handleRestore}
            startIcon={
              restoreMutation.isPending ? <CircularProgress color="inherit" size={16} /> : undefined
            }
            variant="contained"
          >
            <FormattedMessage
              {...(restoreMutation.isPending ? messages.restoring : messages.restore)}
            />
          </Button>
        </Box>
      </Box>
    </Drawer>
  );
}
