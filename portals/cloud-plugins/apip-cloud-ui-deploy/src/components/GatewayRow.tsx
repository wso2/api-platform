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
import { Box, Button, Card, CardContent, Collapse, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Eye } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
// eslint-disable-next-line @typescript-eslint/no-unused-vars
import DeploymentStatusBar from './DeploymentStatusBar';
import EndpointUrlDrawer from './EndpointUrlDrawer';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { relativeTime } from '../utils/time';
import { gatewayStatusTone } from '../utils/status';
import type { Gateway } from '../types';

export type GatewayRowProps = {
  gateway: Gateway;
  /** Used only to label the scope of this gateway's drawers, e.g. "Development · EU Gateway". */
  environmentName: string;
  onRetry: () => void;
  onStop: () => void;
};

const GatewayRow: FC<GatewayRowProps> = ({ gateway, environmentName, onRetry, onStop }) => {
  const [expanded, setExpanded] = useState(false);
  const [endpointUrlOpen, setEndpointUrlOpen] = useState(false);
  const tone = gatewayStatusTone(gateway.status);
  const scopeLabel = `${environmentName} · ${gateway.name}`;

  const actionLabel = gateway.status === 'failed' ? 'Re deploy' : 'Stop deployment';
  const actionDisabled = gateway.status === 'none' || gateway.status === 'deploying';
  const handleActionClick = gateway.status === 'failed' ? onRetry : onStop;

  return (
    <Card>
      <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
        <Box
          role="button"
          tabIndex={0}
          onClick={() => setExpanded((prev) => !prev)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') setExpanded((prev) => !prev);
          }}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            cursor: 'pointer',
            userSelect: 'none',
          }}
        >
          <StatusDot tone={tone.tone} />
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
              {gateway.name}
            </Typography>
          </Box>
          <StatusPill tone={tone} variant="outlined" />
          <Box sx={{ display: 'flex', color: 'text.secondary' }}>
            {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </Box>
        </Box>

        <Collapse in={expanded}>
          <Box sx={{ pt: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {/* <DeploymentStatusBar tone={tone} /> */}

            {gateway.status !== 'none' ? (
              <Card>
                <CardContent
                  sx={{
                    p: 1.25,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    '&:last-child': { pb: 1.25 },
                  }}
                >
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>
                      ID {gateway.buildId}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Deployed {gateway.deployedAt ? relativeTime(gateway.deployedAt) : '—'}
                    </Typography>
                  </Box>
                </CardContent>
              </Card>
            ) : null}

            <ActionRow
              label="Environment Variables"
              icon={<Eye size={14} />}
              onClick={() => setEndpointUrlOpen(true)}
            />

            <Button fullWidth variant="outlined" color="error" disabled={actionDisabled} onClick={handleActionClick}>
              {actionLabel}
            </Button>
          </Box>
        </Collapse>
      </CardContent>

      <EndpointUrlDrawer
        open={endpointUrlOpen}
        onClose={() => setEndpointUrlOpen(false)}
        scopeLabel={scopeLabel}
        endpointUrl={gateway.endpointUrl}
      />
    </Card>
  );
};

export default GatewayRow;
