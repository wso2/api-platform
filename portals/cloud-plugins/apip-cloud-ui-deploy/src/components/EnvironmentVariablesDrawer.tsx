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

export type EnvironmentVariablesDrawerProps = {
  open: boolean;
  onClose: () => void;
  /** Where these variables apply — an environment name, or "Environment · Gateway" for a gateway-scoped view. */
  scopeLabel: string;
  count: number;
};

const SAMPLE_NAMES = [
  'API_BASE_URL',
  'LOG_LEVEL',
  'FEATURE_FLAGS',
  'REGION',
  'CACHE_TTL_SECONDS',
  'MAX_RETRIES',
  'REQUEST_TIMEOUT_MS',
  'DEBUG_MODE',
];

/** Reused for both environment-level and gateway-level "Environment Variables" links — only the scope and count differ. */
const EnvironmentVariablesDrawer: FC<EnvironmentVariablesDrawerProps> = ({ open, onClose, scopeLabel, count }) => {
  const variables = Array.from({ length: count }, (_, index) => SAMPLE_NAMES[index % SAMPLE_NAMES.length]);

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box sx={{ width: 360, p: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Environment Variables</Typography>
          <IconButton size="small" onClick={onClose} aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {scopeLabel}
        </Typography>

        {variables.length === 0 ? (
          <Typography variant="body2" color="text.disabled">
            No environment variables configured.
          </Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column' }}>
            {variables.map((name, index) => (
              <Box key={`${name}-${index}`}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', py: 1.25 }}>
                  <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                    {name}
                  </Typography>
                  <Typography variant="body2" color="text.disabled">
                    ••••••••
                  </Typography>
                </Box>
                {index < variables.length - 1 ? <Divider sx={{ borderStyle: 'dashed' }} /> : null}
              </Box>
            ))}
          </Box>
        )}
      </Box>
    </Drawer>
  );
};

export default EnvironmentVariablesDrawer;
