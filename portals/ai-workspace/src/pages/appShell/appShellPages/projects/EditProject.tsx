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

import React, { useEffect, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
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

function EditProjectForm() {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const { currentOrganization, projectsForCurrentOrganization } = useAppShell();
  const { updateProject } = useProjects();
  const showSnackbar = useAIWorkspaceSnackbar();

  const projectsListPath = buildOrgPath(currentOrganization, '/projects/list');

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Pre-fill from the already-loaded project list
  useEffect(() => {
    if (initialized || !projectId) return;
    const project = projectsForCurrentOrganization.find((p) => p.id === projectId);
    if (project) {
      setName(project.name);
      setDescription(project.description ?? '');
      setInitialized(true);
    }
  }, [projectId, projectsForCurrentOrganization, initialized]);

  const handleSave = async () => {
    const trimmedName = name.trim();
    if (!trimmedName || !projectId) return;

    try {
      setIsSubmitting(true);
      await updateProject(projectId, {
        name: trimmedName,
        ...(description.trim() ? { description: description.trim() } : {}),
      });
      showSnackbar('Project updated successfully.', 'success');
      if (projectsListPath) navigate(projectsListPath);
    } catch (error) {
      const message =
        (error as any)?.response?.data?.description ||
        (error as any)?.response?.data?.message ||
        (error instanceof Error ? error.message : null) ||
        'Failed to update project.';
      showSnackbar(message, 'error');
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
          <PageTitle.Header>Edit Project</PageTitle.Header>
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
                onChange={(e) => setName(e.target.value)}
                autoFocus
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
                onChange={(e) => setDescription(e.target.value)}
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
            onClick={handleSave}
            disabled={!name.trim() || isSubmitting}
          >
            {isSubmitting ? (
              <CircularProgress size={18} sx={{ color: 'inherit', mr: 1 }} />
            ) : null}
            Save Changes
          </Button>
        </Box>
      </Box>
    </PageContent>
  );
}

export default function EditProject() {
  return (
    <ProjectsProvider>
      <EditProjectForm />
    </ProjectsProvider>
  );
}
