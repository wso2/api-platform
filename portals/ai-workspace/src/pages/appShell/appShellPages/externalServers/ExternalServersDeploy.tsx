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

import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  MenuItem,
  PageContent,
  Select,
  Stack,
  TextField,
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
import {
  checkMCPServerPublished,
  publishMCPServer,
  unpublishMCPServer,
} from '../../../../apis/MCP/mcpDevPortalApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { DEV_PORTAL_BASE_URL, PLATFORM_API_BASE_URL } from '../../../../config.env';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type { MCPServer } from '../../../../utils/types';
import ExternalServerStepBanner from '../quickStart/ExternalServerStepBanner';
import type { ExternalServerStepBannerStepId } from '../quickStart/ExternalServerStepBanner';

function buildEndpointUrl(vhost: string, context: string): string {
  const normalizedVhost = vhost.trim();
  const base = /^https?:\/\//i.test(normalizedVhost)
    ? normalizedVhost.replace(/\/+$/, '')
    : `https://${normalizedVhost.replace(/\/+$/, '')}`;
  const ctx = context.trim().replace(/\/+$/, '');
  const normalizedCtx = ctx ? (ctx.startsWith('/') ? ctx : `/${ctx}`) : '';
  return `${base}${normalizedCtx}/mcp`;
}

type ExternalServersDeployLayoutProps = {
  serverId: string;
};

function ExternalServersDeployLayout({ serverId }: ExternalServersDeployLayoutProps) {
  const navigate = useNavigate();
  const { deployments, gateways, isLoading: isGatewaysLoading, isLoadingDeployments } = useGatewayDeploy();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const showSnackbar = useAIWorkspaceSnackbar();

  const [server, setServer] = useState<MCPServer | null>(null);
  const [isPublished, setIsPublished] = useState(false);
  const [isPublishActionLoading, setIsPublishActionLoading] = useState(false);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [isUnpublishConfirmOpen, setIsUnpublishConfirmOpen] = useState(false);
  const [publishDialogGatewayId, setPublishDialogGatewayId] = useState('');

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    mcpProxiesApis
      .getMCPServer(serverId, organizationId, apimBaseUrl)
      .then((res) => { if (!cancelled) setServer(res); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [serverId, organizationId, apimBaseUrl]);

  const deployedGateways = useMemo(() => {
    if (!deployments?.list || !gateways.length) return [];
    const deployedIds = new Set(
      deployments.list.filter((d) => d.status === 'DEPLOYED').map((d) => d.gatewayId)
    );
    return gateways.filter((g) => deployedIds.has(g.id));
  }, [deployments, gateways]);

  // Auto-select first gateway when dialog opens while gateways are still loading
  useEffect(() => {
    if (isPublishDialogOpen && !publishDialogGatewayId && deployedGateways.length > 0) {
      setPublishDialogGatewayId(deployedGateways[0].id);
    }
  }, [isPublishDialogOpen, publishDialogGatewayId, deployedGateways]);

  useEffect(() => {
    if (!server || !organizationId || !currentOrganization?.handle) return;
    let cancelled = false;
    void checkMCPServerPublished(DEV_PORTAL_BASE_URL, currentOrganization.handle, server.id)
      .then((published) => { if (!cancelled) setIsPublished(published); })
      .catch(() => { if (!cancelled) setIsPublished(false); });
    return () => { cancelled = true; };
  }, [server, organizationId, currentOrganization?.handle]);

  const hasDeployments = (deployments?.list ?? []).some((d) => d.status === 'DEPLOYED');
  const hasPolicies = (server?.policies?.length ?? 0) > 0;
  const isDialogLoading = isGatewaysLoading || isLoadingDeployments;

  const mcpHubViewUrl = useMemo(() => {
    if (!isPublished || !server || !currentOrganization?.handle) return null;
    return `${DEV_PORTAL_BASE_URL.replace(/\/?$/, '')}/${encodeURIComponent(currentOrganization.handle)}/views/default/mcp/${encodeURIComponent(server.id)}`;
  }, [isPublished, server, currentOrganization?.handle]);

  const previewEndpointUrl = useMemo(() => {
    const gw = deployedGateways.find((g) => g.id === publishDialogGatewayId);
    if (!gw) return '';
    return buildEndpointUrl(gw.vhost ?? '', server?.context ?? '');
  }, [deployedGateways, publishDialogGatewayId, server?.context]);

  const refreshPublishStatus = async () => {
    if (!server || !currentOrganization?.handle) return;
    try {
      setIsPublished(await checkMCPServerPublished(DEV_PORTAL_BASE_URL, currentOrganization.handle, server.id));
    } catch { /* ignore */ }
  };

  const handleConfirmPublish = async () => {
    if (!server || !organizationId || !currentOrganization?.handle || !previewEndpointUrl) return;
    setIsPublishActionLoading(true);
    setIsPublishDialogOpen(false);
    try {
      await publishMCPServer(apimBaseUrl, server.id, organizationId, { orgHandle: currentOrganization.handle, remoteUrl: previewEndpointUrl });
      showSnackbar('MCP Proxy published to MCP Hub.', 'success');
      await refreshPublishStatus();
    } catch {
      showSnackbar('Failed to publish MCP Proxy.', 'error');
    } finally {
      setIsPublishActionLoading(false);
    }
  };

  const handleConfirmUnpublish = async () => {
    if (!server || !organizationId || !currentOrganization?.handle) return;
    setIsPublishActionLoading(true);
    setIsUnpublishConfirmOpen(false);
    try {
      await unpublishMCPServer(apimBaseUrl, server.id, organizationId, currentOrganization.handle);
      showSnackbar('MCP Proxy unpublished from MCP Hub.', 'success');
      await refreshPublishStatus();
    } catch {
      showSnackbar('Failed to unpublish MCP Proxy.', 'error');
    } finally {
      setIsPublishActionLoading(false);
    }
  };

  const handleStepClick = (stepId: ExternalServerStepBannerStepId) => {
    if (stepId === 'add-policies') {
      navigate(-1);
    } else if (stepId === 'publish-to-devportal') {
      if (isPublished && mcpHubViewUrl) {
        window.open(mcpHubViewUrl, '_blank', 'noopener,noreferrer');
      } else {
        setPublishDialogGatewayId(deployedGateways[0]?.id || '');
        setIsPublishDialogOpen(true);
      }
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
        isPublished={isPublished}
        devPortalUrl={mcpHubViewUrl}
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

      <Dialog open={isPublishDialogOpen} onClose={() => setIsPublishDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Publish to MCP Hub</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <Typography variant="body2" color="text.secondary">
              Select the gateway endpoint that clients will use to connect to this MCP proxy in the MCP Hub.
            </Typography>
            {isDialogLoading ? (
              <Stack direction="row" alignItems="center" spacing={1}>
                <CircularProgress size={16} />
                <Typography variant="caption" color="text.secondary">Loading gateways...</Typography>
              </Stack>
            ) : null}
            <FormControl fullWidth>
              <FormLabel>Gateway</FormLabel>
              <Select size="small" value={publishDialogGatewayId} onChange={(e) => setPublishDialogGatewayId(String(e.target.value))}>
                {deployedGateways.map((gateway) => (
                  <MenuItem key={gateway.id} value={gateway.id}>
                    {gateway.displayName || gateway.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            {previewEndpointUrl ? (
              <FormControl fullWidth>
                <FormLabel>Endpoint URL</FormLabel>
                <TextField size="small" value={previewEndpointUrl} slotProps={{ input: { readOnly: true } }} />
              </FormControl>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setIsPublishDialogOpen(false)}>Cancel</Button>
          <Button variant="contained" disabled={!publishDialogGatewayId || isPublishActionLoading} onClick={() => void handleConfirmPublish()}>
            Publish
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={isUnpublishConfirmOpen} onClose={() => setIsUnpublishConfirmOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Unpublish from MCP Hub</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Are you sure you want to unpublish <strong>{server?.name}</strong> from the MCP Hub? Clients will no longer be able to discover it.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setIsUnpublishConfirmOpen(false)}>Cancel</Button>
          <Button variant="contained" color="error" disabled={isPublishActionLoading} onClick={() => void handleConfirmUnpublish()}>
            Unpublish
          </Button>
        </DialogActions>
      </Dialog>
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
