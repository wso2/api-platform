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
import { Box, Button, Card, CardContent, Divider, Tooltip, Typography } from '@wso2/oxygen-ui';
import StatusDot from './StatusDot';
import { relativeTime } from '../utils/time';
import { activeGatewayCount } from '../utils/status';
import type { BuildRecord, Environment } from '../types';

export type BuildAreaCardProps = {
  buildHistory: BuildRecord[];
  targetEnvironment: Environment;
  onDeployClick: () => void;
};

const BuildAreaCard: FC<BuildAreaCardProps> = ({ buildHistory, targetEnvironment, onDeployClick }) => {
  const latestBuild = buildHistory[0] ?? null;
  const canDeploy = activeGatewayCount(targetEnvironment.gateways) > 0;
  const deployDisabledReason = canDeploy
    ? ''
    : `All gateways in ${targetEnvironment.name} are inactive. Activate a gateway before deploying.`;

  return (
    <Card
      sx={{
        flex: '0 0 300px',
        width: 300,
        alignSelf: 'flex-start',
      }}
    >
      <CardContent sx={{ p: 2.5, '&:last-child': { pb: 2.5 } }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build Area</Typography>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
          <Divider />

          <Tooltip title={deployDisabledReason}>
            <span style={{ display: 'block' }}>
              <Button
                fullWidth
                variant="contained"
                disabled={!canDeploy}
                onClick={onDeployClick}
                sx={{ fontWeight: 600 }}
              >
                Deploy
              </Button>
            </span>
          </Tooltip>

          <Card>
            <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
              {latestBuild ? (
                <>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                    <StatusDot tone={latestBuild.result === 'Success' ? 'success' : 'error'} />
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>
                      {latestBuild.result}
                    </Typography>
                    <Typography variant="caption" color="text.disabled">
                      · {relativeTime(latestBuild.when)}
                    </Typography>
                  </Box>
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ pl: 2.25, mt: 0.5 }}>
                    ID {latestBuild.buildId}
                  </Typography>
                </>
              ) : (
                <Typography variant="body2" color="text.disabled">
                  No builds yet.
                </Typography>
              )}
            </CardContent>
          </Card>
        </Box>
      </CardContent>
    </Card>
  );
};

export default BuildAreaCard;
