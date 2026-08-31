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

export type SecuritySettingsDrawerProps = {
  open: boolean;
  onClose: () => void;
};

/** Read-only summary; applies pipeline-wide rather than per-gateway, so there's nothing to pick a gateway for here. */
const SETTINGS = [
  { label: 'Authentication', value: 'OAuth2' },
  { label: 'Mutual TLS (mTLS)', value: 'Disabled' },
  { label: 'IP Allowlisting', value: 'Not configured' },
  { label: 'Rate Limiting', value: 'Inherited from environment' },
];

const SecuritySettingsDrawer: FC<SecuritySettingsDrawerProps> = ({ open, onClose }) => (
  <Drawer anchor="right" open={open} onClose={onClose}>
    <Box sx={{ width: 360, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Security Settings</Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      </Box>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Applies to every gateway across all environments in this pipeline.
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

export default SecuritySettingsDrawer;
