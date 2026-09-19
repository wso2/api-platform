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
import { Box, Button, Card, CardContent, Chip, Divider, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Rocket } from '@wso2/oxygen-ui-icons-react';
import GatewayRow from './GatewayRow';
import { activeGatewayCount } from '../utils/status';
import type { Environment } from '../types';

export type ProviderEnvironmentCardProps = {
  environment: Environment;
  busy: boolean;
  onDeployClick: () => void;
  onStopGateway: (gatewayId: string) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

/**
 * One environment the provider can be deployed to, with every AI gateway in it.
 *
 * Each card carries its own Deploy button rather than a promote button: a provider
 * belongs to the organization, so there is no pipeline behind it and no order in
 * which its environments must be reached. Every environment is deployed to
 * directly, and the card says so by offering the same action everywhere.
 */
const ProviderEnvironmentCard: FC<ProviderEnvironmentCardProps> = ({
  environment,
  busy,
  onDeployClick,
  onStopGateway,
}) => {
  const { gateways } = environment;
  const activeCount = activeGatewayCount(gateways);
  // What the environment is serving: the distinct builds across gateways that are
  // actually up. A settling or stopped gateway is not part of what it serves.
  const runningBuilds = Array.from(
    new Set(
      gateways
        .filter((gateway) => gateway.status === 'DEPLOYED' && !!gateway.buildId)
        .map((gateway) => gateway.buildId as string)
    )
  ).sort();
  // Nowhere to deploy is a state of the environment, not of the provider: an
  // environment with no AI gateway, or none of them up, cannot receive a
  // deployment, so the button says why instead of offering an action that fails.
  const deployDisabledReason =
    gateways.length === 0
      ? `${environment.name} has no AI gateway to deploy to. Add one to deploy here.`
      : activeCount === 0
        ? `Every gateway in ${environment.name} is inactive. Activate one to deploy here.`
        : '';

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column' }}>
      <CardContent
        sx={{
          p: 2.5,
          display: 'flex',
          flexDirection: 'column',
          gap: 1.5,
          flex: 1,
          '&:last-child': { pb: 2.5 },
        }}
      >
        {/* Name on the left, what the environment is serving on the right. The build is
            reported per gateway, so more than one showing means this environment's
            gateways are split across builds — worth saying rather than quietly
            listing the first. */}
        <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 1 }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={{ fontSize: 16, fontWeight: 600 }}>{environment.name}</Typography>
            <Typography variant="body2" color="text.secondary">
              {activeCount} of {gateways.length} gateway{gateways.length === 1 ? '' : 's'} active
            </Typography>
          </Box>
          <Box sx={{ textAlign: 'right', flexShrink: 0 }}>
            <Typography sx={{ ...sectionLabelSx, display: 'block' }}>Build</Typography>
            {runningBuilds.length === 0 ? (
              <Typography variant="caption" color="text.disabled">
                Nothing deployed
              </Typography>
            ) : runningBuilds.length === 1 ? (
              <Chip
                label={runningBuilds[0]}
                size="small"
                variant="outlined"
                sx={{ height: 20, fontSize: '0.7rem', mt: 0.25 }}
              />
            ) : (
              <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.25 }}>
                <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                  {runningBuilds.map((id) => (
                    <Chip
                      key={id}
                      label={id}
                      size="small"
                      color="warning"
                      variant="outlined"
                      sx={{ height: 20, fontSize: '0.7rem' }}
                    />
                  ))}
                </Box>
                <Typography variant="caption" color="warning.main">
                  Split across builds
                </Typography>
              </Box>
            )}
          </Box>
        </Box>

        <Divider />

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
          <Typography sx={sectionLabelSx}>Gateways</Typography>
          <Chip label={gateways.length} size="small" sx={{ height: 18, fontSize: 11 }} />
        </Box>

        {gateways.length === 0 ? (
          <Typography variant="caption" color="text.disabled">
            No AI gateway is bound to this environment yet.
          </Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {gateways.map((gateway) => (
              <GatewayRow
                key={gateway.id}
                gateway={gateway}
                environmentName={environment.name}
                busy={busy}
                // A provider's upstream is the provider's, not the deployment's.
                showEndpointUrl={false}
                onStop={() => onStopGateway(gateway.id)}
              />
            ))}
          </Box>
        )}

        <Box sx={{ mt: 'auto', pt: 0.5 }}>
          <Divider sx={{ mb: 1.5 }} />
          <Tooltip title={deployDisabledReason}>
            <span style={{ display: 'block' }}>
              <Button
                fullWidth
                variant="contained"
                startIcon={<Rocket size={16} />}
                disabled={busy || deployDisabledReason !== ''}
                onClick={onDeployClick}
              >
                Deploy to {environment.name}
              </Button>
            </span>
          </Tooltip>
        </Box>
      </CardContent>
    </Card>
  );
};

export default ProviderEnvironmentCard;
