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

import { Fragment, useEffect, useState, type FC } from 'react';
import { Alert, Box, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import BuildAreaCard from './components/BuildAreaCard';
import EnvironmentCard from './components/EnvironmentCard';
import PipelineConnector from './components/PipelineConnector';
import NoGatewayState from './components/NoGatewayState';
import DeployDialog from './components/DeployDialog';
import { undeletableBuildReasons } from './utils/status';
import type { ApiOwnership, Build, Environment } from './types';

export type DeployPageProps = {
  /** Pipeline environments in promotion order, as the server returns them. */
  environments: Environment[];
  /** The API's builds, newest first. */
  builds: Build[];
  /** Display name of the API, shown when there is no gateway to deploy to. */
  apiName: string;
  /** Creates the self-hosted gateway in the named environment. */
  onAddGateway: (environment: string) => void;
  /** Creates the managed gateway in the named environment. */
  onAddCloudGateway: (environment: string) => void;
  /** Set while that gateway is being built and until it is active. */
  provisioning?: { startedAt: number; environment: string };
  /** Set while the self-hosted gateway registration request is in flight. */
  selfHostedPending?: { startedAt: number };
  /** The backend URL the API is defined against; the deploy form starts from it. */
  apiEndpointUrl?: string;
  /**
   * Whether this artifact's deployments take a backend URL of their own. False for
   * the kinds that carry their upstream in their definition, which have nothing
   * per-gateway to ask for.
   */
  takesEndpoint: boolean;
  /**
   * Who manages this API's deployments. Its pipeline is always shown, but deploy,
   * promote and stop are enabled only once the control plane is confirmed to own it.
   */
  ownership: ApiOwnership;
  busy: boolean;
  /**
   * Deploys to `target`; `from` is set when this is a promotion. Every gateway
   * goes in one call with its own endpoint, because an environment runs a single
   * build of an API at a time.
   */
  onDeploy: (
    target: Environment,
    gateways: { gatewayId: string; endpointUrl?: string }[],
    from?: Environment,
    buildId?: string
  ) => void;
  onStopGateway: (environment: Environment, gatewayId: string) => void;
  /** Deletes a build, freeing a slot when the API is at its build limit. */
  onDeleteBuild: (buildId: string) => void;
};

/**
 * The pipeline laid out left to right: the build area, then one card per
 * environment in promotion order. That order comes from the server rather than
 * being arranged here, so the view cannot imply a promotion the pipeline does
 * not allow.
 */
/** Why the deploy actions are disabled in each ownership state that disables them. */
const LOCKED_REASONS: Record<ApiOwnership, string | undefined> = {
  checking: 'Checking whether this API can be deployed from here.',
  'control-plane': undefined,
  gateway: 'This API was created from a gateway, which manages its deployments.',
  unverified: "Couldn't confirm who manages this API's deployments, so deploy actions are disabled.",
};

type DialogState = {
  targetIndex: number;
  sourceIndex?: number;
  buildId?: string;
  createBuild?: boolean;
} | null;

const DeployPage: FC<DeployPageProps> = ({
  environments,
  builds,
  apiName,
  onAddGateway,
  onAddCloudGateway,
  provisioning,
  selfHostedPending,
  apiEndpointUrl,
  takesEndpoint,
  ownership,
  busy,
  onDeploy,
  onStopGateway,
  onDeleteBuild,
}) => {
  const [dialog, setDialog] = useState<DialogState>(null);
  // Why deploy, promote and stop are unavailable; undefined when they are allowed.
  const lockedReason = LOCKED_REASONS[ownership];

  // A dialog opened before the actions were locked must not stay usable.
  useEffect(() => {
    if (lockedReason) setDialog(null);
  }, [lockedReason]);

  const target = dialog ? (environments[dialog.targetIndex] ?? null) : null;
  const source =
    dialog?.sourceIndex !== undefined ? environments[dialog.sourceIndex] : undefined;

  const handleConfirm = (
    gateways: { gatewayId: string; endpointUrl?: string }[],
    buildId?: string
  ) => {
    if (target && !lockedReason) onDeploy(target, gateways, source, buildId);
    setDialog(null);
  };

  // Environments exist but none has a gateway, so there is nowhere to deploy yet.
  const hasNoGateways =
    environments.length > 0 && environments.every((environment) => environment.gateways.length === 0);

  return (
    <PageContent fullWidth>
      <PageTitle sx={{ mb: 2, flexShrink: 0 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
      </PageTitle>

      {ownership === 'gateway' && (
        <Alert severity="info" sx={{ mb: 2 }}>
          This API was created from a gateway. You can view its deployments, but deploy,
          promote and stop actions are managed by the gateway and are unavailable here.
        </Alert>
      )}
      {ownership === 'unverified' && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {LOCKED_REASONS.unverified} Reload the page to try again.
        </Alert>
      )}

      {hasNoGateways ? (
        <NoGatewayState
          apiName={apiName}
          environments={environments.map((environment) => environment.name)}
          onAddSelfHosted={onAddGateway}
          onAddCloud={onAddCloudGateway}
          provisioning={provisioning}
          selfHostedPending={selfHostedPending}
        />
      ) : environments.length === 0 ? (
        <Box
          sx={{
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
      ) : (
        <Box
          sx={{
            flex: 1,
            width: '100%',
            maxWidth: '100%',
            minWidth: 0,
            minHeight: 0,
            overflow: 'auto',
            overscrollBehavior: 'contain',
            scrollbarGutter: 'stable',
            WebkitOverflowScrolling: 'touch',
            pb: 2,
            '&::-webkit-scrollbar': {
              width: 10,
              height: 10,
            },
            '&::-webkit-scrollbar-thumb': {
              bgcolor: 'action.disabled',
              borderRadius: 5,
            },
            '&::-webkit-scrollbar-track': {
              bgcolor: 'action.hover',
              borderRadius: 5,
            },
          }}
        >
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              width: 'max-content',
              minWidth: '100%',
            }}
          >
            <BuildAreaCard
              builds={builds}
              targetEnvironment={environments[0]}
              undeletableBuilds={undeletableBuildReasons(environments)}
              busy={busy}
              lockedReason={lockedReason}
              onDeployClick={(buildId, createBuild) =>
                setDialog({ targetIndex: 0, buildId, createBuild })
              }
              onDeleteBuild={onDeleteBuild}
            />

            {environments.map((environment, index) => (
              <Fragment key={environment.name}>
                <PipelineConnector />
                <EnvironmentCard
                  environment={environment}
                  nextEnvironment={environments[index + 1]}
                  busy={busy}
                  lockedReason={lockedReason}
                  takesEndpoint={takesEndpoint}
                  onPromoteClick={() =>
                    setDialog({ targetIndex: index + 1, sourceIndex: index })
                  }
                  onStopGateway={(gatewayId) => onStopGateway(environment, gatewayId)}
                />
              </Fragment>
            ))}
          </Box>
        </Box>
      )}

      <DeployDialog
        open={dialog !== null}
        mode={source ? 'promote' : 'deploy'}
        environment={target}
        sourceEnvironment={source}
        builds={builds}
        apiEndpointUrl={apiEndpointUrl}
        takesEndpoint={takesEndpoint}
        initialBuildId={dialog?.buildId}
        createBuild={dialog?.createBuild ?? false}
        submitting={busy}
        onClose={() => setDialog(null)}
        onConfirm={handleConfirm}
      />
    </PageContent>
  );
};

export default DeployPage;
