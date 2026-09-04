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

import { Fragment, useRef, useState, type FC } from "react";
import { Box, PageContent, PageTitle } from "@wso2/oxygen-ui";
import BuildAreaCard from "./components/BuildAreaCard";
import EnvironmentCard from "./components/EnvironmentCard";
import PipelineConnector from "./components/PipelineConnector";
import DeployDialog from "./components/DeployDialog";
import { seedBuildHistory, seedEnvironments } from "./mocks/deploy.mock";
import type { BuildRecord, Environment, Gateway } from "./types";

/** How long a gateway stays in `deploying` before the mock flips it to `active`. */
const DEPLOY_DURATION_MS = 1100;

export type DeployPageProps = {
  notify?: (message: string) => void;
};

type DialogState = { mode: "deploy" | "promote"; environmentId: string } | null;

const findEnvironmentById = (environments: Environment[], id: string) =>
  environments.find((environment) => environment.id === id);

const DeployPage: FC<DeployPageProps> = ({ notify }) => {
  const [environments, setEnvironments] = useState<Environment[]>(() =>
    seedEnvironments(),
  );
  const [buildHistory, setBuildHistory] = useState<BuildRecord[]>(() =>
    seedBuildHistory(),
  );
  const [dialog, setDialog] = useState<DialogState>(null);
  const buildCounter = useRef(1043);

  const dialogEnvironment = dialog
    ? (findEnvironmentById(environments, dialog.environmentId) ?? null)
    : null;

  const updateGateways = (
    environmentId: string,
    gatewayIds: readonly string[],
    updater: (gateway: Gateway) => Gateway,
  ) => {
    setEnvironments((prev) =>
      prev.map((environment) => {
        if (environment.id !== environmentId) return environment;
        return {
          ...environment,
          gateways: environment.gateways.map((gateway) =>
            gatewayIds.includes(gateway.id) ? updater(gateway) : gateway,
          ),
        };
      }),
    );
  };

  const runDeployment = (environmentId: string, gatewayId: string, endpointUrl: string) => {
    updateGateways(environmentId, [gatewayId], (gateway) => ({
      ...gateway,
      status: "deploying",
    }));

    setTimeout(() => {
      const buildId = `b-${buildCounter.current++}`;
      const when = new Date().toISOString();

      updateGateways(environmentId, [gatewayId], (gateway) => ({
        ...gateway,
        status: "active",
        buildId,
        deployedAt: when,
        endpointUrl,
        history: [{ result: "Success", buildId, when }, ...gateway.history],
      }));

      setBuildHistory((prev) => [
        {
          id: `build-${buildId}-${environmentId}`,
          buildId,
          result: "Success",
          when,
          targetEnvironmentId: environmentId,
          targetGatewayCount: 1,
        },
        ...prev,
      ]);

      const environmentName =
        findEnvironmentById(environments, environmentId)?.name ?? environmentId;
      notify?.(`Deployed build ${buildId} to ${environmentName}.`);
    }, DEPLOY_DURATION_MS);
  };

  const handleDeployClick = () => {
    const first = environments[0];
    if (!first) return;
    setDialog({ mode: "deploy", environmentId: first.id });
  };

  const handlePromoteClick = (environmentIndex: number) => {
    const next = environments[environmentIndex + 1];
    if (!next) return;
    setDialog({ mode: "promote", environmentId: next.id });
  };

  const handleConfirmDialog = (gatewayId: string, endpointUrl: string) => {
    if (!dialog) return;
    runDeployment(dialog.environmentId, gatewayId, endpointUrl);
    setDialog(null);
  };

  const handleStop = (environmentId: string, gatewayId: string) => {
    setEnvironments((prev) =>
      prev.map((environment) => {
        if (environment.id !== environmentId) return environment;
        return {
          ...environment,
          gateways: environment.gateways.map((gateway) =>
            gateway.id === gatewayId
              ? { ...gateway, status: "none" as const }
              : gateway,
          ),
        };
      }),
    );
    const environmentName =
      findEnvironmentById(environments, environmentId)?.name ?? environmentId;
    notify?.(`Stopped gateway in ${environmentName}.`);
  };

  const handleRetry = (environmentId: string, gatewayId: string) => {
    const gateway = findEnvironmentById(environments, environmentId)?.gateways.find(
      (candidate) => candidate.id === gatewayId,
    );
    runDeployment(environmentId, gatewayId, gateway?.endpointUrl ?? "");
  };

  return (
    <PageContent
      fullWidth
      sx={{
        boxSizing: "border-box",
        display: "flex",
        flexDirection: "column",
        width: { xs: "calc(100dvw - 64px)", md: "calc(100dvw - 250px)" },
        maxWidth: { xs: "calc(100dvw - 64px)", md: "calc(100dvw - 250px)" },
        height: "100%",
        minWidth: 0,
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      <PageTitle sx={{ mb: 2, flexShrink: 0 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
      </PageTitle>

      <Box
        sx={{
          flex: 1,
          width: "100%",
          maxWidth: "100%",
          minWidth: 0,
          minHeight: 0,
          overflow: "auto",
          overscrollBehavior: "contain",
          scrollbarGutter: "stable",
          WebkitOverflowScrolling: "touch",
          pb: 2,
          "&::-webkit-scrollbar": {
            width: 10,
            height: 10,
          },
          "&::-webkit-scrollbar-thumb": {
            bgcolor: "action.disabled",
            borderRadius: 5,
          },
          "&::-webkit-scrollbar-track": {
            bgcolor: "action.hover",
            borderRadius: 5,
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "flex-start",
            width: "max-content",
            minWidth: "100%",
          }}
        >
          <BuildAreaCard
            buildHistory={buildHistory}
            targetEnvironment={environments[0]}
            onDeployClick={handleDeployClick}
          />

          {environments.map((env, index) => (
            <Fragment key={env.id}>
              <PipelineConnector />
              <EnvironmentCard
                environment={env}
                nextEnvironment={environments[index + 1]}
                onPromoteClick={() => handlePromoteClick(index)}
                onStopGateway={(gatewayId) => handleStop(env.id, gatewayId)}
                onRetryGateway={(gatewayId) => handleRetry(env.id, gatewayId)}
              />
            </Fragment>
          ))}
        </Box>
      </Box>

      <DeployDialog
        open={dialog !== null}
        mode={dialog?.mode ?? "deploy"}
        environment={dialogEnvironment}
        onClose={() => setDialog(null)}
        onConfirm={handleConfirmDialog}
      />
    </PageContent>
  );
};

export default DeployPage;
