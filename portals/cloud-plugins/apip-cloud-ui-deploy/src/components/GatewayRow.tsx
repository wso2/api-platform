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
import { Box, Button, Collapse, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp } from '@wso2/oxygen-ui-icons-react';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { relativeTime } from '../utils/time';
import { gatewayStatusTone, statusReasonText } from '../utils/status';
import type { Gateway } from '../types';

export type GatewayRowProps = {
  gateway: Gateway;
  busy: boolean;
  onStop: () => void;
  onRedeploy: () => void;
};

/*
 * Per-gateway environment variables and CORS / rate-limiting / resiliency links
 * were part of the original design of this row. They are left out until the API
 * exposes those settings: the deployment settings it does support are
 * environment-wide, so they live on the environment card rather than here (see
 * `EnvironmentCard`). `CorsResiliencyDrawer` and `EnvironmentVariablesDrawer`
 * remain in this package for that next increment.
 */
const GatewayRow: FC<GatewayRowProps> = ({ gateway, busy, onStop, onRedeploy }) => {
  const [expanded, setExpanded] = useState(false);
  const tone = gatewayStatusTone(gateway.status);
  const isDeployed = gateway.status === 'DEPLOYED';
  const canRedeploy = gateway.status === 'FAILED' || gateway.status === 'UNDEPLOYED';

  return (
    <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden' }}>
      <Box
        role="button"
        tabIndex={0}
        onClick={() => setExpanded((previous) => !previous)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') setExpanded((previous) => !previous);
        }}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          px: 1.5,
          py: 1.25,
          cursor: 'pointer',
          userSelect: 'none',
        }}
      >
        <StatusDot tone={tone.tone} />
        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
          <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
            {gateway.name}
          </Typography>
          {gateway.deployedAt && (
            <Typography variant="caption" color="text.secondary" noWrap display="block">
              {isDeployed ? 'Deployed' : 'Last deployed'} {relativeTime(gateway.deployedAt)}
            </Typography>
          )}
        </Box>
        <StatusPill tone={tone} />
        <Box sx={{ display: 'flex', color: 'text.secondary' }}>
          {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </Box>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ px: 1.5, pb: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          {gateway.status === 'none' ? (
            <Typography variant="caption" color="text.disabled">
              This API has not been deployed to this gateway yet.
            </Typography>
          ) : (
            <Box
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                p: 1.25,
              }}
            >
              <Typography variant="body2" sx={{ fontWeight: 500 }}>
                {gateway.deploymentName ?? gateway.deploymentId?.slice(0, 8)}
              </Typography>
              {gateway.deploymentId && (
                <Typography variant="caption" color="text.secondary" display="block">
                  Build {gateway.deploymentId.slice(0, 8)}
                </Typography>
              )}
            </Box>
          )}

          {gateway.status === 'FAILED' && gateway.statusReason && (
            <Typography variant="caption" color="error.main">
              {statusReasonText(gateway.statusReason)}
            </Typography>
          )}

          {canRedeploy && (
            <Button fullWidth variant="outlined" onClick={onRedeploy} disabled={busy}>
              Redeploy
            </Button>
          )}

          {isDeployed && (
            <Button fullWidth variant="outlined" color="error" onClick={onStop} disabled={busy}>
              Stop deployment
            </Button>
          )}
        </Box>
      </Collapse>
    </Box>
  );
};

export default GatewayRow;
