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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import type { DeploymentParameter, Environment } from '../types';

export type DeployDialogProps = {
  open: boolean;
  /** Promote carries the previous environment's build forward; deploy renders one. */
  mode: 'deploy' | 'promote';
  environment: Environment | null;
  /** Source environment shown in the promote wording. */
  fromEnvironment?: string;
  /** Null while the environment's settings are still loading. */
  parameters: DeploymentParameter[] | null;
  submitting: boolean;
  onClose: () => void;
  onConfirm: (gatewayIds: string[], parameters: Record<string, string>) => void;
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

const DeployDialog: FC<DeployDialogProps> = ({
  open,
  mode,
  environment,
  fromEnvironment,
  parameters,
  submitting,
  onClose,
  onConfirm,
}) => {
  const [selected, setSelected] = useState<string[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});

  // Every gateway is preselected: deploying to all of an environment is the common
  // case, and unchecking is easier than checking.
  useEffect(() => {
    if (open && environment) {
      setSelected(environment.gateways.map((gateway) => gateway.id));
    }
  }, [open, environment]);

  useEffect(() => {
    if (!open || !parameters) return;
    setValues(Object.fromEntries(parameters.map((parameter) => [parameter.name, parameter.value])));
  }, [open, parameters]);

  if (!environment) return null;

  const gateways = environment.gateways;
  const allSelected = selected.length === gateways.length && gateways.length > 0;
  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';

  const errors = (parameters ?? [])
    .map((parameter) => validate(parameter, values[parameter.name] ?? ''))
    .filter((error): error is string => error !== null);

  const toggleGateway = (gatewayId: string) => {
    setSelected((previous) =>
      previous.includes(gatewayId)
        ? previous.filter((id) => id !== gatewayId)
        : [...previous, gatewayId]
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>
        {actionLabel} to {environment.name}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {mode === 'promote'
            ? `The build currently running in ${fromEnvironment ?? 'the previous environment'} is carried forward unchanged, with ${environment.name}'s own settings applied.`
            : `Deploys to ${environment.name} and applies its settings. Gateways already serving this API keep running the same build.`}
        </Typography>

        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
          <Typography sx={sectionLabelSx}>Gateways</Typography>
          {gateways.length > 1 && (
            <FormControlLabel
              control={
                <Checkbox
                  size="small"
                  checked={allSelected}
                  onChange={() =>
                    setSelected(allSelected ? [] : gateways.map((gateway) => gateway.id))
                  }
                />
              }
              label={<Typography variant="body2">Select all</Typography>}
              sx={{ mr: 0 }}
            />
          )}
        </Box>

        {gateways.length === 0 ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {environment.name} has no gateway to deploy to. Add one to this environment first.
          </Alert>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mb: 2 }}>
            {gateways.map((gateway) => {
              const checked = selected.includes(gateway.id);
              return (
                <Box
                  key={gateway.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => toggleGateway(gateway.id)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') toggleGateway(gateway.id);
                  }}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    px: 1.5,
                    py: 1,
                    border: '1px solid',
                    borderColor: checked ? 'primary.main' : 'divider',
                    bgcolor: checked ? 'action.selected' : 'background.paper',
                    borderRadius: 1.5,
                    cursor: 'pointer',
                  }}
                >
                  <Checkbox
                    size="small"
                    checked={checked}
                    onChange={() => toggleGateway(gateway.id)}
                    onClick={(event) => event.stopPropagation()}
                  />
                  <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                    <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
                      {gateway.name}
                    </Typography>
                  </Box>
                  <StatusPill tone={gatewayStatusTone(gateway.status)} />
                </Box>
              );
            })}
          </Box>
        )}

        <Divider sx={{ mb: 2 }} />

        <Typography sx={{ ...sectionLabelSx, mb: 0.5 }}>{environment.name} settings</Typography>
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1.5 }}>
          These apply to every gateway in {environment.name} and are remembered for the next
          deployment.
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
      <DialogActions sx={{ px: 3, pb: 2, display: 'flex', justifyContent: 'space-between' }}>
        <Typography variant="body2" color="text.secondary">
          {selected.length} of {gateways.length} selected
        </Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={selected.length === 0 || errors.length > 0 || submitting}
            onClick={() => onConfirm(selected, values)}
          >
            {submitting ? `${actionLabel}ing...` : actionLabel}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
