/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useState, type FC } from 'react';
import {
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Typography,
} from '@wso2/oxygen-ui';
import StatusPill from './StatusPill';
import { gatewayStatusTone } from '../utils/status';
import type { Environment } from '../types';

export type DeployDialogProps = {
  open: boolean;
  mode: 'deploy' | 'promote';
  environment: Environment | null;
  onClose: () => void;
  onConfirm: (gatewayIds: string[]) => void;
};

const DeployDialog: FC<DeployDialogProps> = ({ open, mode, environment, onClose, onConfirm }) => {
  const [selected, setSelected] = useState<string[]>([]);

  useEffect(() => {
    if (open && environment) {
      setSelected(environment.gateways.map((gateway) => gateway.id));
    }
  }, [open, environment]);

  if (!environment) return null;

  const allSelected = selected.length === environment.gateways.length && environment.gateways.length > 0;
  const actionLabel = mode === 'deploy' ? 'Deploy' : 'Promote';

  const toggleGateway = (gatewayId: string) => {
    setSelected((prev) =>
      prev.includes(gatewayId) ? prev.filter((id) => id !== gatewayId) : [...prev, gatewayId]
    );
  };

  const toggleSelectAll = () => {
    setSelected(allSelected ? [] : environment.gateways.map((gateway) => gateway.id));
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>
        {actionLabel} to {environment.name}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {mode === 'deploy'
            ? `Initial deployment goes to ${environment.name}. Select the gateways to deploy to.`
            : `Select which gateways in ${environment.name} should receive this build.`}
        </Typography>

        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
          <Typography
            sx={{
              fontSize: 12,
              fontWeight: 600,
              color: 'text.secondary',
              textTransform: 'uppercase',
              letterSpacing: '0.04em',
            }}
          >
            Gateways
          </Typography>
          <FormControlLabel
            control={<Checkbox size="small" checked={allSelected} onChange={toggleSelectAll} />}
            label={<Typography variant="body2">Select all</Typography>}
            sx={{ mr: 0 }}
          />
        </Box>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {environment.gateways.map((gateway) => {
            const checked = selected.includes(gateway.id);
            const tone = gatewayStatusTone(gateway.status);
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
                  <Typography variant="caption" color="text.secondary" noWrap display="block">
                    {gateway.region}
                  </Typography>
                </Box>
                <StatusPill tone={tone} />
              </Box>
            );
          })}
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, display: 'flex', justifyContent: 'space-between' }}>
        <Typography variant="body2" color="text.secondary">
          {selected.length} of {environment.gateways.length} selected
        </Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="contained" disabled={selected.length === 0} onClick={() => onConfirm(selected)}>
            {actionLabel}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
};

export default DeployDialog;
