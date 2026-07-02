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
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Drawer,
  IconButton,
  Radio,
  RadioGroup,
  FormControlLabel,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from 'lucide-react';
import type { DeploymentResponse } from '../../../../../utils/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';

interface GatewayDeploymentSelectorProps {
  gatewayId: string;
  open: boolean;
  onClose: () => void;
}

/**
 * Drawer that lists all deployments for a gateway.
 */
export default function GatewayDeploymentSelector({
  gatewayId,
  open,
  onClose,
}: GatewayDeploymentSelectorProps) {
  const { deployments, redeployDeployment, isDeployingToGateway, readOnly } =
    useGatewayDeploy();

  const [selectedDeploymentId, setSelectedDeploymentId] = useState<
    string | null
  >(null);
  const [isRestoring, setIsRestoring] = useState(false);

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

  // The currently deployed one
  const currentDeployedId = useMemo(() => {
    const deployed = gatewayDeployments.find((d) => d.status === 'DEPLOYED');
    return deployed?.deploymentId ?? null;
  }, [gatewayDeployments]);

  const handleRestore = async () => {
    if (!selectedDeploymentId) return;
    setIsRestoring(true);
    try {
      const success = await redeployDeployment(selectedDeploymentId, gatewayId);
      if (success) {
        setSelectedDeploymentId(null);
        onClose();
      }
    } finally {
      setIsRestoring(false);
    }
  };

  const isActionDisabled = isRestoring || isDeployingToGateway;
  const canRestore =
    selectedDeploymentId !== null &&
    selectedDeploymentId !== currentDeployedId &&
    !isActionDisabled;

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      sx={{ '& .MuiDrawer-paper': { width: { xs: '100%', md: 560 } } }}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        {/* Header */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            p: 2,
            borderBottom: '1px solid',
            borderColor: 'divider',
          }}
        >
          <IconButton size="small" onClick={onClose}>
            <ChevronLeft size={20} />
          </IconButton>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>
            Select Deployment to Restore
          </Typography>
        </Box>

        {/* Deployment list */}
        <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
          {gatewayDeployments.length === 0 ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <Typography color="text.secondary">
                No deployments available
              </Typography>
            </Box>
          ) : (
            <RadioGroup
              value={selectedDeploymentId ?? ''}
              onChange={(e) => setSelectedDeploymentId(e.target.value)}
            >
              {gatewayDeployments.map((deployment, index) => {
                const displayNumber = gatewayDeployments.length - index;
                const deploymentName =
                  deployment.name ?? `Deployment ${displayNumber}`;
                const isDeployed = deployment.status === 'DEPLOYED';
                const isArchived = deployment.status === 'ARCHIVED';
                const relativeTime = deployment.createdAt
                  ? getRelativeTime(deployment.createdAt)
                  : null;
                const formattedDate = deployment.createdAt
                  ? new Date(deployment.createdAt).toLocaleString()
                  : null;

                return (
                  <Box
                    key={deployment.deploymentId}
                    sx={{
                      borderBottom: '1px solid',
                      borderColor: 'divider',
                      '&:last-child': { borderBottom: 'none' },
                      '&:hover': { bgcolor: 'action.hover' },
                      borderRadius: 1,
                    }}
                  >
                    <FormControlLabel
                      value={deployment.deploymentId}
                      control={<Radio size="small" />}
                      sx={{
                        m: 0,
                        px: 1,
                        py: 1.5,
                        width: '100%',
                        alignItems: 'flex-start',
                        '& .MuiFormControlLabel-label': { flex: 1 },
                      }}
                      label={
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            width: '100%',
                          }}
                        >
                          <Box>
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 1,
                              }}
                            >
                              <Typography
                                sx={{ fontWeight: 500, fontSize: '0.875rem' }}
                              >
                                {deploymentName}
                              </Typography>
                              {relativeTime && (
                                <Typography
                                  variant="caption"
                                  color="text.secondary"
                                >
                                  {relativeTime}
                                </Typography>
                              )}
                            </Box>
                            {formattedDate && (
                              <Typography
                                variant="caption"
                                color="text.secondary"
                                sx={{ mt: 0.25, display: 'block' }}
                              >
                                {formattedDate}
                              </Typography>
                            )}
                          </Box>

                          <Box
                            sx={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: 1,
                              flexShrink: 0,
                              ml: 2,
                            }}
                          >
                            {isDeployed ? (
                              <Chip
                                size="small"
                                label="Deployed"
                                color="success"
                                variant="outlined"
                              />
                            ) : isArchived ? (
                              <Chip
                                size="small"
                                label="Archived"
                                variant="outlined"
                              />
                            ) : (
                              <Chip
                                size="small"
                                label="Undeployed"
                                color="warning"
                                variant="outlined"
                              />
                            )}
                          </Box>
                        </Box>
                      }
                    />
                  </Box>
                );
              })}
            </RadioGroup>
          )}
        </Box>

        <Box
          sx={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 1,
            p: 2,
            borderTop: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Button
            variant="outlined"
            onClick={onClose}
            disabled={isActionDisabled}
            color="secondary"
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="primary"
            onClick={handleRestore}
            disabled={readOnly || !canRestore}
            startIcon={
              isRestoring ? (
                <CircularProgress size={16} color="inherit" />
              ) : undefined
            }
          >
            {isRestoring ? 'Restoring...' : 'Restore'}
          </Button>
        </Box>
      </Box>
    </Drawer>
  );
}

function getRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) return `${diffDays}d ago`;
  if (diffHours > 0) return `${diffHours}h ago`;
  if (diffMins > 0) return `${diffMins}m ago`;
  return 'just now';
}
