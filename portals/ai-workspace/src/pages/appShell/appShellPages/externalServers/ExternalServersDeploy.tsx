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
  Alert,
  Button,
  CircularProgress,
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
  server: MCPServer | null;
};

function ExternalServersDeployLayout({ server }: ExternalServersDeployLayoutProps) {
  const navigate = useNavigate();
  const { deployments } = useGatewayDeploy();

  const hasDeployments = (deployments?.list ?? []).some((d) => d.status === 'DEPLOYED');
  const hasPolicies = (server?.policies?.length ?? 0) > 0;
  const isReadOnly = Boolean(server?.readOnly);

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
        serverName={server?.displayName}
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
        {isReadOnly && (
          <Alert severity="info">
            This MCP proxy was created from a gateway. You can view its
            deployments, but deploy, redeploy, restore and undeploy actions are
            managed by the gateway and are unavailable in AI Workspace.
          </Alert>
        )}
        <GatewayDeployMainSection showConfigureOption={false} />
      </Stack>
    </PageContent>
  );
}

export default function ExternalServersDeploy() {
  const { serverId } = useParams<{ serverId: string }>();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const [server, setServer] = useState<MCPServer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    setLoading(true);
    setError(false);
    mcpProxiesApis
      .getMCPServer(serverId, PLATFORM_API_BASE_URL)
      .then((res) => { if (!cancelled) setServer(res); })
      .catch(() => { if (!cancelled) setError(true); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [serverId, organizationId, apimBaseUrl]);

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

  if (loading) {
    return (
      <PageContent fullWidth>
        <Stack alignItems="center" sx={{ py: 6 }}>
          <CircularProgress />
        </Stack>
      </PageContent>
    );
  }

  // A failed or empty fetch must NOT fall back to a writable deploy UI: the
  // read-only guard is derived from the server, so rendering the deploy actions
  // without a resolved server would expose deploy/undeploy on a gateway-managed
  // (read-only) proxy. Show an error instead and gate the writable UI entirely.
  if (error || !server) {
    return (
      <PageContent fullWidth>
        <Alert severity="error">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.deploy.failed.to.load.server"
            defaultMessage="Failed to load the MCP proxy. Please try again."
          />
        </Alert>
      </PageContent>
    );
  }

  return (
    <GatewayDeployProvider
      apiId={serverId}
      resourceType="mcp-server"
      readOnly={Boolean(server.readOnly)}
    >
      <ExternalServersDeployLayout server={server} />
    </GatewayDeployProvider>
  );
}
