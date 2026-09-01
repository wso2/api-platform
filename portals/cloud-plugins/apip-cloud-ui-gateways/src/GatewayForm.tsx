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

import { useState, type FC } from 'react';
import {
  Box,
  Button,
  FormControl,
  FormLabel,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import EnvironmentSelect from './components/EnvironmentSelect';
import GatewayTypeSelector from './components/GatewayTypeSelector';
import { createGateway, getGateway, listEnvironments, updateGateway } from './mocks/gatewaysStore';
import type { NotifySeverity } from './hostPort';
import type { GatewayType } from './types';

export type GatewayFormProps = {
  /** Editing an existing gateway pre-fills the form and changes the page's labels. Omit (or 'create') for a blank gateway. */
  mode?: 'create' | 'edit';
  /** Required when `mode` is 'edit' — the gateway to load and update. */
  gatewayId?: string;
  onBack: () => void;
  notify?: (message: string, severity?: NotifySeverity) => void;
};

const GatewayForm: FC<GatewayFormProps> = ({ mode = 'create', gatewayId, onBack, notify }) => {
  const isEdit = mode === 'edit' && !!gatewayId;
  const editingGateway = isEdit ? getGateway(gatewayId as string) : undefined;

  const [environments] = useState(() => listEnvironments());
  const [type, setType] = useState<GatewayType>(editingGateway?.type ?? 'ai');
  const [name, setName] = useState(editingGateway?.name ?? '');
  const [description, setDescription] = useState(editingGateway?.description ?? '');
  const [url, setUrl] = useState(editingGateway?.url ?? '');
  const [environmentId, setEnvironmentId] = useState(editingGateway?.environmentId ?? '');

  const canSubmit = name.trim().length > 0 && url.trim().length > 0 && environmentId.length > 0;

  const handleSubmit = () => {
    const input = {
      name: name.trim(),
      description: description.trim() || undefined,
      type,
      environmentId,
      url: url.trim(),
    };

    if (isEdit && editingGateway) {
      updateGateway(editingGateway.id, input);
      notify?.(`Gateway "${input.name}" updated.`, 'success');
    } else {
      createGateway(input);
      notify?.(`Gateway "${input.name}" created.`, 'success');
    }
    onBack();
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onBack}>
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>{isEdit ? 'Edit Gateway' : 'Add Gateway'}</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel required>Gateway Type</FormLabel>
              <GatewayTypeSelector value={type} onChange={setType} readOnly={isEdit} />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                required
                placeholder="Enter gateway name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                autoFocus
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
                placeholder="Enter description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel required>URL</FormLabel>
              <TextField
                fullWidth
                required
                placeholder="https://localhost:8443"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel required>Environment</FormLabel>
              <EnvironmentSelect environments={environments} value={environmentId} onChange={setEnvironmentId} />
            </FormControl>
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" onClick={onBack}>
            Cancel
          </Button>
          <Tooltip title={!canSubmit ? 'Fill in the required fields to continue.' : ''}>
            <span>
              <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
                {isEdit ? 'Save Changes' : 'Add Gateway'}
              </Button>
            </span>
          </Tooltip>
        </Box>
      </Box>
    </PageContent>
  );
};

export default GatewayForm;
