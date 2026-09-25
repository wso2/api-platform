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

import { useEffect, useMemo, type Dispatch, type MutableRefObject, type SetStateAction } from 'react';
import {
  Alert,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { useGatewayList } from '../../../../../hooks/useGateway';
import {
  useEnvironments,
  type EnvironmentOption,
} from '../../../../../hooks/useEnvironments';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import type { GatewayFormState } from './types';
import {
  AVAILABLE_GATEWAY_VERSIONS,
  generateGatewayName,
  getDisplayUrl,
  MAX_GATEWAY_DESCRIPTION_LENGTH,
  MAX_GATEWAY_NAME_LENGTH,
  normalizeVhost,
} from './utils';

type AddGatewayStepProps = {
  formState: GatewayFormState;
  setFormState: Dispatch<SetStateAction<GatewayFormState>>;
  preferredGatewayId: string | null;
  onPreferredGatewayChange: (gatewayId: string) => void;
  onGatewayCreated: (gateway: HybridGateway) => void;
  onCanCreateChange: (canCreate: boolean) => void;
  onIsCreatingChange: (isCreating: boolean) => void;
  createRef: MutableRefObject<(() => Promise<void>) | null>;
};

export default function AddGatewayStep({
  formState,
  setFormState,
  preferredGatewayId,
  onPreferredGatewayChange,
  onGatewayCreated,
  onCanCreateChange,
  onIsCreatingChange,
  createRef,
}: AddGatewayStepProps) {
  const { gateways, isLoading, createGateway, isCreating } = useGatewayList();
  const { environments, isLoading: isLoadingEnvironments } = useEnvironments();

  const aiGateways = useMemo(
    () => gateways.filter((gateway) => gateway.functionalityType === 'ai'),
    [gateways]
  );

  useEffect(() => {
    if (environments.length > 0 && !formState.environment) {
      setFormState((prev) => ({
        ...prev,
        environment: environments[0].id,
      }));
    }
  }, [environments, formState.environment, setFormState]);

  useEffect(() => {
    if (preferredGatewayId) {
      return;
    }

    const fallbackGatewayId = aiGateways[0]?.id;
    if (fallbackGatewayId) {
      onPreferredGatewayChange(fallbackGatewayId);
    }
  }, [aiGateways, onPreferredGatewayChange, preferredGatewayId]);

  const canCreateGateway =
    formState.displayName.trim().length > 0 &&
    formState.displayName.length <= MAX_GATEWAY_NAME_LENGTH &&
    formState.description.length <= MAX_GATEWAY_DESCRIPTION_LENGTH &&
    normalizeVhost(formState.vhost).length > 0 &&
    formState.environment.trim().length > 0 &&
    formState.version.trim().length > 0;

  useEffect(() => {
    onCanCreateChange(canCreateGateway);
  }, [canCreateGateway, onCanCreateChange]);

  useEffect(() => {
    onIsCreatingChange(isCreating);
  }, [isCreating, onIsCreatingChange]);

  const handleCreateGateway = async () => {
    if (!canCreateGateway) {
      return;
    }

    try {
      // The gateway's URL goes up as `endpoints`, matching AddGateway — a
      // gateway can carry more than one, the wizard just collects the first.
      const normalizedVhost = normalizeVhost(formState.vhost);
      const createdGateway = await createGateway({
        displayName: formState.displayName.trim(),
        id: generateGatewayName(formState.displayName),
        description: formState.description.trim() || undefined,
        endpoints: normalizedVhost ? [normalizedVhost] : undefined,
        functionalityType: 'ai',
        environment: formState.environment || undefined,
        version: formState.version || undefined,
      });

      onPreferredGatewayChange(createdGateway.id);
      onGatewayCreated(createdGateway);
    } catch {
      // The shared gateway hook already surfaces the error to the user.
    }
  };

  createRef.current = handleCreateGateway;

  if (isCreating) {
    return (
      <Stack alignItems="center" justifyContent="center" spacing={2} sx={{ py: 8 }}>
        <CircularProgress size={32} />
        <Typography variant="body2" color="text.secondary">
          Creating AI Gateway...
        </Typography>
      </Stack>
    );
  }

  return (
    <Stack spacing={2.5}>
      {isLoading ? (
        <Stack direction="row" spacing={1.5} alignItems="center">
          <CircularProgress size={20} />
          <Typography variant="body2" color="text.secondary">
            Loading AI gateways...
          </Typography>
        </Stack>
      ) : null}

      {aiGateways.length > 0 ? (
        <Alert severity="info">
          {preferredGatewayId
            ? 'You can keep using an existing AI Gateway, or create a new one for this provider.'
            : 'Existing AI Gateways are already available. You can continue with one of them or create a new gateway here.'}
        </Alert>
      ) : (
        <Alert severity="warning">
          No AI Gateways are available yet. Create one to continue with deployment.
        </Alert>
      )}

      <Grid container spacing={2}>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel required>Gateway Name</FormLabel>
            <TextField
              fullWidth
              value={formState.displayName}
              onChange={(event) =>
                setFormState((prev) => ({
                  ...prev,
                  displayName: event.target.value,
                }))
              }
              placeholder="Acme Default Gateway"
              error={formState.displayName.length > MAX_GATEWAY_NAME_LENGTH}
              helperText={
                formState.displayName.length > MAX_GATEWAY_NAME_LENGTH
                  ? `Name must not exceed ${MAX_GATEWAY_NAME_LENGTH} characters.`
                  : undefined
              }
            />
          </FormControl>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth required>
            <FormLabel required>Gateway Version</FormLabel>
            <Select
              value={formState.version}
              onChange={(event) =>
                setFormState((prev) => ({
                  ...prev,
                  version: String(event.target.value),
                }))
              }
            >
              {AVAILABLE_GATEWAY_VERSIONS.map((version) => (
                <MenuItem key={version} value={version}>
                  {version}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth required>
            <FormLabel required>Associated Environment</FormLabel>
            <Select
              value={formState.environment}
              onChange={(event) =>
                setFormState((prev) => ({
                  ...prev,
                  environment: String(event.target.value),
                }))
              }
              disabled={isLoadingEnvironments || environments.length === 0}
            >
              {environments.map((environment: EnvironmentOption) => (
                <MenuItem key={environment.id} value={environment.id}>
                  {environment.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel required>URL</FormLabel>
            <TextField
              fullWidth
              value={getDisplayUrl(formState.vhost)}
              onChange={(event) =>
                setFormState((prev) => ({
                  ...prev,
                  vhost: normalizeVhost(event.target.value),
                }))
              }
              placeholder="https://localhost:8443"
            />
          </FormControl>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>Description (Optional)</FormLabel>
            <TextField
              fullWidth
              multiline
              minRows={3}
              value={formState.description}
              onChange={(event) =>
                setFormState((prev) => ({
                  ...prev,
                  description: event.target.value,
                }))
              }
              placeholder="Enter description"
              error={formState.description.length > MAX_GATEWAY_DESCRIPTION_LENGTH}
              helperText={
                formState.description.length > MAX_GATEWAY_DESCRIPTION_LENGTH
                  ? `Description must not exceed ${MAX_GATEWAY_DESCRIPTION_LENGTH} characters.`
                  : undefined
              }
            />
          </FormControl>
        </Grid>
      </Grid>
    </Stack>
  );
}
