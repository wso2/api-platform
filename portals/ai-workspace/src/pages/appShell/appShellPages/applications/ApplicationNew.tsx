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

import React, { useMemo, useState } from 'react';
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
import { useApplications } from '../../../../contexts/ApplicationsContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';

type FormState = {
  name: string;
  description: string;
};

function buildApplicationHandle(name: string, takenIds: Set<string>): string {
  const base = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, '');

  const normalizedBase = base || 'application';
  if (!takenIds.has(normalizedBase)) return normalizedBase;

  let suffix = 2;
  while (takenIds.has(`${normalizedBase}-${suffix}`)) {
    suffix += 1;
  }

  return `${normalizedBase}-${suffix}`;
}

export default function ApplicationNew() {
  const navigate = useNavigate();
  const { applications, createApplication } = useApplications();
  const { currentProject, currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const isProjectLevel = Boolean(currentProject?.id);
  const applicationsPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/applications')
    : buildOrgPath(currentOrganization, '/applications');
  const takenIds = useMemo(
    () => new Set(applications.map((application) => application.id)),
    [applications]
  );

  const [formState, setFormState] = useState<FormState>({
    name: '',
    description: '',
  });
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleCreate = async () => {
    const trimmedName = formState.name.trim();
    if (!trimmedName) return;
    const appHandle = buildApplicationHandle(trimmedName, takenIds);

    try {
      setIsSubmitting(true);

      const newApplication = await createApplication({
        id: appHandle,
        name: trimmedName,
        type: 'genai',
        description: formState.description.trim() || undefined,
        projectId: currentProject?.id,
      });

      showSnackbar('Application created successfully.', 'success');
      navigate(
        isProjectLevel
          ? buildProjectPath(
              currentOrganization,
              currentProject,
              `/applications/${newApplication.id}`
            )
          : buildOrgPath(
              currentOrganization,
              `/applications/${newApplication.id}`
            ),
        {
          state: { applicationAdded: true },
        }
      );
    } catch (error) {
      const description =
        (error as any)?.response?.data?.description ||
        (error as any)?.response?.data?.message ||
        (error instanceof Error ? error.message : null) ||
        'Failed to create application.';
      showSnackbar(description, 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={applicationsPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>Create GenAI Application</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 720 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Name</FormLabel>
              <TextField
                fullWidth
                placeholder="Documentation Assistant"
                value={formState.name}
                onChange={(event) =>
                  setFormState((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }))
                }
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
                placeholder="Short description of the application."
                value={formState.description}
                onChange={(event) =>
                  setFormState((prev) => ({
                    ...prev,
                    description: event.target.value,
                  }))
                }
              />
            </FormControl>
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button
            variant="outlined"
            color="secondary"
            component={RouterLink}
            to={applicationsPath}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={!formState.name.trim() || isSubmitting}
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
