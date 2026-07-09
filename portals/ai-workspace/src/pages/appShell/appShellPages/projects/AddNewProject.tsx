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

import React, { useState } from 'react';
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { ProjectsProvider, useProjects } from '../../../../contexts/ProjectsContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage, getFieldErrors, fieldErrorsToMap } from '../../../../utils/apiError';

// Backend field names (from CreateProjectRequest) mapped onto this form's state keys.
const FIELD_NAME_MAP: Record<string, 'name' | 'description'> = {
  displayName: 'name',
  description: 'description',
};

function AddNewProjectForm() {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { createProject } = useProjects();
  const showSnackbar = useAIWorkspaceSnackbar();

  const projectsListPath = buildOrgPath(currentOrganization, '/projects/list');

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const handleCreate = async () => {
    const trimmedName = name.trim();
    if (!trimmedName) return;

    try {
      setIsSubmitting(true);
      setFieldErrors({});
      await createProject({
        displayName: trimmedName,
        ...(description.trim() ? { description: description.trim() } : {}),
      });
      showSnackbar('Project created successfully.', 'success');
      if (projectsListPath) navigate(projectsListPath);
    } catch (error) {
      const backendFieldErrors = getFieldErrors(error);
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
        const message = getErrorMessage(error, 'Failed to create project.');
        showSnackbar(message, 'error');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={projectsListPath ?? ''}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>Create New Project</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 720 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Name</FormLabel>
              <TextField
                fullWidth
                placeholder="My AI Project"
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, name: '' }));
                }}
                autoFocus
                error={Boolean(fieldErrors.name)}
                helperText={fieldErrors.name}
                data-cyid="project-name-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Description</FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={3}
                placeholder="Short description of the project."
                value={description}
                onChange={(e) => {
                  setDescription(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, description: '' }));
                }}
                error={Boolean(fieldErrors.description)}
                helperText={fieldErrors.description}
                data-cyid="project-description-input"
              />
            </FormControl>
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button
            variant="outlined"
            color="secondary"
            component={RouterLink}
            to={projectsListPath ?? ''}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={!name.trim() || isSubmitting}
            data-cyid="create-project-button"
          >
            {isSubmitting ? (
              <CircularProgress size={18} sx={{ color: 'inherit', mr: 1 }} />
            ) : null}
            Create
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}

export default function AddNewProject() {
  return (
    <ProjectsProvider>
      <AddNewProjectForm />
    </ProjectsProvider>
  );
}
