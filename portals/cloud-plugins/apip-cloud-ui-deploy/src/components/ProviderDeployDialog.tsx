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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormLabel,
  MenuItem,
  Select,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import type { Build, Environment, Gateway } from '../types';

export type ProviderDeployDialogProps = {
  open: boolean;
  environment: Environment | null;
  /** The provider's builds, newest first. */
  builds: Build[];
  /** Whether a gateway can name the header its credential is sent in (api-key upstreams). */
  takesAuthHeader: boolean;
  submitting: boolean;
  onClose: () => void;
  /**
   * Confirms the deploy with every selected gateway; they go in one call. A gateway's
   * `apiKey` is the key as typed — it is exchanged for a stored secret before the
   * deploy, so nothing here holds it beyond this call. `buildId` is empty when the
   * provider is to be shipped as it stands.
   */
  onConfirm: (
    gateways: { gatewayId: string; apiKey?: string; authHeader?: string }[],
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
 * Choosing which gateways of one environment to deploy the provider to.
 *
 * There is no build to pick: a provider is deployed as it stands, so the choice is
 * only where it goes. Everything else follows the same rules a pipeline deploy
 * does, because the backend is the same — the environment is deployed as a set,
 * and a gateway the provider is already live on has to stay in it.
 */
const ProviderDeployDialog: FC<ProviderDeployDialogProps> = ({
  open,
  environment,
  builds,
  takesAuthHeader,
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
  // Empty means "as it stands now", which is what a deploy does when it names no
  // build: the platform snapshots the definition and deploys that snapshot.
  const [buildId, setBuildId] = useState('');

  useEffect(() => {
    if (!open) return;
    setSelectedIds(null);
    setKeyDrafts({});
    setHeaderDrafts({});
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

  const toggleGateway = (id: string) => {
    if (lockedIds.includes(id)) return;
    setSelectedIds(
      selection.includes(id) ? selection.filter((each) => each !== id) : [...selection, id]
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>
        Deploy to {environment.name}
      </DialogTitle>
      <DialogContent>
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
                  {build.description ? ` — ${build.description}` : ''}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        <Box sx={{ mb: 1 }}>
          <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateways</FormLabel>
          {lockedIds.length > 0 ? (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
              Gateways already deployed on stay selected — the environment deploys together.
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
                  {/* The credential is per gateway, which is the point of the field:
                      two gateways of one environment can hold different accounts with
                      the same vendor. Left empty, the gateway keeps whatever it is
                      already using — the provider's own key on a first deploy. */}
                  {isSelected ? (
                    <TextField
                      fullWidth
                      size="small"
                      type="password"
                      autoComplete="off"
                      sx={{ mt: 1 }}
                      label="API key"
                      value={keyDrafts[gateway.id] ?? ''}
                      onChange={(event) =>
                        setKeyDrafts({ ...keyDrafts, [gateway.id]: event.target.value })
                      }
                      helperText="Leave empty to keep the current key."
                    />
                  ) : null}
                  {/* The header only matters where a key is being given, and only an
                      api-key upstream has a header to choose — basic and bearer send
                      Authorization by definition. */}
                  {isSelected && takesAuthHeader && keyDrafts[gateway.id]?.trim() ? (
                    <TextField
                      fullWidth
                      size="small"
                      sx={{ mt: 1 }}
                      label="Header"
                      placeholder="Authorization"
                      value={headerDrafts[gateway.id] ?? ''}
                      onChange={(event) =>
                        setHeaderDrafts({ ...headerDrafts, [gateway.id]: event.target.value })
                      }
                      helperText="Leave empty to keep the provider's header."
                    />
                  ) : null}
                </Box>
              );
            })}
          </Box>
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canConfirm || submitting}
          onClick={() =>
            onConfirm(
              selected.map((gateway) => ({
                gatewayId: gateway.id,
                apiKey: keyDrafts[gateway.id]?.trim() || undefined,
                authHeader: headerDrafts[gateway.id]?.trim() || undefined,
              })),
              buildId || undefined
            )
          }
        >
          {submitting ? 'Deploying...' : 'Deploy'}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ProviderDeployDialog;
