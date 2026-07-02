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

import { useState, useEffect } from 'react';
import { useNavigate, useParams, Link as RouterLink } from 'react-router-dom';
import {
  Box,
  Button,
  TextField,
  Typography,
  CircularProgress,
  Alert,
  PageContent,
  Stack,
  FormControl,
  FormLabel,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { useGatewayList } from '../../../../hooks/useGateway';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { getGateways } from '../../../../apis/gateway/gatewayApi';

// Validation constants
const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;

const normalizeVhost = (value: string): string => {
  const trimmed = value.trim();
  if (trimmed.startsWith('https://')) return trimmed.slice(8);
  if (trimmed.startsWith('http://')) return trimmed.slice(7);
  return trimmed;
};

const getDisplayUrl = (vhost: string): string => {
  if (!vhost || !vhost.trim()) return '';
  const trimmed = vhost.trim();
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    return trimmed;
  }
  return `https://${trimmed}`;
};

export default function EditGateway() {
  const navigate = useNavigate();
  const { gatewayName } = useParams<{ gatewayName: string }>();
  const { currentOrganization } = useAppShell();
  const { updateGatewayById, isUpdating } = useGatewayList();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [gatewayId, setGatewayId] = useState<string>('');
  const [displayName, setDisplayName] = useState('');
  const [description, setDescription] = useState('');
  const [functionalityType, setFunctionalityType] = useState('regular');
  const [vhost, setVhost] = useState('');
  const [otherEndpoints, setOtherEndpoints] = useState<string[]>([]);

  // Load gateway data
  useEffect(() => {
    const loadGateway = async () => {
      if (!gatewayName || !currentOrganization?.uuid) return;

      try {
        const response = await getGateways(currentOrganization.uuid);
        const foundGateway = response.data?.list?.find(
          (gw) => gw.id === gatewayName
        );

        if (foundGateway) {
          setGatewayId(foundGateway.id);
          setDisplayName(foundGateway.displayName || foundGateway.name);
          setDescription(foundGateway.description || '');
          setFunctionalityType(foundGateway.functionalityType || 'regular');
          setVhost(foundGateway.endpoints?.[0] || foundGateway.vhost || '');
          setOtherEndpoints(foundGateway.endpoints?.slice(1) || []);
        } else {
          setError('Gateway not found');
        }
      } catch (err: any) {
        console.error('Failed to load gateway:', err);
        setError(err?.message || 'Failed to load gateway');
      } finally {
        setLoading(false);
      }
    };

    loadGateway();
  }, [gatewayName, currentOrganization?.uuid]);

  const isFormValid = (): boolean => {
    if (!displayName || displayName.trim().length === 0) return false;
    if (displayName.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    const normalizedVhost = normalizeVhost(vhost || '');
    if (!normalizedVhost || normalizedVhost.length === 0) return false;
    if (!functionalityType) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!gatewayId) return;

    try {
      const normalizedVhost = normalizeVhost(vhost);
      await updateGatewayById(gatewayId, {
        displayName,
        endpoints: normalizedVhost
          ? [normalizedVhost, ...otherEndpoints]
          : undefined,
        functionalityType,
        description: description || undefined,
      });

      showSnackbar('AI Gateway updated successfully', 'success');

      // Redirect to the gateway view page
      const viewPath = buildOrgPath(
        currentOrganization,
        `/gateways/view/${gatewayName}`
      );
      navigate(viewPath);
    } catch (err: any) {
      showSnackbar(
        err?.message || 'Failed to update self-hosted gateway',
        'error'
      );
    }
  };

  const handleCancel = () => {
    const listPath = buildOrgPath(currentOrganization, '/gateways');
    navigate(listPath);
  };

  if (loading) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 400,
          }}
        >
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={buildOrgPath(currentOrganization, '/gateways')}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ mb: 2 }}
        >
          Back to list
        </Button>
        <Alert severity="error">{error}</Alert>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={buildOrgPath(currentOrganization, '/gateways')}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to list
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit AI Gateway
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the configuration for your AI gateway
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Typography variant="h6" sx={{ mb: 2 }}>
            General Details
          </Typography>

          <Stack spacing={3}>
            <FormControl fullWidth>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                required
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Enter gateway name"
                error={displayName.length > MAX_NAME_LENGTH}
                helperText={
                  displayName.length > MAX_NAME_LENGTH
                    ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${displayName.length}/${MAX_NAME_LENGTH})`
                    : ''
                }
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel>Description (Optional)</FormLabel>
              <TextField
                fullWidth
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Enter description"
                multiline
                minRows={2}
                error={description.length > MAX_DESCRIPTION_LENGTH}
                helperText={
                  description.length > MAX_DESCRIPTION_LENGTH
                    ? `Description must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH})`
                    : ''
                }
              />
            </FormControl>
          </Stack>
        </Box>

        {/* <Box sx={{ mb: 4 }}>
          <Typography variant="h6" sx={{ mb: 2 }}>
            Gateway Configuration
          </Typography>

          <Stack spacing={3}>
            <FormControl fullWidth>
              <FormLabel required>URL</FormLabel>
              <TextField
                fullWidth
                required
                value={getDisplayUrl(vhost)}
                onChange={(e) => setVhost(normalizeVhost(e.target.value))}
                placeholder="Enter URL"
              />
            </FormControl>
          </Stack>
        </Box> */}

        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" color="secondary" onClick={handleCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={isUpdating || !isFormValid()}
          >
            {isUpdating ? 'Updating...' : 'Update'}
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}
