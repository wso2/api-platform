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

import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Alert,
  Box,
  Card,
  Chip,
  Grid,
  IconButton,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  CheckCircle2,
  Package,
  Plus,
  Rocket,
  X,
} from '@wso2/oxygen-ui-icons-react';
import { getGateways } from '../../../../apis/gatewayApis';
import { getLLMProviders } from '../../../../apis/llmProviderApis';
import { getLLMProxyDeployments } from '../../../../apis/llmProxiesApis';
import * as proxyApis from '../../../../apis/proxyApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useRole } from '../../../../contexts/RoleContext';
import type { ProjectBase, Proxy } from '../../../../utils/types';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import {
  readOrganizationFlagMap,
  sortByLatest,
  writeOrganizationFlag,
} from '../../../../utils/quickStartUtils';
import QuickstartbannerImg from '../../../../assets/images/QuickstartbannerImg.svg';

const PROXY_QUICK_START_DISMISS_KEY =
  'aiWorkspace.proxyQuickStartBanner.dismissedOrganizations';
const PROXY_QUICK_START_CONSUMED_KEY =
  'aiWorkspace.proxyQuickStartBanner.consumedOrganizations';

type StepIconComponent = React.ComponentType<{ size?: string | number }>;

type StepDefinition = {
  id: string;
  title: string;
  icon: StepIconComponent;
  path: string;
  completed: boolean;
};

type CompletionState = {
  hasProviders: boolean;
  hasGateways: boolean;
  hasProxies: boolean;
  hasProxyDeployments: boolean;
  hasProxyConsumptions: boolean;
  latestProject: ProjectBase | null;
  latestProxyId: string;
};

const INITIAL_COMPLETION_STATE: CompletionState = {
  hasProviders: false,
  hasGateways: false,
  hasProxies: false,
  hasProxyDeployments: false,
  hasProxyConsumptions: false,
  latestProject: null,
  latestProxyId: '',
};

function getLatestProject(projects: ProjectBase[]): ProjectBase | null {
  const sortedProjects = sortByLatest(projects, (project) => [
    project.updatedAt,
    project.createdDate,
  ]);
  return sortedProjects[0] ?? null;
}

type ProxyWithProject = {
  project: ProjectBase;
  proxy: Proxy;
};

function getLatestProxyEntry(
  entries: ProxyWithProject[]
): ProxyWithProject | null {
  const sortedEntries = sortByLatest(entries, (entry) => [
    entry.proxy.updatedAt,
    entry.proxy.createdAt,
  ]);
  return sortedEntries[0] ?? null;
}

export default function ProxyQuickStartBanner() {
  const navigate = useNavigate();
  const {
    currentOrganization,
    projectsForCurrentOrganization,
    isProjectsLoading,
    userName,
    userEmail,
  } = useAppShell();
  const { role } = useRole();
  const organizationId = currentOrganization?.uuid ?? '';
  const isDeveloper = role === 'developer';
  const welcomeName =
    userName?.trim() ||
    userEmail?.trim() ||
    currentOrganization?.name ||
    'there';

  const [isDismissed, setIsDismissed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [completion, setCompletion] = useState<CompletionState>(
    INITIAL_COMPLETION_STATE
  );

  const latestProject = useMemo(
    () => getLatestProject(projectsForCurrentOrganization),
    [projectsForCurrentOrganization]
  );

  useEffect(() => {
    if (!isDeveloper) {
      setIsDismissed(false);
      return;
    }
    if (!organizationId) {
      setIsDismissed(false);
      return;
    }

    const dismissedOrganizations = readOrganizationFlagMap(
      PROXY_QUICK_START_DISMISS_KEY
    );
    setIsDismissed(Boolean(dismissedOrganizations[organizationId]));
  }, [organizationId, isDeveloper]);

  useEffect(() => {
    if (!isDeveloper) {
      setCompletion(INITIAL_COMPLETION_STATE);
      setIsLoading(false);
      return;
    }
    if (!organizationId || isDismissed || isProjectsLoading) {
      setCompletion(INITIAL_COMPLETION_STATE);
      setIsLoading(false);
      return;
    }

    let isCancelled = false;

    const fetchCompletionState = async () => {
      setIsLoading(true);

      const [providersResult, gatewaysResult] = await Promise.allSettled([
        getLLMProviders(organizationId, PLATFORM_API_BASE_URL),
        getGateways(organizationId),
      ]);

      const hasProviders =
        providersResult.status === 'fulfilled'
          ? (providersResult.value.list?.length ?? 0) > 0
          : false;
      const gateways =
        gatewaysResult.status === 'fulfilled'
          ? (gatewaysResult.value.list ?? []).filter(
              (gw) => gw.functionalityType === 'ai'
            )
          : [];
      const hasGateways = gateways.length > 0;
      const hasPrerequisites = hasProviders && hasGateways;
      const consumedByOrg = Boolean(
        readOrganizationFlagMap(PROXY_QUICK_START_CONSUMED_KEY)[organizationId]
      );

      let hasProxies = false;
      let hasProxyDeployments = false;
      let latestProxyId = '';
      let latestProjectForRouting = latestProject;

      if (hasPrerequisites && projectsForCurrentOrganization.length > 0) {
        const proxiesByProject = await Promise.allSettled(
          projectsForCurrentOrganization.map(async (project) => {
            const proxiesResponse = await proxyApis.getProxies(
              organizationId,
              project.id,
              PLATFORM_API_BASE_URL
            );
            return {
              project,
              proxies: proxiesResponse.list ?? [],
            };
          })
        );

        const proxyEntries: ProxyWithProject[] = proxiesByProject
          .filter(
            (
              result
            ): result is PromiseFulfilledResult<{
              project: ProjectBase;
              proxies: Proxy[];
            }> => result.status === 'fulfilled'
          )
          .flatMap(({ value }) =>
            value.proxies
              .filter((proxy) => Boolean(proxy.id))
              .map((proxy) => ({
                project: value.project,
                proxy,
              }))
          );

        hasProxies = proxyEntries.length > 0;
        const latestProxyEntry = getLatestProxyEntry(proxyEntries);
        latestProxyId = latestProxyEntry?.proxy.id ?? '';
        latestProjectForRouting = latestProxyEntry?.project ?? latestProject;

        if (proxyEntries.length > 0 && gateways.length > 0) {
          const deploymentChecks = await Promise.allSettled(
            proxyEntries.flatMap(({ proxy }) =>
              gateways
                .filter((gateway) => Boolean(gateway.id))
                .map((gateway) =>
                  getLLMProxyDeployments(
                    proxy.id,
                    organizationId,
                    PLATFORM_API_BASE_URL,
                    gateway.id
                  )
                )
            )
          );
          hasProxyDeployments = deploymentChecks.some(
            (result) =>
              result.status === 'fulfilled' &&
              (result.value.list?.length ?? 0) > 0
          );
        }
      }

      if (isCancelled) return;

      setCompletion({
        hasProviders,
        hasGateways,
        hasProxies,
        hasProxyDeployments,
        hasProxyConsumptions: consumedByOrg,
        latestProject: latestProjectForRouting,
        latestProxyId,
      });
      setIsLoading(false);
    };

    fetchCompletionState().catch(() => {
      if (isCancelled) return;
      setCompletion(INITIAL_COMPLETION_STATE);
      setIsLoading(false);
    });

    return () => {
      isCancelled = true;
    };
  }, [
    organizationId,
    isDeveloper,
    latestProject,
    projectsForCurrentOrganization,
    isDismissed,
    isProjectsLoading,
    PLATFORM_API_BASE_URL,
  ]);

  const defaultProjectsPath = buildOrgPath(currentOrganization, '/projects');
  const createProxyPath = completion.latestProject
    ? buildProjectPath(
        currentOrganization,
        completion.latestProject,
        '/proxies/new'
      )
    : defaultProjectsPath;
  const deployProxyPath =
    completion.latestProject && completion.latestProxyId
      ? buildProjectPath(
          currentOrganization,
          completion.latestProject,
          `/proxies/${completion.latestProxyId}/deploy`
        )
      : createProxyPath;
  const consumeProxyPath =
    completion.latestProject && completion.latestProxyId
      ? buildProjectPath(
          currentOrganization,
          completion.latestProject,
          `/proxies/${completion.latestProxyId}`
        )
      : createProxyPath;

  const steps = useMemo<StepDefinition[]>(
    () => [
      {
        id: 'create-proxy',
        title: 'Create\nApp LLM Proxy',
        icon: Plus,
        path: createProxyPath,
        completed: completion.hasProxies,
      },
      {
        id: 'deploy-proxy',
        title: 'Deploy\nApp LLM Proxy',
        icon: Rocket,
        path: deployProxyPath,
        completed: completion.hasProxyDeployments,
      },
      {
        id: 'consume-proxy',
        title: 'Consume\nApp LLM Proxy',
        icon: Package,
        path: consumeProxyPath,
        completed: completion.hasProxyConsumptions,
      },
    ],
    [completion, createProxyPath, deployProxyPath, consumeProxyPath]
  );

  const hasPrerequisites = completion.hasProviders && completion.hasGateways;
  const areAllStepsCompleted =
    completion.hasProxies &&
    completion.hasProxyDeployments &&
    completion.hasProxyConsumptions;

  useEffect(() => {
    if (!organizationId || isDismissed || isLoading || !areAllStepsCompleted) {
      return;
    }

    writeOrganizationFlag(PROXY_QUICK_START_DISMISS_KEY, organizationId);
    setIsDismissed(true);
  }, [organizationId, isDismissed, isLoading, areAllStepsCompleted]);

  const handleCloseBanner = () => {
    if (!organizationId) return;
    writeOrganizationFlag(PROXY_QUICK_START_DISMISS_KEY, organizationId);
    setIsDismissed(true);
  };

  const handleStepClick = (step: StepDefinition) => {
    if (step.id === 'consume-proxy' && organizationId) {
      writeOrganizationFlag(PROXY_QUICK_START_CONSUMED_KEY, organizationId);
      setCompletion((previous) => ({
        ...previous,
        hasProxyConsumptions: true,
      }));
    }
    navigate(step.path);
  };

  if (!isDeveloper || !organizationId || isDismissed) {
    return null;
  }

  return (
    <Card
      sx={{
        width: '100%',
        position: 'relative',
        overflow: 'hidden',
        borderColor: '#f97316',
        borderRadius: '12px',
      }}
    >
      <IconButton
        size="small"
        onClick={handleCloseBanner}
        aria-label="Close proxy quick start banner"
        sx={{
          position: 'absolute',
          top: 8,
          right: 8,
          zIndex: 1,
        }}
      >
        <X size={16} />
      </IconButton>

      <Grid
        container
        columns={12}
        sx={{ px: { xs: 1.5, md: 2 }, py: 0, width: '100%', m: 0 }}
      >
        <Grid size={{ xs: 12, xl: 10 }} sx={{ minWidth: 0 }}>
          <Box
            sx={{ minWidth: 0, py: { xs: 1.5, md: 1.75 }, pl: { md: 0.75 } }}
          >
            <Stack spacing={1.1}>
              <Box>
                <Chip
                  label="Quick Start"
                  size="small"
                  sx={{
                    height: 28,
                    px: 0.85,
                    fontWeight: 600,
                    borderRadius: '4px',
                    background:
                      'linear-gradient(90deg, #f15a24 0%, #ff8a00 50%, #f97316 100%)',
                    color: '#ffffff',
                  }}
                />
              </Box>

              <Box sx={{ pb: 1 }}>
                <Typography
                  sx={{
                    lineHeight: 1.2,
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    fontSize: 14,
                  }}
                >
                  <Box component="span" sx={{ fontWeight: 700 }}>
                    Welcome {welcomeName}!
                  </Box>{' '}
                  Let&apos;s get started with AI!
                </Typography>
                <Typography
                  sx={{
                    fontWeight: 700,
                    mb: 0.35,
                    lineHeight: 1.5,
                    fontSize: 24,
                  }}
                >
                  Expose your 1st App LLM Proxy
                </Typography>
              </Box>

              {!hasPrerequisites && !isLoading ? (
                <Alert
                  severity="warning"
                  sx={{ fontSize: 14, maxWidth: 'fit-content' }}
                >
                  You need one or more Gateways and LLM Providers please contact
                  Organization admin.
                </Alert>
              ) : (
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: { xs: 0.5, md: 0.9 },
                    overflowX: 'auto',
                    overflowY: 'hidden',
                    pb: 0.5,
                    pr: 0.5,
                    minWidth: 0,
                    scrollbarWidth: 'thin',
                  }}
                >
                  {isLoading
                    ? Array.from({ length: 3 }).map((_, index) => (
                        <React.Fragment key={`proxy-skeleton-step-${index}`}>
                          <Box
                            sx={{
                              border: '1px solid',
                              borderColor: '#d2d8e1',
                              bgcolor: '#eceff4',
                              display: 'flex',
                              alignItems: 'center',
                              gap: 1,
                              px: 1.35,
                              py: 0.75,
                              borderRadius: 999,
                              minWidth: {
                                xs: 162,
                                sm: 170,
                                md: 178,
                                lg: 168,
                                xl: 0,
                              },
                              maxWidth: {
                                xs: 190,
                                sm: 198,
                                md: 206,
                                lg: 196,
                                xl: 198,
                              },
                              flex: { xs: '0 0 auto', xl: '1 1 0%' },
                            }}
                          >
                            <Skeleton
                              variant="circular"
                              width={36}
                              height={36}
                            />
                            <Box sx={{ flex: 1 }}>
                              <Skeleton
                                variant="text"
                                width="85%"
                                height={18}
                              />
                              <Skeleton
                                variant="text"
                                width="62%"
                                height={18}
                              />
                            </Box>
                          </Box>
                          {index < 2 ? (
                            <Box
                              sx={{
                                color: '#9aa3b2',
                                display: 'flex',
                                alignItems: 'center',
                                flexShrink: 0,
                              }}
                            >
                              <ArrowRight size={16} />
                            </Box>
                          ) : null}
                        </React.Fragment>
                      ))
                    : steps.map((step, index) => {
                        const Icon = step.icon;
                        const nextStep = steps[index + 1];
                        const isConnectorLine =
                          Boolean(nextStep) &&
                          step.completed &&
                          nextStep.completed;

                        return (
                          <React.Fragment key={step.id}>
                            <Box
                              component="button"
                              type="button"
                              onClick={() => handleStepClick(step)}
                              sx={{
                                border: '1px solid',
                                borderColor: step.completed
                                  ? '#ff6b00'
                                  : '#d2d8e1',
                                bgcolor: '#ffffff',
                                color: 'text.primary',
                                display: 'flex',
                                alignItems: 'center',
                                gap: 1,
                                px: 1.35,
                                py: 0.75,
                                borderRadius: 999,
                                minWidth: {
                                  xs: 162,
                                  sm: 170,
                                  md: 178,
                                  lg: 168,
                                  xl: 0,
                                },
                                maxWidth: {
                                  xs: 190,
                                  sm: 198,
                                  md: 206,
                                  lg: 196,
                                  xl: 198,
                                },
                                flex: { xs: '0 0 auto', xl: '1 1 0%' },
                                cursor: 'pointer',
                                whiteSpace: 'nowrap',
                                '&:hover': {
                                  boxShadow: 2,
                                },
                              }}
                            >
                              <Box
                                sx={{
                                  width: 36,
                                  height: 36,
                                  borderRadius: '50%',
                                  display: 'flex',
                                  alignItems: 'center',
                                  justifyContent: 'center',
                                  border: '1px solid',
                                  borderColor: step.completed
                                    ? '#ff6b00'
                                    : '#d7dce5',
                                  bgcolor: step.completed
                                    ? '#ff6b00'
                                    : '#e2e5ed',
                                  color: step.completed ? '#ffffff' : '#4d5466',
                                  flexShrink: 0,
                                }}
                              >
                                <Icon size={16} />
                              </Box>

                              <Typography
                                sx={{
                                  fontWeight: 600,
                                  color: '#444a5b',
                                  textAlign: 'left',
                                  whiteSpace: 'pre-line',
                                  lineHeight: 1.3,
                                  flex: 1,
                                  fontSize: 13,
                                }}
                              >
                                {step.title}
                              </Typography>

                              {step.completed ? (
                                <Box
                                  sx={{
                                    color: '#20b974',
                                    display: 'flex',
                                    alignItems: 'center',
                                    flexShrink: 0,
                                  }}
                                >
                                  <CheckCircle2 size={20} />
                                </Box>
                              ) : null}
                            </Box>

                            {index < steps.length - 1 ? (
                              isConnectorLine ? (
                                <Box
                                  sx={{
                                    width: { xs: 16, md: 24, xl: 22 },
                                    height: 2,
                                    bgcolor: '#ff6b00',
                                    borderRadius: 1,
                                    flexShrink: 0,
                                  }}
                                />
                              ) : (
                                <Box
                                  sx={{
                                    color: '#9aa3b2',
                                    display: 'flex',
                                    alignItems: 'center',
                                    flexShrink: 0,
                                  }}
                                >
                                  <ArrowRight size={16} />
                                </Box>
                              )
                            ) : null}
                          </React.Fragment>
                        );
                      })}
                </Box>
              )}
            </Stack>
          </Box>
        </Grid>

        <Grid
          size={{ xs: 12, xl: 2 }}
          sx={{
            display: { xs: 'none', xl: 'flex' },
            alignItems: 'flex-end',
            justifyContent: 'flex-end',
            width: '100%',
            flexShrink: 0,
            overflow: 'hidden',
            pr: { xl: 0.25 },
          }}
        >
          <Box
            component="img"
            src={QuickstartbannerImg}
            alt="Quick Start"
            sx={{
              width: '94%',
              height: 'auto',
              maxHeight: 175,
              objectFit: 'contain',
              display: 'block',
              alignSelf: 'flex-end',
            }}
          />
        </Grid>
      </Grid>
    </Card>
  );
}
