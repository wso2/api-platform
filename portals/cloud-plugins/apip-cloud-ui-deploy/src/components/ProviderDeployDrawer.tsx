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

import { useEffect, useState, type FC } from 'react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Drawer,
  FormControl,
  FormControlLabel,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import type { ProviderUpstream } from '../providerDeployApi';
import type { Build, Environment, Gateway } from '../types';

export type ProviderDeployDrawerProps = {
  open: boolean;
  environment: Environment | null;
  /** The provider's builds, newest first. */
  builds: Build[];
  /** What the provider itself uses, which each gateway's fields start from. */
  upstream: ProviderUpstream;
  submitting: boolean;
  onClose: () => void;
  /**
   * Confirms the deploy with every selected gateway; they go in one call. A gateway's
   * `apiKey` is the key as typed — it is exchanged for a stored secret before the
   * deploy, so nothing here holds it beyond this call. `buildId` is empty when the
   * provider is to be shipped as it stands.
   */
  onConfirm: (
    gateways: {
      gatewayId: string;
      apiKey?: string;
      authHeader?: string;
      endpointUrl?: string;
    }[],
    buildId?: string
  ) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

const pickDefaultGateway = (gateways: Gateway[]): Gateway | null =>
  gateways.find((gateway) => gateway.isDefault) ??
  gateways.find((gateway) => gateway.health === 'active') ??
  gateways[0] ??
  null;

/**
 * Choosing which gateways of one environment to deploy the provider to, and what each of
 * them talks to.
 *
 * A drawer rather than a modal: this is the portal's shape for configuring a deployment,
 * and the per-gateway fields need room to breathe rather than a dialog that grows a
 * scrollbar as soon as two gateways are picked.
 *
 * There is no build to pick: a provider is deployed as it stands, so the choice is
 * only where it goes. Everything else follows the same rules a pipeline deploy
 * does, because the backend is the same — the environment is deployed as a set,
 * and a gateway the provider is already live on has to stay in it.
 */
const ProviderDeployDrawer: FC<ProviderDeployDrawerProps> = ({
  open,
  environment,
  builds,
  upstream,
  submitting,
  onClose,
  onConfirm,
}) => {
  // Only an explicit choice is held: null means "whatever this environment says",
  // resolved below. Holding the resolved list instead would leave the first render
  // of a freshly opened dialog with nothing selected, and would let a background
  // refresh overwrite what the user had ticked.
  const [selectedIds, setSelectedIds] = useState<string[] | null>(null);
  // Keys as typed, per gateway. Deliberately not seeded from what a gateway is
  // running: a deployment's credential is write-only, exactly as the provider's own
  // is, so there is nothing to read back and an empty field means "leave it as it is"
  // rather than "clear it".
  const [keyDrafts, setKeyDrafts] = useState<Record<string, string>>({});
  const [headerDrafts, setHeaderDrafts] = useState<Record<string, string>>({});
  const [endpointDrafts, setEndpointDrafts] = useState<Record<string, string>>({});
  // Empty means "as it stands now", which is what a deploy does when it names no
  // build: the platform snapshots the definition and deploys that snapshot.
  const [buildId, setBuildId] = useState('');

  useEffect(() => {
    if (!open) return;
    setSelectedIds(null);
    setKeyDrafts({});
    setHeaderDrafts({});
    setEndpointDrafts({});
    setBuildId('');
  }, [open]);

  if (!environment) return null;

  // A gateway the provider is LIVE on must stay in the set: the environment is
  // deployed together, so this deploy has to reach it too. Keyed on status rather
  // than on deploymentId — a stopped gateway keeps its id, and dropping it from
  // the set is exactly what stopping it is for.
  const liveStatuses = ['DEPLOYED', 'DEPLOYING', 'FAILED'];
  const alreadyDeployed = environment.gateways.filter((gateway) =>
    liveStatuses.includes(gateway.status)
  );
  const lockedIds = alreadyDeployed.map((gateway) => gateway.id);

  const defaultSelection =
    lockedIds.length > 0
      ? lockedIds
      : [pickDefaultGateway(environment.gateways)?.id].filter((id): id is string => !!id);
  const selection = selectedIds ?? defaultSelection;
  const selected = environment.gateways.filter((gateway) => selection.includes(gateway.id));

  // Whether a gateway can receive a deployment is its own health, not the state of
  // what is on it: a healthy gateway with nothing deployed is exactly what a first
  // deployment targets.
  const inactiveSelected = selected.filter((gateway) => gateway.health !== 'active');
  const inactiveLocked = inactiveSelected.filter((gateway) => lockedIds.includes(gateway.id));
  const inactiveSelectable = inactiveSelected.filter(
    (gateway) => !lockedIds.includes(gateway.id)
  );
  const canConfirm = selected.length > 0 && inactiveSelected.length === 0;

  // Each field opens on what the gateway is running, falling back to what the provider
  // itself uses. A credential is the exception: it is write-only, so it can only ever
  // start empty and an empty field means "keep what is there".
  const endpointFor = (gatewayId: string) =>
    endpointDrafts[gatewayId] ??
    environment.gateways.find((gateway) => gateway.id === gatewayId)?.endpointUrl ??
    upstream.url ??
    '';
  const headerFor = (gatewayId: string) => headerDrafts[gatewayId] ?? upstream.authHeader ?? '';
  // Only an api-key upstream has a credential this form can name: basic and bearer send
  // Authorization by definition, and none/other carry no credential at all.
  const takesCredential = upstream.authType === 'api-key';

  const toggleGateway = (id: string) => {
    if (lockedIds.includes(id)) return;
    setSelectedIds(
      selection.includes(id) ? selection.filter((each) => each !== id) : [...selection, id]
    );
  };

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      sx={{ '& .MuiDrawer-paper': { width: { xs: '100%', sm: 560 }, maxWidth: '100%' } }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          p: 2,
          borderBottom: 1,
          borderColor: 'divider',
        }}
      >
        <Typography variant="h6">Deploy to {environment.name}</Typography>
        <IconButton onClick={onClose} disabled={submitting} size="small" aria-label="Close">
          <X size={20} />
        </IconButton>
      </Box>

      <Box sx={{ p: 3, overflowY: 'auto', flex: 1 }}>
        {inactiveSelectable.length > 0 ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {inactiveSelectable.map((gateway) => gateway.name).join(', ')}
            {inactiveSelectable.length === 1 ? ' is inactive and ' : ' are inactive and '}
            can't receive a deployment. Unselect
            {inactiveSelectable.length === 1 ? ' it' : ' them'} to continue.
          </Alert>
        ) : null}

        {inactiveLocked.length > 0 ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {inactiveLocked.map((gateway) => gateway.name).join(', ')}
            {inactiveLocked.length === 1
              ? ' is inactive and already has this provider deployed on it, so it cannot be left out of this deployment. Activate it, or stop its deployment in '
              : ' are inactive and already have this provider deployed on them, so they cannot be left out of this deployment. Activate them, or stop their deployments in '}
            {environment.name}, to continue.
          </Alert>
        ) : null}

        {/* What gets deployed. Shipping the provider as it stands is the default and
            needs no separate step: the platform snapshots the definition as part of
            the deploy. Naming an existing build instead is how a gateway is put back
            onto exactly what another one is already running. */}
        <Box sx={{ mb: 2.5 }}>
          <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Build</FormLabel>
          <FormControl fullWidth size="small">
            <Select
              value={buildId}
              onChange={(event) => setBuildId(event.target.value as string)}
              displayEmpty
            >
              <MenuItem value="">Deploy the provider as it stands now</MenuItem>
              {builds.map((build) => (
                <MenuItem key={build.buildId} value={build.buildId}>
                  {build.buildId}
                  {build.description ? ` · ${build.description}` : ''}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        <Box sx={{ mb: 1 }}>
          <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateways</FormLabel>
          {lockedIds.length > 0 ? (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
              Gateways already deployed on stay selected. Stop one to drop it.
            </Typography>
          ) : null}
          <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            {environment.gateways.map((gateway, index) => {
              const isSelected = selection.includes(gateway.id);
              const isLocked = lockedIds.includes(gateway.id);
              return (
                <Box
                  key={gateway.id}
                  sx={{
                    px: 1.5,
                    py: 1,
                    borderTop: index === 0 ? 'none' : '1px solid',
                    borderColor: 'divider',
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
                    <Tooltip
                      title={
                        isLocked
                          ? 'Already deployed here. Stop it first to drop it from this deployment.'
                          : ''
                      }
                    >
                      <FormControlLabel
                        sx={{ mr: 0 }}
                        control={
                          <Checkbox
                            size="small"
                            checked={isSelected}
                            disabled={isLocked}
                            onChange={() => toggleGateway(gateway.id)}
                          />
                        }
                        label={
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <StatusDot tone={gateway.health === 'active' ? 'success' : 'default'} />
                            <Typography variant="body2" sx={{ fontWeight: 500 }}>
                              {gateway.name}
                              {gateway.isDefault ? ' · Default' : ''}
                            </Typography>
                          </Box>
                        }
                      />
                    </Tooltip>
                    <StatusPill tone={gatewayStatusTone(gateway.status)} />
                  </Box>
                  {/* Labelled the way the provider's own connection settings are: the
                      label above the field rather than floating in its border, so a
                      filled field and an empty one read the same. Same order, too. */}
                  {isSelected ? (
                    <Stack spacing={1.5} sx={{ mt: 1.5 }}>
                      <FormControl fullWidth>
                        <FormLabel>Provider Endpoint</FormLabel>
                        <TextField
                          size="small"
                          placeholder="https://api.openai.com/v1"
                          value={endpointFor(gateway.id)}
                          onChange={(event) =>
                            setEndpointDrafts({
                              ...endpointDrafts,
                              [gateway.id]: event.target.value,
                            })
                          }
                        />
                      </FormControl>

                      {takesCredential ? (
                        <FormControl fullWidth>
                          <FormLabel>Authentication Header</FormLabel>
                          <TextField
                            size="small"
                            placeholder="Authorization"
                            value={headerFor(gateway.id)}
                            onChange={(event) =>
                              setHeaderDrafts({
                                ...headerDrafts,
                                [gateway.id]: event.target.value,
                              })
                            }
                          />
                        </FormControl>
                      ) : null}

                      {takesCredential ? (
                        <FormControl fullWidth>
                          <FormLabel>Credentials</FormLabel>
                          <TextField
                            size="small"
                            type="password"
                            autoComplete="off"
                            placeholder="Leave empty to keep the current key"
                            value={keyDrafts[gateway.id] ?? ''}
                            onChange={(event) =>
                              setKeyDrafts({ ...keyDrafts, [gateway.id]: event.target.value })
                            }
                          />
                        </FormControl>
                      ) : null}
                    </Stack>
                  ) : null}
                </Box>
              );
            })}
          </Box>
        </Box>
      </Box>

      <Box
        sx={{
          display: 'flex',
          gap: 2,
          p: 2,
          borderTop: 1,
          borderColor: 'divider',
        }}
      >
        <Button fullWidth color="secondary" variant="outlined" onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          fullWidth
          variant="contained"
          disabled={!canConfirm || submitting}
          startIcon={submitting ? <CircularProgress size={16} /> : null}
          onClick={() =>
            onConfirm(
              selected.map((gateway) => ({
                gatewayId: gateway.id,
                apiKey: keyDrafts[gateway.id]?.trim() || undefined,
                authHeader: headerDrafts[gateway.id]?.trim() || undefined,
                endpointUrl: endpointFor(gateway.id).trim() || undefined,
              })),
              buildId || undefined
            )
          }
        >
          {submitting ? 'Deploying...' : 'Deploy'}
        </Button>
      </Box>
    </Drawer>
  );
};

export default ProviderDeployDrawer;
