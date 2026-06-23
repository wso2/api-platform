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
import { useNavigate, useParams, Link as RouterLink } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { buildOrgPath } from '../../../../utils/projectRouting';
import type {
  ProviderTemplate,
  UpdateProviderTemplateRequest,
} from '../../../../utils/types';

const MAX_NAME_LENGTH = 80;
const MAX_DESCRIPTION_LENGTH = 200;

function EditProviderTemplateForm({ template }: { template: ProviderTemplate }) {
  const templateId = template.id;
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { updateTemplate } = useProviderTemplates();
  const showSnackbar = useAIWorkspaceSnackbar();

  const overviewPath = `${buildOrgPath(
    currentOrganization,
    '/settings/llm-provider-templates'
  )}/${templateId}`;

  const [name, setName] = useState(template.name ?? '');
  const [description, setDescription] = useState(template.description ?? '');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isNameValid = name.trim().length > 0 && name.length <= MAX_NAME_LENGTH;
  const isDescriptionValid = description.length <= MAX_DESCRIPTION_LENGTH;
  const isFormValid = isNameValid && isDescriptionValid;

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    if (!templateId || !isFormValid || isSubmitting) return;

    const { createdAt, createdBy, updatedAt, ...rest } = template;
    const payload: UpdateProviderTemplateRequest = {
      ...rest,
      id: templateId,
      name: name.trim(),
      description: description.trim() || undefined,
    };

    setIsSubmitting(true);
    try {
      await updateTemplate(templateId, payload);
      showSnackbar('Template updated successfully.', 'success');
      navigate(overviewPath);
    } catch (err) {
      showSnackbar(
        err instanceof Error ? err.message : 'Failed to update template.',
        'error'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={overviewPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.back"
          defaultMessage={'Back'}
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.title"
              defaultMessage={'Edit LLM Provider Template'}
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Card sx={{ p: 2 }}>
            <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
              Details
            </Typography>
            <Grid container spacing={2}>
              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth>
                  <FormLabel required>Name</FormLabel>
                  <TextField
                    fullWidth
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Enter template name"
                    error={name.length > MAX_NAME_LENGTH}
                    helperText={
                      name.length > MAX_NAME_LENGTH
                        ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                        : 'Renaming updates every version of this template.'
                    }
                  />
                </FormControl>
              </Grid>
              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth>
                  <FormLabel>Description</FormLabel>
                  <TextField
                    fullWidth
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Enter description"
                    multiline
                    minRows={3}
                    error={description.length > MAX_DESCRIPTION_LENGTH}
                    helperText={
                      description.length > MAX_DESCRIPTION_LENGTH
                        ? `Description must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH})`
                        : ''
                    }
                  />
                </FormControl>
              </Grid>
            </Grid>
          </Card>

          <Box sx={{ mt: 3, display: 'flex', justifyContent: 'flex-start', gap: 1 }}>
            <Button
              variant="outlined"
              component={RouterLink}
              to={overviewPath}
              color="secondary"
              type="button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.cancel"
                defaultMessage={'Cancel'}
              />
            </Button>
            <Button
              variant="contained"
              type="submit"
              disabled={isSubmitting || !isFormValid}
              data-cyid="edit-provider-template-submit"
            >
              {isSubmitting ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.saving"
                  defaultMessage={'Updating...'}
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.save"
                  defaultMessage={'Update'}
                />
              )}
            </Button>
          </Box>
        </Box>
      </Box>
    </PageContent>
  );
}

export default function EditProviderTemplate() {
  const { templateId } = useParams<{ templateId: string }>();
  const { currentOrganization } = useAppShell();
  const [template, setTemplate] = useState<ProviderTemplate | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const listPath = buildOrgPath(currentOrganization, '/settings');

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId) return;

    let isMounted = true;
    setIsLoading(true);
    setError(null);
    providerTemplateApis
      .getProviderTemplate(templateId, organizationId, PLATFORM_API_BASE_URL)
      .then((full) => {
        if (isMounted) setTemplate(full);
      })
      .catch((err: unknown) => {
        if (isMounted) {
          setError(err instanceof Error ? err.message : 'Failed to load template');
        }
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [templateId, currentOrganization?.uuid]);

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 300,
          }}
        >
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error || !template) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={listPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.back"
            defaultMessage={'Back to list'}
          />
        </Button>
        <Typography variant="h6" sx={{ mt: 2 }}>
          {error ? (
            error
          ) : (
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.EditProviderTemplate.notFound"
              defaultMessage={'Template not found'}
            />
          )}
        </Typography>
      </PageContent>
    );
  }

  return <EditProviderTemplateForm key={template.id} template={template} />;
}
