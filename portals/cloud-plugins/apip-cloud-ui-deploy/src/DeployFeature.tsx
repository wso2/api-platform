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
import DeployDialog from './components/DeployDialog';
import { createDeployClient, isSettling } from './deployApi';
import { buildsRunningIn } from './utils/build';
import type { CloudHostPort } from './hostPort';
import type { Build, DeploymentParameter, Environment } from './types';

export type DeployFeatureProps = {
  port: CloudHostPort;
};

/** How often to re-read while a deployment is still settling. */
const POLL_INTERVAL_MS = 4000;

type DialogState = {
  mode: 'deploy' | 'promote';
  target: Environment;
  from?: Environment;
  /**
   * The builds this dialog may send: the prepared ones when deploying, or the ones
   * the source environment is running when promoting — the same set the API will
   * accept.
   */
  buildOptions: string[];
} | null;

const errorMessage = (error: unknown, fallback: string) =>
  error instanceof Error ? error.message : fallback;

/**
 * The extension's `render(port)` result: the API's deployment pipeline, backed by
 * the platform API through the host-injected `apiFetch`.
 *
 * This component owns all data and actions; `DeployPage` only lays the stages out.
 * The server owns the rules — which environments exist, in what order, which
 * gateways they have, and whether a promotion is allowed — so this never assembles
 * a pipeline itself and cannot offer a deployment the server would reject.
 */
const DeployFeature: FC<DeployFeatureProps> = ({ port }) => {
  const { apiFetch, projectHandle, apiHandle, notify } = port;

  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<DialogState>(null);
  const [parameters, setParameters] = useState<DeploymentParameter[] | null>(null);

  const client = useMemo(
    () =>
      projectHandle && apiHandle
        ? createDeployClient(apiFetch, projectHandle, apiHandle)
        : null,
    [apiFetch, projectHandle, apiHandle]
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

  // A deployment settles asynchronously once its gateway acknowledges, so poll
  // while anything is in flight and stop as soon as everything has settled.
  useEffect(() => {
    if (!client) return undefined;
    const timer = setInterval(() => {
      if (settlingRef.current) void load({ quiet: true });
    }, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [client, load]);

  // The settings shown are the ones the chosen gateway is currently deployed with,
  // so they are read when the dialog opens and again whenever the gateway changes.
  const loadParameters = useCallback(
    async (gatewayId: string) => {
      if (!client) return;
      setParameters(null);
      try {
        setParameters(await client.getParameters(gatewayId));
      } catch (parameterError) {
        // The settings are a prefill, not a precondition: report the failure and
        // let the deployment proceed with what the form has.
        notify(errorMessage(parameterError, 'Unable to load the gateway settings.'), 'warning');
        setParameters([]);
      }
    },
    [client, notify]
  );

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

  const handleConfirm = (
    gatewayId: string,
    buildId: string,
    values: Record<string, string>
  ) => {
    if (!dialog || !client) return;
    const { mode, target, from } = dialog;
    setDialog(null);
    void runAction(
      () =>
        client.deploy({
          environment: target.name,
          gatewayId,
          // The build is named, so this cannot drift to a newer one prepared
          // between opening the dialog and confirming it. On a promotion the
          // server checks it against what the source is running anyway.
          buildId: buildId || undefined,
          fromEnvironment: mode === 'promote' ? from?.name : undefined,
          parameters: values,
        }),
      `${mode === 'promote' ? 'Promoting to' : 'Deploying to'} ${target.name}.`,
      `Unable to ${mode === 'promote' ? 'promote to' : 'deploy to'} ${target.name}.`
    );
  };

  const handlePrepare = () => {
    if (!client) return;
    void runAction(
      async () => {
        await client.prepare();
      },
      'Build prepared. Deploy it to an environment when you are ready.',
      'Unable to prepare a build.'
    );
  };

  const openDialog = useCallback(
    (state: NonNullable<DialogState>) => {
      setDialog(state);
      setParameters(null);
    },
    []
  );

  const handleStop = (environment: Environment, gatewayId: string) => {
    const gateway = environment.gateways.find((candidate) => candidate.id === gatewayId);
    if (!client || !gateway?.deploymentId) return;
    void runAction(
      () => client.undeploy(environment.name, gatewayId, gateway.deploymentId!),
      `Stopping ${gateway.name}.`,
      `Unable to stop ${gateway.name}.`
    );
  };

  // Redeploying sends a gateway the build it is already running, so it comes back
  // on the same artifact rather than picking up edits or a newer build.
  const handleRedeploy = (environment: Environment, gatewayId: string) => {
    const gateway = environment.gateways.find((candidate) => candidate.id === gatewayId);
    if (!client || !gateway) return;
    const index = environments.findIndex((candidate) => candidate.name === environment.name);
    const previous = index > 0 ? environments[index - 1] : undefined;
    void runAction(
      () =>
        client.deploy({
          environment: environment.name,
          gatewayId,
          buildId: gateway.buildId,
          // A later stage can only be deployed to by promoting into it.
          fromEnvironment: index > 0 ? previous?.name : undefined,
        }),
      `Redeploying ${gateway.name}.`,
      `Unable to redeploy ${gateway.name}.`
    );
  };

  if (!projectHandle || !apiHandle) {
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

  return (
    <PageContent fullWidth sx={{ minWidth: 0 }}>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Deploy</PageTitle.Header>
        <PageTitle.SubHeader>
          Deploy this API through your project&apos;s pipeline. Each environment is promoted into
          from the one before it, so what runs in a later environment is what was tested earlier.
        </PageTitle.SubHeader>
      </PageTitle>

      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      ) : error ? (
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
      ) : (
        <DeployPage
          environments={environments}
          builds={builds}
          busy={busy}
          onPrepare={handlePrepare}
          onDeploy={(environment) =>
            openDialog({
              mode: 'deploy',
              target: environment,
              // Newest last, so the dialog can default to it.
              buildOptions: builds.map((build) => build.buildId).reverse(),
            })
          }
          onPromote={(target, from) =>
            openDialog({ mode: 'promote', target, from, buildOptions: buildsRunningIn(from) })
          }
          onEditSettings={(environment) =>
            openDialog({
              mode: 'deploy',
              target: environment,
              buildOptions: builds.map((build) => build.buildId).reverse(),
            })
          }
          onStopGateway={handleStop}
          onRedeployGateway={handleRedeploy}
        />
      )}

      <DeployDialog
        open={dialog !== null}
        mode={dialog?.mode ?? 'deploy'}
        environment={dialog?.target ?? null}
        fromEnvironment={dialog?.from?.name}
        buildOptions={dialog?.buildOptions ?? []}
        parameters={parameters}
        submitting={busy}
        onClose={() => setDialog(null)}
        onGatewayChange={(gatewayId) => void loadParameters(gatewayId)}
        onConfirm={handleConfirm}
      />
    </PageContent>
  );
};

export default DeployFeature;
