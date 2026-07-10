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
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type { MCPServer } from '../../../../utils/types';
import { getErrorMessage, getFieldErrors } from '../../../../utils/apiError';
import { useMemo } from 'react';

const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;
const MAX_CONTEXT_LENGTH = 255;

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

// Backend field names (from MCPServer's update payload) mapped onto this form's state keys.
const FIELD_NAME_MAP: Record<string, 'name' | 'description' | 'context'> = {
  displayName: 'name',
  description: 'description',
  context: 'context',
};

export default function EditExternalServer() {
  const navigate = useNavigate();
  const { serverId, projectSlug } = useParams<{
    serverId: string;
    projectSlug: string;
  }>();
  const {
    currentOrganization,
    currentProject,
    projectsForCurrentOrganization,
  } = useAppShell();
  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/mcp-proxy'
  );

  const showSnackbar = useAIWorkspaceSnackbar();

  const [server, setServer] = useState<MCPServer | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [context, setContext] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const isReadOnlyServer = Boolean(server?.readOnly);

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    const fetchServer = async () => {
      try {
        setIsLoading(true);
        const response = await mcpProxiesApis.getMCPServer(
          serverId,
          apimBaseUrl
        );
        if (!cancelled) {
          setServer(response);
          setName(response.displayName || '');
          setDescription(response.description || '');
          setContext(response.context || '');
        }
      } catch {
        // handled by loading state
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };
    fetchServer();
    return () => {
      cancelled = true;
    };
  }, [serverId, organizationId, apimBaseUrl]);

  const isContextChanged =
    server !== null && context !== (server.context || '');

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    if (context.length > MAX_CONTEXT_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    // Allowed even for gateway-created MCP proxies: name/context stay locked
    // (part of the runtime artifact), so only the description can change, which the
    // control plane accepts without altering the gateway runtime artifact.
    if (!serverId || !server) return;

    setIsSubmitting(true);
    setFieldErrors({});
    try {
      const fullPayload = {
        ...server,
        displayName: name,
        description: description || undefined,
        context: context || undefined,
      };
      // Remove read-only fields before sending
      delete (fullPayload as any).createdAt;
      delete (fullPayload as any).updatedAt;

      await mcpProxiesApis.updateMCPServer(
        serverId,
        fullPayload,
        apimBaseUrl
      );

      showSnackbar('MCP Proxy updated successfully', 'success');

      const viewPath = `${listPath}/${serverId}`;
      navigate(viewPath);
    } catch (err) {
      const backendFieldErrors = getFieldErrors(err);
      const mappedErrors: Record<string, string> = {};
      let hasUnmapped = false;
      backendFieldErrors?.forEach(({ field, message }) => {
        const formField = FIELD_NAME_MAP[field];
        if (formField) {
          mappedErrors[formField] = message;
        } else {
          hasUnmapped = true;
        }
      });
      if (Object.keys(mappedErrors).length > 0) {
        setFieldErrors(mappedErrors);
      }
      if (hasUnmapped || Object.keys(mappedErrors).length === 0) {
        showSnackbar(
          getErrorDescription(err, 'Failed to update MCP Proxy'),
          'error'
        );
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    const viewPath = `${listPath}/${serverId}`;
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

  if (!server) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={listPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ mb: 2 }}
        >
          Back to list
        </Button>
        <Alert severity="error">MCP Proxy not found</Alert>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={`${listPath}/${serverId}`}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to MCP Proxy
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit MCP Proxy
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the details for your MCP Proxy
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Stack spacing={3}>
            {isReadOnlyServer ? (
              <Alert severity="info">
                This MCP proxy was created from a gateway. The name and
                context are part of the gateway runtime configuration and are
                read-only here; only the description can be edited.
              </Alert>
            ) : null}
            {isContextChanged && (
              <Alert severity="warning">
                You have modified the context of this MCP Proxy. After
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
                  disabled={isReadOnlyServer}
                  onChange={(e) => {
                    setName(e.target.value);
                    setFieldErrors((prev) => ({ ...prev, name: '' }));
                  }}
                  placeholder="Enter server name"
                  error={name.length > MAX_NAME_LENGTH || Boolean(fieldErrors.name)}
                  helperText={
                    fieldErrors.name ||
                    (name.length > MAX_NAME_LENGTH
                      ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                      : '')
                  }
                />
              </FormControl>
            </Box>

            <FormControl fullWidth>
              <FormLabel>Description</FormLabel>
              <TextField
                fullWidth
                value={description}
                onChange={(e) => {
                  setDescription(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, description: '' }));
                }}
                placeholder="Enter description"
                multiline
                minRows={2}
                error={description.length > MAX_DESCRIPTION_LENGTH || Boolean(fieldErrors.description)}
                helperText={
                  fieldErrors.description ||
                  (description.length > MAX_DESCRIPTION_LENGTH
                    ? `Description must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH})`
                    : '')
                }
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel>Context</FormLabel>
              <TextField
                fullWidth
                value={context}
                disabled={isReadOnlyServer}
                onChange={(e) => {
                  setContext(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, context: '' }));
                }}
                placeholder="Enter context path"
                error={context.length > MAX_CONTEXT_LENGTH || Boolean(fieldErrors.context)}
                helperText={
                  fieldErrors.context ||
                  (context.length > MAX_CONTEXT_LENGTH
                    ? `Context must not exceed ${MAX_CONTEXT_LENGTH} characters (${context.length}/${MAX_CONTEXT_LENGTH})`
                    : '')
                }
              />
            </FormControl>
          </Stack>
        </Box>

        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" onClick={handleCancel}>
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
