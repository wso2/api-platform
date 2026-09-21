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
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Rocket } from '@wso2/oxygen-ui-icons-react';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import { activeGatewayCount, deployedGatewayCount } from '../utils/status';
import type { Environment } from '../types';

export type ProviderEnvironmentRowProps = {
  environment: Environment;
  expanded: boolean;
  onToggleExpand: (expanded: boolean) => void;
  busy: boolean;
  onDeployClick: () => void;
  onStopGateway: (gatewayId: string) => void;
};

/**
 * One environment, as a row that opens onto its gateways.
 *
 * A row rather than a card in a grid: environments are a list the page is read down, and
 * the portal states its lists this way — full width, one outlined accordion per item, the
 * action on the right of the summary. What matters at a glance is on the summary line, so
 * the page can be understood without opening anything.
 */
const ProviderEnvironmentRow: FC<ProviderEnvironmentRowProps> = ({
  environment,
  expanded,
  onToggleExpand,
  busy,
  onDeployClick,
  onStopGateway,
}) => {
  const { gateways } = environment;
  const activeCount = activeGatewayCount(gateways);
  const deployedCount = deployedGatewayCount(gateways);
  // What the environment is serving: the distinct builds across gateways that are up. More
  // than one means its gateways are split, which is worth saying rather than picking one.
  const runningBuilds = Array.from(
    new Set(
      gateways
        .filter((gateway) => gateway.status === 'DEPLOYED' && !!gateway.buildId)
        .map((gateway) => gateway.buildId as string)
    )
  ).sort();

  const deployDisabledReason =
    gateways.length === 0
      ? 'No AI gateway is bound to this environment yet.'
      : activeCount === 0
        ? 'Every gateway here is inactive. Activate one to deploy.'
        : '';

  return (
    <Accordion
      expanded={expanded}
      onChange={(_, isExpanded) => onToggleExpand(isExpanded)}
      variant="outlined"
      sx={{
        borderRadius: '8px',
        overflow: 'hidden',
        '&:before': { display: 'none' },
        '&.Mui-expanded': { margin: 0, borderRadius: '8px' },
      }}
    >
      <AccordionSummary
        sx={{
          px: 3,
          py: 1.5,
          // Two lines of content need room MUI's default summary height does not give,
          // and the expanded height must not jump when the row opens.
          minHeight: 72,
          '&.Mui-expanded': { minHeight: 72 },
          '& .MuiAccordionSummary-content': {
            m: 0,
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 2,
            '&.Mui-expanded': { m: 0 },
          },
        }}
      >
        <Box sx={{ minWidth: 0 }}>
          {/* What this provider is doing here comes first; what the environment could do
              is the quieter second line. */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0 }}>
            <Typography sx={{ fontWeight: 600, fontSize: 15 }}>{environment.name}</Typography>
            <Typography variant="body2" color="text.secondary" noWrap>
              {gateways.length === 0
                ? 'No gateways'
                : `Deployed on ${deployedCount} of ${gateways.length} gateway${gateways.length === 1 ? '' : 's'}`}
            </Typography>
          {runningBuilds.length === 1 ? (
            <Chip
              size="small"
              variant="outlined"
              label={runningBuilds[0]}
              sx={{ height: 20, fontSize: '0.7rem' }}
            />
          ) : runningBuilds.length > 1 ? (
            <Chip
              size="small"
              color="warning"
              variant="outlined"
              label={`${runningBuilds.length} builds`}
              sx={{ height: 20, fontSize: '0.7rem' }}
            />
          ) : (
            <Typography variant="caption" color="text.disabled">
              Nothing deployed
            </Typography>
          )}
          </Box>
          {gateways.length > 0 ? (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
              {activeCount} of {gateways.length} gateway{gateways.length === 1 ? '' : 's'} active
            </Typography>
          ) : null}
        </Box>

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexShrink: 0 }}>
          {/* Stops the summary toggling when the action is used. */}
          <Box component="span" onClick={(event) => event.stopPropagation()}>
            <Tooltip title={deployDisabledReason}>
              <span>
                <Button
                  size="small"
                  variant="contained"
                  startIcon={<Rocket size={15} />}
                  disabled={busy || deployDisabledReason !== ''}
                  onClick={onDeployClick}
                >
                  Deploy
                </Button>
              </span>
            </Tooltip>
          </Box>
          <ChevronDown
            size={20}
            style={{
              transition: 'transform 0.2s ease',
              transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)',
            }}
          />
        </Box>
      </AccordionSummary>

      <AccordionDetails sx={{ px: 3, pb: 3, pt: 0.5 }}>
        {gateways.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No AI gateway is bound to this environment yet.
          </Typography>
        ) : (
          <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            {gateways.map((gateway, index) => {
              // Stopping is offered on the gateway's own line rather than behind another
              // disclosure: it is the only thing there is to do to a gateway here, and a
              // second thing to open before reaching it read as though it were missing.
              const canStop =
                !busy && !!gateway.deploymentId && ['DEPLOYED', 'FAILED'].includes(gateway.status);
              return (
                <Box
                  key={gateway.id}
                  sx={{
                    px: 2,
                    py: 1.75,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.5,
                    borderTop: index === 0 ? 'none' : '1px solid',
                    borderColor: 'divider',
                  }}
                >
                  <StatusDot tone={gateway.health === 'active' ? 'success' : 'default'} />
                  <Box sx={{ minWidth: 0, flexGrow: 1 }}>
                    {/* The state sits with the name it describes, leaving the right of the
                        row to the one action. */}
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                      <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
                        {gateway.name}
                        {gateway.isDefault ? ' · Default' : ''}
                      </Typography>
                      <StatusPill tone={gatewayStatusTone(gateway.status)} variant="outlined" />
                    </Box>
                    {gateway.host ? (
                      <Typography variant="caption" color="text.secondary" noWrap display="block">
                        {gateway.host}
                      </Typography>
                    ) : null}
                  </Box>
                  <Button
                    size="small"
                    color="error"
                    variant="outlined"
                    disabled={!canStop}
                    onClick={() => onStopGateway(gateway.id)}
                  >
                    Stop
                  </Button>
                </Box>
              );
            })}
          </Box>
        )}
      </AccordionDetails>
    </Accordion>
  );
};

export default ProviderEnvironmentRow;
