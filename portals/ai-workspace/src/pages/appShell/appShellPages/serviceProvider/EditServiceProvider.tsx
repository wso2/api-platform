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
import {
  LLMProviderProvider,
  useLLMProvider,
} from '../../../../contexts/llmProvider';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';

const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;
const MAX_VERSION_LENGTH = 50;
const MAX_CONTEXT_LENGTH = 255;

function EditServiceProviderForm() {
  const navigate = useNavigate();
  const { providerId } = useParams<{ providerId: string }>();
  const { currentOrganization } = useAppShell();
  const { provider, isLoading, error, updateProvider } = useLLMProvider();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [version, setVersion] = useState('');
  const [context, setContext] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isContextOrVersionChanged =
    provider !== null &&
    (version !== (provider.version || '') ||
      context !== (provider.context || ''));

  useEffect(() => {
    if (provider) {
      setName(provider.name || '');
      setDescription(provider.description || '');
      setVersion(provider.version || '');
      setContext(provider.context || '');
    }
  }, [provider]);

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    if (version.length > MAX_VERSION_LENGTH) return false;
    if (context.length > MAX_CONTEXT_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!providerId) return;

    setIsSubmitting(true);
    try {
      const fullPayload = {
        ...provider,
        name,
        description: description || undefined,
        version: version || undefined,
        context: context || undefined,
      };
      // Remove read-only fields before sending
      delete (fullPayload as any).status;
      delete (fullPayload as any).createdAt;
      delete (fullPayload as any).createdBy;
      delete (fullPayload as any).updatedAt;
      delete (fullPayload as any).lastUpdated;

      await updateProvider(fullPayload);

      showSnackbar('Service Provider updated successfully', 'success');

      const viewPath = buildOrgPath(
        currentOrganization,
        `/service-provider/${providerId}`
      );
      navigate(viewPath);
    } catch (err: any) {
      showSnackbar(
        err?.message || 'Failed to update service provider',
        'error'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    const viewPath = buildOrgPath(
      currentOrganization,
      `/service-provider/${providerId}`
    );
    navigate(viewPath);
  };

  if (isLoading) {
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
          to={buildOrgPath(currentOrganization, '/service-provider')}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ mb: 2 }}
        >
          Back to list
        </Button>
        <Alert severity="error">{error.message}</Alert>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={buildOrgPath(
          currentOrganization,
          `/service-provider/${providerId}`
        )}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to Service Provider
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit Service Provider
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the details for your service provider
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Stack spacing={3}>
            {isContextOrVersionChanged && (
              <Alert severity="warning">
                You have modified the context or version of this service
                provider. After updating, you will need to redeploy on the
                gateway for the changes to take effect.
              </Alert>
            )}
            <Box sx={{ display: 'flex', gap: 2 }}>
              <FormControl sx={{ flex: 1 }}>
                <FormLabel required>Name</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Enter service provider name"
                  error={name.length > MAX_NAME_LENGTH}
                  helperText={
                    name.length > MAX_NAME_LENGTH
                      ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                      : ''
                  }
                />
              </FormControl>

              <FormControl sx={{ flex: 0.4 }}>
                <FormLabel>Version</FormLabel>
                <TextField
                  fullWidth
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  placeholder="e.g., 1.0"
                  error={version.length > MAX_VERSION_LENGTH}
                  helperText={
                    version.length > MAX_VERSION_LENGTH
                      ? `Version must not exceed ${MAX_VERSION_LENGTH} characters (${version.length}/${MAX_VERSION_LENGTH})`
                      : ''
                  }
                />
              </FormControl>
            </Box>

            <FormControl fullWidth>
              <FormLabel>Description</FormLabel>
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

            <FormControl fullWidth>
              <FormLabel>Context</FormLabel>
              <TextField
                fullWidth
                value={context}
                onChange={(e) => setContext(e.target.value)}
                placeholder="Enter context path"
                error={context.length > MAX_CONTEXT_LENGTH}
                helperText={
                  context.length > MAX_CONTEXT_LENGTH
                    ? `Context must not exceed ${MAX_CONTEXT_LENGTH} characters (${context.length}/${MAX_CONTEXT_LENGTH})`
                    : ''
                }
              />
            </FormControl>
          </Stack>
        </Box>

        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" onClick={handleCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={isSubmitting || !isFormValid()}
          >
            {isSubmitting ? 'Updating...' : 'Update'}
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}

export default function EditServiceProvider() {
  const { providerId } = useParams<{ providerId: string }>();

  if (!providerId) {
    return (
      <PageContent fullWidth>
        <Alert severity="error">Provider ID is missing</Alert>
      </PageContent>
    );
  }

  return (
    <LLMProviderProvider providerId={providerId}>
      <EditServiceProviderForm />
    </LLMProviderProvider>
  );
}
