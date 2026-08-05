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
  Typography,
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
  const [valueTouched, setValueTouched] = useState(false);
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

  const isValueValid = value.trim().length > 0;

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    if (!handle || !isValueValid || isSubmitting) return;

    setIsSubmitting(true);
    try {
      await updateSecret(handle, {
        value,
        name: displayName.trim() || undefined,
        description: description.trim() || undefined,
      });
      if (!isMounted()) return;
      showSnackbar('Secret rotated successfully.', 'success');
      navigate(overviewPath);
    } catch (err) {
      if (!isMounted()) return;
      showSnackbar(getErrorMessage(err, 'Failed to rotate secret.'), 'error');
    } finally {
      if (isMounted()) setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button component={RouterLink} to={overviewPath} size="small" startIcon={<ChevronLeft size={20} />}>
        Back to secret
      </Button>

      {isLoading ? (
        <Box sx={{ mt: 3, maxWidth: 720 }}>
          <Skeleton variant="text" width="30%" height={40} />
          <Skeleton variant="rounded" height={220} sx={{ mt: 2 }} />
        </Box>
      ) : loadError || !secret ? (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={loadError ?? new Error('Secret not found.')} onRetry={fetchSecret} />
        </Box>
      ) : (
        <>
          <Stack spacing={0.5} mt={2} mb={2}>
            <PageTitle>
              <PageTitle.Header>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.secret.RotateSecret.title"
                  defaultMessage="Rotate secret"
                />
              </PageTitle.Header>
            </PageTitle>
          </Stack>

          <Box sx={{ maxWidth: 720 }}>
            <Alert severity="info" sx={{ mb: 3 }}>
              <strong>No redeployment needed.</strong> The handle stays the same, so every reference to it keeps
              working — gateways pick up the new value automatically on their next sync. Rotating a deprecated
              secret reactivates it.
            </Alert>

            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                gap: 0.5,
                p: 1.5,
                mb: 3,
                borderRadius: 1,
                border: 1,
                borderColor: 'divider',
                bgcolor: 'action.hover',
              }}
            >
              <Typography variant="caption" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em' }} color="text.secondary">
                Rotating
              </Typography>
              <Typography sx={{ fontFamily: 'monospace' }}>{secret.id}</Typography>
            </Box>

            <Box component="form" onSubmit={handleSubmit} noValidate>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel required>New value</FormLabel>
                    <TextField
                      fullWidth
                      required
                      type={valueVisible ? 'text' : 'password'}
                      value={value}
                      onChange={(e) => setValue(e.target.value)}
                      onBlur={() => setValueTouched(true)}
                      placeholder="Paste the new credential"
                      autoComplete="new-password"
                      error={valueTouched && !isValueValid}
                      helperText={valueTouched && !isValueValid ? 'A new value is required.' : ''}
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

                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <FormLabel>Display Name (Optional)</FormLabel>
                    <TextField fullWidth value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
                  </FormControl>
                </Grid>

                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <FormLabel>Description (Optional)</FormLabel>
                    <TextField
                      fullWidth
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      placeholder="Reason for rotation, expiry info…"
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
                  disabled={isSubmitting || !isValueValid}
                  data-cyid="rotate-secret-submit"
                >
                  {isSubmitting ? 'Rotating…' : 'Rotate secret'}
                </Button>
              </Box>
            </Box>
          </Box>
        </>
      )}
    </PageContent>
  );
}
