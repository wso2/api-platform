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
import { Box, Button, Divider, Collapse, IconButton, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Pencil, Wrench } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
import CorsResiliencyDrawer from './CorsResiliencyDrawer';
import DeploymentStatusBar from './DeploymentStatusBar';
import EnvironmentVariablesDrawer from './EnvironmentVariablesDrawer';
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

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

const GatewayRow: FC<GatewayRowProps> = ({ gateway, environmentName, onRetry, onStop }) => {
  const [expanded, setExpanded] = useState(false);
  const [envVarsOpen, setEnvVarsOpen] = useState(false);
  const [corsOpen, setCorsOpen] = useState(false);
  const tone = gatewayStatusTone(gateway.status);
  const scopeLabel = `${environmentName} · ${gateway.name}`;

  return (
    <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden' }}>
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
          <Typography variant="caption" color="text.secondary" noWrap display="block">
            {gateway.region}
          </Typography>
        </Box>
        <StatusPill tone={tone} />
        <Box sx={{ display: 'flex', color: 'text.secondary' }}>
          {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </Box>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ px: 1.5, pb: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          <DeploymentStatusBar tone={tone} />

          {gateway.status !== 'none' ? (
            <Box
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                p: 1.25,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
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
              <IconButton size="small" aria-label="Edit build" sx={{ p: 0 }}>
                <Pencil size={14} />
              </IconButton>
            </Box>
          ) : null}

          <ActionRow label="Environment Variables" icon={<Wrench size={14} />} onClick={() => setEnvVarsOpen(true)} />
          <Divider sx={{ borderStyle: 'dashed' }} />
          <ActionRow
            label="CORS, Rate Limiting and Resiliency"
            icon={<Wrench size={14} />}
            onClick={() => setCorsOpen(true)}
          />

          <Box>
            <Typography sx={{ ...sectionLabelSx, mb: 0.75 }}>Deployment History</Typography>
            {gateway.history.length === 0 ? (
              <Typography variant="caption" color="text.disabled">
                No deployments yet.
              </Typography>
            ) : (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                {gateway.history.map((deployment, index) => (
                  <Box key={`${deployment.buildId}-${index}`} sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                    <StatusDot tone={deployment.result === 'Success' ? 'success' : 'error'} />
                    <Typography variant="caption">{deployment.result}</Typography>
                    <Typography variant="caption" color="text.disabled">
                      · {deployment.buildId} · {relativeTime(deployment.when)}
                    </Typography>
                  </Box>
                ))}
              </Box>
            )}
          </Box>

          {gateway.status === 'failed' ? (
            <Button fullWidth variant="outlined" color="error" onClick={onRetry}>
              Retry deployment
            </Button>
          ) : null}

          {gateway.status === 'active' ? (
            <Button fullWidth variant="outlined" color="error" onClick={onStop}>
              Stop deployment
            </Button>
          ) : null}
        </Box>
      </Collapse>

      <EnvironmentVariablesDrawer
        open={envVarsOpen}
        onClose={() => setEnvVarsOpen(false)}
        scopeLabel={scopeLabel}
        count={gateway.envVars}
      />
      <CorsResiliencyDrawer open={corsOpen} onClose={() => setCorsOpen(false)} scopeLabel={scopeLabel} />
    </Box>
  );
};

export default GatewayRow;
