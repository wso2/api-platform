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
import { Box, Button, Chip, Divider, Typography } from '@wso2/oxygen-ui';
import { MoveRight, Wrench } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
import CorsResiliencyDrawer from './CorsResiliencyDrawer';
import EnvironmentVariablesDrawer from './EnvironmentVariablesDrawer';
import GatewayRow from './GatewayRow';
import { activeGatewayCount, hasAnyDeployment } from '../utils/status';
import type { Environment } from '../types';

export type EnvironmentCardProps = {
  environment: Environment;
  nextEnvironmentName?: string;
  onPromoteClick: () => void;
  onStopGateway: (gatewayId: string) => void;
  onRetryGateway: (gatewayId: string) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

const EnvironmentCard: FC<EnvironmentCardProps> = ({
  environment,
  nextEnvironmentName,
  onPromoteClick,
  onStopGateway,
  onRetryGateway,
}) => {
  const [envVarsOpen, setEnvVarsOpen] = useState(false);
  const [corsOpen, setCorsOpen] = useState(false);

  const { gateways } = environment;
  const activeCount = activeGatewayCount(gateways);
  const deployed = hasAnyDeployment(gateways);

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
          {activeCount} of {gateways.length} gateways active
        </Typography>
      </Box>

      <Divider />

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
        <Typography sx={sectionLabelSx}>Gateways</Typography>
        <Chip label={gateways.length} size="small" sx={{ height: 18, fontSize: 11 }} />
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {gateways.map((gateway) => (
          <GatewayRow
            key={gateway.id}
            gateway={gateway}
            environmentName={environment.name}
            onRetry={() => onRetryGateway(gateway.id)}
            onStop={() => onStopGateway(gateway.id)}
          />
        ))}
      </Box>

      <Divider />

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        <ActionRow label="Environment Variables" icon={<Wrench size={14} />} onClick={() => setEnvVarsOpen(true)} />
        <Divider sx={{ borderStyle: 'dashed' }} />
        <ActionRow
          label="CORS, Rate Limiting and Resiliency"
          icon={<Wrench size={14} />}
          onClick={() => setCorsOpen(true)}
        />
      </Box>

      {nextEnvironmentName ? (
        deployed ? (
          <Button fullWidth variant="contained" startIcon={<MoveRight size={16} />} onClick={onPromoteClick}>
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

      <EnvironmentVariablesDrawer
        open={envVarsOpen}
        onClose={() => setEnvVarsOpen(false)}
        scopeLabel={environment.name}
        count={environment.envVars}
      />
      <CorsResiliencyDrawer open={corsOpen} onClose={() => setCorsOpen(false)} scopeLabel={environment.name} />
    </Box>
  );
};

export default EnvironmentCard;
