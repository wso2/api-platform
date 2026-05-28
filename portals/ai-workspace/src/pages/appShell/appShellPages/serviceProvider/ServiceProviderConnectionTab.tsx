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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import type { ProviderTemplate } from '../../../../utils/types';
import { logger } from '../../../../utils/logger';

const MASKED_CREDENTIAL_VALUE = '******';

export default function ServiceProviderConnectionTab() {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  const { currentOrganization } = useAppShell();
  const initializedProviderIdRef = useRef<string | null>(null);
  const [providerEndpoint, setProviderEndpoint] = useState('');
  const [authenticationType, setAuthenticationType] = useState('');
  const [authenticationHeader, setAuthenticationHeader] = useState('');
  const [credentialValue, setCredentialValue] = useState('');
  const [isCredentialMasked, setIsCredentialMasked] = useState(false);
  const [hasCredentialChanged, setHasCredentialChanged] = useState(false);
  const [showCredential, setShowCredential] = useState(false);
  const [providerTemplate, setProviderTemplate] =
    useState<ProviderTemplate | null>(null);
  const showSnackbar = useAIWorkspaceSnackbar();

  const valuePrefix = useMemo(() => {
    return providerTemplate?.metadata?.auth?.valuePrefix || '';
  }, [providerTemplate]);

  useEffect(() => {
    const templateId = provider?.template;
    const organizationId = currentOrganization?.uuid;

    if (!templateId || !organizationId) {
      setProviderTemplate(null);
      return;
    }

    let isMounted = true;
    void (async () => {
      try {
        const template = await providerTemplateApis.getProviderTemplate(
          templateId,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        if (!isMounted) return;
        setProviderTemplate(template);
      } catch (fetchError) {
        if (!isMounted) return;
        logger.error(
          `Failed to fetch provider template ${templateId}:`,
          fetchError
        );
        setProviderTemplate(null);
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [PLATFORM_API_BASE_URL, currentOrganization?.uuid, provider?.template]);

  useEffect(() => {
    if (!provider) return;
    setProviderEndpoint(provider.upstream?.main?.url || '');
    setAuthenticationType(provider.upstream?.main?.auth?.type || 'api-key');
    setAuthenticationHeader(provider.upstream?.main?.auth?.header || '');

    if (initializedProviderIdRef.current === provider.id) {
      return;
    }

    initializedProviderIdRef.current = provider.id;
    setCredentialValue(MASKED_CREDENTIAL_VALUE);
    setIsCredentialMasked(true);
    setHasCredentialChanged(false);
  }, [provider]);

  const handleUpdateProviderEndpoint = async (value = providerEndpoint) => {
    if (!provider || isLoading || error) return;
    const nextUrl = value.trim();
    if (!nextUrl || nextUrl === (provider.upstream?.main?.url || '')) return;
    try {
      const {
        status,
        createdAt,
        createdBy,
        updatedAt,
        lastUpdated,
        ...updatePayload
      } = provider;
      await updateProvider({
        ...updatePayload,
        upstream: {
          main: {
            url: nextUrl,
            auth: {
              type: provider.upstream?.main?.auth?.type || '',
              header: provider.upstream?.main?.auth?.header || '',
              value: provider.upstream?.main?.auth?.value || '',
            },
          },
        },
      });
      if (!isDraftMode) {
        showSnackbar('Updated Provider Endpoint.', 'success');
      }
    } catch {
      if (!isDraftMode) {
        showSnackbar('Failed to update Provider Endpoint.', 'error');
      }
    }
  };

  const handleUpdateAuthentication = async (value = authenticationType) => {
    if (!provider || isLoading || error) return;
    const nextType = value.trim();
    if (!nextType || nextType === (provider.upstream?.main?.auth?.type || ''))
      return;
    try {
      const {
        status,
        createdAt,
        createdBy,
        updatedAt,
        lastUpdated,
        ...updatePayload
      } = provider;
      await updateProvider({
        ...updatePayload,
        upstream: {
          main: {
            url: provider.upstream?.main?.url || '',
            auth: {
              type: nextType,
              header: provider.upstream?.main?.auth?.header || '',
              value: provider.upstream?.main?.auth?.value || '',
            },
          },
        },
      });
      if (!isDraftMode) {
        showSnackbar('Updated Authentication.', 'success');
      }
    } catch {
      if (!isDraftMode) {
        showSnackbar('Failed to update Authentication.', 'error');
      }
    }
  };

  const handleUpdateAuthenticationHeader = async (
    value = authenticationHeader
  ) => {
    if (!provider || isLoading || error) return;
    const nextHeader = value.trim();
    if (nextHeader === (provider.upstream?.main?.auth?.header || '')) return;

    try {
      const {
        status,
        createdAt,
        createdBy,
        updatedAt,
        lastUpdated,
        ...updatePayload
      } = provider;
      await updateProvider({
        ...updatePayload,
        upstream: {
          main: {
            url: provider.upstream?.main?.url || '',
            auth: {
              type: provider.upstream?.main?.auth?.type || '',
              header: nextHeader,
              value: provider.upstream?.main?.auth?.value || '',
            },
          },
        },
      });
      if (!isDraftMode) {
        showSnackbar('Updated Authentication Header.', 'success');
      }
    } catch {
      if (!isDraftMode) {
        showSnackbar('Failed to update Authentication Header.', 'error');
      }
    }
  };

  const handleUpdateCredential = async (value = credentialValue) => {
    if (!provider || isLoading || error) return;
    if (isCredentialMasked) return;
    const nextValue = value.trim();
    if (nextValue === MASKED_CREDENTIAL_VALUE) return;
    const fullValue = valuePrefix
      ? nextValue.startsWith(valuePrefix)
        ? nextValue
        : `${valuePrefix}${nextValue}`
      : nextValue;
    if (fullValue === (provider.upstream?.main?.auth?.value || '')) return;
    try {
      const {
        status,
        createdAt,
        createdBy,
        updatedAt,
        lastUpdated,
        ...updatePayload
      } = provider;
      await updateProvider({
        ...updatePayload,
        upstream: {
          main: {
            url: provider.upstream?.main?.url || '',
            auth: {
              type: provider.upstream?.main?.auth?.type || '',
              header: provider.upstream?.main?.auth?.header || '',
              value: fullValue,
            },
          },
        },
      });
      if (!isDraftMode) {
        showSnackbar('Updated Credentials.', 'success');
      }
      setHasCredentialChanged(false);
    } catch {
      if (!isDraftMode) {
        showSnackbar('Failed to update Credentials.', 'error');
      }
    }
  };

  return (
    <>
      <Stack spacing={2} sx={{ maxWidth: { xs: '100%', md: 720 } }}>
        <FormControl fullWidth>
          <FormLabel>Provider Endpoint</FormLabel>
          <TextField
            size="small"
            value={providerEndpoint}
            onChange={(e) => {
              const nextValue = e.target.value;
              setProviderEndpoint(nextValue);
              if (isDraftMode) {
                void handleUpdateProviderEndpoint(nextValue);
              }
            }}
            onBlur={() => {
              if (!isDraftMode) {
                void handleUpdateProviderEndpoint();
              }
            }}
          />
        </FormControl>

        <FormControl fullWidth>
          <FormLabel>Authentication</FormLabel>
          <Select
            size="small"
            value={authenticationType || 'api-key'}
            onChange={(e) => {
              const nextValue = String(e.target.value);
              setAuthenticationType(nextValue);
              if (isDraftMode) {
                void handleUpdateAuthentication(nextValue);
              }
            }}
            onBlur={() => {
              if (!isDraftMode) {
                void handleUpdateAuthentication();
              }
            }}
          >
            <MenuItem value="api-key">api-key</MenuItem>
          </Select>
        </FormControl>

        <FormControl fullWidth>
          <FormLabel>Authentication Header</FormLabel>
          <TextField
            size="small"
            value={authenticationHeader}
            onChange={(e) => {
              const nextValue = e.target.value;
              setAuthenticationHeader(nextValue);
              if (isDraftMode) {
                void handleUpdateAuthenticationHeader(nextValue);
              }
            }}
            onBlur={() => {
              if (!isDraftMode) {
                void handleUpdateAuthenticationHeader();
              }
            }}
          />
        </FormControl>

        <FormControl fullWidth>
          <FormLabel>Credentials</FormLabel>
          <TextField
            size="small"
            type={showCredential ? 'text' : 'password'}
            value={credentialValue}
            onFocus={() => {
              if (isCredentialMasked) {
                setCredentialValue('');
                setIsCredentialMasked(false);
                setHasCredentialChanged(false);
              }
            }}
            onChange={(e) => {
              const nextValue = e.target.value;
              setCredentialValue(nextValue);
              setHasCredentialChanged(true);
              if (isDraftMode && !isCredentialMasked) {
                void handleUpdateCredential(nextValue);
              }
            }}
            onBlur={() => {
              if (!isDraftMode && !isCredentialMasked && hasCredentialChanged) {
                void handleUpdateCredential();
              }
            }}
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
          />
        </FormControl>
      </Stack>
    </>
  );
}
