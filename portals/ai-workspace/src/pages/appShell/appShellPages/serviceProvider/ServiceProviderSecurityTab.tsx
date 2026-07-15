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

import React, { useState, useEffect } from 'react';
import {
  Box,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { FormattedMessage } from 'react-intl';

export default function ServiceProviderSecurityTab() {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  const [authenticationType, setAuthenticationType] = useState('');
  const [apiKeyEnabled, setApiKeyEnabled] = useState(true);
  const [keyValue, setKeyValue] = useState('');
  const [keyIn, setKeyIn] = useState<'header' | 'query'>('header');
  const [valuePrefix, setValuePrefix] = useState('');
  const showSnackbar = useAIWorkspaceSnackbar();
  const isReadOnlyProvider = Boolean(provider?.readOnly);
  const isSecurityFormDisabled =
    !apiKeyEnabled || isLoading || Boolean(error) || isReadOnlyProvider;

  useEffect(() => {
    if (!provider) return;
    const authType = provider.security?.apiKey ? 'apiKey' : '';
    setAuthenticationType(authType);
    setApiKeyEnabled(
      provider.security?.apiKey?.enabled ?? provider.security?.enabled ?? true
    );
    setKeyValue(provider.security?.apiKey?.key || '');
    setKeyIn(provider.security?.apiKey?.in || 'header');
    setValuePrefix(provider.security?.apiKey?.valuePrefix || '');
  }, [provider]);

  const updateSecurity = async (
    nextKey: string,
    nextIn: 'header' | 'query',
    nextEnabled: boolean,
    nextValuePrefix: string
  ) => {
    if (!provider || isLoading || error || isReadOnlyProvider) return;
    const {
      status,
      createdAt,
      createdBy,
      updatedAt,
      lastUpdated,
      ...updatePayload
    } = provider;
    const security = provider.security;
    const apiKey = security?.apiKey;

    try {
      await updateProvider({
        ...updatePayload,
        security: {
          ...security,
          enabled: nextEnabled,
          apiKey: {
            ...apiKey,
            enabled: nextEnabled,
            key: nextKey,
            in: nextIn,
            valuePrefix: nextValuePrefix,
          },
        },
      });
      if (!isDraftMode) {
        showSnackbar('Updated security settings.', 'success');
      }
    } catch {
      if (!isDraftMode) {
        showSnackbar('Failed to update security.', 'error');
      }
    }
  };

  const handleAuthChange = async (event: any) => {
    const nextType = String(event.target.value || '').trim();
    if (!nextType || nextType === authenticationType) return;
    setAuthenticationType(nextType);
    await updateSecurity(keyValue.trim(), keyIn, apiKeyEnabled, valuePrefix.trim());
  };

  const handleApiKeyEnabledChange = async (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    const nextEnabled = event.target.checked;
    if (nextEnabled === apiKeyEnabled) return;
    setApiKeyEnabled(nextEnabled);
    await updateSecurity(keyValue.trim(), keyIn, nextEnabled, valuePrefix.trim());
  };

  const handleKeyBlur = async () => {
    if (!provider || isLoading || error) return;
    const nextKey = keyValue.trim();
    if (!nextKey || nextKey === (provider.security?.apiKey?.key || '')) {
      return;
    }
    await updateSecurity(nextKey, keyIn, apiKeyEnabled, valuePrefix.trim());
  };

  const handleValuePrefixBlur = async () => {
    if (!provider || isLoading || error) return;
    const nextValuePrefix = valuePrefix.trim();
    if (nextValuePrefix === (provider.security?.apiKey?.valuePrefix || '')) {
      return;
    }
    await updateSecurity(keyValue.trim(), keyIn, apiKeyEnabled, nextValuePrefix);
  };

  const handleKeyInChange = async (event: any) => {
    const nextIn = event.target.value as 'header' | 'query';
    if (nextIn === keyIn) return;
    setKeyIn(nextIn);
    await updateSecurity(keyValue.trim(), nextIn, apiKeyEnabled, valuePrefix.trim());
  };

  return (
    <>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 5 }}>
          <Stack spacing={2}>
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                justifyContent: 'space-between',
                mb: 0.5,
              }}
            >
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.authentication"
                  defaultMessage={'Authentication'}
                />
              </Typography>
              {authenticationType === 'apiKey' && (
                <Tooltip
                  title={
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.apiKeyEnabled.tooltip"
                      defaultMessage={
                        'Require consumers to include this API key when sending requests.'
                      }
                    />
                  }
                >
                  <Switch
                    size="small"
                    checked={apiKeyEnabled}
                    disabled={isLoading || Boolean(error) || isReadOnlyProvider}
                    onChange={handleApiKeyEnabledChange}
                  />
                </Tooltip>
              )}
            </Box>
            <FormControl fullWidth size="small">
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.authentication.2"
                  defaultMessage={'Authentication'}
                />
              </FormLabel>
              <Select
                value={authenticationType}
                disabled={isSecurityFormDisabled}
                onChange={handleAuthChange}
              >
                <MenuItem value="apiKey">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.apikey"
                    defaultMessage={'apiKey'}
                  />
                </MenuItem>
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.api.key"
                  defaultMessage={'API Key'}
                />
              </FormLabel>
              <TextField
                size="small"
                value={keyValue}
                disabled={isSecurityFormDisabled}
                onChange={(event) => {
                  const nextKey = event.target.value;
                  setKeyValue(nextKey);
                  if (isDraftMode) {
                    void updateSecurity(nextKey.trim(), keyIn, apiKeyEnabled, valuePrefix.trim());
                  }
                }}
                onBlur={() => {
                  if (!isDraftMode) {
                    void handleKeyBlur();
                  }
                }}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.api.key.value.prefix"
                  defaultMessage={'API Key Value Prefix'}
                />
              </FormLabel>
              <TextField
                size="small"
                value={valuePrefix}
                disabled={isSecurityFormDisabled}
                placeholder="Bearer"
                onChange={(event) => {
                  const nextValuePrefix = event.target.value;
                  setValuePrefix(nextValuePrefix);
                  if (isDraftMode) {
                    void updateSecurity(
                      keyValue.trim(),
                      keyIn,
                      apiKeyEnabled,
                      nextValuePrefix.trim()
                    );
                  }
                }}
                onBlur={() => {
                  if (!isDraftMode) {
                    void handleValuePrefixBlur();
                  }
                }}
              />
            </FormControl>
            <FormControl fullWidth size="small">
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.key.location"
                  defaultMessage={'Key Location'}
                />
              </FormLabel>
              <Select
                value={keyIn}
                disabled={isSecurityFormDisabled}
                onChange={handleKeyInChange}
              >
                <MenuItem value="header">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.header"
                    defaultMessage={'header'}
                  />
                </MenuItem>
                <MenuItem value="query">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.query"
                    defaultMessage={'query'}
                  />
                </MenuItem>
              </Select>
            </FormControl>
          </Stack>
        </Grid>
      </Grid>
    </>
  );
}
