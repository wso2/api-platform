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
  Box,
  Button,
  Card,
  CardContent,
  FormControl,
  FormLabel,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useApiPortal, useUpdateApiPortal } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import type {
  ApiPortal,
  ApiPortalAuthType,
  UpdateApiPortalInput,
} from '../../types/domain';
import { isValidUrl } from '../apis/develop/developEdit';
import { AUTH_TYPE_OPTIONS } from './apiPortalDisplay';
import { IdpCredentialsFields } from './IdpCredentialsFields';

export function ApiPortalEditPage() {
  const { orgHandle = '', apiPortalId = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const apiPortalQuery = useApiPortal(orgHandle, apiPortalId);
  const updateApiPortal = useUpdateApiPortal(orgHandle, apiPortalId);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [url, setUrl] = useState('');
  const [authType, setAuthType] = useState<ApiPortalAuthType>('local');
  const [stsTokenUrl, setStsTokenUrl] = useState('');
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [seededId, setSeededId] = useState<string>();

  // Seed the editable fields once per loaded record. A poll/refetch after
  // save must not clobber in-progress edits, so this only re-seeds when the
  // API Portal id itself changes (i.e. navigating to a different one).
  useEffect(() => {
    if (!apiPortalQuery.data || apiPortalQuery.data.id === seededId) return;
    const apiPortal = apiPortalQuery.data;
    setName(apiPortal.name);
    setDescription(apiPortal.description || '');
    setUrl(apiPortal.url || '');
    setAuthType(apiPortal.authType);
    // stsTokenUrl/clientId aren't secret and are returned by the backend, so
    // they seed from the record like any other field. clientSecret is the
    // one write-only field — it always seeds blank.
    setStsTokenUrl(apiPortal.authConfig?.stsTokenUrl || '');
    setClientId(apiPortal.authConfig?.clientId || '');
    setClientSecret('');
    setSeededId(apiPortal.id);
  }, [apiPortalQuery.data, seededId]);

  if (apiPortalQuery.isLoading) {
    return <LoadingState label="Loading API Portal" />;
  }
  if (apiPortalQuery.error) {
    return <ErrorState message="Unable to load API Portal" />;
  }
  if (!apiPortalQuery.data) {
    return <ErrorState title="API Portal not found" />;
  }

  const apiPortal: ApiPortal = apiPortalQuery.data;
  const isOAuth2 = authType === 'oauth2';
  // stsTokenUrl/clientId aren't secret — they're stored/returned and behave
  // like any other required field once oauth2 is active. clientSecret is the
  // one write-only field: it's never returned, so blank means "keep the
  // existing secret" whenever one already exists — only when switching into
  // oauth2 from a different auth type is there no existing secret to fall
  // back to, so it's required in that case.
  const switchingToOAuth2 = isOAuth2 && apiPortal.authType !== 'oauth2';
  const urlValid = url.trim() !== '' && isValidUrl(url);
  const stsTokenUrlEntered = stsTokenUrl.trim() !== '';
  const stsTokenUrlValid = stsTokenUrlEntered && isValidUrl(stsTokenUrl);
  const clientIdEntered = clientId.trim() !== '';
  const clientSecretEntered = clientSecret.trim() !== '';
  const idpFieldsValid =
    !isOAuth2 ||
    (stsTokenUrlValid &&
      clientIdEntered &&
      (!switchingToOAuth2 || clientSecretEntered));
  const isDirty =
    seededId === apiPortal.id &&
    (name !== apiPortal.name ||
      description !== (apiPortal.description || '') ||
      url !== (apiPortal.url || '') ||
      authType !== apiPortal.authType ||
      stsTokenUrl !== (apiPortal.authConfig?.stsTokenUrl || '') ||
      clientId !== (apiPortal.authConfig?.clientId || '') ||
      clientSecretEntered);
  const canSave =
    isDirty &&
    name.trim() !== '' &&
    urlValid &&
    idpFieldsValid &&
    !updateApiPortal.isPending;

  const goBackToDetail = () =>
    navigate(routes.apiPortalDetail(orgHandle, apiPortal.id));

  const save = () => {
    const basePayload = {
      name: name.trim(),
      url: url.trim(),
      description: description || undefined,
    };
    const input: UpdateApiPortalInput = isOAuth2
      ? {
          ...basePayload,
          authType: 'oauth2',
          authConfig: {
            stsTokenUrl: stsTokenUrl.trim(),
            clientId: clientId.trim(),
            // Only sent when the user actually typed a new one — blank
            // must never overwrite the existing secret with ''.
            ...(clientSecretEntered ? { clientSecret } : {}),
          },
        }
      : { ...basePayload, authType: 'local' };
    updateApiPortal.mutate(input, {
      onSuccess: (updated) => {
        notify(`API Portal "${updated.name}" updated.`, 'success');
        navigate(routes.apiPortalDetail(orgHandle, updated.id));
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Failed to update API Portal',
          'error'
        ),
    });
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.apiPortalDetail(orgHandle, apiPortal.id)}>
          <PageTitle.BackButton>Back to {apiPortal.name}</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>Edit API Portal</PageTitle.Header>
        <PageTitle.SubHeader>
          Update details and outbound authentication for {apiPortal.name}.
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ maxWidth: 720 }}>
        <Card>
          <CardContent>
            <Stack spacing={3}>
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
                <FormLabel>URL</FormLabel>
                <TextField
                  error={url !== '' && !isValidUrl(url)}
                  helperText={
                    url !== '' && !isValidUrl(url) ? 'Enter a valid URL' : undefined
                  }
                  onChange={(event) => setUrl(event.target.value)}
                  placeholder="https://api-portal.example.com"
                  value={url}
                />
              </FormControl>

              <FormControl fullWidth>
                <FormLabel>Authentication</FormLabel>
                <Select
                  onChange={(event) =>
                    setAuthType(event.target.value as ApiPortalAuthType)
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

              {isOAuth2 && (
                <Stack spacing={1}>
                  {!switchingToOAuth2 && (
                    <Typography color="text.secondary" variant="caption">
                      Client secret is never displayed after saving — leave it
                      blank to keep the existing one, or enter a new value to
                      replace it.
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

              <Stack direction="row" justifyContent="flex-end" spacing={1.5}>
                <Button
                  disabled={updateApiPortal.isPending}
                  onClick={goBackToDetail}
                  variant="outlined"
                >
                  Cancel
                </Button>
                <Button disabled={!canSave} onClick={save} variant="contained">
                  {updateApiPortal.isPending ? 'Saving…' : 'Save changes'}
                </Button>
              </Stack>
            </Stack>
          </CardContent>
        </Card>
      </Box>
    </PageContent>
  );
}
