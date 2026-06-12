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

import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Button,
  PageContent,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import {
  GatewayDeployProvider,
  useGatewayDeploy,
} from '../../../../contexts/GatewayDeployContext';
import { GatewayDeployMainSection } from '../../../../Components/GatewayDeploy';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import type { MCPServer } from '../../../../utils/types';
import ExternalServerStepBanner from '../quickStart/ExternalServerStepBanner';
import type { ExternalServerStepBannerStepId } from '../quickStart/ExternalServerStepBanner';

type ExternalServersDeployLayoutProps = {
  serverId: string;
};

function ExternalServersDeployLayout({ serverId }: ExternalServersDeployLayoutProps) {
  const navigate = useNavigate();
  const { deployments } = useGatewayDeploy();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const [server, setServer] = useState<MCPServer | null>(null);

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    mcpProxiesApis
      .getMCPServer(serverId, organizationId, apimBaseUrl)
      .then((res) => { if (!cancelled) setServer(res); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [serverId, organizationId, apimBaseUrl]);

  const hasDeployments = (deployments?.list ?? []).some((d) => d.status === 'DEPLOYED');
  const hasPolicies = (server?.policies?.length ?? 0) > 0;

  const handleStepClick = (stepId: ExternalServerStepBannerStepId) => {
    if (stepId === 'add-policies') {
      navigate(-1);
    }
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={24} />} onClick={() => navigate(-1)}>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.externalServers.deploy.back.to.external.server"
          defaultMessage="Back to MCP Proxy"
        />
      </Button>

      <ExternalServerStepBanner
        serverName={server?.name}
        hasPolicies={hasPolicies}
        hasDeployments={hasDeployments}
        onStepClick={handleStepClick}
      />

      <Stack spacing={2} sx={{ mt: 2 }}>
        <Typography variant="h3">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.deploy.deploy.to.gateway"
            defaultMessage="Deploy to Gateway"
          />
        </Typography>
        <Typography variant="subtitle2">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.deploy.deploy.external.server.to.your.gateways"
            defaultMessage="Deploy MCP Proxy to your Gateways"
          />
        </Typography>
        <GatewayDeployMainSection showConfigureOption={false} />
      </Stack>
    </PageContent>
  );
}

export default function ExternalServersDeploy() {
  const { serverId } = useParams<{ serverId: string }>();

  if (!serverId) {
    return (
      <PageContent fullWidth>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.deploy.server.id.is.required"
            defaultMessage="Server ID is required"
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <GatewayDeployProvider apiId={serverId} resourceType="mcp-server">
      <ExternalServersDeployLayout serverId={serverId} />
    </GatewayDeployProvider>
  );
}
