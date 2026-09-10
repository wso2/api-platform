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

import { Fragment, useState, type FC } from 'react';
import { Box, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import BuildAreaCard from './components/BuildAreaCard';
import EnvironmentCard from './components/EnvironmentCard';
import PipelineConnector from './components/PipelineConnector';
import DeployDialog from './components/DeployDialog';
import type { Build, Environment } from './types';

export type DeployPageProps = {
  /** Pipeline environments in promotion order, as the server returns them. */
  environments: Environment[];
  /** The API's builds, newest first. */
  builds: Build[];
  /** The backend URL the API is defined against; the deploy form starts from it. */
  apiEndpointUrl?: string;
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
  onRetryGateway: (environment: Environment, gatewayId: string) => void;
};

/**
 * The pipeline laid out left to right: the build area, then one card per
 * environment in promotion order. That order comes from the server rather than
 * being arranged here, so the view cannot imply a promotion the pipeline does
 * not allow.
 */
type DialogState = {
  targetIndex: number;
  sourceIndex?: number;
  buildId?: string;
  createBuild?: boolean;
} | null;

const DeployPage: FC<DeployPageProps> = ({
  environments,
  builds,
  apiEndpointUrl,
  busy,
  onDeploy,
  onStopGateway,
  onRetryGateway,
}) => {
  const [dialog, setDialog] = useState<DialogState>(null);

  const target = dialog ? (environments[dialog.targetIndex] ?? null) : null;
  const source =
    dialog?.sourceIndex !== undefined ? environments[dialog.sourceIndex] : undefined;

  const handleConfirm = (
    gateways: { gatewayId: string; endpointUrl?: string }[],
    buildId?: string
  ) => {
    if (target) onDeploy(target, gateways, source, buildId);
    setDialog(null);
  };

  return (
    <PageContent
      fullWidth
      sx={{
        boxSizing: 'border-box',
        display: 'flex',
        flexDirection: 'column',
        width: { xs: 'calc(100dvw - 64px)', md: 'calc(100dvw - 250px)' },
        maxWidth: { xs: 'calc(100dvw - 64px)', md: 'calc(100dvw - 250px)' },
        height: '100%',
        minWidth: 0,
        minHeight: 0,
        overflow: 'hidden',
      }}
    >
      <PageTitle sx={{ mb: 2, flexShrink: 0 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
      </PageTitle>

      {environments.length === 0 ? (
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
              busy={busy}
              onDeployClick={(buildId, createBuild) =>
                setDialog({ targetIndex: 0, buildId, createBuild })
              }
            />

            {environments.map((environment, index) => (
              <Fragment key={environment.name}>
                <PipelineConnector />
                <EnvironmentCard
                  environment={environment}
                  nextEnvironment={environments[index + 1]}
                  busy={busy}
                  onPromoteClick={() =>
                    setDialog({ targetIndex: index + 1, sourceIndex: index })
                  }
                  onStopGateway={(gatewayId) => onStopGateway(environment, gatewayId)}
                  onRetryGateway={(gatewayId) => onRetryGateway(environment, gatewayId)}
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
