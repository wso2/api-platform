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

import type {
  GatewayDeployment,
  GatewayDeploymentStatus,
} from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';

const STATUS_CHIP: Record<
  GatewayDeploymentStatus,
  { label: string; color: 'success' | 'warning' | 'error' | 'default' }
> = {
  DEPLOYED: { label: 'Deployed', color: 'success' },
  DEPLOYING: { label: 'Deploying', color: 'warning' },
  UNDEPLOYING: { label: 'Undeploying', color: 'warning' },
  UNDEPLOYED: { label: 'Undeployed', color: 'warning' },
  FAILED: { label: 'Failed', color: 'error' },
  ARCHIVED: { label: 'Archived', color: 'default' },
};

export function DeploymentStatusChip({
  status,
}: {
  status: GatewayDeploymentStatus;
}) {
  const { label, color } = STATUS_CHIP[status];
  return <Chip color={color} label={label} size="small" variant="outlined" />;
}

type GatewayDeploymentRowProps = {
  deployment: GatewayDeployment;
  isCurrentDeployment: boolean;
  /** Delete is only offered inside the full-history drawer. */
  onDelete?: (deployment: GatewayDeployment) => void;
  deleteDisabled?: boolean;
};

/** Single deployment row in the deployment history (ai-workspace layout). */
export function GatewayDeploymentRow({
  deployment,
  isCurrentDeployment,
  onDelete,
  deleteDisabled,
}: GatewayDeploymentRowProps) {
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
          <Box component="span" sx={{ color: 'success.main', mr: 1, mt: 0.25 }}>
            ●
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                flexWrap: 'wrap',
                gap: 1,
              }}
            >
              <Typography
                component="span"
                sx={{ fontSize: '0.875rem', fontWeight: 500 }}
              >
                {deployment.name}
              </Typography>
              {created && (
                <Typography
                  color="text.secondary"
                  component="span"
                  variant="caption"
                >
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
                {new Date(created).toLocaleString()}
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
            <Chip label="Latest" size="small" variant="outlined" />
          )}
          <DeploymentStatusChip status={deployment.status} />
          {onDelete && isSettled && (
            <IconButton
              aria-label={`Delete ${deployment.name}`}
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
