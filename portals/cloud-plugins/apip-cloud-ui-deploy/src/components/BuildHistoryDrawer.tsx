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
import StatusDot from './StatusDot';
import { relativeTime } from '../utils/time';
import type { BuildRecord, Environment } from '../types';

export type BuildHistoryDrawerProps = {
  open: boolean;
  onClose: () => void;
  buildHistory: BuildRecord[];
  environments: Environment[];
};

const BuildHistoryDrawer: FC<BuildHistoryDrawerProps> = ({ open, onClose, buildHistory, environments }) => {
  const environmentName = (id: string) => environments.find((environment) => environment.id === id)?.name ?? id;

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box sx={{ width: 380, p: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
          <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build and Deployment History</Typography>
          <IconButton size="small" onClick={onClose} aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        {buildHistory.length === 0 ? (
          <Typography variant="body2" color="text.disabled">
            No builds yet.
          </Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column' }}>
            {buildHistory.map((build, index) => (
              <Box key={build.id}>
                <Box sx={{ py: 1.5 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                    <StatusDot tone={build.result === 'Success' ? 'success' : 'error'} />
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>
                      {build.result}
                    </Typography>
                    <Typography variant="caption" color="text.disabled">
                      · {relativeTime(build.when)}
                    </Typography>
                  </Box>
                  <Box sx={{ pl: 2.25, mt: 0.25 }}>
                    <Typography variant="caption" color="text.secondary" display="block">
                      ID {build.buildId}
                    </Typography>
                    <Typography variant="caption" color="text.secondary" display="block">
                      {environmentName(build.targetEnvironmentId)} · {build.targetGatewayCount} gateway
                      {build.targetGatewayCount === 1 ? '' : 's'}
                    </Typography>
                  </Box>
                </Box>
                {index < buildHistory.length - 1 ? <Divider /> : null}
              </Box>
            ))}
          </Box>
        )}
      </Box>
    </Drawer>
  );
};

export default BuildHistoryDrawer;
