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
import ProviderEnvironmentRow from './components/ProviderEnvironmentRow';
import ProviderDeployDrawer from './components/ProviderDeployDrawer';
import ProviderBuildsCard from './components/ProviderBuildsCard';
import { undeletableBuildReasons } from './utils/status';
import type { ProviderUpstream } from './providerDeployApi';
import type { Build, Environment } from './types';

export type ProviderDeployPageProps = {
  /** The organization's environments, in the order the server returned them. */
  environments: Environment[];
  /** The provider's builds, newest first. */
  builds: Build[];
  /** What the provider itself uses, which each gateway's fields start from. */
  upstream: ProviderUpstream;
  busy: boolean;
  onDeploy: (
    target: Environment,
    gateways: {
      gatewayId: string;
      apiKey?: string;
      authHeader?: string;
      endpointUrl?: string;
    }[],
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
  upstream,
  busy,
  onDeploy,
  onStopGateway,
  onDeleteBuild,
}) => {
  const [target, setTarget] = useState<Environment | null>(null);
  // Open the first environment initially so its gateway details are immediately visible.
  const [expandedEnvironments, setExpandedEnvironments] = useState<string[]>(() =>
    environments[0] ? [environments[0].name] : []
  );

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

      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          flexDirection: { xs: 'column', md: 'row' },
          gap: 3,
          mt: 4,
        }}
      >
        <ProviderBuildsCard
          builds={builds}
          undeletableBuilds={undeletableBuildReasons(environments)}
          busy={busy}
          onDeleteBuild={onDeleteBuild}
        />

        <Box sx={{ minWidth: 0, flex: 1, width: { xs: '100%', md: 'auto' } }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
            <Typography
              sx={{
                color: 'text.secondary',
                fontSize: 12,
                fontWeight: 600,
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
              }}
            >
              Environments
            </Typography>
            <Box
              sx={{
                minWidth: 28,
                height: 24,
                px: 1,
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 12,
              }}
            >
              <Typography variant="caption" sx={{ fontWeight: 600 }}>
                {environments.length}
              </Typography>
            </Box>
          </Box>

          {environments.length === 0 ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <Typography variant="body1" gutterBottom>
                No environments yet
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Add an environment to deploy this provider.
              </Typography>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              {environments.map((environment) => (
                <ProviderEnvironmentRow
                  key={environment.name}
                  environment={environment}
                  expanded={expandedEnvironments.includes(environment.name)}
                  onToggleExpand={(isExpanded) =>
                    setExpandedEnvironments((previous) =>
                      isExpanded
                        ? [...previous, environment.name]
                        : previous.filter((name) => name !== environment.name)
                    )
                  }
                  busy={busy}
                  onDeployClick={() => setTarget(environment)}
                  onStopGateway={(gatewayId) => onStopGateway(environment, gatewayId)}
                />
              ))}
            </Box>
          )}
        </Box>
      </Box>

      <ProviderDeployDrawer
        open={openTarget !== null}
        environment={openTarget}
        builds={builds}
        upstream={upstream}
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
