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
