/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Grid, PageContent, Stack, Typography } from '@wso2/oxygen-ui';
import { Bot, Dock, Handshake, Workflow } from '@wso2/oxygen-ui-icons-react';
import McpMenuIcon from '../../../../assets/icons/McpMenuIcon';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  LLMProvidersProvider,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import { ProxiesProvider, useProxies } from '../../../../contexts/proxy';
import { MCPServersProvider, useMCPServers } from '../../../../contexts/MCP';
import {
  ApplicationsProvider,
  useApplications,
} from '../../../../contexts/ApplicationsContext';
import { getAgentProxies } from '../../../../apis/agent/agentProxiesApis';
import NoProviders from '../../../../assets/images/NoProviders.svg';
import NoProxies from '../../../../assets/images/NoProxies.svg';
import NoMCPServers from '../../../../assets/images/NoMCPServers.svg';
import NoAgents from '../../../../assets/images/NoAgents.svg';
import NoApplications from '../../../../assets/images/NoApplications.svg';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { buildProjectPath } from '../../../../utils/projectRouting';
import type { AgentProxyListItem } from '../../../../utils/types';
import ProjectsList from '../projects/ProjectsList';
import ProxyQuickStartBanner from '../projects/ProxyQuickStartBanner';
import KindSummaryCard from './KindSummaryCard';
import KindDetailPanel, { KindDetailItem } from './KindDetailPanel';
import { trackOverviewPageView } from '../../../../utils/app-insights';

type ResourceKind =
  | 'llm-providers'
  | 'llm-proxies'
  | 'mcp-proxies'
  | 'agent-proxies'
  | 'applications';

export default function Overview(): React.JSX.Element {
  const { currentProject } = useAppShell();

  if (!currentProject?.id) {
    return <ProjectsList disableRedirect />;
  }

  return (
    <LLMProvidersProvider>
      <ProxiesProvider>
        <MCPServersProvider>
          <ApplicationsProvider>
            <OverviewContent />
          </ApplicationsProvider>
        </MCPServersProvider>
      </ProxiesProvider>
    </LLMProvidersProvider>
  );
}

function OverviewContent(): React.JSX.Element {
  const navigate = useNavigate();
  const { user, hasPermission } = useAppAuth();
  const { currentProject, currentOrganization } = useAppShell();
  const [selectedKind, setSelectedKind] =
    useState<ResourceKind>('llm-providers');

  const providers = useLLMProviders();
  const proxies = useProxies();
  const mcpServers = useMCPServers();
  const applications = useApplications();

  // Agent proxies have no shared context, so the list is fetched here.
  const [agentProxies, setAgentProxies] = useState<AgentProxyListItem[]>([]);
  const [agentCount, setAgentCount] = useState(0);
  const [isAgentLoading, setIsAgentLoading] = useState(true);
  const [agentError, setAgentError] = useState<Error | null>(null);

  const projectId = currentProject?.id ?? '';

  const loadAgentProxies = React.useCallback(async () => {
    if (!projectId) return;
    setIsAgentLoading(true);
    setAgentError(null);
    try {
      const response = await getAgentProxies(projectId, PLATFORM_API_BASE_URL);
      setAgentProxies(response.list ?? []);
      setAgentCount(response.count ?? response.list?.length ?? 0);
    } catch (error) {
      setAgentError(error as Error);
    } finally {
      setIsAgentLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    void loadAgentProxies();
  }, [loadAgentProxies]);

  useEffect(() => {
    if (user?.email) {
      trackOverviewPageView(currentOrganization?.uuid, user.email, user.email);
    }
  }, []);

  const path = (suffix: string) =>
    buildProjectPath(currentOrganization, currentProject, suffix);

  const kinds = useMemo(
    () => [
      {
        id: 'llm-providers' as const,
        label: 'LLM Providers',
        icon: <Handshake size={26} />,
        count: providers.providersResponse.count,
        isLoading: providers.isLoading,
      },
      {
        id: 'llm-proxies' as const,
        label: 'App LLM Proxies',
        icon: <Workflow size={26} />,
        count: proxies.proxiesResponse.count,
        isLoading: proxies.isLoading,
      },
      {
        id: 'mcp-proxies' as const,
        label: 'MCP Proxies',
        icon: <McpMenuIcon size={26} />,
        count: mcpServers.mcpServersResponse.count,
        isLoading: mcpServers.isLoading,
      },
      {
        id: 'agent-proxies' as const,
        label: 'Agent Proxies',
        icon: <Bot size={26} />,
        count: agentCount,
        isLoading: isAgentLoading,
      },
      {
        id: 'applications' as const,
        label: 'GenAI Applications',
        icon: <Dock size={26} />,
        count: applications.applicationsResponse.count,
        isLoading: applications.isLoading,
      },
    ],
    [
      providers.providersResponse.count,
      providers.isLoading,
      proxies.proxiesResponse.count,
      proxies.isLoading,
      mcpServers.mcpServersResponse.count,
      mcpServers.isLoading,
      agentCount,
      isAgentLoading,
      applications.applicationsResponse.count,
      applications.isLoading,
    ]
  );

  const panel = useMemo(() => {
    switch (selectedKind) {
      case 'llm-proxies':
        return {
          title: 'App LLM Proxies',
          description: 'Route application traffic through a managed LLM proxy.',
          totalCount: proxies.proxiesResponse.count,
          items: proxies.proxiesResponse.list.map<KindDetailItem>((proxy) => ({
            id: proxy.id ?? proxy.displayName,
            displayName: proxy.displayName,
            subtitle: proxy.description,
            updatedAt: proxy.updatedAt ?? proxy.createdAt,
          })),
          isLoading: proxies.isLoading,
          error: proxies.error,
          onRetry: proxies.refreshProxies,
          viewAllPath: path('/proxies'),
          createPath: path('/proxies/create'),
          createLabel: 'Add LLM proxy',
          canCreate: hasPermission(SCOPES.LLM_PROXY_CREATE),
          emptyImage: NoProxies,
          emptyTitle: 'Create your first App LLM Proxy',
          emptyDescription:
            'Set up an App LLM Proxy to route model traffic and manage AI access across your applications.',
          onItemClick: (id: string) => navigate(path(`/proxies/${id}`)),
        };
      case 'mcp-proxies':
        return {
          title: 'MCP Proxies',
          description:
            'Expose tools, prompts and resources through your AI gateway.',
          totalCount: mcpServers.mcpServersResponse.count,
          items: mcpServers.mcpServersResponse.list.map<KindDetailItem>(
            (server) => ({
              id: server.id,
              displayName: server.displayName,
              subtitle: server.description,
              updatedAt: server.updatedAt ?? server.createdAt,
            })
          ),
          isLoading: mcpServers.isLoading,
          error: mcpServers.error,
          onRetry: mcpServers.refreshMCPServers,
          viewAllPath: path('/mcp-proxy'),
          createPath: path('/mcp-proxy/create'),
          createLabel: 'Add MCP proxy',
          canCreate: hasPermission(SCOPES.MCP_PROXY_CREATE),
          emptyImage: NoMCPServers,
          emptyTitle: 'Create your first MCP Proxy',
          emptyDescription:
            'Set up an MCP Proxy to expose tools, prompts, and resources through your AI gateway workflows.',
          onItemClick: (id: string) => navigate(path(`/mcp-proxy/${id}`)),
        };
      case 'agent-proxies':
        return {
          title: 'Agent Proxies',
          description: 'Front an A2A agent with policies, keys and a gateway.',
          totalCount: agentCount,
          items: agentProxies.map<KindDetailItem>((agentProxy) => ({
            id: agentProxy.id,
            displayName: agentProxy.displayName,
            subtitle: agentProxy.description,
            updatedAt: agentProxy.updatedAt ?? agentProxy.createdAt,
          })),
          isLoading: isAgentLoading,
          error: agentError,
          onRetry: () => void loadAgentProxies(),
          viewAllPath: path('/agent-proxy'),
          createPath: path('/agent-proxy/create'),
          createLabel: 'Add Agent proxy',
          canCreate: hasPermission(SCOPES.AGENT_PROXY_CREATE),
          emptyImage: NoAgents,
          emptyTitle: 'Create your first Agent Proxy',
          emptyDescription:
            'Set up an Agent Proxy to expose skills, tasks, and messages through your AI gateway workflows.',
          onItemClick: (id: string) => navigate(path(`/agent-proxy/${id}`)),
        };
      case 'applications':
        return {
          title: 'GenAI Applications',
          description: 'Applications that consume the APIs in this project.',
          totalCount: applications.applicationsResponse.count,
          items: applications.applicationsResponse.list.map<KindDetailItem>(
            (application) => ({
              id: application.id,
              displayName: application.displayName,
              subtitle: application.description,
              updatedAt: application.updatedAt ?? application.createdAt,
            })
          ),
          isLoading: applications.isLoading,
          error: applications.error,
          onRetry: applications.refreshApplications,
          viewAllPath: path('/applications'),
          createPath: path('/applications/create'),
          createLabel: 'Add application',
          canCreate: hasPermission(SCOPES.APPLICATION_CREATE),
          emptyImage: NoApplications,
          emptyTitle: 'Create your first GenAI Application',
          emptyDescription:
            'Set up a GenAI application to securely consume AI services through your workspace.',
          onItemClick: (id: string) => navigate(path(`/applications/${id}`)),
        };
      default:
        return {
          title: 'LLM Providers',
          description:
            'Connect the model providers your workspace routes traffic to.',
          totalCount: providers.providersResponse.count,
          items: providers.providersResponse.list.map<KindDetailItem>(
            (provider) => ({
              id: provider.id ?? provider.displayName,
              displayName: provider.displayName,
              subtitle: provider.template
                ? `Template: ${provider.template}`
                : provider.description,
              updatedAt:
                provider.lastUpdated ??
                provider.updatedAt ??
                provider.createdAt,
            })
          ),
          isLoading: providers.isLoading,
          error: providers.error,
          onRetry: providers.refreshProviders,
          viewAllPath: path('/service-provider'),
          createPath: path('/service-provider/create'),
          createLabel: 'Add LLM provider',
          canCreate: hasPermission(SCOPES.LLM_PROVIDER_CREATE),
          emptyImage: NoProviders,
          emptyTitle: 'Create your first LLM Provider',
          emptyDescription:
            'Set up an LLM provider to start connecting models and powering AI applications in your workspace.',
          onItemClick: (id: string) => navigate(path(`/service-provider/${id}`)),
        };
    }
  }, [
    selectedKind,
    providers,
    proxies,
    mcpServers,
    applications,
    agentProxies,
    agentCount,
    isAgentLoading,
    agentError,
  ]);

  return (
    <PageContent fullWidth>
      <Stack spacing={3}>
        {hasPermission(SCOPES.LLM_PROXY_CREATE) ? <ProxyQuickStartBanner /> : null}

        <Box>
          <Typography variant="h5" sx={{ fontWeight: 600 }}>
            Overview
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Resources in the {currentProject?.displayName ?? 'current'} project.
          </Typography>
        </Box>

        <Grid container spacing={2}>
          {kinds.map((kind) => (
            <Grid key={kind.id} size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
              <KindSummaryCard
                label={kind.label}
                icon={kind.icon}
                count={kind.count}
                isLoading={kind.isLoading}
                selected={selectedKind === kind.id}
                onSelect={() => setSelectedKind(kind.id)}
              />
            </Grid>
          ))}
        </Grid>

        <KindDetailPanel {...panel} />
      </Stack>
    </PageContent>
  );
}
