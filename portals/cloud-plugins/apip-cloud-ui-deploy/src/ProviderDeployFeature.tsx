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
import ProviderDeployPage from './ProviderDeployPage';
import { createProviderDeployClient, type ProviderUpstream } from './providerDeployApi';
import { isSettling } from './utils/status';
import type { CloudHostPort } from './hostPort';
import type { Build, Environment } from './types';

export type ProviderDeployFeatureProps = {
  port: CloudHostPort;
  /**
   * The provider's handle. Read off the route by the host rather than carried on
   * the Port, which is scoped to a project — and a provider has none.
   */
  artifactHandle?: string;
};

/** How often to re-read while a deployment is still settling. */
const POLL_INTERVAL_MS = 4000;

const errorMessage = (error: unknown, fallback: string) =>
  error instanceof Error ? error.message : fallback;

/**
 * Deploying an LLM provider across the organization's environments.
 *
 * The counterpart to DeployFeature for an artifact that has no project: a provider
 * belongs to the organization, so there is no deployment pipeline behind it, no
 * promotion between its environments and no build to choose — a deploy ships the
 * provider as it stands. What it shares with the pipeline page is everything that
 * matters per gateway, because the server owns those rules: which environments
 * exist, which of their gateways can run a provider, and which one an environment
 * deploys to by default.
 */
const ProviderDeployFeature: FC<ProviderDeployFeatureProps> = ({ port, artifactHandle }) => {
  const { apiFetch, apiHandle, notify } = port;
  const handle = artifactHandle ?? apiHandle;

  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  // What the provider itself uses, which the deploy form starts from.
  const [upstream, setUpstream] = useState<ProviderUpstream>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const client = useMemo(
    () => (handle ? createProviderDeployClient(apiFetch, handle) : null),
    [apiFetch, handle]
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
        setError(errorMessage(loadError, 'Unable to load the environments.'));
      } finally {
        if (!options.quiet) setLoading(false);
      }
    },
    [client]
  );

  useEffect(() => {
    void load();
  }, [load]);

  // Read once per provider rather than on every settling poll: how it authenticates is
  // not deployment state, and failing to read it must leave the rest of the page working.
  useEffect(() => {
    if (!client) return;
    void client.readUpstream().then(setUpstream, () => setUpstream({}));
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
        // Re-read after a failure too. A refusal is often the page acting on something
        // that has since moved — a deployment already stopped, a build already gone — and
        // leaving the stale view up invites the same click again.
        await load({ quiet: true });
      } finally {
        setBusy(false);
      }
    },
    [client, load, notify]
  );

  /**
   * Any key typed in the dialog is exchanged for a stored secret first, and only the
   * reference travels on to the deploy — the same exchange the provider's own API key
   * goes through when it is set. A gateway given no key is deployed without one, which
   * leaves it on whatever credential it is already using.
   *
   * The exchange happens per gateway before any deploy is sent, so a key that cannot be
   * stored fails the whole action rather than deploying some gateways on their new key
   * and the rest on their old one.
   *
   * Every reference this action creates is tracked, and any the deploy did not go on to
   * use is discarded — otherwise a failed deploy would leave the key it stored sitting
   * in the organization's secrets, referenced by nothing and belonging to no one.
   */
  const handleDeploy = (
    target: Environment,
    gateways: { gatewayId: string; apiKey?: string; authHeader?: string; endpointUrl?: string }[],
    buildId?: string
  ) => {
    if (!client || gateways.length === 0) return;
    void runAction(
      async () => {
        const stored: string[] = [];
        try {
          const targets = await Promise.all(
            gateways.map(async (gateway) => {
              if (!gateway.apiKey) {
                return { gatewayId: gateway.gatewayId, endpointUrl: gateway.endpointUrl };
              }
              const name =
                target.gateways.find((candidate) => candidate.id === gateway.gatewayId)?.name ??
                gateway.gatewayId;
              const reference = await client.storeCredential(
                gateway.apiKey,
                `${handle} · ${target.name} · ${name}`
              );
              stored.push(reference);
              return {
                gatewayId: gateway.gatewayId,
                apiKey: reference,
                authHeader: gateway.authHeader,
                endpointUrl: gateway.endpointUrl,
              };
            })
          );
          await client.deploy({ environment: target.name, gateways: targets, buildId });
        } catch (deployError) {
          // Discarding is safe even when the deploy's outcome is unknown: the platform
          // refuses to delete a secret a deployment references, so a key that did reach
          // a gateway survives. The original failure is what the user is told about.
          await Promise.allSettled(stored.map((reference) => client.discardCredential(reference)));
          throw deployError;
        }
      },
      `Deploying to ${target.name}.`,
      `Unable to deploy to ${target.name}.`
    );
  };

  /**
   * Deleting a build frees a slot once the provider is at its build limit and
   * deploying is refused. The platform is the authority on whether a build can go — a
   * gateway may have claimed it since the page last loaded — so a refusal surfaces as
   * it comes back rather than being predicted here.
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

  if (!handle) {
    return (
      <PageContent fullWidth sx={{ minWidth: 0 }}>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Deploy</PageTitle.Header>
        </PageTitle>
        <Typography variant="body2" color="text.secondary">
          Open a provider to deploy it.
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
    <ProviderDeployPage
      environments={environments}
      builds={builds}
      upstream={upstream}
      busy={busy}
      onDeleteBuild={handleDeleteBuild}
      onDeploy={handleDeploy}
      onStopGateway={handleStop}
    />
  );
};

export default ProviderDeployFeature;
