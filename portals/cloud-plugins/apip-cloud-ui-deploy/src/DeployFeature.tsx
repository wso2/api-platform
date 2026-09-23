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

import { useCallback, useEffect, useMemo, useRef, useState, type FC } from 'react';
import { Box, Button, CircularProgress, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import DeployPage from './DeployPage';
import { createDeployClient, takesEndpointUrl, type ArtifactKind } from './deployApi';
import { isSettling } from './utils/status';
import type { CloudHostPort } from './hostPort';
import type { Build, Environment } from './types';

export type DeployFeatureProps = {
  port: CloudHostPort;
  /**
   * The artifact kind being deployed. Defaults to REST APIs, which is what the
   * console's API Deploy page shows.
   */
  kind?: ArtifactKind;
  /**
   * The artifact's handle, for hosts whose Port does not carry one — the AI
   * Workspace reads it off the route and passes it in. Falls back to the Port's
   * `apiHandle`.
   */
  artifactHandle?: string;
};

/** How often to re-read while a deployment is still settling. */
const POLL_INTERVAL_MS = 4000;

const errorMessage = (error: unknown, fallback: string) =>
  error instanceof Error ? error.message : fallback;

/**
 * The extension's `render(port)` result: the API's deployment pipeline, backed by
 * the platform API through the host-injected `apiFetch`.
 *
 * This component owns all data and actions; `DeployPage` only lays the stages
 * out. The server owns the rules — which environments exist, in what order, which
 * gateways they have, and whether a promotion is allowed — so this never
 * assembles a pipeline itself and cannot offer a deployment the server would
 * reject.
 */
const DeployFeature: FC<DeployFeatureProps> = ({ port, kind = 'RestApi', artifactHandle }) => {
  const { apiFetch, projectHandle, apiHandle, notify } = port;
  const handle = artifactHandle ?? apiHandle;

  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [apiEndpointUrl, setApiEndpointUrl] = useState<string | undefined>(undefined);
  // Whether this kind's deployments carry a backend URL at all. A REST API's does;
  // an MCP server's and an LLM proxy's upstream belongs to the artifact, so the form
  // neither asks for one nor sends one.
  const takesEndpoint = takesEndpointUrl(kind);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const client = useMemo(
    () =>
      projectHandle && handle
        ? createDeployClient(apiFetch, projectHandle, handle, kind)
        : null,
    [apiFetch, projectHandle, handle, kind]
  );

  // Kept in a ref so the poll can read the latest state without restarting on
  // every refresh.
  const settlingRef = useRef(false);
  settlingRef.current = isSettling(environments);

  const load = useCallback(
    async (options: { quiet?: boolean } = {}) => {
      if (!client) return;
      if (!options.quiet) setLoading(true);
      try {
        const [stages, prepared] = await Promise.all([
          client.listEnvironments(),
          client.listBuilds(),
        ]);
        setEnvironments(stages);
        setBuilds(prepared);
        setError(null);
      } catch (loadError) {
        setError(errorMessage(loadError, 'Unable to load the deployment pipeline.'));
      } finally {
        if (!options.quiet) setLoading(false);
      }
    },
    [client]
  );

  useEffect(() => {
    void load();
  }, [load]);

  // Read once per API rather than on every settling poll: the API's own backend
  // URL is not pipeline state, and it is only the deploy form's starting value,
  // so failing to read it must leave the rest of the page working.
  useEffect(() => {
    if (!client) return;
    void client.readApiEndpointUrl().then(setApiEndpointUrl, () => setApiEndpointUrl(undefined));
  }, [client]);

  // A deployment settles asynchronously once its gateway acknowledges, so poll
  // while anything is in flight and stop as soon as everything has settled.
  useEffect(() => {
    if (!client) return undefined;
    const timer = setInterval(() => {
      if (settlingRef.current) void load({ quiet: true });
    }, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [client, load]);

  const runAction = useCallback(
    async (action: () => Promise<void>, success: string, failure: string) => {
      if (!client) return;
      setBusy(true);
      try {
        await action();
        notify(success, 'success');
        await load({ quiet: true });
      } catch (actionError) {
        notify(errorMessage(actionError, failure), 'error');
      } finally {
        setBusy(false);
      }
    },
    [client, load, notify]
  );

  /**
   * Deploy and promote are the same call: what separates them is whether a source
   * environment is named. Deploying to the first environment either ships a
   * selected build or creates a new one; promoting carries the source's build
   * forward.
   */
  const handleDeploy = (
    target: Environment,
    gateways: { gatewayId: string; endpointUrl?: string }[],
    from?: Environment,
    buildId?: string
  ) => {
    if (!client || gateways.length === 0) return;
    void runAction(
      () =>
        client.deploy({
          environment: target.name,
          gateways,
          fromEnvironment: from?.name,
          buildId,
        }),
      `${from ? 'Promoting to' : 'Deploying to'} ${target.name}.`,
      `Unable to ${from ? 'promote to' : 'deploy to'} ${target.name}.`
    );
  };

  /**
   * Deleting a build is how a slot is freed once the API is at its limit and
   * deploying is refused for it. The platform is the authority on whether a build
   * can go — a gateway may have claimed it since the page last loaded — so a
   * refusal surfaces as it comes back rather than being predicted here.
   */
  const handleDeleteBuild = (buildId: string) => {
    if (!client) return;
    void runAction(
      () => client.deleteBuild(buildId),
      `Deleted build ${buildId}.`,
      `Unable to delete build ${buildId}.`
    );
  };

  const handleStop = (environment: Environment, gatewayId: string) => {
    const gateway = environment.gateways.find((candidate) => candidate.id === gatewayId);
    if (!client || !gateway?.deploymentId) return;
    void runAction(
      () => client.undeploy(environment.name, gatewayId, gateway.deploymentId!),
      `Stopping ${gateway.name}.`,
      `Unable to stop ${gateway.name}.`
    );
  };

  // `handle`, not the Port's apiHandle: the AI Workspace passes the artifact in
  // rather than carrying it on the Port, and this guard is what decides whether the
  // page can load at all.
  if (!projectHandle || !handle) {
    return (
      <PageContent fullWidth sx={{ minWidth: 0 }}>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Deploy</PageTitle.Header>
        </PageTitle>
        <Typography variant="body2" color="text.secondary">
          Open an API within a project to deploy it.
        </Typography>
      </PageContent>
    );
  }

  if (loading) {
    return (
      <PageContent fullWidth sx={{ minWidth: 0 }}>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Deploy</PageTitle.Header>
        </PageTitle>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth sx={{ minWidth: 0 }}>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Deploy</PageTitle.Header>
        </PageTitle>
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
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {error}
          </Typography>
          <Button variant="outlined" size="small" onClick={() => void load()}>
            Retry
          </Button>
        </Box>
      </PageContent>
    );
  }

  return (
    <DeployPage
      environments={environments}
      builds={builds}
      apiEndpointUrl={apiEndpointUrl}
      takesEndpoint={takesEndpoint}
      busy={busy}
      onDeploy={handleDeploy}
      onStopGateway={handleStop}
      onDeleteBuild={handleDeleteBuild}
    />
  );
};

export default DeployFeature;
