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
import { Box, Button, Chip, Divider, Typography } from '@wso2/oxygen-ui';
import { MoveRight, Wrench } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
import GatewayRow from './GatewayRow';
import { activeGatewayCount, hasAnyDeployment } from '../utils/status';
import type { Environment } from '../types';

export type EnvironmentCardProps = {
  environment: Environment;
  /** Set when a later stage exists, which is what makes promotion offerable. */
  nextEnvironmentName?: string;
  /** Set when this stage is the pipeline's entry, the only one deployed to directly. */
  isEntry: boolean;
  busy: boolean;
  onDeploy: () => void;
  onPromote: () => void;
  onEditSettings: () => void;
  onStopGateway: (gatewayId: string) => void;
  onRedeployGateway: (gatewayId: string) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

/*
 * The environment variables and CORS / rate-limiting / resiliency links from the
 * original design are left out until the API exposes those settings. What it does
 * support — the endpoint and virtual hosts an environment deploys with — is
 * reached through "Deployment settings" below.
 */
const EnvironmentCard: FC<EnvironmentCardProps> = ({
  environment,
  nextEnvironmentName,
  isEntry,
  busy,
  onDeploy,
  onPromote,
  onEditSettings,
  onStopGateway,
  onRedeployGateway,
}) => {
  const { gateways } = environment;
  const activeCount = activeGatewayCount(gateways);
  const promotable = hasAnyDeployment(gateways);

  return (
    <Box
      sx={{
        flex: '0 0 392px',
        width: 392,
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '14px',
        p: 2.5,
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        alignSelf: 'flex-start',
      }}
    >
      <Box>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>{environment.name}</Typography>
        <Typography variant="body2" color="text.secondary">
          {activeCount} of {gateways.length} gateway{gateways.length === 1 ? '' : 's'} active
        </Typography>
      </Box>

      <Divider />

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
        <Typography sx={sectionLabelSx}>Gateways</Typography>
        <Chip label={gateways.length} size="small" sx={{ height: 18, fontSize: 11 }} />
      </Box>

      {gateways.length === 0 ? (
        <Typography variant="caption" color="text.disabled">
          No gateway is bound to this environment yet.
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {gateways.map((gateway) => (
            <GatewayRow
              key={gateway.id}
              gateway={gateway}
              busy={busy}
              onStop={() => onStopGateway(gateway.id)}
              onRedeploy={() => onRedeployGateway(gateway.id)}
            />
          ))}
        </Box>
      )}

      <Divider />

      <ActionRow
        label="Deployment settings"
        icon={<Wrench size={14} />}
        onClick={onEditSettings}
      />

      {/*
        The entry environment is the only one that can be deployed to directly;
        every later one is reached by promoting from the stage before it, which is
        what keeps a higher environment from running an artifact that skipped a
        lower one. The API enforces the same rule.
      */}
      {isEntry && (
        <Button fullWidth variant="contained" onClick={onDeploy} disabled={busy}>
          Deploy
        </Button>
      )}

      {nextEnvironmentName ? (
        promotable ? (
          <Button
            fullWidth
            variant={isEntry ? 'outlined' : 'contained'}
            startIcon={<MoveRight size={16} />}
            onClick={onPromote}
            disabled={busy}
          >
            Promote to {nextEnvironmentName}
          </Button>
        ) : (
          <Box
            sx={{
              textAlign: 'center',
              py: 1,
              borderRadius: 1.5,
              bgcolor: 'action.disabledBackground',
              color: 'text.disabled',
              fontSize: 13,
            }}
          >
            Deploy here before promoting
          </Box>
        )
      ) : null}
    </Box>
  );
};

export default EnvironmentCard;
