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

import React, { useEffect, useRef, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Skeleton,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { getSecret, updateSecret, type SecretMetadata } from '../../../../apis/secretApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import useIsMounted from '../../../../hooks/useIsMounted';
import { getErrorMessage } from '../../../../utils/apiError';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

export default function RotateSecret(): React.JSX.Element {
  const { handle } = useParams<{ handle: string }>();
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const isMounted = useIsMounted();

  const [secret, setSecret] = useState<SecretMetadata | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<Error | null>(null);

  const [value, setValue] = useState('');
  const [valueVisible, setValueVisible] = useState(false);
  const [displayName, setDisplayName] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const listPath = buildOrgPath(currentOrganization, '/settings/secrets');
  const overviewPath = handle ? `${listPath}/${handle}` : listPath;

  // Guards against out-of-order responses: if the handle changes while a fetch for
  // the previous handle is still in flight, that response must not overwrite the
  // form fields now shown for the newly-active handle — otherwise a submit could
  // send the previous secret's display name/description to the current handle.
  const requestIdRef = useRef(0);

  const fetchSecret = async () => {
    if (!handle) return;
    const requestId = ++requestIdRef.current;
    try {
      setIsLoading(true);
      setLoadError(null);
      const response = await getSecret(handle);
      if (!isMounted() || requestIdRef.current !== requestId) return; // superseded or unmounted
      setSecret(response);
      setDisplayName(response.displayName);
      setDescription(response.description ?? '');
    } catch (err) {
      if (!isMounted() || requestIdRef.current !== requestId) return;
      setLoadError(err instanceof Error ? err : new Error('Failed to load secret.'));
    } finally {
      if (isMounted() && requestIdRef.current === requestId) setIsLoading(false);
    }
  };

  useEffect(() => {
    void fetchSecret();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handle]);

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    if (!handle || isSubmitting) return;

    const trimmedName = displayName.trim();
    if (!trimmedName) {
      showSnackbar('Display name is required.', 'error');
      return;
    }

    setIsSubmitting(true);
    try {
      await updateSecret(handle, {
        value: value.trim(),
        name: trimmedName,
        // Sent even when '' so an explicit clear reaches the backend — unlike
        // name, an empty description is a valid, meaningful value here.
        description: description.trim(),
      });
      if (!isMounted()) return;
      showSnackbar('Secret updated successfully.', 'success');
      navigate(overviewPath);
    } catch (err) {
      if (!isMounted()) return;
      showSnackbar(getErrorMessage(err, 'Failed to update secret.'), 'error');
    } finally {
      if (isMounted()) setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button component={RouterLink} to={overviewPath} size="small" startIcon={<ChevronLeft size={24} />}>
        Back to secret
      </Button>

      {isLoading ? (
        <Box sx={{ mt: 3, maxWidth: 820 }}>
          <Skeleton variant="text" width="30%" height={40} />
          <Skeleton variant="rounded" height={220} sx={{ mt: 2 }} />
        </Box>
      ) : loadError || !secret ? (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={loadError ?? new Error('Secret not found.')} onRetry={fetchSecret} />
        </Box>
      ) : (
        <>
          <Stack spacing={2} mt={2}>
            <PageTitle>
              <PageTitle.Header>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.secret.RotateSecret.title"
                  defaultMessage="Update secret"
                />
              </PageTitle.Header>
            </PageTitle>
          </Stack>

          <Box sx={{ mt: 2, maxWidth: 820 }}>
            <Alert severity="info" sx={{ mb: 3 }}>
              This will get automatically synced to the gateway.
            </Alert>

            <Box component="form" onSubmit={handleSubmit} noValidate>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <FormLabel>Display Name</FormLabel>
                    <TextField fullWidth value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
                  </FormControl>
                </Grid>

                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <FormLabel>Handle</FormLabel>
                    <TextField
                      fullWidth
                      disabled
                      value={secret.id}
                      slotProps={{ input: { style: { fontFamily: 'monospace' } } }}
                      helperText="Immutable after creation."
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
                      placeholder="Reason for rotation, expiry info…"
                    />
                  </FormControl>
                </Grid>

                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel>Value</FormLabel>
                    <TextField
                      fullWidth
                      type={valueVisible ? 'text' : 'password'}
                      value={value}
                      onChange={(e) => setValue(e.target.value)}
                      placeholder="Paste the new credential"
                      autoComplete="new-password"
                      helperText="Leave blank to keep the current value. Providing a new value reactivates a deprecated secret."
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
                      data-cyid="rotate-secret-value-input"
                    />
                  </FormControl>
                </Grid>
              </Grid>

              <Box sx={{ mt: 3, display: 'flex', justifyContent: 'flex-start', gap: 1 }}>
                <Button variant="outlined" component={RouterLink} to={overviewPath} color="secondary" type="button">
                  Cancel
                </Button>
                <Button
                  variant="contained"
                  type="submit"
                  disabled={isSubmitting}
                  data-cyid="rotate-secret-submit"
                >
                  {isSubmitting ? 'Updating…' : 'Update secret'}
                </Button>
              </Box>
            </Box>
          </Box>
        </>
      )}
    </PageContent>
  );
}
