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
import { useProxy } from '../../../../contexts/proxy';
import type {
  ProxySecurityConfig,
  ProxyApiKeySecurity,
} from '../../../../utils/types';
import { FormattedMessage } from 'react-intl';

/**
 * Security tab – mirrors the LLM Provider security tab but uses
 * ProxySecurityConfig ({ enabled, apiKey: { enabled, key, in } }).
 *
 * Changes are staged locally until the user clicks Save.
 */
export default function LLMProxySecurityTab() {
  const { proxy, isLoading, error, setLocalProxy } = useProxy();
  const isReadOnlyProxy = Boolean(proxy?.readOnly);

  const [authenticationType, setAuthenticationType] = useState('');
  const [apiKeyEnabled, setApiKeyEnabled] = useState(true);
  const [keyName, setKeyName] = useState('');
  const [keyLocation, setKeyLocation] = useState<'header' | 'query'>('header');

  const isFormDisabled =
    !apiKeyEnabled || isLoading || Boolean(error) || isReadOnlyProxy;

  useEffect(() => {
    if (!proxy) return;
    const sec = proxy.security;
    const authType = sec?.apiKey ? 'apiKey' : '';
    setAuthenticationType(authType);
    setApiKeyEnabled(sec?.apiKey?.enabled ?? sec?.enabled ?? true);
    setKeyName(sec?.apiKey?.key || '');
    setKeyLocation(sec?.apiKey?.in || 'header');
  }, [proxy]);

  const stageSecurity = (
    nextAuthType: string,
    nextKey: string,
    nextIn: 'header' | 'query',
    nextEnabled: boolean
  ) => {
    const apiKey: ProxyApiKeySecurity | undefined =
      nextAuthType === 'apiKey'
        ? { enabled: nextEnabled, key: nextKey, in: nextIn }
        : undefined;

    const nextSecurity: ProxySecurityConfig = {
      enabled: nextAuthType === 'apiKey' && nextEnabled,
      apiKey,
    };
    setLocalProxy((prev) => (prev ? { ...prev, security: nextSecurity } : prev));
  };

  const handleAuthChange = (event: { target: { value: string } }) => {
    if (isReadOnlyProxy) return;
    const nextType = event.target.value;
    setAuthenticationType(nextType);
    stageSecurity(nextType, keyName, keyLocation, apiKeyEnabled);
  };

  const handleApiKeyEnabledChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (isReadOnlyProxy) return;
    const nextEnabled = event.target.checked;
    setApiKeyEnabled(nextEnabled);
    stageSecurity(authenticationType, keyName, keyLocation, nextEnabled);
  };

  const handleKeyNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (isReadOnlyProxy) return;
    const nextKey = event.target.value;
    setKeyName(nextKey);
    stageSecurity(authenticationType, nextKey, keyLocation, apiKeyEnabled);
  };

  const handleKeyLocationChange = (event: { target: { value: string } }) => {
    if (isReadOnlyProxy) return;
    const nextIn = event.target.value as 'header' | 'query';
    setKeyLocation(nextIn);
    stageSecurity(authenticationType, keyName, nextIn, apiKeyEnabled);
  };

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, md: 6 }}>
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
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxySecurityTab.authentication"
                defaultMessage={'Authentication'}
              />
            </Typography>
            {authenticationType === 'apiKey' && (
              <Tooltip
                title={
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxySecurityTab.apiKeyEnabled.tooltip"
                    defaultMessage={'Require consumers to include this API key when sending requests.'}
                  />
                }
              >
                <Switch
                  size="small"
                  checked={apiKeyEnabled}
                  disabled={isLoading || Boolean(error) || isReadOnlyProxy}
                  onChange={handleApiKeyEnabledChange}
                />
              </Tooltip>
            )}
          </Box>

          <FormControl fullWidth size="small">
            <FormLabel>Authentication type</FormLabel>
            <Select
              value={authenticationType}
              onChange={handleAuthChange}
              disabled={isReadOnlyProxy}
            >
              <MenuItem value="apiKey">API Key</MenuItem>
            </Select>
          </FormControl>

          {authenticationType === 'apiKey' && (
            <>
              <FormControl fullWidth>
                <FormLabel>Key name</FormLabel>
                <TextField
                  size="small"
                  placeholder="X-API-Key"
                  value={keyName}
                  disabled={isFormDisabled}
                  onChange={handleKeyNameChange}
                />
              </FormControl>

              <FormControl fullWidth size="small">
                <FormLabel>Sent in</FormLabel>
                <Select value={keyLocation} disabled={isFormDisabled} onChange={handleKeyLocationChange}>
                  <MenuItem value="header">Header</MenuItem>
                  <MenuItem value="query">Query parameter</MenuItem>
                </Select>
              </FormControl>
            </>
          )}
        </Stack>
      </Grid>
    </Grid>
  );
}
