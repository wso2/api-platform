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

import { useRestoreDeployment, type Deployment } from '@/api/resources/restApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';
import { DeploymentStatusChip } from './GatewayDeploymentRow';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.title',
    defaultMessage: 'Select Deployment to Restore',
    description: 'Heading of the drawer for picking an earlier deployment to put back in service.',
  },
  closeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.closeLabel',
    defaultMessage: 'Close',
    description: 'Accessible label for the icon button that dismisses the drawer.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.empty',
    defaultMessage: 'No deployments available',
    description: 'Shown when the gateway has no deployment history to restore from.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.cancel',
    defaultMessage: 'Cancel',
  },
  restore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.restore',
    defaultMessage: 'Restore',
    description: 'Button that puts the selected earlier deployment back in service. Verb.',
  },
  restoring: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.restoring',
    defaultMessage: 'Restoring...',
    description: 'Label on the Restore button while the request is in flight.',
  },
  restoreStarted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeploymentSelector.restoreStarted',
    defaultMessage: 'Restoring "{deploymentName}".',
    description:
      'Toast confirming a restore was requested. {deploymentName} is user-supplied; do not translate it.',
  },
});

type GatewayDeploymentSelectorProps = {
  /** Handle of the API these deployments belong to. */
  restApiId: string;
  /** Deployments on this gateway, newest first. */
  deployments: Deployment[];
  open: boolean;
  onClose: () => void;
};

/**
 * Right-hand drawer to pick a previous deployment and restore it on the
 * gateway (ai-workspace "Select Deployment to Restore").
 */
export function GatewayDeploymentSelector({
  restApiId,
  deployments,
  open,
  onClose,
}: GatewayDeploymentSelectorProps) {
  const intl = useIntl();
  // `useFormatters`, not the module-scope `Intl.*` in `utils/relativeTime`:
  // that one freezes its locale at import, so it never follows a locale switch.
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
      { restApiId, deploymentId: deployment.deploymentId },
      // No `onError`: the query client's `onMutationError` already notifies.
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
