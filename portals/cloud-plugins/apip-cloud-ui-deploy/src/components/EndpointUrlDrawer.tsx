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
import { Box, Drawer, IconButton, TextField, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';

export type EndpointUrlDrawerProps = {
  open: boolean;
  onClose: () => void;
  /** Where this endpoint applies, e.g. "Development · EU Gateway". */
  scopeLabel: string;
  endpointUrl?: string;
};

const EndpointUrlDrawer: FC<EndpointUrlDrawerProps> = ({ open, onClose, scopeLabel, endpointUrl }) => (
  <Drawer anchor="right" open={open} onClose={onClose}>
    <Box sx={{ width: 360, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Endpoint URL</Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      </Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        {scopeLabel}
      </Typography>

      <TextField
        fullWidth
        size="small"
        value={endpointUrl ?? ''}
        placeholder="No endpoint URL configured yet."
        slotProps={{ input: { readOnly: true } }}
        sx={{ '& .MuiInputBase-input': { fontFamily: 'monospace' } }}
      />
    </Box>
  </Drawer>
);

export default EndpointUrlDrawer;
