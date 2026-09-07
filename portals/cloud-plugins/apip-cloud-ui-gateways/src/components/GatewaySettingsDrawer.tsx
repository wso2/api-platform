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

import type { FC } from 'react';
import { Box, Divider, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { relativeTime } from '../utils/time';
import { gatewayTypeLabel } from '../utils/gateway';
import type { Gateway, Environment } from '../types';

export type GatewaySettingsDrawerProps = {
  open: boolean;
  onClose: () => void;
  gateway: Gateway | null;
  environments: Environment[];
};

const GatewaySettingsDrawer: FC<GatewaySettingsDrawerProps> = ({ open, onClose, gateway, environments }) => {
  if (!gateway) return null;

  const environmentName = environments.find((environment) => environment.id === gateway.environmentId)?.name ?? '—';

  const rows = [
    { label: 'Type', value: gatewayTypeLabel(gateway.type) },
    { label: 'Environment', value: environmentName },
    { label: 'URL', value: gateway.url || '—' },
    { label: 'Status', value: gateway.status === 'active' ? 'Active' : 'Inactive' },
    { label: 'Version', value: gateway.version || '—' },
    { label: 'Critical', value: gateway.isCritical ? 'Yes' : 'No' },
    { label: 'Created', value: relativeTime(gateway.createdAt) },
    { label: 'Last Updated', value: relativeTime(gateway.updatedAt) },
  ];

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box sx={{ width: 360, p: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Gateway Configuration</Typography>
          <IconButton size="small" onClick={onClose} aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {gateway.name}
        </Typography>

        <Box sx={{ display: 'flex', flexDirection: 'column' }}>
          {rows.map((row, index) => (
            <Box key={row.label}>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', py: 1.25 }}>
                <Typography variant="body2">{row.label}</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ wordBreak: 'break-all', textAlign: 'right', ml: 2 }}>
                  {row.value}
                </Typography>
              </Box>
              {index < rows.length - 1 ? <Divider sx={{ borderStyle: 'dashed' }} /> : null}
            </Box>
          ))}
        </Box>

        {gateway.description ? (
          <>
            <Divider sx={{ mt: 1, mb: 1.5 }} />
            <Typography variant="body2" sx={{ fontWeight: 500, mb: 0.5 }}>
              Description
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {gateway.description}
            </Typography>
          </>
        ) : null}
      </Box>
    </Drawer>
  );
};

export default GatewaySettingsDrawer;
