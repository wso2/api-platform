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
import { ProxyProvider, useProxy } from '../../../../contexts/proxy';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { DisabledActionTooltip } from '../../../../utils/readOnlyArtifacts';

const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;
const MAX_VERSION_LENGTH = 50;
const MAX_CONTEXT_LENGTH = 255;

function EditLLMProxyForm() {
  const navigate = useNavigate();
  const { proxyId } = useParams<{ proxyId: string }>();
  const { currentOrganization, currentProject } = useAppShell();
  const { proxy, isLoading, error, updateProxy } = useProxy();
  const showSnackbar = useAIWorkspaceSnackbar();
  const isReadOnlyProxy = Boolean(proxy?.readOnly);

  const isProjectLevel = Boolean(currentProject?.id);
  const proxiesPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/proxies')
    : buildOrgPath(currentOrganization, '/proxies');

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [version, setVersion] = useState('');
  const [context, setContext] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isContextOrVersionChanged =
    proxy !== null &&
    (version !== (proxy.version || '') || context !== (proxy.context || ''));

  useEffect(() => {
    if (proxy) {
      setName(proxy.name || '');
      setDescription(proxy.description || '');
      setVersion(proxy.version || '');
      setContext(proxy.context || '');
    }
  }, [proxy]);

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    if (version.length > MAX_VERSION_LENGTH) return false;
    if (context.length > MAX_CONTEXT_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!proxyId || isReadOnlyProxy) return;

    setIsSubmitting(true);
    try {
      const fullPayload = {
        ...proxy,
        name,
        description: description || undefined,
        version: version || undefined,
        context: context || undefined,
      };
      // Remove read-only fields before sending
      delete (fullPayload as any).createdAt;
      delete (fullPayload as any).createdBy;
      delete (fullPayload as any).updatedAt;

      await updateProxy(fullPayload);

      showSnackbar('Proxy updated successfully', 'success');

      const viewPath = `${proxiesPath}/${proxyId}`;
      navigate(viewPath);
    } catch (err: any) {
      showSnackbar(err?.message || 'Failed to update proxy', 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    const viewPath = `${proxiesPath}/${proxyId}`;
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
          to={proxiesPath}
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
        to={`${proxiesPath}/${proxyId}`}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to Proxy
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit Proxy
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the details for your proxy
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Stack spacing={3}>
            {isReadOnlyProxy ? (
              <Alert severity="info">
                This proxy was created from a gateway. Editing is unavailable
                in AI Workspace.
              </Alert>
            ) : null}
            {isContextOrVersionChanged && (
              <Alert severity="warning">
                You have modified the context or version of this proxy. After
                updating, you will need to redeploy on the gateway for the
                changes to take effect.
              </Alert>
            )}
            <Box sx={{ display: 'flex', gap: 2 }}>
              <FormControl sx={{ flex: 1 }}>
                <FormLabel required>Name</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={name}
                  disabled={isReadOnlyProxy}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Enter proxy name"
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
                  disabled={isReadOnlyProxy}
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
                disabled={isReadOnlyProxy}
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
                disabled={isReadOnlyProxy}
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
          <DisabledActionTooltip disabled={isReadOnlyProxy}>
            <span>
              <Button
                variant="contained"
                onClick={handleSubmit}
                disabled={isReadOnlyProxy || isSubmitting || !isFormValid()}
              >
                {isSubmitting ? 'Updating...' : 'Update'}
              </Button>
            </span>
          </DisabledActionTooltip>
        </Box>
      </Box>
    </PageContent>
  );
}

export default function EditLLMProxy() {
  const { proxyId } = useParams<{ proxyId: string }>();

  if (!proxyId) {
    return (
      <PageContent fullWidth>
        <Alert severity="error">Proxy ID is missing</Alert>
      </PageContent>
    );
  }

  return (
    <ProxyProvider proxyId={proxyId}>
      <EditLLMProxyForm />
    </ProxyProvider>
  );
}
