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
  gateways.find((gateway) => gateway.status === 'active') ??
  gateways[0] ??
  null;

const DeployDialog: FC<DeployDialogProps> = ({ open, mode, environment, onClose, onConfirm }) => {
  const [gatewayId, setGatewayId] = useState('');
  const [endpointUrl, setEndpointUrl] = useState('');
  const [urlTouched, setUrlTouched] = useState(false);

  useEffect(() => {
    if (open && environment) {
      const defaultGateway = pickDefaultGateway(environment.gateways);
      setGatewayId(defaultGateway?.id ?? '');
      setEndpointUrl(defaultGateway?.endpointUrl ?? '');
      setUrlTouched(false);
    }
  }, [open, environment]);

  if (!environment) return null;

  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';
  const selectedGateway = environment.gateways.find((gateway) => gateway.id === gatewayId) ?? null;
  const isSingleGateway = environment.gateways.length === 1;
  const isSelectedInactive = selectedGateway ? selectedGateway.status !== 'active' : false;
  const urlMissing = endpointUrl.trim().length === 0;
  const canConfirm = !!selectedGateway && !isSelectedInactive && !urlMissing;

  const handleSelectGateway = (id: string) => {
    setGatewayId(id);
    const gateway = environment.gateways.find((candidate) => candidate.id === id);
    setEndpointUrl(gateway?.endpointUrl ?? '');
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
            ? `Initial deployment goes to ${environment.name}. Select the gateway to deploy to.`
            : `Select which gateway in ${environment.name} should receive this build.`}
        </Typography>

        {isSelectedInactive ? (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {selectedGateway?.name} is inactive and can't receive a deployment. Choose an active gateway to continue.
          </Alert>
        ) : null}

        {isSingleGateway ? (
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
              <StatusDot tone={gatewayStatusTone(selectedGateway!.status).tone} />
              <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
                  {selectedGateway!.name}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap display="block">
                  {selectedGateway!.region}
                </Typography>
              </Box>
              <StatusPill tone={gatewayStatusTone(selectedGateway!.status)} />
            </Box>
          </Box>
        ) : (
          <Box sx={{ mb: 2.5 }}>
            <FormLabel sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Gateway</FormLabel>
            <FormControl fullWidth size="small">
              <Select
                value={gatewayId}
                onChange={(event) => handleSelectGateway(event.target.value as string)}
                renderValue={(value) => {
                  const gateway = environment.gateways.find((candidate) => candidate.id === value);
                  if (!gateway) return null;
                  return (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <StatusDot tone={gatewayStatusTone(gateway.status).tone} />
                      <Typography variant="body2">{gateway.name}</Typography>
                    </Box>
                  );
                }}
              >
                {environment.gateways.map((gateway) => (
                  <MenuItem key={gateway.id} value={gateway.id}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <StatusDot tone={gatewayStatusTone(gateway.status).tone} />
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
            onChange={(event) => setEndpointUrl(event.target.value)}
            onBlur={() => setUrlTouched(true)}
            error={urlTouched && urlMissing}
            helperText={urlTouched && urlMissing ? 'Endpoint URL is required.' : ' '}
          />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          disabled={!canConfirm}
          onClick={() => onConfirm(gatewayId, endpointUrl.trim())}
        >
          {actionLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
