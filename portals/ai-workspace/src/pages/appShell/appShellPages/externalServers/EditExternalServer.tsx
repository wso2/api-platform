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
import { useMemo } from 'react';
import { DisabledActionTooltip } from '../../../../utils/readOnlyArtifacts';

const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;
const MAX_VERSION_LENGTH = 50;
const MAX_CONTEXT_LENGTH = 255;

type ErrorResponse = {
  response?: {
    data?: {
      description?: unknown;
      message?: unknown;
    };
  };
};

function getErrorDescription(error: unknown, fallback: string): string {
  const responseData = (error as ErrorResponse)?.response?.data;
  const description = responseData?.description;
  const message = responseData?.message;

  if (typeof description === 'string' && description.trim()) {
    return description;
  }

  if (typeof message === 'string' && message.trim()) {
    return message;
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return fallback;
}

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
  const [version, setVersion] = useState('');
  const [context, setContext] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const isReadOnlyServer = Boolean(server?.readOnly);

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    const fetchServer = async () => {
      try {
        setIsLoading(true);
        const response = await mcpProxiesApis.getMCPServer(
          serverId,
          organizationId,
          apimBaseUrl
        );
        if (!cancelled) {
          setServer(response);
          setName(response.name || '');
          setDescription(response.description || '');
          setVersion(response.version || '');
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

  const isContextOrVersionChanged =
    server !== null &&
    (version !== (server.version || '') || context !== (server.context || ''));

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    if (version.length > MAX_VERSION_LENGTH) return false;
    if (context.length > MAX_CONTEXT_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!serverId || !server || isReadOnlyServer) return;

    setIsSubmitting(true);
    try {
      const fullPayload = {
        ...server,
        name,
        description: description || undefined,
        version: version || undefined,
        context: context || undefined,
      };
      // Remove read-only fields before sending
      delete (fullPayload as any).createdAt;
      delete (fullPayload as any).updatedAt;

      await mcpProxiesApis.updateMCPServer(
        serverId,
        fullPayload,
        organizationId,
        apimBaseUrl
      );

      showSnackbar('MCP Proxy updated successfully', 'success');

      const viewPath = `${listPath}/${serverId}`;
      navigate(viewPath);
    } catch (err) {
      showSnackbar(
        getErrorDescription(err, 'Failed to update MCP Proxy'),
        'error'
      );
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
                This MCP proxy was created from a gateway. Editing is
                unavailable in AI Workspace.
              </Alert>
            ) : null}
            {isContextOrVersionChanged && (
              <Alert severity="warning">
                You have modified the context or version of this MCP Proxy.
                After updating, you will need to redeploy on the gateway for
                the changes to take effect.
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
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Enter server name"
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
                  disabled={isReadOnlyServer}
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
                disabled={isReadOnlyServer}
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
                disabled={isReadOnlyServer}
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
          <Button variant="outlined" onClick={handleCancel}>
            Cancel
          </Button>
          <DisabledActionTooltip disabled={isReadOnlyServer}>
            <span>
              <Button
                variant="contained"
                onClick={handleSubmit}
                disabled={isReadOnlyServer || isSubmitting || !isFormValid()}
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
