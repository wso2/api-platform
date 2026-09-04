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
import type { Environment, Gateway, GatewayInput } from './types';

export type GatewaysFeatureProps = {
  port: AIWorkspaceHostPort;
};

/**
 * The extension's `render(port)` result: a self-contained list/create/edit flow
 * that switches view with local state rather than nested routes (so the plugin
 * never depends on the host's router instance — the same shape as the pipelines
 * feature). It owns the data: it loads `/managed-gateways` and `/environments`
 * through the host-injected `apiFetch` and feeds the presentational list/form.
 */
const GatewaysFeature: FC<GatewaysFeatureProps> = ({ port }) => {
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

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [gatewayList, environmentList] = await Promise.all([
        client.listGateways(),
        client.listEnvironments(),
      ]);
      setGateways(gatewayList);
      setEnvironments(environmentList);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Unable to load gateways.');
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

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

  const removeGateway = useCallback(
    async (id: string, name: string) => {
      try {
        await client.deleteGateway(id);
        notify(`Gateway "${name}" deleted.`, 'success');
        await load();
      } catch (deleteError) {
        notify(
          deleteError instanceof Error ? deleteError.message : 'Unable to delete the gateway.',
          'error'
        );
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
      view === 'edit' ? gateways.find((gateway) => gateway.id === editingGatewayId) : undefined;
    return (
      <GatewayForm
        mode={view}
        gateway={editingGateway}
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
      gateways={gateways}
      port={port}
      onAddClick={() => setView('create')}
      onEditClick={(gatewayId) => {
        setEditingGatewayId(gatewayId);
        setView('edit');
      }}
      onDelete={removeGateway}
    />
  );
};

export default GatewaysFeature;
