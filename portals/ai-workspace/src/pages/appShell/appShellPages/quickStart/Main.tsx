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

import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Form,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  PageContent,
  Select,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight } from '@wso2/oxygen-ui-icons-react';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useLLMProviders } from '../../../../contexts/llmProvider';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import type { ProjectBase } from '../../../../utils/types';
import { getGateways } from '../../../../apis/gatewayApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import { getManageGatewaysOption } from './manageGatewaysOption';
import { getMCPProxyOption } from './mcpProxyOption';
import { getProviderProxyOption } from './providerProxyOption';
import type { QuickStartOption, QuickStartOptionId } from './types';

const MAX_GATEWAYS_PER_ORG = 3;

type GatewayCounts = {
  ai: number;
  api: number;
};

function getGatewayCounts(gateways: Gateway[] = []): GatewayCounts {
  return gateways.reduce(
    (counts, gateway) => {
      if (gateway.functionalityType === 'ai') {
        counts.ai += 1;
      }

      if (gateway.functionalityType === 'regular') {
        counts.api += 1;
      }

      return counts;
    },
    { ai: 0, api: 0 }
  );
}

function getGatewayQuotaTooltip({ ai, api }: GatewayCounts): string {
  return `You cannot continue because your organization already has ${ai} AI gateway${
    ai === 1 ? '' : 's'
  } and ${api} API gateway${
    api === 1 ? '' : 's'
  }. The maximum limit is 3 gateways in total.`;
}

type QuickStartOptionCardProps = {
  option: QuickStartOption;
  selected: boolean;
  onSelect: (optionId: QuickStartOptionId) => void;
};

function QuickStartOptionCard({
  option,
  selected,
  onSelect,
}: QuickStartOptionCardProps) {
  return (
    <Form.CardButton
      selected={selected}
      onClick={() => onSelect(option.id)}
      sx={{
        px: 2.5,
        py: 2.25,
        textAlign: 'left',
        width: '100%',
        position: 'relative',
        transition:
          'border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease',
        '&:hover': {
          borderColor: selected ? 'primary.main' : 'text.disabled',
          backgroundColor: selected ? 'action.hover' : 'action.selected',
        },
        '&:focus-visible': {
          outline: '2px solid',
          outlineColor: 'primary.main',
          outlineOffset: '2px',
        },
      }}
    >
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: option.badge
            ? '44px minmax(0, 1fr) auto'
            : '44px minmax(0, 1fr)',
          columnGap: 2,
          alignItems: 'center',
          width: '100%',
        }}
      >
        <Box
          component="img"
          src={option.imageSrc}
          alt={option.label}
          sx={{
            width: 44,
            height: 44,
            flexShrink: 0,
            objectFit: 'contain',
            display: 'block',
          }}
        />
        <Box sx={{ minWidth: 0 }}>
          <Typography
            variant="h4"
            sx={{ fontSize: '1rem', fontWeight: 600, mb: 0.5 }}
          >
            {option.label}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {option.description}
          </Typography>
        </Box>
        {option.badge ? (
          <Chip
            label={option.badge}
            size="small"
            color="primary"
            sx={{
              borderRadius: 0.4,
              fontWeight: 600,
              justifySelf: 'end',
              alignSelf: 'center',
              flexShrink: 0,
            }}
          />
        ) : null}
      </Box>
    </Form.CardButton>
  );
}

export default function QuickStart(): JSX.Element {
  const navigate = useNavigate();
  const {
    currentOrganization,
    currentProject,
    projectsForCurrentOrganization,
    setCurrentProject,
    userName,
    userEmail,
  } = useAppShell();
  const { hasPermission } = useAppAuth();
  const { providersResponse } = useLLMProviders();
  const [selectedOptionId, setSelectedOptionId] = useState<QuickStartOptionId>(
    hasPermission(SCOPES.LLM_PROVIDER_CREATE) ? 'provider-proxy' : 'mcp-proxy'
  );
  const [isProjectSelectionOpen, setIsProjectSelectionOpen] = useState(false);
  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [gatewayCounts, setGatewayCounts] = useState<GatewayCounts>({
    ai: 0,
    api: 0,
  });

  const welcomeName =
    userName?.trim() ||
    userEmail?.trim() ||
    currentOrganization?.name ||
    'there';

  const orgHomePath = buildOrgPath(currentOrganization, '/home');
  const projectHomePath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/home')
    : orgHomePath;
  const mcpNewPath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/mcp-proxy/new')
    : buildOrgPath(currentOrganization, '/mcp-proxy/new');

  const allQuickStartOptions: QuickStartOption[] = [
    getProviderProxyOption(() =>
      buildOrgPath(currentOrganization, '/service-provider/new')
    ),
    getManageGatewaysOption(() =>
      buildOrgPath(currentOrganization, '/gateways/new')
    ),
    getMCPProxyOption(() => mcpNewPath),
  ];

  const quickStartOptions = allQuickStartOptions.filter((option) => {
    if (option.id === 'manage-gateways') return hasPermission(SCOPES.GATEWAY_MANAGE);
    if (option.id === 'provider-proxy') return hasPermission(SCOPES.LLM_PROVIDER_CREATE);
    return true;
  });

  const selectedOption =
    quickStartOptions.find((option) => option.id === selectedOptionId) ||
    quickStartOptions[0];
  const selectedProject =
    projectsForCurrentOrganization.find(
      (project) => project.id === selectedProjectId
    ) ?? null;
  const providers = providersResponse.list;
  const providerCount =
    providersResponse.count ??
    providersResponse.pagination?.total ??
    providers.length;
  const isProviderQuotaReached = false;
  const isGatewayQuotaReached =
    gatewayCounts.ai + gatewayCounts.api >= MAX_GATEWAYS_PER_ORG;
  const nextButtonTooltip =
    selectedOptionId === 'provider-proxy' && isProviderQuotaReached
      ? 'You cannot continue because your organization has reached the maximum limit of 5 LLM providers.'
      : selectedOptionId === 'manage-gateways' && isGatewayQuotaReached
      ? getGatewayQuotaTooltip(gatewayCounts)
      : '';
  const isNextDisabled =
    (selectedOptionId === 'provider-proxy' && isProviderQuotaReached) ||
    (selectedOptionId === 'manage-gateways' && isGatewayQuotaReached);

  useEffect(() => {
    if (selectedOptionId !== 'manage-gateways' || !currentOrganization?.uuid) {
      return;
    }

    let isCancelled = false;

    const fetchGatewayCounts = async () => {
      try {
        const gatewaysResponse = await getGateways(currentOrganization.uuid);
        const nextGatewayCounts = getGatewayCounts(gatewaysResponse.list ?? []);

        if (!isCancelled) {
          setGatewayCounts(nextGatewayCounts);
        }
      } catch {
        if (!isCancelled) {
          setGatewayCounts({ ai: 0, api: 0 });
        }
      }
    };

    void fetchGatewayCounts();

    return () => {
      isCancelled = true;
    };
  }, [currentOrganization?.uuid, selectedOptionId]);

  const handleOpenProjectSelection = () => {
    setSelectedProjectId((currentSelectedProjectId) => {
      if (currentSelectedProjectId) return currentSelectedProjectId;
      return projectsForCurrentOrganization[0]?.id ?? '';
    });
    setIsProjectSelectionOpen(true);
  };

  const handleCloseProjectSelection = () => {
    setIsProjectSelectionOpen(false);
  };

  const handleGoToMCPProject = () => {
    if (!selectedProject) return;

    setCurrentProject(selectedProject as ProjectBase);
    navigate(
      buildProjectPath(currentOrganization, selectedProject, '/mcp-proxy/new')
    );
    setIsProjectSelectionOpen(false);
  };

  const handleNext = async () => {
    if (selectedOption.id === 'provider-proxy' && isProviderQuotaReached) {
      return;
    }

    if (selectedOption.id === 'manage-gateways' && currentOrganization?.uuid) {
      try {
        const gatewaysResponse = await getGateways(currentOrganization.uuid);
        const nextGatewayCounts = getGatewayCounts(gatewaysResponse.list ?? []);

        setGatewayCounts(nextGatewayCounts);

        if (
          nextGatewayCounts.ai + nextGatewayCounts.api >=
          MAX_GATEWAYS_PER_ORG
        ) {
          return;
        }
      } catch {
        // Allow navigation when the gateway count cannot be verified.
      }
    }

    if (selectedOption.id === 'mcp-proxy' && !currentProject) {
      handleOpenProjectSelection();
      return;
    }

    if (selectedOption.navigationScope === 'organization') {
      setCurrentProject(null);
    }

    navigate(selectedOption.getNextPath());
  };

  return (
    <PageContent fullWidth>
      <Box
        sx={{
          width: '100%',
          // maxWidth: 1300,4
          mx: 'auto',
        }}
      >
        <Card
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            backgroundColor: 'background.paper',
            px: { xs: 2.5, md: 4 },
            py: { xs: 3, md: 4 },
            boxShadow: '0 6px 18px rgba(15, 23, 42, 0.04)',
          }}
        >
          <Stack spacing={0.5} sx={{ mb: 4 }}>
            <Typography variant="body1" color="text.secondary">
              Hi{' '}
              <Box
                component="span"
                sx={{
                  fontSize: '1.125rem',
                  fontWeight: 700,
                  color: 'text.primary',
                }}
              >
                {welcomeName}
              </Box>
              , Welcome to AI Workspace
            </Typography>
            <Typography
              variant="h3"
              sx={{
                fontWeight: 700,
                lineHeight: 1.15,
                letterSpacing: '-0.02em',
              }}
            >
              What would you like to set up first?
            </Typography>
          </Stack>

          <Grid container spacing={4}>
            <Grid size={{ xs: 12, lg: 6 }}>
              <Stack spacing={2}>
                {quickStartOptions.map((option) => (
                  <QuickStartOptionCard
                    key={option.id}
                    option={option}
                    selected={selectedOption.id === option.id}
                    onSelect={setSelectedOptionId}
                  />
                ))}
              </Stack>
            </Grid>

            <Grid size={{ xs: 12, lg: 6 }}>
              <Stack spacing={3}>
                <Box
                  component="img"
                  src={selectedOption.previewImageSrc}
                  alt={selectedOption.label}
                  sx={{
                    width: '100%',
                    maxWidth: 480,
                    height: 'auto',
                    display: 'block',
                    mx: 'auto',
                    filter: 'drop-shadow(0 8px 16px rgba(15, 23, 42, 0.08))',
                  }}
                />

                <Box>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ mb: 2, fontWeight: 600 }}
                  >
                    What you&apos;ll do:
                  </Typography>

                  <Stack spacing={0}>
                    {selectedOption.steps.map((step, index) => (
                      <Stack
                        key={step.title}
                        direction="row"
                        spacing={2}
                        alignItems="stretch"
                      >
                        <Stack
                          alignItems="center"
                          sx={{ width: 18, flexShrink: 0 }}
                        >
                          <Box
                            sx={{
                              width: 12,
                              height: 12,
                              borderRadius: '50%',
                              backgroundColor: selectedOption.accentColor,
                              mt: 0.75,
                            }}
                          />
                          {index < selectedOption.steps.length - 1 ? (
                            <Box
                              sx={{
                                width: 2,
                                flex: 1,
                                minHeight: 34,
                                backgroundColor: 'divider',
                                mt: 0.5,
                              }}
                            />
                          ) : null}
                        </Stack>

                        <Box
                          sx={{
                            pb: index < selectedOption.steps.length - 1 ? 2 : 0,
                          }}
                        >
                          <Typography
                            variant="body1"
                            sx={{ fontWeight: 600, mb: 0.25 }}
                          >
                            {step.title}
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            {step.description}
                          </Typography>
                        </Box>
                      </Stack>
                    ))}
                  </Stack>
                </Box>
              </Stack>
            </Grid>
          </Grid>

          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={2}
            alignItems={{ xs: 'stretch', sm: 'center' }}
            justifyContent="space-between"
            sx={{
              mt: 4,
              pt: 3,
              borderTop: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Button
              variant="text"
              onClick={() => navigate(projectHomePath)}
              sx={{ alignSelf: { xs: 'stretch', sm: 'auto' } }}
            >
              Skip and go to overview
            </Button>

            <Tooltip
              title={nextButtonTooltip}
              disableHoverListener={!nextButtonTooltip}
            >
              <Box
                component="span"
                sx={{ alignSelf: { xs: 'stretch', sm: 'auto' } }}
              >
                <Button
                  variant="contained"
                  endIcon={<ArrowRight size={16} />}
                  onClick={handleNext}
                  disabled={isNextDisabled}
                  sx={{ alignSelf: { xs: 'stretch', sm: 'auto' } }}
                >
                  Next
                </Button>
              </Box>
            </Tooltip>
          </Stack>
        </Card>
      </Box>

      <Dialog
        open={isProjectSelectionOpen}
        onClose={handleCloseProjectSelection}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Select Project</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            <Typography variant="body2" color="text.secondary">
              Select a project to create the MCP Proxy in project scope.
            </Typography>
            <FormControl fullWidth>
              <FormLabel>Project</FormLabel>
              <Select
                value={selectedProjectId}
                onChange={(event) =>
                  setSelectedProjectId(event.target.value as string)
                }
                displayEmpty
                disabled={projectsForCurrentOrganization.length === 0}
              >
                {projectsForCurrentOrganization.length === 0 ? (
                  <MenuItem value="" disabled>
                    No projects available
                  </MenuItem>
                ) : (
                  projectsForCurrentOrganization.map((project) => (
                    <MenuItem key={project.id} value={project.id}>
                      {project.displayName}
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleCloseProjectSelection}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleGoToMCPProject}
            disabled={!selectedProject}
          >
            Next
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
