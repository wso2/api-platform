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

export type DeployDialogProps = {
  open: boolean;
  mode: 'deploy' | 'promote';
  environment: Environment | null;
  /** The environment a promotion carries the build out of. */
  sourceEnvironment?: Environment;
  builds: Build[];
  /** The backend URL the API is defined against; the endpoint field starts from it. */
  apiEndpointUrl?: string;
  initialBuildId?: string;
  /** A new build will be created on confirmation; the latest build is informational. */
  createBuild: boolean;
  submitting: boolean;
  onClose: () => void;
  /**
   * Confirms the deploy with every selected gateway and its endpoint. Plural
   * because an environment runs a single build of an API at a time, so the
   * gateways go in one call.
   */
  onConfirm: (
    gateways: { gatewayId: string; endpointUrl?: string }[],
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

const DeployDialog: FC<DeployDialogProps> = ({
  open,
  mode,
  environment,
  sourceEnvironment,
  builds,
  apiEndpointUrl,
  initialBuildId,
  createBuild,
  submitting,
  onClose,
  onConfirm,
}) => {
  // Only what the user has chosen themselves is held here: an empty gateway id
  // and a null draft both mean "whatever this environment says", resolved below.
  // Holding the resolved values instead would leave the first render of a freshly
  // opened dialog with nothing selected, since the effect that filled them ran
  // after it, and would let a background refresh overwrite a half-typed URL.
  const [selectedIds, setSelectedIds] = useState<string[] | null>(null);
  const [buildId, setBuildId] = useState('');
  const [endpointDrafts, setEndpointDrafts] = useState<Record<string, string>>({});
  const [urlTouched, setUrlTouched] = useState(false);

  useEffect(() => {
    if (!open) return;
    setSelectedIds(null);
    setBuildId('');
    setEndpointDrafts({});
    setUrlTouched(false);
  }, [open]);

  if (!environment) return null;

  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';
  const availableBuilds =
    mode === 'promote'
      ? builds.filter((build) =>
          sourceEnvironment?.gateways.some((gateway) => gateway.buildId === build.buildId)
        )
      : builds;
  const selectedBuildId = buildId || initialBuildId || availableBuilds[0]?.buildId || '';
  // A gateway the API is already deployed on must stay in the set: the
  // environment runs one build, so this deploy has to reach it too. They are
  // shown ticked and locked, and undeploying is the way to drop one.
  const alreadyDeployed = environment.gateways.filter((gateway) => !!gateway.deploymentId);
  const lockedIds = alreadyDeployed.map((gateway) => gateway.id);

  // Until the user touches the list, the selection is the already-deployed
  // gateways, or the environment's default for a first deploy.
  const defaultSelection =
    lockedIds.length > 0
      ? lockedIds
      : [pickDefaultGateway(environment.gateways)?.id].filter((id): id is string => !!id);
  const selection = selectedIds ?? defaultSelection;
  const selected = environment.gateways.filter((gateway) => selection.includes(gateway.id));

  // What a gateway already serves comes first, so redeploying keeps the endpoint
  // it is running; one with nothing on it starts from the API's own backend URL.
  const endpointFor = (gateway: Gateway) =>
    endpointDrafts[gateway.id] ?? gateway.endpointUrl ?? apiEndpointUrl ?? '';

  const isSingleGateway = environment.gateways.length === 1;
  // Whether a gateway can receive a deployment is its own health, not the state of
  // what is on it: a healthy gateway with nothing deployed is exactly what a first
  // deployment targets.
  const inactiveSelected = selected.filter((gateway) => gateway.health !== 'active');
  const missingUrls = selected.filter((gateway) => endpointFor(gateway).trim().length === 0);
  const canConfirm =
    selected.length > 0 &&
    inactiveSelected.length === 0 &&
    missingUrls.length === 0 &&
    (createBuild || selectedBuildId.length > 0);

  const toggleGateway = (id: string) => {
    // Locked gateways cannot be unticked — the backend refuses a deploy that drops
    // them, so offering it here would only produce an error.
    if (lockedIds.includes(id)) return;
    setSelectedIds(
      selection.includes(id) ? selection.filter((each) => each !== id) : [...selection, id]
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>
        {actionLabel} to {environment.name}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {mode === 'deploy'
            ? `Deploys this API as it stands now to ${environment.name}. Select the gateway to deploy to.`
            : `Carries a build running in ${sourceEnvironment?.name ?? 'the previous environment'} forward to ${environment.name}, with the endpoint you give here.`}
        </Typography>

        {inactiveSelected.length > 0 ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {inactiveSelected.map((gateway) => gateway.name).join(', ')}
            {inactiveSelected.length === 1 ? ' is inactive and ' : ' are inactive and '}
            can't receive a deployment. Unselect{inactiveSelected.length === 1 ? ' it' : ' them'} to
            continue.
          </Alert>
        ) : null}

        {isSingleGateway && selected.length === 1 ? (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateway</FormLabel>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 1,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                px: 1.5,
                py: 1,
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <StatusDot tone={selected[0].health === 'active' ? 'success' : 'default'} />
                <Box>
                  <Typography variant="body2" sx={{ fontWeight: 500 }}>
                    {selected[0].name}
                  </Typography>
                  {selected[0].host ? (
                    <Typography variant="caption" color="text.secondary">
                      {selected[0].host}
                    </Typography>
                  ) : null}
                </Box>
              </Box>
              <StatusPill tone={gatewayStatusTone(selected[0].status)} />
            </Box>
            <TextField
              fullWidth
              size="small"
              required
              sx={{ mt: 1 }}
              label="Endpoint URL"
              placeholder="https://api.example.com"
              value={endpointFor(selected[0])}
              onChange={(event) =>
                setEndpointDrafts({ ...endpointDrafts, [selected[0].id]: event.target.value })
              }
              onBlur={() => setUrlTouched(true)}
              error={urlTouched && endpointFor(selected[0]).trim().length === 0}
              helperText={
                urlTouched && endpointFor(selected[0]).trim().length === 0
                  ? 'Endpoint URL is required.'
                  : ' '
              }
            />
          </Box>
        ) : (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateways</FormLabel>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
              {lockedIds.length > 0
                ? 'Every gateway this API is already deployed on stays selected — an environment runs one build at a time. Undeploy a gateway to stop deploying to it.'
                : 'Select the gateways to deploy to. They all receive the same build.'}
            </Typography>
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
                            ? 'Already deployed here. Undeploy it first to stop deploying to it.'
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
                    {/* The endpoint is per gateway: two gateways of one environment
                        can serve different backends, so each selected one gets its
                        own field rather than sharing a single value. */}
                    {isSelected ? (
                      <TextField
                        fullWidth
                        size="small"
                        required
                        sx={{ mt: 1 }}
                        label="Endpoint URL"
                        placeholder="https://api.example.com"
                        value={endpointFor(gateway)}
                        onChange={(event) =>
                          setEndpointDrafts({ ...endpointDrafts, [gateway.id]: event.target.value })
                        }
                        onBlur={() => setUrlTouched(true)}
                        error={urlTouched && endpointFor(gateway).trim().length === 0}
                        helperText={
                          urlTouched && endpointFor(gateway).trim().length === 0
                            ? 'Endpoint URL is required.'
                            : ' '
                        }
                      />
                    ) : null}
                  </Box>
                );
              })}
            </Box>
          </Box>
        )}

        {createBuild ? null : (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Build</FormLabel>
            <FormControl fullWidth size="small">
              <Select
                value={selectedBuildId}
                onChange={(event) => setBuildId(event.target.value as string)}
                displayEmpty
                disabled={availableBuilds.length === 0}
              >
                {availableBuilds.length === 0 ? (
                  <MenuItem value="" disabled>
                    No deployed builds available
                  </MenuItem>
                ) : (
                  availableBuilds.map((build) => (
                    <MenuItem key={build.buildId} value={build.buildId}>
                      {build.buildId}
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>
          </Box>
        )}

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
                endpointUrl: endpointFor(gateway).trim(),
              })),
              createBuild ? undefined : selectedBuildId
            )
          }
        >
          {submitting ? `${actionLabel}ing...` : actionLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
