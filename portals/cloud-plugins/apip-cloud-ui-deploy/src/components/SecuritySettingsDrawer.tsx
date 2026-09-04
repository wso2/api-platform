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
 * NOT WIRED YET. Security settings are not yet customizable per deployment. This drawer is
 * retained, unwired, for the increment that adds them. Its contents are a
 * placeholder summary, not real data.
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
