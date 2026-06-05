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

import { useState, useEffect, useCallback } from 'react';
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
import { useApplications } from '../../../../contexts/ApplicationsContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { applicationApis } from '../../../../apis/applicationApis';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type { Application } from '../../../../utils/types';

const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;

export default function EditApplication() {
  const navigate = useNavigate();
  const { applicationId } = useParams<{ applicationId: string }>();
  const { currentOrganization, currentProject } = useAppShell();
  const { updateApplication, getApplicationById } = useApplications();
  const showSnackbar = useAIWorkspaceSnackbar();
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const isProjectLevel = Boolean(currentProject?.id);
  const applicationsPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/applications')
    : buildOrgPath(currentOrganization, '/applications');

  const [application, setApplication] = useState<Application | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const loadApplication = useCallback(async () => {
    if (!applicationId || !currentOrganization?.uuid) {
      setIsLoading(false);
      return;
    }
    try {
      setIsLoading(true);
      const cached = getApplicationById(applicationId);
      if (cached) {
        setApplication(cached);
        setName(cached.name || '');
        setDescription(cached.description || '');
      }
      const fetched = await applicationApis.getApplication(
        applicationId,
        currentOrganization.uuid,
        apimBaseUrl
      );
      setApplication(fetched);
      setName(fetched.name || '');
      setDescription(fetched.description || '');
    } catch {
      // handled by loading state
    } finally {
      setIsLoading(false);
    }
  }, [
    applicationId,
    currentOrganization?.uuid,
    getApplicationById,
    apimBaseUrl,
  ]);

  useEffect(() => {
    void loadApplication();
  }, [loadApplication]);

  const isFormValid = (): boolean => {
    if (!name || name.trim().length === 0) return false;
    if (name.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    return true;
  };

  const handleSubmit = async () => {
    if (!applicationId || !application) return;

    setIsSubmitting(true);
    try {
      await updateApplication(applicationId, {
        name,
        description: description || undefined,
      });

      showSnackbar('Application updated successfully', 'success');

      const viewPath = `${applicationsPath}/${applicationId}`;
      navigate(viewPath);
    } catch (err: any) {
      showSnackbar(err?.message || 'Failed to update application', 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    const viewPath = `${applicationsPath}/${applicationId}`;
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

  if (!application) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={applicationsPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ mb: 2 }}
        >
          Back to list
        </Button>
        <Alert severity="error">Application not found</Alert>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={`${applicationsPath}/${applicationId}`}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ mb: 2 }}
      >
        Back to Application
      </Button>

      <Box sx={{ maxWidth: 800 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h4" sx={{ mb: 0.5 }}>
            Edit Application
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Update the details for your application
          </Typography>
        </Box>

        <Box sx={{ mb: 4 }}>
          <Stack spacing={3}>
            <FormControl fullWidth>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Enter application name"
                error={name.length > MAX_NAME_LENGTH}
                helperText={
                  name.length > MAX_NAME_LENGTH
                    ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                    : ''
                }
              />
            </FormControl>

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
