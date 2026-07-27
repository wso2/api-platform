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
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { createSecret, type SecretType } from '../../../../apis/secretApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage } from '../../../../utils/apiError';

const MAX_NAME_LENGTH = 120;
const MAX_DESCRIPTION_LENGTH = 300;
const HANDLE_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;

function toHandle(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default function CreateSecret(): React.JSX.Element {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleTouched, setHandleTouched] = useState(false);
  const [description, setDescription] = useState('');
  const [value, setValue] = useState('');
  const [valueVisible, setValueVisible] = useState(false);
  const [type, setType] = useState<SecretType>('GENERIC');
  const [nameTouched, setNameTouched] = useState(false);
  const [valueTouched, setValueTouched] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const listPath = buildOrgPath(currentOrganization, '/settings/secrets');

  const handleNameChange = (nextName: string) => {
    setDisplayName(nextName);
    if (!handleTouched) setHandle(toHandle(nextName));
  };

  const isNameValid = displayName.trim().length > 0 && displayName.length <= MAX_NAME_LENGTH;
  const isDescriptionValid = description.length <= MAX_DESCRIPTION_LENGTH;
  const isHandleValid = HANDLE_PATTERN.test(handle);
  const isValueValid = value.trim().length > 0;
  const isFormValid = isNameValid && isHandleValid && isDescriptionValid && isValueValid;

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    if (!isFormValid || isSubmitting) return;

    setIsSubmitting(true);
    try {
      const secret = await createSecret({
        id: handle,
        displayName: displayName.trim(),
        description: description.trim() || undefined,
        value,
        type,
      });
      showSnackbar('Secret created successfully.', 'success');
      navigate(`${listPath}/${secret.id}`);
    } catch (err) {
      showSnackbar(getErrorMessage(err, 'Failed to create secret.'), 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button component={RouterLink} to={listPath} size="small" startIcon={<ChevronLeft size={20} />}>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.secret.CreateSecret.back"
          defaultMessage="Back to Secrets"
        />
      </Button>

      <Stack spacing={0.5} mt={2} mb={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.secret.CreateSecret.title"
              defaultMessage="Create a secret"
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ maxWidth: 820 }}>
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6 }}>
              <FormControl fullWidth>
                <FormLabel required>Display Name</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={displayName}
                  onChange={(e) => handleNameChange(e.target.value)}
                  onBlur={() => setNameTouched(true)}
                  placeholder="e.g. WSO2 OpenAI API Key"
                  error={nameTouched && !isNameValid}
                  helperText={
                    nameTouched && displayName.trim().length === 0
                      ? 'Display name is required.'
                      : displayName.length > MAX_NAME_LENGTH
                        ? `Must not exceed ${MAX_NAME_LENGTH} characters.`
                        : ''
                  }
                  data-cyid="secret-name-input"
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, sm: 6 }}>
              <FormControl fullWidth>
                <FormLabel required>Handle</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={handle}
                  onChange={(e) => {
                    setHandleTouched(true);
                    setHandle(e.target.value.toLowerCase());
                  }}
                  placeholder="wso2-openai-key"
                  slotProps={{ input: { style: { fontFamily: 'monospace' } } }}
                  error={handle.length > 0 && !isHandleValid}
                  helperText={
                    handle.length > 0 && !isHandleValid
                      ? 'Lowercase letters, numbers, and single hyphens only.'
                      : 'Immutable after creation — referenced as {{ secret "' + (handle || 'handle') + '" }}.'
                  }
                  data-cyid="secret-handle-input"
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel>Description (Optional)</FormLabel>
                <TextField
                  fullWidth
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="e.g. Gemini 1.5 Pro — production project"
                  error={!isDescriptionValid}
                  helperText={
                    !isDescriptionValid
                      ? `Must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH}).`
                      : ''
                  }
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, sm: 8 }}>
              <FormControl fullWidth>
                <FormLabel required>Value</FormLabel>
                <TextField
                  fullWidth
                  required
                  type={valueVisible ? 'text' : 'password'}
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  onBlur={() => setValueTouched(true)}
                  placeholder="Paste your credential here"
                  error={valueTouched && !isValueValid}
                  helperText={
                    valueTouched && !isValueValid
                      ? 'A value is required.'
                      : 'Encrypted immediately on save — this is the only time it can be viewed. It is never returned by the API again.'
                  }
                  slotProps={{
                    input: {
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            size="small"
                            onClick={() => setValueVisible((v) => !v)}
                            aria-label={valueVisible ? 'Hide value' : 'Show value'}
                          >
                            {valueVisible ? <EyeOff size={18} /> : <Eye size={18} />}
                          </IconButton>
                        </InputAdornment>
                      ),
                    },
                  }}
                  data-cyid="secret-value-input"
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, sm: 4 }}>
              <FormControl fullWidth>
                <FormLabel>Type</FormLabel>
                <Select value={type} onChange={(e) => setType(e.target.value as SecretType)}>
                  <MenuItem value="GENERIC">Generic</MenuItem>
                  <MenuItem value="CERTIFICATE">Certificate</MenuItem>
                </Select>
              </FormControl>
            </Grid>
          </Grid>

          <Box sx={{ mt: 3, display: 'flex', justifyContent: 'flex-start', gap: 1 }}>
            <Button variant="outlined" component={RouterLink} to={listPath} color="secondary" type="button">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.secret.CreateSecret.cancel"
                defaultMessage="Cancel"
              />
            </Button>
            <Button
              variant="contained"
              type="submit"
              disabled={isSubmitting || !isFormValid}
              data-cyid="create-secret-submit"
            >
              {isSubmitting ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.secret.CreateSecret.creating"
                  defaultMessage="Creating…"
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.secret.CreateSecret.create"
                  defaultMessage="Create secret"
                />
              )}
            </Button>
          </Box>
        </Box>
      </Box>
    </PageContent>
  );
}
