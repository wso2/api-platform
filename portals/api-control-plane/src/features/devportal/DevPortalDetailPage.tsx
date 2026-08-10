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

import { useEffect, useState } from 'react';
import {
  Button,
  Card,
  CardContent,
  Chip,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Link, useParams } from 'react-router-dom';

import { useDevPortal, useUpdateDevPortal } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import type { DevPortal, DevPortalAuthType } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import {
  AUTH_TYPE_OPTIONS,
  STATUS_CHIP_COLOR,
  STATUS_LABEL,
} from './devPortalDisplay';
import { IdpCredentialsFields } from './IdpCredentialsFields';

export function DevPortalDetailPage() {
  const { orgHandle = '', devPortalId = '' } = useParams();
  const { notify } = useNotifications();
  const devPortalQuery = useDevPortal(orgHandle, devPortalId);
  const updateDevPortal = useUpdateDevPortal(orgHandle, devPortalId);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [url, setUrl] = useState('');
  const [authType, setAuthType] = useState<DevPortalAuthType>('local');
  const [stsTokenUrl, setStsTokenUrl] = useState('');
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [seededId, setSeededId] = useState<string>();

  // Seed the editable fields once per loaded record — a poll/refetch after
  // save must not clobber in-progress edits, so this only re-seeds when the
  // devportal id itself changes (i.e. navigating to a different one).
  useEffect(() => {
    if (!devPortalQuery.data || devPortalQuery.data.id === seededId) return;
    const devPortal = devPortalQuery.data;
    setName(devPortal.name);
    setDescription(devPortal.description || '');
    setUrl(devPortal.url || '');
    setAuthType(devPortal.authType);
    setStsTokenUrl('');
    setClientId('');
    setClientSecret('');
    setSeededId(devPortal.id);
  }, [devPortalQuery.data, seededId]);

  if (devPortalQuery.isLoading) {
    return <LoadingState label="Loading devportal" />;
  }
  if (devPortalQuery.error) {
    return <ErrorState message="Unable to load devportal" />;
  }
  if (!devPortalQuery.data) {
    return <ErrorState title="Devportal not found" />;
  }

  const devPortal: DevPortal = devPortalQuery.data;
  const isIdpAuth = authType === 'idp_client_credentials';
  // Credentials are never returned by the backend (write-only), so an
  // already-idp devportal always reloads with these three fields blank.
  // Once idp is already the active auth type, each field updates
  // independently — leaving one blank keeps its existing value, so changing
  // just the secret (say) doesn't require re-entering the other two. Only
  // when switching into idp from a different auth type is there no existing
  // credential to fall back to, so all three are required in that case.
  const switchingToIdp = isIdpAuth && devPortal.authType !== 'idp_client_credentials';
  const stsTokenUrlEntered = stsTokenUrl.trim() !== '';
  const clientIdEntered = clientId.trim() !== '';
  const clientSecretEntered = clientSecret.trim() !== '';
  const anyCredentialEntered =
    stsTokenUrlEntered || clientIdEntered || clientSecretEntered;
  const idpFieldsValid =
    !isIdpAuth ||
    !switchingToIdp ||
    (stsTokenUrlEntered && clientIdEntered && clientSecretEntered);
  // Gate on the fields actually having changed from the loaded record — not
  // just "is currently valid" — otherwise Save starts enabled on page load
  // with zero edits. Only compare once this record's fields have been seeded
  // (seededId === devPortal.id), so the one render before the seeding effect
  // runs can't momentarily read as dirty.
  const isDirty =
    seededId === devPortal.id &&
    (name !== devPortal.name ||
      description !== (devPortal.description || '') ||
      url !== (devPortal.url || '') ||
      authType !== devPortal.authType ||
      anyCredentialEntered);
  const canSave =
    isDirty &&
    name.trim() !== '' &&
    url.trim() !== '' &&
    idpFieldsValid &&
    !updateDevPortal.isPending;

  const save = () => {
    updateDevPortal.mutate(
      {
        name,
        url: url.trim(),
        authType,
        description: description || undefined,
        // Only the fields the user actually typed into are sent — an
        // untouched field must never overwrite the existing value with ''.
        ...(isIdpAuth && stsTokenUrlEntered
          ? { stsTokenUrl: stsTokenUrl.trim() }
          : {}),
        ...(isIdpAuth && clientIdEntered ? { clientId: clientId.trim() } : {}),
        ...(isIdpAuth && clientSecretEntered ? { clientSecret } : {}),
      },
      {
        onSuccess: (updated) => {
          notify(`Devportal "${updated.name}" updated.`, 'success');
          setStsTokenUrl('');
          setClientId('');
          setClientSecret('');
        },
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Failed to update devportal',
            'error'
          ),
      }
    );
  };

  const cancel = () => {
    setName(devPortal.name);
    setDescription(devPortal.description || '');
    setUrl(devPortal.url || '');
    setAuthType(devPortal.authType);
    setStsTokenUrl('');
    setClientId('');
    setClientSecret('');
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.devportal(orgHandle)}>
          <PageTitle.BackButton>Back to Dev Portal</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>{devPortal.name}</PageTitle.Header>
        <PageTitle.SubHeader>{devPortal.url || devPortal.handle}</PageTitle.SubHeader>
      </PageTitle>

      <Grid container spacing={3}>
        {/* Editable form */}
        <Grid size={{ xs: 12, md: 8 }}>
          <Card>
            <CardContent>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                Devportal settings
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
                Update the devportal's details and authentication.
              </Typography>

              <Stack spacing={3} sx={{ mt: 3 }}>
                <FormControl fullWidth>
                  <FormLabel>Name</FormLabel>
                  <TextField
                    onChange={(event) => setName(event.target.value)}
                    value={name}
                  />
                </FormControl>

                <FormControl fullWidth>
                  <FormLabel>Description (optional)</FormLabel>
                  <TextField
                    multiline
                    minRows={2}
                    onChange={(event) => setDescription(event.target.value)}
                    value={description}
                  />
                </FormControl>

                <FormControl fullWidth>
                  <FormLabel>Authentication</FormLabel>
                  <Select
                    onChange={(event) =>
                      setAuthType(event.target.value as DevPortalAuthType)
                    }
                    size="small"
                    value={authType}
                  >
                    {AUTH_TYPE_OPTIONS.map((option) => (
                      <MenuItem key={option.value} value={option.value}>
                        {option.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>

                {isIdpAuth && (
                  <Stack spacing={1}>
                    {!switchingToIdp && (
                      <Typography color="text.secondary" variant="caption">
                        Credentials are never displayed after saving. Leave a
                        field blank to keep its existing value — fill in only
                        the ones you want to change.
                      </Typography>
                    )}
                    <IdpCredentialsFields
                      clientId={clientId}
                      clientSecret={clientSecret}
                      onClientIdChange={setClientId}
                      onClientSecretChange={setClientSecret}
                      onStsTokenUrlChange={setStsTokenUrl}
                      stsTokenUrl={stsTokenUrl}
                    />
                  </Stack>
                )}

                <FormControl fullWidth>
                  <FormLabel>URL</FormLabel>
                  <TextField
                    onChange={(event) => setUrl(event.target.value)}
                    placeholder="https://devportal.example.com"
                    value={url}
                  />
                </FormControl>

                <Stack direction="row" justifyContent="flex-end" spacing={1.5}>
                  <Button
                    disabled={!isDirty || updateDevPortal.isPending}
                    onClick={cancel}
                    variant="outlined"
                  >
                    Cancel
                  </Button>
                  <Button disabled={!canSave} onClick={save} variant="contained">
                    {updateDevPortal.isPending ? 'Saving…' : 'Save changes'}
                  </Button>
                </Stack>
              </Stack>
            </CardContent>
          </Card>
        </Grid>

        {/* Details */}
        <Grid size={{ xs: 12, md: 4 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography sx={{ fontWeight: 700, mb: 1 }} variant="h6">
                Details
              </Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 2 }}>
                <Chip
                  color={STATUS_CHIP_COLOR[devPortal.workflowStatus]}
                  label={STATUS_LABEL[devPortal.workflowStatus]}
                  size="small"
                />
              </Stack>
              <Typography color="text.secondary" variant="body2">
                Handle: {devPortal.handle}
              </Typography>
              {devPortal.createdAt && (
                <Typography
                  color="text.secondary"
                  sx={{ display: 'block', mt: 2 }}
                  variant="caption"
                >
                  Created {relativeTime(devPortal.createdAt)}
                </Typography>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </PageContent>
  );
}
