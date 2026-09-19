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
import { Box, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import ProviderEnvironmentCard from './components/ProviderEnvironmentCard';
import ProviderDeployDialog from './components/ProviderDeployDialog';
import ProviderBuildsCard from './components/ProviderBuildsCard';
import { undeletableBuildReasons } from './utils/status';
import type { Build, Environment } from './types';

export type ProviderDeployPageProps = {
  /** The organization's environments, in the order the server returned them. */
  environments: Environment[];
  /** The provider's builds, newest first. */
  builds: Build[];
  /** Whether a gateway can name the header its credential is sent in (api-key upstreams). */
  takesAuthHeader: boolean;
  busy: boolean;
  onDeploy: (
    target: Environment,
    gateways: { gatewayId: string; apiKey?: string; authHeader?: string }[],
    buildId?: string
  ) => void;
  onStopGateway: (environment: Environment, gatewayId: string) => void;
  /** Deletes a build, freeing a slot when the provider is at its build limit. */
  onDeleteBuild: (buildId: string) => void;
};

/**
 * The provider's environments, laid out as a grid.
 *
 * Deliberately not the pipeline's left-to-right row: that layout exists to show a
 * promotion order, and a provider has none. Every environment here is reachable
 * directly, so none of them comes before another and the grid says that by giving
 * them equal standing.
 */
const ProviderDeployPage: FC<ProviderDeployPageProps> = ({
  environments,
  builds,
  takesAuthHeader,
  busy,
  onDeploy,
  onStopGateway,
  onDeleteBuild,
}) => {
  const [target, setTarget] = useState<Environment | null>(null);

  // The dialog renders from the freshly loaded environment rather than the one
  // captured when it opened, so a background refresh keeps its gateway list and
  // statuses current while it is open.
  const openTarget = target
    ? (environments.find((environment) => environment.name === target.name) ?? null)
    : null;

  return (
    <PageContent fullWidth sx={{ minWidth: 0 }}>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
        <PageTitle.SubHeader>
          Deploy this provider to the gateways of your environments.
        </PageTitle.SubHeader>
      </PageTitle>

      <ProviderBuildsCard
        builds={builds}
        undeletableBuilds={undeletableBuildReasons(environments)}
        busy={busy}
        onDeleteBuild={onDeleteBuild}
      />

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
            This organization has no environments yet. Add one to deploy this provider.
          </Typography>
        </Box>
      ) : (
        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: { xs: '1fr', sm: 'repeat(auto-fill, minmax(360px, 1fr))' },
            alignItems: 'start',
          }}
        >
          {environments.map((environment) => (
            <ProviderEnvironmentCard
              key={environment.name}
              environment={environment}
              busy={busy}
              onDeployClick={() => setTarget(environment)}
              onStopGateway={(gatewayId) => onStopGateway(environment, gatewayId)}
            />
          ))}
        </Box>
      )}

      <ProviderDeployDialog
        open={openTarget !== null}
        environment={openTarget}
        builds={builds}
        takesAuthHeader={takesAuthHeader}
        submitting={busy}
        onClose={() => setTarget(null)}
        onConfirm={(gateways, buildId) => {
          if (openTarget) onDeploy(openTarget, gateways, buildId);
          setTarget(null);
        }}
      />
    </PageContent>
  );
};

export default ProviderDeployPage;
