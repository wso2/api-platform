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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import type { Environment, Gateway } from '../types';

export type DeployDialogProps = {
  open: boolean;
  mode: 'deploy' | 'promote';
  environment: Environment | null;
  /** The environment a promotion carries the build out of. */
  sourceEnvironmentName?: string;
  submitting: boolean;
  onClose: () => void;
  onConfirm: (gatewayId: string, endpointUrl: string) => void;
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
  sourceEnvironmentName,
  submitting,
  onClose,
  onConfirm,
}) => {
  // Only what the user has chosen themselves is held here: an empty gateway id
  // and a null draft both mean "whatever this environment says", resolved below.
  // Holding the resolved values instead would leave the first render of a freshly
  // opened dialog with nothing selected, since the effect that filled them ran
  // after it, and would let a background refresh overwrite a half-typed URL.
  const [gatewayId, setGatewayId] = useState('');
  const [endpointDraft, setEndpointDraft] = useState<string | null>(null);
  const [urlTouched, setUrlTouched] = useState(false);

  useEffect(() => {
    if (!open) return;
    setGatewayId('');
    setEndpointDraft(null);
    setUrlTouched(false);
  }, [open]);

  if (!environment) return null;

  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';
  const selectedGateway =
    environment.gateways.find((gateway) => gateway.id === gatewayId) ??
    pickDefaultGateway(environment.gateways);
  const endpointUrl = endpointDraft ?? selectedGateway?.endpointUrl ?? '';
  const isSingleGateway = environment.gateways.length === 1;
  // Whether the gateway can receive a deployment is its own health, not the state
  // of what is deployed on it: a healthy gateway with nothing deployed is exactly
  // what a first deployment targets.
  const isSelectedInactive = selectedGateway ? selectedGateway.health !== 'active' : false;
  const urlMissing = endpointUrl.trim().length === 0;
  const canConfirm = !!selectedGateway && !isSelectedInactive && !urlMissing;

  const handleSelectGateway = (id: string) => {
    setGatewayId(id);
    setEndpointDraft(null);
    setUrlTouched(false);
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
            : `Carries the build running in ${sourceEnvironmentName ?? 'the previous environment'} forward to ${environment.name}, with the endpoint you give here.`}
        </Typography>

        {isSelectedInactive ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {selectedGateway?.name} is inactive and can't receive a deployment. Choose an active gateway to continue.
          </Alert>
        ) : null}

        {isSingleGateway && selectedGateway ? (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateway</FormLabel>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                px: 1.5,
                py: 1,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1.5,
              }}
            >
              <StatusDot tone={selectedGateway.health === 'active' ? 'success' : 'default'} />
              <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
                  {selectedGateway.name}
                </Typography>
                {selectedGateway.host ? (
                  <Typography variant="caption" color="text.secondary" noWrap display="block">
                    {selectedGateway.host}
                  </Typography>
                ) : null}
              </Box>
              <StatusPill tone={gatewayStatusTone(selectedGateway.status)} />
            </Box>
          </Box>
        ) : (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateway</FormLabel>
            <FormControl fullWidth size="small">
              <Select
                value={selectedGateway?.id ?? ''}
                onChange={(event) => handleSelectGateway(event.target.value as string)}
                renderValue={(value) => {
                  const gateway = environment.gateways.find((candidate) => candidate.id === value);
                  if (!gateway) return null;
                  return (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <StatusDot tone={gateway.health === 'active' ? 'success' : 'default'} />
                      <Typography variant="body2">{gateway.name}</Typography>
                    </Box>
                  );
                }}
              >
                {environment.gateways.map((gateway) => (
                  <MenuItem key={gateway.id} value={gateway.id}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <StatusDot tone={gateway.health === 'active' ? 'success' : 'default'} />
                      <Typography variant="body2" sx={{ fontWeight: 500 }}>
                        {gateway.name}
                        {gateway.isDefault ? ' · Default' : ''}
                      </Typography>
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>
        )}

        <Box>
          <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Endpoint URL</FormLabel>
          <TextField
            fullWidth
            size="small"
            required
            placeholder="https://api.example.com"
            value={endpointUrl}
            onChange={(event) => setEndpointDraft(event.target.value)}
            onBlur={() => setUrlTouched(true)}
            error={urlTouched && urlMissing}
            helperText={urlTouched && urlMissing ? 'Endpoint URL is required.' : ' '}
          />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canConfirm || submitting}
          onClick={() => {
            if (selectedGateway) onConfirm(selectedGateway.id, endpointUrl.trim());
          }}
        >
          {submitting ? `${actionLabel}ing...` : actionLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
