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

import React from 'react';
import {
  Card,
  CardContent,
  Chip,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  Box,
  CircularProgress,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import type { ProviderTemplate } from '../../../../../utils/types';
import type { FormState } from './serviceProviderTypes';
import { FormattedMessage } from 'react-intl';

type ProviderTemplateFormFieldsProps = {
  formState: FormState;
  setFormState: React.Dispatch<React.SetStateAction<FormState>>;
  showCredential: boolean;
  setShowCredential: React.Dispatch<React.SetStateAction<boolean>>;
  template?: ProviderTemplate | null;
  isLoading: boolean;
  error: Error | null;
  /** Backend field-level validation errors (from the create/update API call), keyed by FormState field name. */
  fieldErrors?: Partial<Record<keyof FormState, string>>;
};

const VERSION_PATTERN = /^v\d+\.\d+$/;
const CONTEXT_PATTERN = /^\/([a-zA-Z0-9_\-/]*[^/])?$/;

const toProviderId = (name: string): string =>
  name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const buildAutoContext = (name: string): string => {
  const id = toProviderId(name);
  return id ? `/${id}` : '/';
};

export default function ProviderTemplateFormFields({
  formState,
  setFormState,
  showCredential,
  setShowCredential,
  template,
  isLoading,
  error,
  fieldErrors = {},
}: ProviderTemplateFormFieldsProps) {
  const [versionTouched, setVersionTouched] = React.useState(false);
  const [contextTouched, setContextTouched] = React.useState(false);
  const contextEditedRef = React.useRef(false);

  if (isLoading) {
    return (
      <Grid size={{ xs: 12 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2 }}>
          <CircularProgress size={20} />
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.loading.template.details"
              defaultMessage={'Loading template details...'}
            />
          </Typography>
        </Box>
      </Grid>
    );
  }

  if (error) {
    return (
      <Grid size={{ xs: 12 }}>
        <Typography variant="body2" color="error">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.failed.to.load.template"
            defaultMessage={'Failed to load template:'}
          />{' '}
          {error.message}
        </Typography>
      </Grid>
    );
  }

  const hasTemplateUrl = Boolean(template?.metadata?.endpointUrl);
  const hasTemplateAuthType = Boolean(template?.metadata?.auth?.type);
  const hasTemplateAuthHeader = Boolean(template?.metadata?.auth?.header);
  const trimmedVersion = formState.version.trim();
  const versionErrorMessage = !versionTouched
    ? ''
    : !trimmedVersion
      ? 'Version is required.'
      : !VERSION_PATTERN.test(trimmedVersion)
        ? 'Version must match v<major>.<minor> (e.g., v1.0).'
        : '';
  const contextErrorMessage =
    !contextTouched || !formState.context
      ? ''
      : !CONTEXT_PATTERN.test(formState.context)
        ? 'Invalid context path (e.g., /mycontext).'
        : '';

  return (
    <>
      {/* Row 2: Name + Version */}
      <Grid size={{ xs: 12, md: 8 }}>
        <FormControl fullWidth>
          <FormLabel required>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.name"
              defaultMessage={'Name'}
            />
          </FormLabel>
          <TextField
            fullWidth
            value={formState.name}
            onChange={(e) => {
              const newName = e.target.value;
              setFormState((prev) => ({
                ...prev,
                name: newName,
                context: contextEditedRef.current ? prev.context : buildAutoContext(newName),
              }));
            }}
            placeholder={`WSO2 ${template?.displayName || ''} Provider`}
            data-cyid="provider-name-input"
            error={Boolean(fieldErrors.name)}
            helperText={fieldErrors.name}
          />
        </FormControl>
      </Grid>

      <Grid size={{ xs: 12, md: 4 }}>
        <FormControl fullWidth>
          <FormLabel required>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.version"
              defaultMessage={'Version'}
            />
          </FormLabel>
          <TextField
            fullWidth
            value={formState.version}
            onChange={(e) => {
              const newVersion = e.target.value;
              setFormState((prev) => ({
                ...prev,
                version: newVersion,
                context: contextEditedRef.current ? prev.context : buildAutoContext(prev.name),
              }));
            }}
            onBlur={() => setVersionTouched(true)}
            placeholder="v1.0"
            error={Boolean(versionErrorMessage) || Boolean(fieldErrors.version)}
            helperText={fieldErrors.version || versionErrorMessage || undefined}
            data-cyid="provider-version-input"
          />
        </FormControl>
      </Grid>

      {/* Row 4: Description */}
      <Grid size={{ xs: 12 }}>
        <FormControl fullWidth>
          <FormLabel>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.description"
              defaultMessage={'Description'}
            />
          </FormLabel>
          <TextField
            fullWidth
            multiline
            minRows={2}
            value={formState.description}
            onChange={(e) =>
              setFormState((prev) => ({ ...prev, description: e.target.value }))
            }
            placeholder={`Primary ${template?.displayName || ''} provider`}
            data-cyid="provider-description-input"
            error={Boolean(fieldErrors.description)}
            helperText={fieldErrors.description}
          />
        </FormControl>
      </Grid>

      {/* Row 5: Context */}
      <Grid size={{ xs: 12 }}>
        <FormControl fullWidth>
          <FormLabel>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.context"
              defaultMessage={'Context'}
            />
          </FormLabel>
          <TextField
            fullWidth
            value={formState.context}
            onChange={(e) => {
              contextEditedRef.current = true;
              setFormState((prev) => ({ ...prev, context: e.target.value }));
            }}
            onBlur={() => setContextTouched(true)}
            placeholder="/"
            error={Boolean(contextErrorMessage) || Boolean(fieldErrors.context)}
            helperText={fieldErrors.context || contextErrorMessage || undefined}
            data-cyid="provider-context-input"
          />
        </FormControl>
      </Grid>

      {/* Upstream URL - only show if not provided by template */}
      {!hasTemplateUrl && (
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.upstream.url"
                defaultMessage={'Upstream URL'}
              />
            </FormLabel>
            <TextField
              fullWidth
              value={formState.upstreamUrl}
              onChange={(e) =>
                setFormState((prev) => ({
                  ...prev,
                  upstreamUrl: e.target.value,
                }))
              }
              placeholder="https://api.openai.com/v1"
              error={Boolean(fieldErrors.upstreamUrl)}
              helperText={fieldErrors.upstreamUrl || 'The base URL of the upstream LLM provider'}
              data-cyid="provider-upstream-url-input"
            />
          </FormControl>
        </Grid>
      )}

      {/* Auth Type - only show if not provided by template */}
      {!hasTemplateAuthType && (
        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.authentication.type"
                defaultMessage={'Authentication Type'}
              />
            </FormLabel>
            <Select
              value={formState.upstreamAuthType}
              onChange={(e) =>
                setFormState((prev) => ({
                  ...prev,
                  upstreamAuthType: e.target.value,
                }))
              }
            >
              {['api-key'].map((type) => (
                <MenuItem key={type} value={type}>
                  {type}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>
      )}

      {/* Auth Header - only show if not provided by template */}
      {!hasTemplateAuthHeader && (
        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.auth.header.name"
                defaultMessage={'Auth Header Name'}
              />
            </FormLabel>
            <TextField
              fullWidth
              value={formState.upstreamAuthHeader}
              onChange={(e) =>
                setFormState((prev) => ({
                  ...prev,
                  upstreamAuthHeader: e.target.value,
                }))
              }
              placeholder="Authorization"
            />
          </FormControl>
        </Grid>
      )}

      {/* Auth Value - always show; optional, provider can be created without it */}
      <Grid size={{ xs: 12 }}>
        <FormControl fullWidth>
          <FormLabel>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.ProviderTemplateFormFields.api.key"
              defaultMessage={'API Key'}
            />
          </FormLabel>
          <TextField
            fullWidth
            value={formState.upstreamAuthValue}
            onChange={(e) =>
              setFormState((prev) => ({
                ...prev,
                upstreamAuthValue: e.target.value,
              }))
            }
            type={showCredential ? 'text' : 'password'}
            data-cyid="provider-api-key-input"
            slotProps={{
              input: {
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() => setShowCredential((prev) => !prev)}
                      aria-label={
                        showCredential ? 'Hide credentials' : 'Show credentials'
                      }
                    >
                      {showCredential ? (
                        <EyeOff size={18} />
                      ) : (
                        <Eye size={18} />
                      )}
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
            placeholder="Enter your API key or token (optional)"
            // helperText={
            //   template?.metadata?.auth?.valuePrefix
            //     ? `Will be prefixed with: ${template.metadata.auth.valuePrefix}`
            //     : 'Your authentication credential for the upstream provider'
            // }
          />
        </FormControl>
      </Grid>

      {/* Show what values are being used from template */}
      {/* {(hasTemplateUrl || hasTemplateAuthType || hasTemplateAuthHeader) && (
        <Grid size={{ xs: 12 }}>
          <Card variant="outlined" sx={{ backgroundColor: 'action.hover' }}>
            <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: 'block', mb: 1 }}
              >
                Values from template:
              </Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                {hasTemplateUrl && (
                  <Chip
                    size="small"
                    label={`URL: ${template?.metadata?.endpointUrl}`}
                    variant="outlined"
                  />
                )}
                {hasTemplateAuthType && (
                  <Chip
                    size="small"
                    label={`Auth: ${template?.metadata?.auth?.type}`}
                    variant="outlined"
                  />
                )}
                {hasTemplateAuthHeader && (
                  <Chip
                    size="small"
                    label={`Header: ${template?.metadata?.auth?.header}`}
                    variant="outlined"
                  />
                )}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      )} */}
    </>
  );
}

