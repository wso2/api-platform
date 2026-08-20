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

import { useCreateApiPortal } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { routes } from '../../routes/paths';
import type { ApiPortalAuthType, CreateApiPortalInput } from '../../types/domain';
import { isValidUrl } from '../apis/develop/developEdit';
import { AUTH_TYPE_OPTIONS } from './apiPortalDisplay';
import { IdpCredentialsFields } from './IdpCredentialsFields';

const HANDLE_PATTERN = /^[a-z0-9-]{3,64}$/;

const slugify = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64)
    .replace(/^-+|-+$/g, '');

export function ApiPortalCreatePage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const createApiPortal = useCreateApiPortal(orgHandle);

  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);
  // Auto-derived from Name and locked by default; the identifier is
  // permanent once the API Portal is created, so editing it directly is an
  // explicit, deliberate action rather than something you fall into while
  // typing the Name.
  const [handleLocked, setHandleLocked] = useState(true);
  const [description, setDescription] = useState('');
  const [authType, setAuthType] = useState<ApiPortalAuthType>(
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
    !createApiPortal.isPending;

  const submit = () => {
    const basePayload = {
      name: displayName,
      handle,
      url: url.trim(),
      description: description || undefined,
    };
    const input: CreateApiPortalInput = isIdpAuth
      ? {
          ...basePayload,
          authType: 'idp_client_credentials',
          stsTokenUrl: stsTokenUrl.trim(),
          clientId: clientId.trim(),
          clientSecret,
        }
      : { ...basePayload, authType: 'local' };
    createApiPortal.mutate(input, {
      onSuccess: (apiPortal) => {
        notify(`API Portal "${apiPortal.name}" provisioned.`, 'success');
        navigate(routes.apiPortal(orgHandle));
      },
      onError: (error) =>
        notify(
          error instanceof Error
            ? error.message
            : 'Failed to provision API Portal',
          'error'
        ),
    });
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.apiPortal(orgHandle)}>
          <PageTitle.BackButton>Back to API Portal</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>Provision an API Portal</PageTitle.Header>
        <PageTitle.SubHeader>
          Register an API Portal, then connect it to the platform.
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ maxWidth: 720 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <FormLabel htmlFor="api-portal-name">Name</FormLabel>
            <TextField
              id="api-portal-name"
              onChange={(event) => onDisplayNameChange(event.target.value)}
              placeholder="Production API Portal"
              value={displayName}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="api-portal-handle">Identifier</FormLabel>
            <TextField
              id="api-portal-handle"
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
              placeholder="prod-api-portal"
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
                  readOnly: handleLocked,
                },
              }}
              value={handle}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="api-portal-description">
              Description (optional)
            </FormLabel>
            <TextField
              id="api-portal-description"
              multiline
              minRows={2}
              onChange={(event) => setDescription(event.target.value)}
              value={description}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="api-portal-auth-type">Authentication</FormLabel>
            <Select
              id="api-portal-auth-type"
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
            <FormLabel htmlFor="api-portal-url">URL</FormLabel>
            <TextField
              id="api-portal-url"
              error={url !== '' && !isValidUrl(url)}
              helperText={
                url !== '' && !isValidUrl(url) ? 'Enter a valid URL' : undefined
              }
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://api-portal.example.com"
              value={url}
            />
          </FormControl>

          <Stack direction="row" justifyContent="flex-end" spacing={1.5}>
            <Button
              component={Link}
              to={routes.apiPortal(orgHandle)}
              variant="outlined"
            >
              Cancel
            </Button>
            <Button disabled={!canSubmit} onClick={submit} variant="contained">
              {createApiPortal.isPending
                ? 'Provisioning…'
                : 'Provision API Portal'}
            </Button>
          </Stack>
        </Stack>
      </Box>
    </PageContent>
  );
}
