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

import { Fragment, type FC } from 'react';
import { Box, Typography } from '@wso2/oxygen-ui';
import BuildCard from './components/BuildCard';
import EnvironmentCard from './components/EnvironmentCard';
import PipelineConnector from './components/PipelineConnector';
import type { Build, Environment } from './types';

export type DeployPageProps = {
  /** Pipeline environments in promotion order. */
  environments: Environment[];
  /** Prepared builds, newest first. */
  builds: Build[];
  busy: boolean;
  onPrepare: () => void;
  onDeploy: (environment: Environment) => void;
  onPromote: (target: Environment, from: Environment) => void;
  onEditSettings: (environment: Environment) => void;
  onStopGateway: (environment: Environment, gatewayId: string) => void;
  onRedeployGateway: (environment: Environment, gatewayId: string) => void;
};

/**
 * The pipeline laid out left to right, one card per environment in promotion
 * order. The order comes from the API rather than being arranged here, so the
 * view cannot imply a promotion the pipeline does not allow.
 */
const DeployPage: FC<DeployPageProps> = ({
  environments,
  builds,
  busy,
  onPrepare,
  onDeploy,
  onPromote,
  onEditSettings,
  onStopGateway,
  onRedeployGateway,
}) => {
  if (environments.length === 0) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 3 }}>
        <BuildCard builds={builds} busy={busy} onPrepare={onPrepare} />
        <Box
          sx={{
            flexGrow: 1,
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1.5,
            py: 6,
            px: 3,
            textAlign: 'center',
          }}
        >
          <Typography variant="body2" color="text.secondary">
            This project&apos;s deployment pipeline has no environments yet. Add environments to
            the pipeline to deploy this API.
          </Typography>
        </Box>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 0,
        width: '100%',
        minWidth: 0,
        overflowX: 'auto',
        pb: 1,
      }}
    >
      <BuildCard builds={builds} busy={busy} onPrepare={onPrepare} />

      {environments.map((environment, index) => (
        <Fragment key={environment.name}>
          <PipelineConnector />
          <EnvironmentCard
            environment={environment}
            isEntry={index === 0}
            nextEnvironmentName={environments[index + 1]?.name}
            busy={busy}
            onDeploy={() => onDeploy(environment)}
            onPromote={() => {
              const next = environments[index + 1];
              if (next) onPromote(next, environment);
            }}
            onEditSettings={() => onEditSettings(environment)}
            onStopGateway={(gatewayId) => onStopGateway(environment, gatewayId)}
            onRedeployGateway={(gatewayId) => onRedeployGateway(environment, gatewayId)}
          />
        </Fragment>
      ))}
    </Box>
  );
};

export default DeployPage;
