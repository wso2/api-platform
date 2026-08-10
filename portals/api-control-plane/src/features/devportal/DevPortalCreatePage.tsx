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
  Typography,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateDevPortal } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { routes } from '../../routes/paths';
import type { DevPortalAuthType } from '../../types/domain';

const HANDLE_PATTERN = /^[a-z0-9-]{3,64}$/;

const slugify = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);

const AUTH_TYPES: { value: DevPortalAuthType; label: string }[] = [
  { value: 'local', label: 'Local' },
  { value: 'idp_client_credentials', label: 'IdP Client Credentials' },
];

export function DevPortalCreatePage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const createDevPortal = useCreateDevPortal();

  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);
  const [description, setDescription] = useState('');
  const [authType, setAuthType] = useState<DevPortalAuthType>(
    AUTH_TYPES[0].value
  );
  const [url, setUrl] = useState('');
  const [stsTokenUrl, setStsTokenUrl] = useState('');
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [secretVisible, setSecretVisible] = useState(false);

  const onDisplayNameChange = (value: string) => {
    setDisplayName(value);
    if (!handleEdited) setHandle(slugify(value));
  };

  const handleValid = HANDLE_PATTERN.test(handle);
  const isIdpAuth = authType === 'idp_client_credentials';
  const idpFieldsValid =
    !isIdpAuth ||
    (stsTokenUrl.trim() !== '' &&
      clientId.trim() !== '' &&
      clientSecret.trim() !== '');
  const canSubmit =
    displayName.trim() !== '' &&
    handleValid &&
    url.trim() !== '' &&
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
            <FormLabel>Display name</FormLabel>
            <TextField
              onChange={(event) => onDisplayNameChange(event.target.value)}
              placeholder="Production Devportal"
              value={displayName}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Name</FormLabel>
            <TextField
              error={handle !== '' && !handleValid}
              helperText="Lowercase letters, numbers, hyphens; 3–64 chars (unique per org)."
              onChange={(event) => {
                setHandleEdited(true);
                setHandle(event.target.value);
              }}
              placeholder="prod-devportal"
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
              {AUTH_TYPES.map((option) => (
                <MenuItem key={option.value} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          {isIdpAuth && (
            <Box
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 2,
                p: 2.25,
              }}
            >
              <Typography
                color="text.secondary"
                sx={{
                  display: 'block',
                  fontWeight: 600,
                  letterSpacing: '.12em',
                  mb: 1.5,
                }}
                variant="caption"
              >
                IDP CLIENT CREDENTIALS
              </Typography>
              <Stack spacing={2}>
                <FormControl fullWidth>
                  <FormLabel>STS token URL</FormLabel>
                  <TextField
                    onChange={(event) => setStsTokenUrl(event.target.value)}
                    placeholder="https://idp.example.com/oauth2/token"
                    value={stsTokenUrl}
                  />
                </FormControl>

                <FormControl fullWidth>
                  <FormLabel>Client ID</FormLabel>
                  <TextField
                    onChange={(event) => setClientId(event.target.value)}
                    value={clientId}
                  />
                </FormControl>

                <FormControl fullWidth>
                  <FormLabel>Client secret</FormLabel>
                  <TextField
                    onChange={(event) => setClientSecret(event.target.value)}
                    slotProps={{
                      input: {
                        endAdornment: (
                          <InputAdornment position="end">
                            <IconButton
                              aria-label={
                                secretVisible
                                  ? 'Hide client secret'
                                  : 'Show client secret'
                              }
                              onClick={() => setSecretVisible((v) => !v)}
                              size="small"
                            >
                              {secretVisible ? (
                                <EyeOff size={16} />
                              ) : (
                                <Eye size={16} />
                              )}
                            </IconButton>
                          </InputAdornment>
                        ),
                      },
                    }}
                    type={secretVisible ? 'text' : 'password'}
                    value={clientSecret}
                  />
                </FormControl>
              </Stack>
            </Box>
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
