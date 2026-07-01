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

import { useEffect, useMemo } from 'react';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useNavigate } from 'react-router-dom';
import { Grid, PageContent } from '@wso2/oxygen-ui';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  LLMProvidersProvider,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import { ProxiesProvider } from '../../../../contexts/proxy';
import { MCPServersProvider } from '../../../../contexts/MCP';
import { ApplicationsProvider } from '../../../../contexts/ApplicationsContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import ProjectsList from '../projects/ProjectsList';
import ProxyQuickStartBanner from '../projects/ProxyQuickStartBanner';
import ServiceProvidersSummaryCard, {
  ServiceProviderSummaryItem,
} from '../serviceProvider/ServiceProvidersSummaryCard';
import LLMProxiesSummaryCardSection from '../proxies/LLMProxiesSummaryCardSection';
import MCPProxiesSummaryCardSection from '../externalServers/MCPProxiesSummaryCardSection';
import GenAIApplicationsSummaryCardSection from '../applications/GenAIApplicationsSummaryCardSection';
import { trackOverviewPageView } from '../../../../utils/app-insights';

export default function Overview() {
  const navigate = useNavigate();
  const { user, hasPermission } = useAppAuth();
  const { currentProject, currentOrganization, setCurrentProject } =
    useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);

  const proxiesPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/proxies'
  );
  const newProxyPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/proxies/new'
  );
  const mcpProxiesPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/mcp-proxy'
  );
  const newMCPProxyPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/mcp-proxy/new'
  );
  const applicationsPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/applications'
  );
  const newApplicationPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/applications/new'
  );
  const serviceProvidersPath = buildProjectPath(
    currentOrganization,
    currentProject,
    '/service-provider'
  );

  const handleProvidersSeeMore = () => navigate(serviceProvidersPath);
  const handleAddProvider =
    hasPermission(SCOPES.LLM_PROVIDER_CREATE) && isProjectLevel
      ? () => {
          setCurrentProject(null);
          navigate(buildOrgPath(currentOrganization, '/service-provider/new'));
        }
      : undefined;

  const handleProviderClick = (providerId: string) => {
    navigate(
      buildProjectPath(
        currentOrganization,
        currentProject,
        `/service-provider/${providerId}`
      )
    );
  };

  const handleProxyClick = (proxyId: string) => {
    navigate(
      buildProjectPath(currentOrganization, currentProject, `/proxies/${proxyId}`)
    );
  };

  const handleMCPProxyClick = (proxyId: string) => {
    navigate(
      buildProjectPath(
        currentOrganization,
        currentProject,
        `/mcp-proxy/${proxyId}`
      )
    );
  };

  const handleApplicationClick = (applicationId: string) => {
    navigate(
      buildProjectPath(
        currentOrganization,
        currentProject,
        `/applications/${applicationId}`
      )
    );
  };

  useEffect(() => {
    const trackOverviewVisit = async () => {
      try {
        if (user?.email) {
          trackOverviewPageView(
            currentOrganization?.uuid,
            user.email,
            user.email
          );
        }
      } catch (error) {
        // Silently fail if auth is not available
      }
    };

    trackOverviewVisit();
  }, []);

  if (!currentProject?.id) {
    return <ProjectsList disableRedirect />;
  }

  return (
    <PageContent fullWidth>
      <Grid container spacing={3} sx={{ width: '100%', m: 0 }}>
        {hasPermission(SCOPES.LLM_PROXY_CREATE) ? (
          <Grid size={{ xs: 12 }} sx={{ width: '100%' }}>
            <ProxyQuickStartBanner />
          </Grid>
        ) : null}

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          <LLMProvidersProvider>
            <ServiceProvidersSummaryCardSection
              onSeeMore={handleProvidersSeeMore}
              onAddProvider={handleAddProvider}
              onCreateProvider={handleAddProvider}
              onProviderClick={handleProviderClick}
            />
          </LLMProvidersProvider>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          <ProxiesProvider>
            <LLMProxiesSummaryCardSection
              proxiesPath={proxiesPath}
              newProxyPath={newProxyPath}
              onProxyClick={handleProxyClick}
            />
          </ProxiesProvider>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          <MCPServersProvider>
            <MCPProxiesSummaryCardSection
              mcpProxiesPath={mcpProxiesPath}
              newMCPProxyPath={newMCPProxyPath}
              onMCPProxyClick={handleMCPProxyClick}
            />
          </MCPServersProvider>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          <ApplicationsProvider>
            <GenAIApplicationsSummaryCardSection
              applicationsPath={applicationsPath}
              newApplicationPath={newApplicationPath}
              onApplicationClick={handleApplicationClick}
            />
          </ApplicationsProvider>
        </Grid>
      </Grid>
    </PageContent>
  );
}

function ServiceProvidersSummaryCardSection({
  onSeeMore,
  onAddProvider,
  onCreateProvider,
  onProviderClick,
}: {
  onSeeMore: () => void;
  onAddProvider?: () => void;
  onCreateProvider?: () => void;
  onProviderClick: (providerId: string) => void;
}) {
  const {
    providersResponse,
    isLoading: isProvidersLoading,
    error: providersError,
    refreshProviders,
  } = useLLMProviders();

  const providerItems = useMemo<ServiceProviderSummaryItem[]>(
    () =>
      providersResponse.list.map((provider) => ({
        id: provider.id ?? provider.displayName,
        displayName: provider.displayName,
        status: provider.status ?? 'Unknown',
        lastUpdated:
          provider.lastUpdated ?? provider.updatedAt ?? provider.createdAt,
        description: provider.description,
        template: provider.template,
        modelCount:
          provider.modelProviders?.reduce(
            (total, mp) => total + (mp.models?.length ?? 0),
            0
          ) ?? 0,
      })),
    [providersResponse.list]
  );

  return (
    <ServiceProvidersSummaryCard
      providers={providerItems}
      totalCount={providersResponse.count}
      isLoading={isProvidersLoading}
      error={providersError}
      onRetry={refreshProviders}
      onSeeMore={onSeeMore}
      onAddProvider={providersResponse.count >= 5 ? undefined : onAddProvider}
      onCreateProvider={onCreateProvider}
      showSeeMore={providersResponse.count >= 5}
      onProviderClick={onProviderClick}
    />
  );
}
