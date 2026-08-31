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

import { useState, type FC } from 'react';
import { Box, Button, Divider, Typography } from '@wso2/oxygen-ui';
import { RefreshCw, Wrench } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
import BuildHistoryDrawer from './BuildHistoryDrawer';
import SecuritySettingsDrawer from './SecuritySettingsDrawer';
import StatusDot from './StatusDot';
import { relativeTime } from '../utils/time';
import type { BuildRecord, Environment } from '../types';

export type BuildAreaCardProps = {
  buildHistory: BuildRecord[];
  environments: Environment[];
  onDeployClick: () => void;
};

const BuildAreaCard: FC<BuildAreaCardProps> = ({ buildHistory, environments, onDeployClick }) => {
  const [securityOpen, setSecurityOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);

  const environmentName = (id: string) => environments.find((environment) => environment.id === id)?.name ?? id;

  return (
    <Box
      sx={{
        flex: '0 0 300px',
        width: 300,
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '14px',
        p: 2.5,
        display: 'flex',
        flexDirection: 'column',
        alignSelf: 'flex-start',
      }}
    >
      <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build Area</Typography>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
        <Divider />

        <Button fullWidth variant="contained" onClick={onDeployClick} sx={{ fontWeight: 600 }}>
          Deploy
        </Button>

        <ActionRow label="Security Settings" icon={<Wrench size={14} />} onClick={() => setSecurityOpen(true)} />
        <Divider sx={{ borderStyle: 'dashed' }} />
        <ActionRow
          label="Build and Deployment History"
          icon={<RefreshCw size={14} />}
          onClick={() => setHistoryOpen(true)}
        />

        <Divider />
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5, overflowY: 'auto', maxHeight: 360 }}>
        {buildHistory.length === 0 ? (
          <Typography variant="body2" color="text.disabled">
            No builds yet.
          </Typography>
        ) : (
          buildHistory.map((build) => (
            <Box key={build.id}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                <StatusDot tone={build.result === 'Success' ? 'success' : 'error'} />
                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                  {build.result}
                </Typography>
                <Typography variant="caption" color="text.disabled">
                  · {relativeTime(build.when)}
                </Typography>
              </Box>
              <Box sx={{ pl: 2.25 }}>
                <Typography variant="caption" color="text.secondary" display="block">
                  ID {build.buildId}
                </Typography>
                <Typography variant="caption" color="text.secondary" display="block">
                  {environmentName(build.targetEnvironmentId)} · {build.targetGatewayCount} gateway
                  {build.targetGatewayCount === 1 ? '' : 's'}
                </Typography>
              </Box>
            </Box>
          ))
        )}
      </Box>

      <SecuritySettingsDrawer open={securityOpen} onClose={() => setSecurityOpen(false)} />
      <BuildHistoryDrawer
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        buildHistory={buildHistory}
        environments={environments}
      />
    </Box>
  );
};

export default BuildAreaCard;
