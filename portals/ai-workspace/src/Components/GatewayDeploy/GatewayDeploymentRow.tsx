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

import React from 'react';
import { Box, Chip, IconButton, Typography } from '@wso2/oxygen-ui';
import { XCircle } from 'lucide-react';
import type { DeploymentResponseToPlatformGateway } from '../../../../../apis/gatewayTypes';

interface GatewayDeploymentRowProps {
  deployment: DeploymentResponseToPlatformGateway;
  index: number;
  totalCount: number;
  isCurrentDeployment: boolean;
  isInDrawer: boolean;
  onDelete?: (deploymentId: string) => void;
}

/**
 * Single deployment row in the deployment history.
 */
export default function GatewayDeploymentRow({
  deployment,
  index,
  totalCount,
  isCurrentDeployment,
  isInDrawer,
  onDelete,
}: GatewayDeploymentRowProps) {
  const isDeployed = deployment.status === 'DEPLOYED';
  const isDeploying = deployment.status === 'DEPLOYING';
  const isUndeploying = deployment.status === 'UNDEPLOYING';
  const isFailed = deployment.status === 'FAILED';
  const isArchived = deployment.status === 'ARCHIVED';
  const displayNumber = totalCount - index;
  const deploymentName = deployment.name ?? `Deployment ${displayNumber}`;

  const formattedDate = deployment.createdAt
    ? new Date(deployment.createdAt).toLocaleString()
    : null;

  const relativeTime = deployment.createdAt
    ? getRelativeTime(deployment.createdAt)
    : null;

  const handleDeleteClick = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (onDelete) {
      onDelete(deployment.deploymentId);
    }
  };

  return (
    <Box
      sx={{
        borderBottom: '1px solid',
        borderColor: 'divider',
        py: 1.5,
        px: 0,
        '&:last-child': { borderBottom: 'none' },
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'nowrap',
        }}
        p={1}
      >
        <Box
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            flex: 1,
            minWidth: 0,
          }}
        >
          <Box
            sx={{
              mr: 1,
              display: 'flex',
              alignItems: 'center',
              color: 'success.main',
              mt: 0.25,
            }}
          >
            ●
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column' }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: 1,
              }}
            >
              <Typography
                sx={{ fontWeight: 500, fontSize: '0.875rem' }}
                component="span"
              >
                {deploymentName}
              </Typography>
              {relativeTime && (
                <Typography
                  variant="caption"
                  color="text.secondary"
                  component="span"
                >
                  {relativeTime}
                </Typography>
              )}
            </Box>
            {formattedDate && (
              <Box sx={{ mt: 0.5 }}>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  component="span"
                >
                  {formattedDate}
                </Typography>
              </Box>
            )}
          </Box>
        </Box>

        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            flexShrink: 0,
            ml: 2,
            gap: 1,
          }}
        >
          {isCurrentDeployment && (
            <Chip size="small" variant="outlined" label="Latest" />
          )}
          {isDeployed ? (
            <Chip
              size="small"
              label="Deployed"
              color="success"
              variant="outlined"
            />
          ) : isDeploying ? (
            <Chip
              size="small"
              label="Deploying"
              color="warning"
              variant="outlined"
            />
          ) : isUndeploying ? (
            <Chip
              size="small"
              label="Undeploying"
              color="warning"
              variant="outlined"
            />
          ) : isFailed ? (
            <Chip
              size="small"
              label="Failed"
              color="error"
              variant="outlined"
            />
          ) : isArchived ? (
            <Chip size="small" label="Archived" variant="outlined" />
          ) : (
            <Chip
              size="small"
              label="Undeployed"
              color="warning"
              variant="outlined"
            />
          )}
          {isInDrawer && !isDeployed && !isDeploying && !isUndeploying && onDelete && (
            <IconButton color="error" size="small" onClick={handleDeleteClick}>
              <XCircle size={16} />
            </IconButton>
          )}
        </Box>
      </Box>
    </Box>
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
