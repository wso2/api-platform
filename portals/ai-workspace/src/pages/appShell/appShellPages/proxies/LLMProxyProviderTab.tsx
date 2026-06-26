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

import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { useLLMProviders } from '../../../../contexts/llmProvider';
import { useProxy } from '../../../../contexts/proxy';
import * as llmProviderApis from '../../../../apis/llmProviderApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { logger } from '../../../../utils/logger';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type { LLMProvider, ProxyApiKeySecurity } from '../../../../utils/types';
import { FormattedMessage } from 'react-intl';

/**
 * Provider tab – lets the user select / change the LLM Service Provider
 * linked to this proxy.
 */
export default function LLMProxyProviderTab() {
  const { proxy, setLocalProxy } = useProxy();
  const { providersResponse, isLoading: isProvidersLoading } =
    useLLMProviders();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid ?? '';
  const showSnackbar = useAIWorkspaceSnackbar();

  const [providerDetail, setProviderDetail] = useState<LLMProvider | null>(
    null
  );
  const [apiKey, setApiKey] = useState('');

  const providerOptions = providersResponse.list;
  const selectedProxyProviderId =
    typeof proxy?.provider === 'string'
      ? proxy.provider
      : proxy?.provider?.id ?? '';

  // Fetch single-provider detail (for vhost) whenever proxy.provider changes
  const fetchProviderDetail = useCallback(
    async (providerId: string) => {
      if (!providerId || !organizationId) {
        setProviderDetail(null);
        return;
      }
      try {
        const detail = await llmProviderApis.getLLMProvider(
          providerId,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        setProviderDetail(detail);
      } catch (err) {
        logger.error('Failed to fetch provider detail:', err);
        setProviderDetail(null);
      }
    },
    [organizationId]
  );

  useEffect(() => {
    if (selectedProxyProviderId) {
      fetchProviderDetail(selectedProxyProviderId);
    } else {
      setProviderDetail(null);
    }
  }, [selectedProxyProviderId, fetchProviderDetail]);

  const mapProviderSecurityToProxySecurity = (provider: LLMProvider) => {
    const providerApiKey = provider.security?.apiKey;
    const apiKey: ProxyApiKeySecurity | undefined = providerApiKey
      ? {
          enabled: Boolean(providerApiKey.enabled),
          key: providerApiKey.key ?? '',
          in: providerApiKey.in ?? 'header',
        }
      : undefined;
    return {
      enabled: Boolean(provider.security?.enabled),
      apiKey,
    };
  };

  const handleSaveApiKey = () => {
    if (!proxy || !apiKey.trim() || !providerDetail) {
      showSnackbar('Please enter an API key.', 'error');
      return;
    }
    const providerId =
      typeof proxy.provider === 'string' ? proxy.provider : proxy.provider?.id;
    if (!providerId) {
      showSnackbar('Please select a provider first.', 'error');
      return;
    }
    const apiKeyHeader = providerDetail.security?.apiKey?.key || 'X-API-Key';
    setLocalProxy((prev) =>
      prev
        ? {
            ...prev,
            provider: {
              id: providerId,
              auth: {
                type: 'api-key',
                header: apiKeyHeader,
                value: apiKey.trim(),
              },
            },
          }
        : prev
    );
    setApiKey('');
  };

  const handleProviderChange = async (event: { target: { value: string } }) => {
    const newProviderId = event.target.value;
    if (!proxy || newProviderId === selectedProxyProviderId) return;

    // Fetch selected provider details and stage related proxy fields.
    try {
      let nextProviderDetail: LLMProvider | null = null;
      if (newProviderId && organizationId) {
        const detail = await llmProviderApis.getLLMProvider(
          newProviderId,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        nextProviderDetail = detail;
        setProviderDetail(detail);
      }
      setLocalProxy((prev) =>
        prev
          ? {
              ...prev,
              provider: newProviderId
                ? {
                    id: newProviderId,
                    auth: {
                      type: nextProviderDetail?.upstream?.main?.auth?.type ?? '',
                      header:
                        nextProviderDetail?.upstream?.main?.auth?.header ?? '',
                      value: nextProviderDetail?.upstream?.main?.auth?.value ?? '',
                    },
                  }
                : undefined,
              vhost: nextProviderDetail?.vhost?.trim() || undefined,
              openapi: nextProviderDetail?.openapi ?? '',
              security: nextProviderDetail
                ? mapProviderSecurityToProxySecurity(nextProviderDetail)
                : prev.security,
            }
          : prev
      );
      setApiKey('');
    } catch (err) {
      logger.error('Failed to update provider:', err);
      showSnackbar('Failed to load provider details.', 'error');
    }
  };

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, md: 6 }}>
        <Stack spacing={2}>
          <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.llm.service.provider"
              defaultMessage={'LLM Provider'}
            />
          </Typography>

          <FormControl fullWidth>
            <FormLabel>Provider</FormLabel>
            <Select
              value={selectedProxyProviderId}
              onChange={handleProviderChange}
              displayEmpty
              disabled={isProvidersLoading}
            >
              {isProvidersLoading ? (
                <MenuItem value="" disabled>
                  Loading providers…
                </MenuItem>
              ) : providerOptions.length === 0 ? (
                <MenuItem value="" disabled>
                  No providers available
                </MenuItem>
              ) : (
                providerOptions.map((p) => (
                  <MenuItem key={p.id} value={p.id}>
                    {p.name}
                  </MenuItem>
                ))
              )}
            </Select>
          </FormControl>

          {proxy?.provider && providerDetail?.security?.apiKey && (
            <Stack spacing={2} sx={{ mt: 3 }}>
              <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.api.key.configuration"
                  defaultMessage={'API Key Configuration'}
                />
              </Typography>

              <Stack spacing={1.5}>
                <Grid container spacing={2}>
                  <Grid size={{ xs: 12, sm: 3 }}>
                    <Stack spacing={0.5}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.api.key.header"
                          defaultMessage={'Header Name'}
                        />
                      </Typography>
                      <Typography
                        variant="body2"
                        sx={{
                          fontFamily:
                            'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                          bgcolor: 'action.hover',
                          px: 1.5,
                          py: 0.75,
                          borderRadius: 1,
                          fontSize: 13,
                        }}
                      >
                        {providerDetail.security.apiKey.key || 'X-API-Key'}
                      </Typography>
                    </Stack>
                  </Grid>

                  <Grid size={{ xs: 12, sm: 9 }}>
                    <FormControl fullWidth>
                      <FormLabel>
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.api.key"
                          defaultMessage={'API Key'}
                        />
                      </FormLabel>
                      <TextField
                        type="password"
                        placeholder="Enter API key"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        fullWidth
                      />
                    </FormControl>
                  </Grid>
                </Grid>

                <Button
                  variant="contained"
                  onClick={handleSaveApiKey}
                  disabled={!apiKey.trim()}
                  sx={{ alignSelf: 'flex-start' }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyProviderTab.save.api.key"
                    defaultMessage={'Save API Key'}
                  />
                </Button>
              </Stack>
            </Stack>
          )}
        </Stack>
      </Grid>
    </Grid>
  );
}
