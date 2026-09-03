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

/**
 * NOT WIRED YET. CORS, rate limiting and resiliency are not yet customizable per
 * deployment: the API exposes only the endpoint and virtual hosts today. This
 * drawer is retained, unwired, for the increment that adds them — at which point
 * it becomes environment-scoped, like the settings that already exist, rather
 * than the per-gateway view sketched here.
 */
import type { FC } from 'react';
import { Box, Divider, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';

export type CorsResiliencyDrawerProps = {
  open: boolean;
  onClose: () => void;
  /** Where these settings apply — an environment name, or "Environment · Gateway" for a gateway-scoped view. */
  scopeLabel: string;
};

const SETTINGS = [
  { label: 'Allowed Origins', value: '*' },
  { label: 'Allowed Methods', value: 'GET, POST, PUT, DELETE' },
  { label: 'Rate Limit', value: '1000 req/min' },
  { label: 'Circuit Breaker', value: 'Enabled' },
  { label: 'Request Timeout', value: '30s' },
];

/** Reused for both environment-level and gateway-level "CORS, Rate Limiting and Resiliency" links — only the scope differs. */
const CorsResiliencyDrawer: FC<CorsResiliencyDrawerProps> = ({ open, onClose, scopeLabel }) => (
  <Drawer anchor="right" open={open} onClose={onClose}>
    <Box sx={{ width: 360, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>CORS, Rate Limiting and Resiliency</Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      </Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        {scopeLabel}
      </Typography>

      <Box sx={{ display: 'flex', flexDirection: 'column' }}>
        {SETTINGS.map((setting, index) => (
          <Box key={setting.label}>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', py: 1.25 }}>
              <Typography variant="body2">{setting.label}</Typography>
              <Typography variant="body2" color="text.secondary">
                {setting.value}
              </Typography>
            </Box>
            {index < SETTINGS.length - 1 ? <Divider sx={{ borderStyle: 'dashed' }} /> : null}
          </Box>
        ))}
      </Box>
    </Box>
  </Drawer>
);

export default CorsResiliencyDrawer;
