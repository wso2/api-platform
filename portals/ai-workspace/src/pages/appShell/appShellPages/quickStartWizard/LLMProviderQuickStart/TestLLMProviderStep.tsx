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

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Box, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { getGateways } from '../../../../../apis/gatewayApis';
import type { Gateway } from '../../../../../apis/gatewayTypes';
import ApiTryOutCurlSnippet from '../../../../../Components/common/ApiTryOutCurlSnippet';
import ServiceProviderDeploymentsCard from '../../serviceProvider/ServiceProviderDeploymentsCard';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { useGatewayDeploy } from '../../../../../contexts/GatewayDeployContext';
import { useLLMProvider } from '../../../../../contexts/llmProvider';
import { buildGatewayInvokeUrl } from './utils';

type TestLLMProviderStepProps = {
  onTestingReadyChange: (isReady: boolean) => void;
};

export default function TestLLMProviderStep({
  onTestingReadyChange,
}: TestLLMProviderStepProps) {
  const { currentOrganization } = useAppShell();
  const { provider, getProviderAPIKeys } = useLLMProvider();
  const { deployments } = useGatewayDeploy();

  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [isLoadingGateways, setIsLoadingGateways] = useState(false);
  const [latestGeneratedApiKey, setLatestGeneratedApiKey] = useState<string | null>(null);
  const [apiKeysCount, setApiKeysCount] = useState(0);

  const apiKeyLocation = provider?.security?.apiKey?.in ?? 'header';
  const apiKeyHeaderName = provider?.security?.apiKey?.key?.trim()
    ? provider.security.apiKey.key.trim()
    : 'X-API-Key';
  const hasAvailableApiKey = apiKeysCount > 0 || Boolean(latestGeneratedApiKey);

  const refreshApiKeys = useCallback(async () => {
    if (!provider?.id) {
      setApiKeysCount(0);
      return;
    }

    try {
      const response = await getProviderAPIKeys();
      setApiKeysCount(response.list?.length ?? 0);
    } catch {
      setApiKeysCount(0);
    }
  }, [getProviderAPIKeys, provider?.id]);

  useEffect(() => {
    onTestingReadyChange(hasAvailableApiKey);
  }, [hasAvailableApiKey, onTestingReadyChange]);

  useEffect(() => {
    void refreshApiKeys();
  }, [refreshApiKeys]);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    if (!organizationId || !provider?.id) {
      setGateways([]);
      setSelectedGatewayId('');
      return;
    }

    let isMounted = true;
    setIsLoadingGateways(true);

    void (async () => {
      try {
        const gatewaysResponse = await getGateways(organizationId);
        if (!isMounted) return;

        const deployedGatewayIds = new Set(
          (deployments?.list || [])
            .filter((deployment) => deployment.status === 'DEPLOYED')
            .map((deployment) => deployment.gatewayId)
        );

        const deployedGateways = (gatewaysResponse.list || []).filter((gateway) =>
          deployedGatewayIds.has(gateway.id)
        );
        setGateways(deployedGateways);
        setSelectedGatewayId((currentSelectedId) => {
          if (
            currentSelectedId &&
            deployedGateways.some((gateway) => gateway.id === currentSelectedId)
          ) {
            return currentSelectedId;
          }
          return deployedGateways[0]?.id || '';
        });
      } catch {
        if (!isMounted) return;
        setGateways([]);
        setSelectedGatewayId('');
      } finally {
        if (isMounted) {
          setIsLoadingGateways(false);
        }
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [currentOrganization?.uuid, deployments?.list, provider?.id]);

  const selectedGateway = useMemo(
    () => gateways.find((gateway) => gateway.id === selectedGatewayId) ?? null,
    [gateways, selectedGatewayId]
  );
  const generatedInvokeUrl = useMemo(
    () => buildGatewayInvokeUrl(selectedGateway?.vhost ?? '', provider?.context ?? '/'),
    [provider?.context, selectedGateway?.vhost]
  );

  const handleCopyInvokeUrl = async () => {
    if (!generatedInvokeUrl) return;
    try {
      await navigator.clipboard.writeText(generatedInvokeUrl);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = generatedInvokeUrl;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  if (!provider) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Stack spacing={2}>
      {gateways.length === 0 && !isLoadingGateways ? (
        <Alert severity="warning">
          No deployed AI gateways are available yet. Deploy the provider first.
        </Alert>
      ) : null}

      {isLoadingGateways ? (
        <Stack direction="row" spacing={1.5} alignItems="center">
          <CircularProgress size={18} />
          <Typography variant="body2" color="text.secondary">
            Loading deployed gateways...
          </Typography>
        </Stack>
      ) : null}

      <ServiceProviderDeploymentsCard
        isGatewaysLoading={isLoadingGateways}
        gateways={gateways}
        selectedGatewayId={selectedGatewayId}
        onGatewayChange={setSelectedGatewayId}
        generatedInvokeUrl={generatedInvokeUrl}
        onCopyInvokeUrl={handleCopyInvokeUrl}
        onLatestGeneratedApiKeyChange={setLatestGeneratedApiKey}
        onApiKeyCreated={() => {
          void refreshApiKeys();
        }}
      />

      {latestGeneratedApiKey && generatedInvokeUrl ? (
        <ApiTryOutCurlSnippet
          apiKey={latestGeneratedApiKey}
          gatewayUrl={generatedInvokeUrl}
          apiKeyHeaderName={apiKeyHeaderName}
          apiKeyLocation={apiKeyLocation}
          providerTemplate={provider.template}
        />
      ) : null}
    </Stack>
  );
}
