/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
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
