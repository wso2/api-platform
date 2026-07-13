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
import { useNavigate, useParams } from 'react-router-dom';
import { Alert, Button, PageContent, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { GatewayDeployProvider } from '../../../../contexts/GatewayDeployContext';
import { GatewayDeployMainSection } from '../../../../Components/GatewayDeploy';
import { FormattedMessage } from 'react-intl';
import { ProxyProvider, useProxy } from '../../../../contexts/proxy';

function LLMProxyDeployContent() {
  const { proxyId } = useParams<{ proxyId: string }>();
  const navigate = useNavigate();
  const { proxy } = useProxy();

  if (!proxyId) {
    return (
      <PageContent>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploy.proxy.id.is.required"
            defaultMessage={'Proxy ID is required'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        size="small"
        startIcon={<ChevronLeft size={24} />}
        onClick={() => navigate(-1)}
      >
        Back to App AI Proxy
      </Button>

      <GatewayDeployProvider
        apiId={proxyId}
        resourceType="proxy"
        readOnly={Boolean(proxy?.readOnly)}
      >
        <Stack spacing={2} sx={{ mt: 2 }}>
          <Typography variant="h3">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploy.deploy.to.gateway"
              defaultMessage={'Deploy to Gateway'}
            />
          </Typography>
          <Typography variant="subtitle2">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploy.deploy.llm.proxy.to.your.gateways"
              defaultMessage={'Deploy App AI Proxy to your Gateways'}
            />
          </Typography>
          {proxy?.readOnly && (
            <Alert severity="info">
              This proxy was created from a gateway. You can view its deployments,
              but deploy, redeploy, restore and undeploy actions are managed by the
              gateway and are unavailable in AI Workspace.
            </Alert>
          )}
          <GatewayDeployMainSection showConfigureOption={false} />
        </Stack>
      </GatewayDeployProvider>
    </PageContent>
  );
}

export default function LLMProxyDeploy() {
  const { proxyId } = useParams<{ proxyId: string }>();

  if (!proxyId) {
    return (
      <PageContent>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploy.proxy.id.is.required"
            defaultMessage={'Proxy ID is required'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <ProxyProvider proxyId={proxyId}>
      <LLMProxyDeployContent />
    </ProxyProvider>
  );
}
