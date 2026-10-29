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
import { Box, CircularProgress } from '@wso2/oxygen-ui';
import GatewayForm from './GatewayForm';
import GatewayOverview from './components/GatewayOverview';
import GatewaysList from './GatewaysList';
import UnexpectedErrorState from './components/UnexpectedErrorState';
import { createGatewaysClient } from './gatewaysApi';
import { CONTROL_PLANE_HOST_PLACEHOLDER } from './components/GatewaySetupSteps';
import { createSelfHostedGatewaysClient } from './selfHostedGatewaysApi';
import type { AIWorkspaceHostPort } from './hostPort';
import type {
  Environment,
  Gateway,
  GatewayHosting,
  GatewayInput,
  GatewayType,
  SelfHostedGatewayInput,
} from './types';

export type GatewaysFeatureProps = {
  port: AIWorkspaceHostPort;
  /**
   * The gateway types this host lets a user create, in the order they are
   * offered. Each host declares its own set — the API console offers API and
   * event gateways, while the AI workspace only ever creates AI gateways and so
   * shows no type picker at all. Required rather than defaulted so a new host
   * has to state which kinds of gateway it is for.
   */
  gatewayTypes: GatewayType[];
};

/**
 * The extension's `render(port)` result: a self-contained list/create/edit flow
 * that switches view with local state rather than nested routes (so the plugin
 * never depends on the host's router instance — the same shape as the pipelines
 * feature). It owns the data: it loads `/managed-gateways`, platform-api's
 * native `/gateways` (self-hosted — see `selfHostedGatewaysApi`) and
 * `/environments` through the host-injected `apiFetch` and feeds the
 * presentational list/form.
 */
/**
 * How often the list re-reads gateway status while a gateway is still coming up.
 * A newly created gateway is inactive until its data-plane gateway finishes
 * provisioning and its controller dials in, which takes tens of seconds — long
 * enough that without this the user has to reload the page to see it go active.
 */
const STATUS_POLL_INTERVAL_MS = 5000;

const GatewaysFeature: FC<GatewaysFeatureProps> = ({ port, gatewayTypes }) => {
  const { apiFetch, notify, resourceLimits } = port;
  const client = useMemo(() => createGatewaysClient(apiFetch), [apiFetch]);
  const selfHostedClient = useMemo(() => createSelfHostedGatewaysClient(apiFetch), [apiFetch]);

  // The hosting the empty state's button asked for; null when Add Gateway was used.
  const [initialHosting, setInitialHosting] = useState<GatewayHosting | null>(null);
  const [view, setView] = useState<'list' | 'create' | 'edit' | 'view'>('list');
  const [editingGatewayId, setEditingGatewayId] = useState<string | null>(null);
  const [viewingGatewayId, setViewingGatewayId] = useState<string | null>(null);
  // The token minted with a just-registered gateway, handed to its setup view; null otherwise.
  const [setupToken, setSetupToken] = useState<string | null>(null);
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Guards against a save being issued twice (the form also disables its button).
  const submittingRef = useRef(false);
  const deletingRef = useRef(false);
  // Refreshes are fired from several places (mount, after save, after each
  // delete) and can overlap. Only the newest one may write state, or a slower
  // earlier response could land last and restore a gateway that was deleted
  // since.
  const loadSeqRef = useRef(0);

  // `silent` is for the status poll below: it must not show the spinner or
  // replace the list with an error page, so it only ever writes fresh data.
  const load = useCallback(
    async ({ silent = false }: { silent?: boolean } = {}) => {
      const seq = ++loadSeqRef.current;
      if (!silent) {
        setLoading(true);
        setError(null);
      }
      try {
        // The two kinds of gateway come from different resources, so one failing must not hide
        // the other: each list is read on its own and only a failure of both is an error.
        const [managedResult, selfHostedResult, environmentResult] = await Promise.allSettled([
          client.listGateways(gatewayTypes),
          selfHostedClient.listGateways(gatewayTypes),
          client.listEnvironments(),
        ]);
        if (seq !== loadSeqRef.current) return;
        if (environmentResult.status === 'rejected') throw environmentResult.reason;
        if (managedResult.status === 'rejected' && selfHostedResult.status === 'rejected') {
          throw managedResult.reason;
        }
        // A list that failed keeps what was last loaded for its kind, rather than emptying it.
        setGateways((previous) => [
          ...(managedResult.status === 'fulfilled'
            ? managedResult.value
            : previous.filter((gateway) => gateway.hosting === 'managed')),
          ...(selfHostedResult.status === 'fulfilled'
            ? selfHostedResult.value
            : previous.filter((gateway) => gateway.hosting === 'self-hosted')),
        ]);
        setEnvironments(environmentResult.value);
        if (!silent && (managedResult.status === 'rejected' || selfHostedResult.status === 'rejected')) {
          notify(
            `${managedResult.status === 'rejected' ? 'WSO2 managed' : 'Self-hosted'} gateways could not be loaded.`,
            'warning'
          );
        }
        // Any load that reaches fresh data clears a previous failure.
        setError(null);
      } catch (loadError) {
        if (seq !== loadSeqRef.current) return;
        // A failed poll is left silent: the list already on screen stays, and
        // the next tick may well succeed. Only a foreground load reports.
        if (!silent) {
          setError(loadError instanceof Error ? loadError.message : 'Unable to load gateways.');
        }
      } finally {
        if (!silent && seq === loadSeqRef.current) setLoading(false);
      }
    },
    [client, selfHostedClient, gatewayTypes, notify]
  );

  useEffect(() => {
    void load();
  }, [load]);

  // A host only ever manages its own kinds of gateway: the AI workspace shows AI
  // gateways and their default, the publisher shows regular and event ones and
  // theirs. Filtering here rather than in the list keeps the count, the status
  // poll and the default badge consistent with what is on screen — the default is
  // marked per type, so an AI default is meaningless in the publisher's view.
  const visibleGateways = useMemo(
    () => gateways.filter((gateway) => gatewayTypes.includes(gateway.type)),
    [gateways, gatewayTypes]
  );

  // Poll only while there is something to wait for: a gateway that is not yet
  // active, or one still being deleted. Once they are all active the interval is
  // torn down, so a settled list costs nothing. Creating another gateway makes
  // this true again and the poll restarts. Deliberately keyed on the boolean, not
  // on `gateways`, so a poll's own result does not reset the interval.
  const awaitingStatus = visibleGateways.some((gateway) => gateway.status !== 'active');

  useEffect(() => {
    if (view !== 'list' || !awaitingStatus) return undefined;
    const timer = window.setInterval(() => {
      // Skip a tick mid-write: the create/delete paths refresh on their own, and
      // the in-flight request would race this one for the newest sequence.
      if (submittingRef.current || deletingRef.current) return;
      void load({ silent: true });
    }, STATUS_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [view, awaitingStatus, load]);

  const submitGateway = useCallback(
    async (input: GatewayInput, gatewayId?: string): Promise<boolean> => {
      if (submittingRef.current) return false;
      submittingRef.current = true;
      try {
        if (gatewayId) {
          await client.updateGateway(gatewayId, input);
          notify(`Gateway "${input.name}" updated.`, 'success');
        } else {
          await client.createGateway(input);
          notify(`Gateway "${input.name}" created.`, 'success');
          // A create consumes quota; an edit does not.
          resourceLimits?.refresh();
        }
        await load();
        return true;
      } catch (submitError) {
        notify(
          submitError instanceof Error ? submitError.message : 'Unable to save the gateway.',
          'error'
        );
        return false;
      } finally {
        submittingRef.current = false;
      }
    },
    [client, load, notify, resourceLimits]
  );

  /**
   * The self-hosted path, mirroring `submitGateway` but against
   * `selfHostedClient` (platform-api's native `/gateways`, not
   * `/managed-gateways`). A freshly created gateway opens on its setup steps with the
   * registration token minted for it; generating another token there revokes that one.
   */
  const submitSelfHostedGateway = useCallback(
    async (input: SelfHostedGatewayInput, gatewayId?: string): Promise<'saved' | 'opened' | 'failed'> => {
      if (submittingRef.current) return 'failed';
      submittingRef.current = true;
      try {
        if (gatewayId) {
          await selfHostedClient.updateGateway(gatewayId, input);
          notify(`Gateway "${input.name}" updated.`, 'success');
          await load();
          return 'saved';
        }
        const { gateway: created, token } = await selfHostedClient.createGateway(input);
        notify(`Self-hosted gateway "${input.name}" registered.`, 'success');
        await load();
        // Open the new gateway's setup steps with the token minted for it, so the
        // administrator can connect it straight away instead of generating another one.
        setViewingGatewayId(created.id);
        setSetupToken(token);
        setView('view');
        setEditingGatewayId(null);
        resourceLimits?.refresh();
        return 'opened';
      } catch (submitError) {
        notify(
          submitError instanceof Error ? submitError.message : 'Unable to save the gateway.',
          'error'
        );
        return 'failed';
      } finally {
        submittingRef.current = false;
      }
    },
    [selfHostedClient, load, notify, resourceLimits]
  );

  const removeGateway = useCallback(
    async (id: string, name: string) => {
      // One delete at a time: the confirm dialog closes on confirm, so a second
      // gateway can otherwise be deleted while the first is still in flight.
      if (deletingRef.current) return;
      deletingRef.current = true;
      try {
        // A gateway torn down in the background stays listed as deleting, and the
        // status poll drops it once it is gone.
        const target = gateways.find((candidate) => candidate.id === id);
        let deletionPending = false;
        if (target?.hosting === 'self-hosted') {
          await selfHostedClient.deleteGateway(id);
          notify(`Gateway "${name}" deleted.`, 'success');
        } else {
          const { pending } = await client.deleteGateway(id);
          deletionPending = pending;
          if (pending) {
            notify(`Gateway "${name}" is being deleted.`, 'info');
          } else {
            notify(`Gateway "${name}" deleted.`, 'success');
          }
        }
        // Drop the row now: if the reload of its kind fails, that kind keeps what was last loaded,
        // and the deleted gateway would otherwise stay listed.
        if (!deletionPending) setGateways((previous) => previous.filter((gateway) => gateway.id !== id));
        await load();
        // Frees quota. Fired when pending too — teardown is already under way.
        resourceLimits?.refresh();
      } catch (deleteError) {
        notify(
          deleteError instanceof Error ? deleteError.message : 'Unable to delete the gateway.',
          'error'
        );
      } finally {
        deletingRef.current = false;
      }
    },
    [client, selfHostedClient, gateways, load, notify, resourceLimits]
  );

  if (loading) {
    return (
      <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', py: 8, width: '100%' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <UnexpectedErrorState
        description="We couldn't load your gateways. This is usually temporary - try again in a moment."
        detail={error}
        onRetry={() => void load()}
      />
    );
  }

  if (view === 'create' || view === 'edit') {
    const editingGateway =
      view === 'edit' ? visibleGateways.find((gateway) => gateway.id === editingGatewayId) : undefined;
    return (
      <GatewayForm
        mode={view}
        gateway={editingGateway}
        types={gatewayTypes}
        gateways={visibleGateways}
        environments={environments}
        gatewayVersions={port.gatewayVersions}
        initialHosting={initialHosting}
        onBack={() => {
          setView('list');
          setEditingGatewayId(null);
        }}
        onSubmit={async (input) => {
          const saved = await submitGateway(input, view === 'edit' ? editingGatewayId ?? undefined : undefined);
          if (saved) {
            setView('list');
            setEditingGatewayId(null);
          }
        }}
        onSubmitSelfHosted={async (input) => {
          const saved = await submitSelfHostedGateway(
            input,
            view === 'edit' ? editingGatewayId ?? undefined : undefined
          );
          if (saved === 'saved') {
            setView('list');
            setEditingGatewayId(null);
          }
        }}
      />
    );
  }

  // A viewed gateway that is gone (deleted elsewhere, or a stale link) falls
  // through to the list below rather than rendering an overview for nothing.
  const viewedGateway =
    view === 'view' ? visibleGateways.find((gateway) => gateway.id === viewingGatewayId) : undefined;
  if (viewedGateway) {
    const environmentNames = new Map(environments.map((environment) => [environment.id, environment.name]));
    return (
      <GatewayOverview
        key={viewedGateway.id}
        gateway={viewedGateway}
        initialRegistrationToken={setupToken}
        environmentNames={environmentNames}
        controlPlaneHost={port.controlPlaneHost ?? CONTROL_PLANE_HOST_PLACEHOLDER}
        gatewayVersions={port.gatewayVersions}
        onBack={() => {
          setView('list');
          setViewingGatewayId(null);
          setSetupToken(null);
        }}
        onEditClick={(gatewayId) => {
          setEditingGatewayId(gatewayId);
          setView('edit');
        }}
        onGenerateRegistrationToken={selfHostedClient.generateToken}
        notify={notify}
      />
    );
  }

  return (
    <GatewaysList
      gateways={visibleGateways}
      environments={environments}
      port={port}
      onAddClick={(hosting) => {
        setInitialHosting(hosting ?? null);
        setView('create');
      }}
      onEditClick={(gatewayId) => {
        setEditingGatewayId(gatewayId);
        setView('edit');
      }}
      onViewClick={(gatewayId) => {
        // The token belongs to the gateway just registered, not to whichever one is opened next.
        setSetupToken(null);
        setViewingGatewayId(gatewayId);
        setView('view');
      }}
      onDelete={removeGateway}
    />
  );
};

export default GatewaysFeature;
