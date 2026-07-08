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

import React, { useMemo, useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  Divider,
  Grid,
  PageContent,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Layers } from '@wso2/oxygen-ui-icons-react';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import {
  LLMProvidersProvider,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import AILoader from '../../../../Components/AILoader';
import ServiceProvidersSummaryCard, {
  ServiceProviderSummaryItem,
} from '../serviceProvider/ServiceProvidersSummaryCard';
import ExploreMoreCard from './ExploreMoreCard';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { FormattedMessage } from 'react-intl';

type ProjectsListProps = {
  disableRedirect?: boolean;
};

function truncateWords(text: string, maxWords: number): string {
  const words = text.trim().split(/\s+/);
  if (words.length <= maxWords) return text.trim();
  return `${words.slice(0, maxWords).join(' ')}…`;
}

export default function ProjectsList({ disableRedirect }: ProjectsListProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { hasPermission } = useAppAuth();
  const {
    projectsForCurrentOrganization,
    isProjectsLoading,
    currentOrganization,
    setCurrentProject,
    error,
    userName,
    userEmail,
  } = useAppShell();
  const welcomeName =
    userName?.trim() || userEmail?.trim() || currentOrganization?.name || 'there';
  const renderServiceProvidersCard = () => {
    const canAddProvider = hasPermission(SCOPES.LLM_PROVIDER_CREATE);
    const handleAddProvider = canAddProvider
      ? () => {
          const path = buildOrgPath(
            currentOrganization,
            '/service-provider/create'
          );
          navigate(path);
        }
      : undefined;
    return (
      <LLMProvidersProvider>
        <ServiceProvidersSummaryCardSection
          onSeeMore={() => {
            if (orgServiceProviderPath) navigate(orgServiceProviderPath);
          }}
          onAddProvider={handleAddProvider}
          onCreateProvider={handleAddProvider}
          onProviderClick={(providerId) => {
            const providerPath = buildOrgPath(
              currentOrganization,
              `/service-provider/${providerId}`
            );
            navigate(providerPath);
          }}
        />
      </LLMProvidersProvider>
    );
  };

  const totalProjects = projectsForCurrentOrganization.length;
  const latestProjects = useMemo(() => {
    return [...projectsForCurrentOrganization]
      .sort((a, b) => {
        const aTime = new Date(a.updatedAt || a.createdAt || 0).getTime();
        const bTime = new Date(b.updatedAt || b.createdAt || 0).getTime();
        return bTime - aTime;
      })
      .slice(0, 5);
  }, [projectsForCurrentOrganization]);

  const orgProjectsPath = buildOrgPath(currentOrganization, '/projects');
  const orgProjectsListPath = buildOrgPath(
    currentOrganization,
    '/projects/list'
  );
  const orgServiceProviderPath = buildOrgPath(
    currentOrganization,
    '/service-provider'
  );

  if (isProjectsLoading) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="center" sx={{ py: 6 }}>
          <AILoader />
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectsList.loading.projects"
              defaultMessage={'Loading projects...'}
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth>
        <Typography color="error">{error}</Typography>
      </PageContent>
    );
  }

  useEffect(() => {
    if (disableRedirect) return;
    if (!orgProjectsPath) return;
    if (location.pathname !== orgProjectsPath) {
      navigate(orgProjectsPath, { replace: true });
    }
  }, [disableRedirect, orgProjectsPath, location.pathname, navigate]);

  return (
    <PageContent fullWidth>
      <Grid container spacing={3} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }} sx={{ width: '100%' }}>
          <Box
            sx={{
              textAlign: 'center',
            }}
            mb={1}
          >
            <Typography variant="h3" sx={{ fontWeight: 700 }}>
              Welcome {welcomeName}!
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectsList.let.s.get.started.with.ai"
                defaultMessage={"Let's get started with AI!"}
              />
            </Typography>
          </Box>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          {renderServiceProvidersCard()}
        </Grid>

        <Grid size={{ xs: 12, md: 6 }} sx={{ width: '100%' }}>
          <Card sx={{ height: '100%', width: '100%' }}>
            <CardHeader
              title="Projects"
              subheader={`Total: ${totalProjects}`}
              slotProps={{
                title: {
                  sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 },
                },
                subheader: { sx: { fontSize: '0.82rem' } },
              }}
              action={
                <Button
                  size="small"
                  onClick={() => {
                    if (orgProjectsListPath) navigate(orgProjectsListPath);
                  }}
                >
                  See more
                </Button>
              }
            />
            <CardContent sx={{ pt: 0,}}>
              {latestProjects.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectsList.no.projects.available"
                    defaultMessage={'No projects available.'}
                  />
                </Typography>
              ) : (
                <Stack divider={<Divider />} spacing={1.5}>
                  {latestProjects.map((project) => (
                    <Box
                      key={project.id}
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 1.5,
                        width: '100%',
                      }}
                    >
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 1.25,
                          minWidth: 0,
                          cursor: 'pointer',
                          py: 1,
                        }}
                        onClick={() => {
                          setCurrentProject(project);
                          navigate(
                            buildProjectPath(
                              currentOrganization,
                              project,
                              '/home'
                            )
                          );
                        }}
                      >
                        <Avatar
                          sx={{
                            width: 36,
                            height: 36,
                            bgcolor: 'primary.light',
                            color: 'primary.contrastText',
                          }}
                        >
                          <Layers size={18} />
                        </Avatar>
                        <Box sx={{ minWidth: 0 }}>
                          <Typography variant="body1" sx={{ fontWeight: 600 }}>
                            {truncateWords(project.displayName || 'No Name', 12)}
                          </Typography>
                          <Typography
                            variant="body2"
                            color="text.secondary"
                            fontSize="0.7rem"
                            noWrap
                          >
                            {truncateWords(project.description?.trim() || '', 12)}
                          </Typography>
                        </Box>
                      </Box>

                      <Stack direction="row" spacing={0.75} alignItems="center" sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}>
                        <Clock size={14} />
                        <Typography variant="caption" color="text.secondary" noWrap>
                          {formatRelativeTime(
                            project.updatedAt || project.createdAt
                          )}
                        </Typography>
                      </Stack>
                    </Box>
                  ))}
                </Stack>
              )}
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12 }} sx={{ width: '100%' }}>
          <ExploreMoreCard />
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
  } = useLLMProviders();

  const providerItems = useMemo<ServiceProviderSummaryItem[]>(
    () =>
      providersResponse.list
        // Only providers with a real id can be turned into a navigable
        // detail route; skip ones missing an id rather than routing to a label.
        .filter((provider) => Boolean(provider.id))
        .map((provider) => ({
          id: provider.id as string,
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
      onSeeMore={onSeeMore}
      onAddProvider={providersResponse.count >= 5 ? undefined : onAddProvider}
      onCreateProvider={onCreateProvider}
      showSeeMore={providersResponse.count >= 5}
      onProviderClick={onProviderClick}
    />
  );
}
