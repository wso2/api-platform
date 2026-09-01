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

import { Fragment, useRef, useState, type FC } from 'react';
import { Box, PageContent, PageTitle } from '@wso2/oxygen-ui';
import BuildAreaCard from './components/BuildAreaCard';
import EnvironmentCard from './components/EnvironmentCard';
import PipelineConnector from './components/PipelineConnector';
import DeployDialog from './components/DeployDialog';
import { seedBuildHistory, seedEnvironments } from './mocks/deploy.mock';
import type { BuildRecord, Environment, Gateway } from './types';

/** How long a gateway stays in `deploying` before the mock flips it to `active`. */
const DEPLOY_DURATION_MS = 1100;

export type DeployPageProps = {
  notify?: (message: string) => void;
};

type DialogState = { mode: 'deploy' | 'promote'; environmentId: string } | null;

const findEnvironmentById = (environments: Environment[], id: string) =>
  environments.find((environment) => environment.id === id);

const DeployPage: FC<DeployPageProps> = ({ notify }) => {
  const [environments, setEnvironments] = useState<Environment[]>(() => seedEnvironments());
  const [buildHistory, setBuildHistory] = useState<BuildRecord[]>(() => seedBuildHistory());
  const [dialog, setDialog] = useState<DialogState>(null);
  const buildCounter = useRef(1043);

  const dialogEnvironment = dialog ? findEnvironmentById(environments, dialog.environmentId) ?? null : null;

  const updateGateways = (
    environmentId: string,
    gatewayIds: readonly string[],
    updater: (gateway: Gateway) => Gateway
  ) => {
    setEnvironments((prev) =>
      prev.map((environment) => {
        if (environment.id !== environmentId) return environment;
        return {
          ...environment,
          gateways: environment.gateways.map((gateway) =>
            gatewayIds.includes(gateway.id) ? updater(gateway) : gateway
          ),
        };
      })
    );
  };

  const runDeployment = (environmentId: string, gatewayIds: string[]) => {
    updateGateways(environmentId, gatewayIds, (gateway) => ({ ...gateway, status: 'deploying' }));

    setTimeout(() => {
      const buildId = `b-${buildCounter.current++}`;
      const when = new Date().toISOString();

      updateGateways(environmentId, gatewayIds, (gateway) => ({
        ...gateway,
        status: 'active',
        buildId,
        deployedAt: when,
        history: [{ result: 'Success', buildId, when }, ...gateway.history],
      }));

      setBuildHistory((prev) => [
        {
          id: `build-${buildId}-${environmentId}`,
          buildId,
          result: 'Success',
          when,
          targetEnvironmentId: environmentId,
          targetGatewayCount: gatewayIds.length,
        },
        ...prev,
      ]);

      const environmentName = findEnvironmentById(environments, environmentId)?.name ?? environmentId;
      notify?.(`Deployed build ${buildId} to ${environmentName}.`);
    }, DEPLOY_DURATION_MS);
  };

  const handleDeployClick = () => {
    const first = environments[0];
    if (!first) return;
    setDialog({ mode: 'deploy', environmentId: first.id });
  };

  const handlePromoteClick = (environmentIndex: number) => {
    const next = environments[environmentIndex + 1];
    if (!next) return;
    setDialog({ mode: 'promote', environmentId: next.id });
  };

  const handleConfirmDialog = (gatewayIds: string[]) => {
    if (!dialog) return;
    runDeployment(dialog.environmentId, gatewayIds);
    setDialog(null);
  };

  const handleStop = (environmentId: string, gatewayId: string) => {
    setEnvironments((prev) =>
      prev.map((environment) => {
        if (environment.id !== environmentId) return environment;
        return {
          ...environment,
          gateways: environment.gateways.map((gateway) =>
            gateway.id === gatewayId ? { ...gateway, status: 'none' as const } : gateway
          ),
        };
      })
    );
    const environmentName = findEnvironmentById(environments, environmentId)?.name ?? environmentId;
    notify?.(`Stopped gateway in ${environmentName}.`);
  };

  const handleRetry = (environmentId: string, gatewayId: string) => {
    runDeployment(environmentId, [gatewayId]);
  };

  return (
    <PageContent fullWidth sx={{ minWidth: 0 }}>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
      </PageTitle>

      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: 0,
          width: '90%',
          minWidth: 0,
          overflowX: 'auto',
          pb: 1,
        }}
      >
        <BuildAreaCard buildHistory={buildHistory} environments={environments} onDeployClick={handleDeployClick} />

        {environments.map((env, index) => (
          <Fragment key={env.id}>
            <PipelineConnector />
            <EnvironmentCard
              environment={env}
              nextEnvironmentName={environments[index + 1]?.name}
              onPromoteClick={() => handlePromoteClick(index)}
              onStopGateway={(gatewayId) => handleStop(env.id, gatewayId)}
              onRetryGateway={(gatewayId) => handleRetry(env.id, gatewayId)}
            />
          </Fragment>
        ))}
      </Box>

      <DeployDialog
        open={dialog !== null}
        mode={dialog?.mode ?? 'deploy'}
        environment={dialogEnvironment}
        onClose={() => setDialog(null)}
        onConfirm={handleConfirmDialog}
      />
    </PageContent>
  );
};

export default DeployPage;
