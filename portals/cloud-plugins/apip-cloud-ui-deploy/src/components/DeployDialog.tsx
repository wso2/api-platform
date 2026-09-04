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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  MenuItem,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import { buildLabel } from '../utils/build';
import type { DeploymentParameter, Environment } from '../types';

export type DeployDialogProps = {
  open: boolean;
  /** Promote carries a build the source environment is running; deploy sends a prepared one. */
  mode: 'deploy' | 'promote';
  environment: Environment | null;
  /** Source environment shown in the promote wording. */
  fromEnvironment?: string;
  /**
   * Builds this dialog may send. Deploying offers the prepared builds; promoting
   * offers only what the source environment is running, which is what the API will
   * accept. Empty when deploying an API that has never been built — the first
   * deploy prepares one itself.
   */
  buildOptions: string[];
  /** Null while the selected gateway's settings are still loading. */
  parameters: DeploymentParameter[] | null;
  submitting: boolean;
  onClose: () => void;
  /** Called when the target gateway changes, so its current settings can be read. */
  onGatewayChange: (gatewayId: string) => void;
  onConfirm: (gatewayId: string, buildId: string, parameters: Record<string, string>) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

/** Client-side check mirroring the API's own, so a typo is caught before a round trip. */
const validate = (parameter: DeploymentParameter, value: string): string | null => {
  if (!value.trim()) return null;
  if (parameter.type === 'url') {
    try {
      const parsed = new URL(value);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        return 'Must be an http or https URL.';
      }
      if (!parsed.host) return 'Must include a host.';
    } catch {
      return 'Must be a valid URL.';
    }
  }
  if (parameter.type === 'host' && /[:/]/.test(value)) {
    return 'Must be a host name, without a scheme or path.';
  }
  return null;
};

/**
 * One deployment goes to one gateway with the settings it is deployed with, which
 * is why the gateway is chosen here and the settings are read from it: the form
 * opens on what that gateway is currently running, not on what an environment was
 * configured with.
 */
const DeployDialog: FC<DeployDialogProps> = ({
  open,
  mode,
  environment,
  fromEnvironment,
  buildOptions,
  parameters,
  submitting,
  onClose,
  onGatewayChange,
  onConfirm,
}) => {
  const [gatewayId, setGatewayId] = useState('');
  const [buildId, setBuildId] = useState('');
  const [values, setValues] = useState<Record<string, string>>({});

  // The environment's first gateway is preselected, and its settings are what the
  // form opens on.
  useEffect(() => {
    if (!open || !environment) return;
    const first = environment.gateways[0]?.id ?? '';
    setGatewayId(first);
    if (first) onGatewayChange(first);
    // onGatewayChange is stable for a given dialog opening; re-running on it would
    // reload the settings on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, environment]);

  // Deploying defaults to the newest build; promoting to what the source is
  // running, which is a single build unless its gateways were deployed separately.
  useEffect(() => {
    if (open) setBuildId(buildOptions[buildOptions.length - 1] ?? '');
  }, [open, buildOptions]);

  useEffect(() => {
    if (!open || !parameters) return;
    setValues(Object.fromEntries(parameters.map((parameter) => [parameter.name, parameter.value])));
  }, [open, parameters]);

  if (!environment) return null;

  const gateways = environment.gateways;
  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';

  const errors = (parameters ?? [])
    .map((parameter) => validate(parameter, values[parameter.name] ?? ''))
    .filter((error): error is string => error !== null);

  const selectGateway = (id: string) => {
    if (id === gatewayId) return;
    setGatewayId(id);
    onGatewayChange(id);
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>
        {actionLabel} to {environment.name}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {mode === 'promote'
            ? `Carries a build ${fromEnvironment ?? 'the previous environment'} is running forward, with the settings you give here.`
            : 'Sends a prepared build to one gateway. Edits made to the API since it was prepared are not included — prepare a new build to pick those up.'}
        </Typography>

        <Typography sx={{ ...sectionLabelSx, mb: 1 }}>Gateway</Typography>
        {gateways.length === 0 ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {environment.name} has no gateway to deploy to. Add one to this environment first.
          </Alert>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mb: 2 }}>
            {gateways.map((gateway) => {
              const selected = gateway.id === gatewayId;
              return (
                <Box
                  key={gateway.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => selectGateway(gateway.id)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') selectGateway(gateway.id);
                  }}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    px: 1.5,
                    py: 1,
                    border: '1px solid',
                    borderColor: selected ? 'primary.main' : 'divider',
                    bgcolor: selected ? 'action.selected' : 'background.paper',
                    borderRadius: 1.5,
                    cursor: 'pointer',
                  }}
                >
                  <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                    <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
                      {gateway.name}
                    </Typography>
                    {gateway.buildId && (
                      <Typography variant="caption" color="text.secondary">
                        Running {buildLabel(gateway.buildId)}
                      </Typography>
                    )}
                  </Box>
                  <StatusPill tone={gatewayStatusTone(gateway.status)} />
                </Box>
              );
            })}
          </Box>
        )}

        {/*
          Promoting offers only the builds the source environment is actually
          running: the API checks the build against that environment's live state,
          so offering anything else would be offering a rejection.
        */}
        {buildOptions.length > 1 ? (
          <TextField
            select
            label="Build"
            value={buildId}
            size="small"
            fullWidth
            sx={{ mb: 2 }}
            helperText={
              mode === 'promote'
                ? `${fromEnvironment ?? 'The source environment'} is running more than one build; choose the one to promote.`
                : 'The prepared build to deploy.'
            }
            onChange={(event) => setBuildId(event.target.value)}
          >
            {buildOptions.map((option) => (
              <MenuItem key={option} value={option}>
                {buildLabel(option)}
              </MenuItem>
            ))}
          </TextField>
        ) : (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1.5,
              px: 1.5,
              py: 1,
              mb: 2,
            }}
          >
            <Typography sx={sectionLabelSx}>{actionLabel}ing</Typography>
            <Typography variant="body2" sx={{ fontWeight: 500 }}>
              {buildOptions.length === 1
                ? buildLabel(buildOptions[0])
                : 'A build prepared from this API as it stands now'}
            </Typography>
          </Box>
        )}

        <Divider sx={{ mb: 2 }} />

        <Typography sx={{ ...sectionLabelSx, mb: 0.5 }}>Settings</Typography>
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1.5 }}>
          These are deployed with this gateway. They start from what it is running now.
        </Typography>

        {parameters === null ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 2 }}>
            <CircularProgress size={20} />
          </Box>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {parameters.map((parameter) => {
              const value = values[parameter.name] ?? '';
              const error = validate(parameter, value);
              return (
                <TextField
                  key={parameter.name}
                  label={parameter.label}
                  helperText={error ?? parameter.description}
                  error={error !== null}
                  value={value}
                  size="small"
                  fullWidth
                  onChange={(event) =>
                    setValues((previous) => ({ ...previous, [parameter.name]: event.target.value }))
                  }
                />
              );
            })}
          </Box>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!gatewayId || errors.length > 0 || submitting}
          onClick={() => onConfirm(gatewayId, buildId, values)}
        >
          {submitting ? `${actionLabel}ing...` : actionLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
