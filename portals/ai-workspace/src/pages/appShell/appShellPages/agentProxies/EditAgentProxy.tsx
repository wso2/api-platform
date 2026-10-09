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

import { useState, useEffect, useMemo } from 'react';
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
  AgentProxyProvider,
  useAgentProxy,
} from '../../../../contexts/agentProxy';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage, getFieldErrors } from '../../../../utils/apiError';

const MAX_NAME_LENGTH = 128;
const MAX_DESCRIPTION_LENGTH = 1023;
const MAX_CONTEXT_LENGTH = 200;

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

// Backend field names (from the AgentProxy update payload) mapped onto this form's state keys.
const FIELD_NAME_MAP: Record<
  string,
  'name' | 'description' | 'context'
> = {
  displayName: 'name',
  description: 'description',
  context: 'context',
};

function EditAgentProxyForm() {
  const navigate = useNavigate();
  const { agentProxyId, projectSlug } = useParams<{
    agentProxyId: string;
    projectSlug: string;
  }>();
  const { currentOrganization, currentProject, projectsForCurrentOrganization } =
    useAppShell();
  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/agent-proxy'
  );

  const showSnackbar = useAIWorkspaceSnackbar();

  const { agentProxy, isLoading, updateAgentProxy } = useAgentProxy();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [context, setContext] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const isReadOnlyAgentProxy = Boolean(agentProxy?.readOnly);

  useEffect(() => {
    if (!agentProxy) return;
    setName(agentProxy.displayName || '');
    setDescription(agentProxy.description || '');
    setContext(agentProxy.context || '');
  }, [agentProxy]);

  const isContextChanged =
    agentProxy !== null && context !== (agentProxy.context || '');

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    if (context.length > MAX_CONTEXT_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!agentProxyId || !agentProxy) return;

    setIsSubmitting(true);
    setFieldErrors({});
    try {
      // PUT replaces the resource, so the loaded proxy is sent back with only
      // these fields changed. Read-only fields are stripped first.
      const {
        createdAt,
        createdBy,
        updatedAt,
        updatedBy,
        readOnly,
        ...rest
      } = agentProxy;

      await updateAgentProxy({
        ...rest,
        displayName: name,
        description: description || undefined,
        context: context || undefined,
      });

      showSnackbar('Agent Proxy updated successfully', 'success');
      navigate(`${listPath}/${agentProxyId}`);
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
          getErrorDescription(err, 'Failed to update Agent Proxy'),
          'error'
        );
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    navigate(`${listPath}/${agentProxyId}`);
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

  if (!agentProxy) {
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
        <Alert severity="error">Agent Proxy not found</Alert>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={`${listPath}/${agentProxyId}`}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to Agent Proxy
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit Agent Proxy
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the details for your Agent Proxy
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Stack spacing={3}>
            {isReadOnlyAgentProxy ? (
              <Alert severity="info">
                This agent proxy was created from a gateway. The name and
                context are part of the gateway runtime configuration and are
                read-only here; only the description can be edited.
              </Alert>
            ) : null}
            {isContextChanged && (
              <Alert severity="warning">
                You have modified the context of this Agent Proxy. After
                updating, you will need to redeploy on the gateway for the
                changes to take effect.
              </Alert>
            )}
            <FormControl fullWidth>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                required
                value={name}
                disabled={isReadOnlyAgentProxy}
                onChange={(e) => {
                  setName(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, name: '' }));
                }}
                placeholder="Enter agent proxy name"
                error={
                  name.length > MAX_NAME_LENGTH || Boolean(fieldErrors.name)
                }
                helperText={
                  fieldErrors.name ||
                  (name.length > MAX_NAME_LENGTH
                    ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                    : '')
                }
              />
            </FormControl>

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
                error={
                  description.length > MAX_DESCRIPTION_LENGTH ||
                  Boolean(fieldErrors.description)
                }
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
                disabled={isReadOnlyAgentProxy}
                onChange={(e) => {
                  setContext(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, context: '' }));
                }}
                placeholder="Enter context path"
                error={
                  context.length > MAX_CONTEXT_LENGTH ||
                  Boolean(fieldErrors.context)
                }
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

export default function EditAgentProxy() {
  const { agentProxyId } = useParams<{ agentProxyId: string }>();

  if (!agentProxyId) {
    return (
      <PageContent fullWidth>
        <Alert severity="error">Agent Proxy ID is missing</Alert>
      </PageContent>
    );
  }

  return (
    <AgentProxyProvider agentProxyId={agentProxyId}>
      <EditAgentProxyForm />
    </AgentProxyProvider>
  );
}
