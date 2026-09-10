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
import { Box, Button, CircularProgress, Typography } from '@wso2/oxygen-ui';
import GatewayForm from './GatewayForm';
import GatewaysList from './GatewaysList';
import { createGatewaysClient } from './gatewaysApi';
import type { AIWorkspaceHostPort } from './hostPort';
import type { Environment, Gateway, GatewayInput, GatewayType } from './types';

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
 * feature). It owns the data: it loads `/managed-gateways` and `/environments`
 * through the host-injected `apiFetch` and feeds the presentational list/form.
 */
/**
 * How often the list re-reads gateway status while a gateway is still coming up.
 * A newly created gateway is inactive until its data-plane gateway finishes
 * provisioning and its controller dials in, which takes tens of seconds — long
 * enough that without this the user has to reload the page to see it go active.
 */
const STATUS_POLL_INTERVAL_MS = 5000;

const GatewaysFeature: FC<GatewaysFeatureProps> = ({ port, gatewayTypes }) => {
  const { apiFetch, notify } = port;
  const client = useMemo(() => createGatewaysClient(apiFetch), [apiFetch]);

  const [view, setView] = useState<'list' | 'create' | 'edit'>('list');
  const [editingGatewayId, setEditingGatewayId] = useState<string | null>(null);
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
        const [gatewayList, environmentList] = await Promise.all([
          client.listGateways(),
          client.listEnvironments(),
        ]);
        if (seq !== loadSeqRef.current) return;
        setGateways(gatewayList);
        setEnvironments(environmentList);
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
    [client]
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
  // active. Once they are all active the interval is torn down, so a settled
  // list costs nothing. Creating another gateway makes this true again and the
  // poll restarts. Deliberately keyed on the boolean, not on `gateways`, so a
  // poll's own result does not reset the interval.
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
    [client, load, notify]
  );

  // Handing the default over is an update on the gateway itself, so it goes
  // through the same client as any other edit. The list refreshes afterwards
  // because the previous holder's badge has to clear too, not just this one's.
  const markDefault = useCallback(
    async (id: string, name: string) => {
      if (submittingRef.current) return;
      const gateway = visibleGateways.find((candidate) => candidate.id === id);
      if (!gateway) return;
      submittingRef.current = true;
      try {
        await client.markGatewayDefault(gateway);
        notify(`"${name}" is now the default gateway for its environment.`, 'success');
        await load();
      } catch (markError) {
        notify(
          markError instanceof Error ? markError.message : 'Unable to mark the gateway as default.',
          'error'
        );
      } finally {
        submittingRef.current = false;
      }
    },
    [client, visibleGateways, load, notify]
  );

  const removeGateway = useCallback(
    async (id: string, name: string) => {
      // One delete at a time: the confirm dialog closes on confirm, so a second
      // gateway can otherwise be deleted while the first is still in flight.
      if (deletingRef.current) return;
      deletingRef.current = true;
      try {
        await client.deleteGateway(id);
        notify(`Gateway "${name}" deleted.`, 'success');
        await load();
      } catch (deleteError) {
        notify(
          deleteError instanceof Error ? deleteError.message : 'Unable to delete the gateway.',
          'error'
        );
      } finally {
        deletingRef.current = false;
      }
    },
    [client, load, notify]
  );

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
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
        environments={environments}
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
      />
    );
  }

  return (
    <GatewaysList
      gateways={visibleGateways}
      environments={environments}
      onAddClick={() => setView('create')}
      onEditClick={(gatewayId) => {
        setEditingGatewayId(gatewayId);
        setView('edit');
      }}
      onDelete={removeGateway}
      onMarkDefault={markDefault}
    />
  );
};

export default GatewaysFeature;
