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

import { useState } from 'react';
import {
  Box,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import { Pencil } from '@wso2/oxygen-ui-icons-react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateDevPortal } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { routes } from '../../routes/paths';
import type { DevPortalAuthType } from '../../types/domain';
import { isValidUrl } from '../apis/develop/developEdit';
import { AUTH_TYPE_OPTIONS } from './devPortalDisplay';
import { IdpCredentialsFields } from './IdpCredentialsFields';

const HANDLE_PATTERN = /^[a-z0-9-]{3,64}$/;

const slugify = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);

export function DevPortalCreatePage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const createDevPortal = useCreateDevPortal();

  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);
  // Auto-derived from Name and locked by default; the identifier is
  // permanent once the devportal is created, so editing it directly is an
  // explicit, deliberate action rather than something you fall into while
  // typing the Name.
  const [handleLocked, setHandleLocked] = useState(true);
  const [description, setDescription] = useState('');
  const [authType, setAuthType] = useState<DevPortalAuthType>(
    AUTH_TYPE_OPTIONS[0].value
  );
  const [url, setUrl] = useState('');
  const [stsTokenUrl, setStsTokenUrl] = useState('');
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');

  const onDisplayNameChange = (value: string) => {
    setDisplayName(value);
    if (!handleEdited) setHandle(slugify(value));
  };

  const handleValid = HANDLE_PATTERN.test(handle);
  const urlValid = url.trim() !== '' && isValidUrl(url);
  const isIdpAuth = authType === 'idp_client_credentials';
  const idpFieldsValid =
    !isIdpAuth ||
    (stsTokenUrl.trim() !== '' &&
      isValidUrl(stsTokenUrl) &&
      clientId.trim() !== '' &&
      clientSecret.trim() !== '');
  const canSubmit =
    displayName.trim() !== '' &&
    handleValid &&
    urlValid &&
    idpFieldsValid &&
    !createDevPortal.isPending;

  const submit = () => {
    createDevPortal.mutate(
      {
        name: displayName,
        handle,
        url: url.trim(),
        authType,
        description: description || undefined,
        ...(isIdpAuth
          ? {
              stsTokenUrl: stsTokenUrl.trim(),
              clientId: clientId.trim(),
              clientSecret,
            }
          : {}),
      },
      {
        onSuccess: (devPortal) => {
          notify(`Devportal "${devPortal.name}" provisioned.`, 'success');
          navigate(routes.devportal(orgHandle));
        },
        onError: (error) =>
          notify(
            error instanceof Error
              ? error.message
              : 'Failed to provision devportal',
            'error'
          ),
      }
    );
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.devportal(orgHandle)}>
          <PageTitle.BackButton>Back to Dev Portal</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>Provision a devportal</PageTitle.Header>
        <PageTitle.SubHeader>
          Register a developer portal, then connect it to the platform.
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ maxWidth: 720 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <FormLabel>Name</FormLabel>
            <TextField
              onChange={(event) => onDisplayNameChange(event.target.value)}
              placeholder="Production Devportal"
              value={displayName}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Identifier</FormLabel>
            <TextField
              disabled={handleLocked}
              error={handle !== '' && !handleValid}
              helperText={
                handle !== '' && !handleValid
                  ? 'Lowercase letters, numbers, hyphens only; 3–64 chars.'
                  : undefined
              }
              onChange={(event) => {
                setHandleEdited(true);
                setHandle(event.target.value);
              }}
              placeholder="prod-devportal"
              slotProps={{
                input: {
                  endAdornment: handleLocked && (
                    <InputAdornment position="end">
                      <Tooltip title="Edit identifier">
                        <IconButton
                          aria-label="Edit identifier"
                          onClick={() => setHandleLocked(false)}
                          size="small"
                        >
                          <Pencil size={16} />
                        </IconButton>
                      </Tooltip>
                    </InputAdornment>
                  ),
                },
              }}
              value={handle}
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
            <IdpCredentialsFields
              clientId={clientId}
              clientSecret={clientSecret}
              onClientIdChange={setClientId}
              onClientSecretChange={setClientSecret}
              onStsTokenUrlChange={setStsTokenUrl}
              stsTokenUrl={stsTokenUrl}
            />
          )}

          <FormControl fullWidth>
            <FormLabel>URL</FormLabel>
            <TextField
              error={url !== '' && !isValidUrl(url)}
              helperText={
                url !== '' && !isValidUrl(url) ? 'Enter a valid URL' : undefined
              }
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://devportal.example.com"
              value={url}
            />
          </FormControl>

          <Stack direction="row" justifyContent="flex-end" spacing={1.5}>
            <Button
              component={Link}
              to={routes.devportal(orgHandle)}
              variant="outlined"
            >
              Cancel
            </Button>
            <Button disabled={!canSubmit} onClick={submit} variant="contained">
              {createDevPortal.isPending
                ? 'Provisioning…'
                : 'Provision devportal'}
            </Button>
          </Stack>
        </Stack>
      </Box>
    </PageContent>
  );
}
