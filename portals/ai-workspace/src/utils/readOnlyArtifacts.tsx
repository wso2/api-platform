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

import type { ReactNode } from 'react';
import { Box, Card, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Lock } from '@wso2/oxygen-ui-icons-react';

export const GATEWAY_MANAGED_ARTIFACT_TOOLTIP =
  'This operation is unavailable because the artifact was created from a gateway.';

export const GATEWAY_MANAGED_SECTION_MESSAGE =
  'This artifact was created from a gateway. This configuration is owned by the gateway and is read-only in AI Workspace.';

/**
 * Read-only banner shown at the top of a configuration section that is part of a
 * gateway-created (data-plane-originated) artifact's runtime configuration. Such
 * sections cannot be edited from the control plane; only non-runtime metadata
 * (description, models, docs) remains editable. Pass a section-specific `message`
 * to clarify which section is locked.
 */
export function GatewayArtifactReadOnlyBanner({
  message = GATEWAY_MANAGED_SECTION_MESSAGE,
}: {
  message?: string;
}) {
  return (
    <Card sx={{ bgcolor: 'action.hover', mb: 2 }}>
      <Stack direction="row" spacing={1.5} alignItems="center" sx={{ p: 2 }}>
        <Lock size={18} />
        <Typography variant="body2">
          <strong>{message}</strong>
        </Typography>
      </Stack>
    </Card>
  );
}

type DisabledActionTooltipProps = {
  children: ReactNode;
  disabled: boolean;
  title?: ReactNode;
  fullWidth?: boolean;
};

export function DisabledActionTooltip({
  children,
  disabled,
  title = GATEWAY_MANAGED_ARTIFACT_TOOLTIP,
  fullWidth = false,
}: DisabledActionTooltipProps) {
  return (
    <Tooltip
      title={disabled ? title : ''}
      disableHoverListener={!disabled}
      disableFocusListener={!disabled}
      disableTouchListener={!disabled}
      placement="top"
    >
      <Box
        component="span"
        sx={fullWidth ? { width: '100%', display: 'inline-flex' } : undefined}
      >
        {children}
      </Box>
    </Tooltip>
  );
}
