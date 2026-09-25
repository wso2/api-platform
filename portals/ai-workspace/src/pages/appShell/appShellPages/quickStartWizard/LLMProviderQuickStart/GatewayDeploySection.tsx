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

import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  Drawer,
  FormControl,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { HardDrive, Plus, X } from '@wso2/oxygen-ui-icons-react';
import { useGatewayList } from '../../../../../hooks/useGateway';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import AddGatewayStep from './AddGatewayStep';
import SetupGatewayStep from './SetupGatewayStep';
import type { GatewayFormState } from './types';

const ADD_NEW_GATEWAY_OPTION = '__add_new_gateway__';

type GatewayDeploySectionProps = {
  gatewayFormState: GatewayFormState;
  setGatewayFormState: Dispatch<SetStateAction<GatewayFormState>>;
  preferredGatewayId: string | null;
  onPreferredGatewayChange: (gatewayId: string) => void;
  createdGateway: HybridGateway | null;
  onGatewayCreated: (gateway: HybridGateway) => void;
  onGatewayChange: (gateway: HybridGateway) => void;
  gatewayRegistrationToken: string | null;
  onRegistrationTokenChange: (token: string | null) => void;
  onGatewayReadyChange: (isReady: boolean) => void;
};

export default function GatewayDeploySection({
  gatewayFormState,
  setGatewayFormState,
  preferredGatewayId,
  onPreferredGatewayChange,
  createdGateway,
  onGatewayCreated,
  onGatewayChange,
  gatewayRegistrationToken,
  onRegistrationTokenChange,
  onGatewayReadyChange,
}: GatewayDeploySectionProps) {
  const { gateways } = useGatewayList();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerStage, setDrawerStage] = useState<'add' | 'setup'>('add');
  const [canCreateGatewayForm, setCanCreateGatewayForm] = useState(false);
  const [isCreatingGateway, setIsCreatingGateway] = useState(false);
  const createGatewayRef = useRef<(() => Promise<void>) | null>(null);

  const aiGateways = useMemo(
    () => gateways.filter((gateway) => gateway.functionalityType === 'ai'),
    [gateways]
  );

  useEffect(() => {
    if (preferredGatewayId || aiGateways.length === 0) {
      return;
    }
    onPreferredGatewayChange(aiGateways[0].id);
  }, [aiGateways, onPreferredGatewayChange, preferredGatewayId]);

  const selectedGatewayId = createdGateway?.id ?? preferredGatewayId;
  const selectedGateway =
    aiGateways.find((gateway) => gateway.id === selectedGatewayId) ??
    createdGateway ??
    null;

  useEffect(() => {
    onGatewayReadyChange(Boolean(selectedGateway?.isActive));
  }, [onGatewayReadyChange, selectedGateway?.isActive]);

  const openAddDrawer = () => {
    setDrawerStage('add');
    setDrawerOpen(true);
  };

  const openSetupDrawer = () => {
    setDrawerStage('setup');
    setDrawerOpen(true);
  };

  const handleCloseDrawer = () => {
    setDrawerOpen(false);
  };

  const handleSelectExisting = (gatewayId: string) => {
    if (gatewayId === ADD_NEW_GATEWAY_OPTION) {
      openAddDrawer();
      return;
    }
    onPreferredGatewayChange(gatewayId);
  };

  const drawerTitle =
    drawerStage === 'add' ? 'Add AI Gateway' : 'Deploy AI Gateway';
  const drawerDescription =
    drawerStage === 'add'
      ? 'Create the gateway where you want to deploy your new AI service.'
      : 'Start the gateway where you want to deploy your AI service.';

  return (
    <Box>
      {selectedGateway ? (
        <Box>
          <FormControl sx={{ minWidth: 300, maxWidth: 500 }}>
            <Select
              value={selectedGateway.id}
              onChange={(event) => handleSelectExisting(String(event.target.value))}
              renderValue={() => (
                <Stack direction="row" spacing={1.5} alignItems="center">
                  <HardDrive size={16} />
                  <Typography variant="body2" noWrap>
                    {selectedGateway.displayName || selectedGateway.name}
                  </Typography>
                  <Chip
                    size="small"
                    variant="outlined"
                    color={selectedGateway.isActive ? 'success' : 'default'}
                    label={selectedGateway.isActive ? 'Active' : 'Not Deployed'}
                  />
                </Stack>
              )}
            >
              {aiGateways.map((gateway) => (
                <MenuItem key={gateway.id} value={gateway.id}>
                  <Stack
                    direction="row"
                    spacing={1.5}
                    alignItems="center"
                    sx={{ width: '100%', justifyContent: 'space-between' }}
                  >
                    <Typography variant="body2">
                      {gateway.displayName || gateway.name}
                    </Typography>
                    <Chip
                      size="small"
                      variant="outlined"
                      color={gateway.isActive ? 'success' : 'default'}
                      label={gateway.isActive ? 'Active' : 'Not Deployed'}
                    />
                  </Stack>
                </MenuItem>
              ))}
              <Divider />
              <MenuItem value={ADD_NEW_GATEWAY_OPTION}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Plus size={14} />
                  <Typography variant="body2">Add new gateway</Typography>
                </Stack>
              </MenuItem>
            </Select>
          </FormControl>

          <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mt: 1 }}>
            {!selectedGateway.isActive ? (
              <Typography variant="caption" color="error.main">
                Gateway is not connected yet.
              </Typography>
            ) : null}
            <Button
              variant="text"
              size="small"
              onClick={openSetupDrawer}
              sx={{ px: 0, minWidth: 'auto' }}
            >
              View Configuration
            </Button>
          </Stack>
        </Box>
      ) : (
        <Button
          variant="contained"
          startIcon={<Plus size={16} />}
          onClick={openAddDrawer}
        >
          Add Gateway
        </Button>
      )}

      <Drawer anchor="right" open={drawerOpen} onClose={handleCloseDrawer}>
        <Box sx={{ width: 560, p: 3, display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 1,
            }}
          >
            <Stack spacing={0.5}>
              <Typography variant="subtitle1">{drawerTitle}</Typography>
              <Typography variant="body2" color="text.secondary">
                {drawerDescription}
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close gateway drawer"
              onClick={handleCloseDrawer}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          <Box sx={{ flex: 1, overflowY: 'auto' }}>
            {drawerStage === 'add' ? (
              <AddGatewayStep
                formState={gatewayFormState}
                setFormState={setGatewayFormState}
                preferredGatewayId={preferredGatewayId}
                onPreferredGatewayChange={onPreferredGatewayChange}
                onGatewayCreated={(gateway) => {
                  onGatewayCreated(gateway);
                  onRegistrationTokenChange(gateway.token ?? null);
                  setDrawerStage('setup');
                }}
                onCanCreateChange={setCanCreateGatewayForm}
                onIsCreatingChange={setIsCreatingGateway}
                createRef={createGatewayRef}
              />
            ) : selectedGatewayId ? (
              <SetupGatewayStep
                gatewayId={selectedGatewayId}
                gateway={createdGateway}
                gatewayVersion={gatewayFormState.version}
                registrationToken={gatewayRegistrationToken}
                onGatewayChange={onGatewayChange}
                onRegistrationTokenChange={onRegistrationTokenChange}
                onGatewayReadyChange={onGatewayReadyChange}
              />
            ) : (
              <Alert severity="warning">
                Create or select an AI gateway to continue.
              </Alert>
            )}
          </Box>

          <Divider sx={{ my: 2 }} />

          <Stack direction="row" spacing={1.5} justifyContent="flex-end">
            {drawerStage === 'add' ? (
              <>
                <Button variant="outlined" color="secondary" onClick={handleCloseDrawer}>
                  Cancel
                </Button>
                <Button
                  variant="contained"
                  disabled={!canCreateGatewayForm || isCreatingGateway}
                  onClick={() => {
                    void createGatewayRef.current?.();
                  }}
                >
                  Add
                </Button>
              </>
            ) : (
              <Button variant="contained" onClick={handleCloseDrawer}>
                {selectedGateway?.isActive ? 'Done' : 'Close'}
              </Button>
            )}
          </Stack>
        </Box>
      </Drawer>
    </Box>
  );
}
