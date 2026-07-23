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

import React, { useMemo, useState } from 'react';
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  Checkbox,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
  Typography,
  Skeleton,
} from '@wso2/oxygen-ui';
import { Clock, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import { useApplications } from '../../../../contexts/ApplicationsContext';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  formatRelativeTime,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import {
  resolveTemplateDisplayName,
  truncateProviderDisplayName,
} from '../../../../utils/providerTemplateDisplay';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import * as llmProviderApis from '../../../../apis/llmProviderApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';

import AnthropicLogo from '../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../assets/brands/openAI.png';
import NoFeatureAvilable from '../../../../assets/images/NoFeatureAvilable.svg';
import NoProviders from '../../../../assets/images/NoProviders.svg';
import { FormattedMessage } from 'react-intl';

const statusChipColor: Record<string, 'success' | 'warning' | 'default'> = {
  Active: 'success',
  Degraded: 'warning',
  Paused: 'default',
};

const PROVIDER_LOGO_MAP: Record<string, string> = {
  openai: OpenAILogo,
  anthropic: AnthropicLogo,
  'azure-openai': AzureLogo,
  'azureai-foundry': AzureLogo,
  'aws-bedrock': AWSBedrockLogo,
  awsbedrock: AWSBedrockLogo,
  'google-vertex': GoogleVertexLogo,
  gemini: GoogleGeminiLogo,
  mistralai: MistralAILogo,
  mistral: MistralAILogo,
};

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trim()}…`;
}

export default function ServiceProviders() {
  const navigate = useNavigate();
  const { hasPermission } = useAppAuth();
  const {
    providersResponse,
    deleteProvider,
    isLoading,
    error: providersError,
    refreshProviders,
  } = useLLMProviders();
  const { applications } = useApplications();
  const { templatesResponse } = useProviderTemplates();
  const { currentProject, currentOrganization, setCurrentProject } =
    useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const canDelete = !isProjectLevel && hasPermission(SCOPES.LLM_PROVIDER_DELETE);
  const newProviderPath = isProjectLevel
    ? buildProjectPath(
        currentOrganization,
        currentProject,
        '/service-provider/create'
      )
    : buildOrgPath(currentOrganization, '/service-provider/create');
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [deleteConfirmationInput, setDeleteConfirmationInput] = useState('');
  const [assignTarget, setAssignTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [checkingProviderId, setCheckingProviderId] = useState<string | null>(
    null
  );
  const [selectedApplicationIds, setSelectedApplicationIds] = useState<
    string[]
  >([]);
  const showSnackbar = useAIWorkspaceSnackbar();

  const hasError = Boolean(providersError);

  // Access the list from the API response
  const providers = providersResponse.list;
  const emptyMessage = 'No Available LLM Providers';
  const isProviderQuotaReached = false;
  const providerQuotaTooltip =
    'You cannot create more providers because your organization has reached the maximum limit of 5 LLM providers.';

  const filteredProviders = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return providers;

    return providers.filter((provider) => {
      const lastUpdated =
        provider.lastUpdated ?? provider.updatedAt ?? provider.createdAt;
      return (
        provider.displayName.toLowerCase().includes(query) ||
        formatRelativeTime(lastUpdated).toLowerCase().includes(query) ||
        (provider.status ?? '').toLowerCase().includes(query)
      );
    });
  }, [providers, searchQuery]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;

    try {
      await deleteProvider(deleteTarget.id);
      showSnackbar('Provider deleted successfully.', 'success');
      setDeleteTarget(null);
      setDeleteConfirmationInput('');
    } catch {
      showSnackbar('Failed to delete provider. Please try again.', 'error');
    }
  };

  const checkProviderUsageAndConfirmDelete = async (
    providerId: string,
    providerName: string
  ) => {
    if (!currentOrganization?.uuid) {
      showSnackbar(
        'Unable to verify App LLM Proxy usage because organization details are unavailable.',
        'error'
      );
      return;
    }

    setCheckingProviderId(providerId);
    try {
      const linkedProxiesResponse = await llmProviderApis.getLLMProviderProxies(
        providerId,
        currentOrganization.uuid,
        apimBaseUrl
      );
      const linkedProxyCount = linkedProxiesResponse.count ?? 0;
      if (linkedProxyCount > 0) {
        const proxyLabel = linkedProxyCount === 1 ? 'Proxy' : 'Proxies';
        const usageVerb = linkedProxyCount === 1 ? 'is' : 'are';
        showSnackbar(
          `Cannot delete "${providerName}" because ${linkedProxyCount} App LLM ${proxyLabel} ${usageVerb} using this provider. Remove or update those proxies first.`,
          'error'
        );
        return;
      }

      setDeleteTarget({ id: providerId, name: providerName });
      setDeleteConfirmationInput('');
    } catch {
      showSnackbar(
        'Failed to verify App LLM Proxy usage for this provider. Deletion has been blocked. Please try again.',
        'error'
      );
    } finally {
      setCheckingProviderId(null);
    }
  };

  const handleAssignConfirm = () => {
    if (!assignTarget) return;
    const count = selectedApplicationIds.length;
    const message =
      count === 1
        ? 'Provider added to 1 application.'
        : `Provider added to ${count} applications.`;
    showSnackbar(message, 'success');
    setAssignTarget(null);
    setSelectedApplicationIds([]);
  };

  const isDeleteConfirmationValid =
    deleteConfirmationInput.trim() === (deleteTarget?.name ?? '').trim();

  const isDeveloper = !hasPermission(SCOPES.LLM_PROVIDER_CREATE);
  const canCreateProvider =
    !isProjectLevel && hasPermission(SCOPES.LLM_PROVIDER_CREATE) && !isProviderQuotaReached;
  const shouldShowCreateProviderButton = !isProjectLevel;
  const orgServiceProviderPath = buildOrgPath(
    currentOrganization,
    '/service-provider'
  );

  if (isProjectLevel) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            minHeight: '60vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            px: 2,
          }}
        >
          <Stack spacing={2} alignItems="center" sx={{ textAlign: 'center' }}>
            <Box
              component="img"
              src={NoFeatureAvilable}
              alt="Organization-level feature"
              sx={{ width: 140, maxWidth: '100%' }}
            />
            <Box
              display="flex"
              flexDirection="column"
              alignItems="center"
              gap={0}
            >
              <Typography variant="h6">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.project.level.disabled.title"
                  defaultMessage={
                    'Provider management is available at the organization level.'
                  }
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.project.level.disabled.subtitle"
                  defaultMessage={
                    'Switch to organization level to view and manage LLM providers.'
                  }
                />
              </Typography>
            </Box>
            <Button
              variant="contained"
              onClick={() => {
                setCurrentProject(null);
                navigate(orgServiceProviderPath);
              }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.project.level.disabled.cta"
                defaultMessage={'Go to Organization level'}
              />
            </Button>
          </Stack>
        </Box>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Box
        sx={{
          display: 'flex',
          alignItems: { xs: 'flex-start', sm: 'center' },
          justifyContent: 'space-between',
          flexWrap: { xs: 'wrap', sm: 'nowrap' },
          gap: 2,
        }}
      >
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.llm.service.providers"
              defaultMessage={'LLM Providers'}
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.manage.and.monitor.your.connected.llm.service.providers"
              defaultMessage={
                'Manage and monitor your connected LLM Providers.'
              }
            />
          </PageTitle.SubHeader>
        </PageTitle>

        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={1.5}
          width={350}
          display="flex"
          justifyContent="flex-end"
        >
          {!isDeveloper && !isProjectLevel && providers.length > 0 && (
            <Tooltip title={isProviderQuotaReached ? providerQuotaTooltip : ''}>
              <Box component="span">
                <Button
                  variant="contained"
                  color="primary"
                  component={RouterLink}
                  to={newProviderPath}
                  startIcon={<Plus size={20} />}
                  disabled={isProviderQuotaReached}
                  data-cyid="add-new-provider-button"
                  sx={{
                    opacity: isProviderQuotaReached ? 0.55 : 1,
                    '&.Mui-disabled': {
                      opacity: isProviderQuotaReached ? 0.55 : 1,
                    },
                  }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.add.new.provider"
                    defaultMessage={'Add New Provider'}
                  />
                </Button>
              </Box>
            </Tooltip>
          )}
        </Stack>
      </Box>

      {/* Loading State */}
      {isLoading && (
        <Grid container spacing={2} sx={{ mt: 1 }}>
          {[0, 1, 2].map((key) => (
            <Grid key={key} size={{ xs: 12, md: 6, lg: 4 }}>
              <Card sx={{ height: '100%' }}>
                <Box sx={{ p: 2 }}>
                  <Stack spacing={1.5}>
                    <Box
                      sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}
                    >
                      <Skeleton variant="circular" width={44} height={44} />
                      <Box sx={{ flex: 1 }}>
                        <Skeleton variant="text" width="60%" height={24} />
                        <Skeleton variant="text" width="80%" height={18} />
                        <Skeleton variant="text" width="50%" height={16} />
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
                      <Skeleton variant="rounded" width={120} height={32} />
                    </Box>
                  </Stack>
                </Box>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}

      {/* Error State */}
      {hasError && !isLoading && (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={providersError} onRetry={refreshProviders} />
        </Box>
      )}

      {/* Providers Section */}
      {!isLoading && !hasError && providers.length === 0 && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            py: 6,
          }}
        >
          <Stack
            spacing={1.5}
            alignItems="center"
            justifyContent="center"
            sx={{ textAlign: 'center', py: 2, width: '100%' }}
          >
            <Box
              component="img"
              src={NoProviders}
              alt="No providers"
              sx={{ width: 140, maxWidth: '80%' }}
            />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.create.your.first.llm.provider"
                defaultMessage={'Create your first LLM Provider'}
              />
            </Typography>

            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 420 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.connect.and.manage.providers.description"
                defaultMessage={
                  'Set up an LLM provider to start connecting models and powering AI applications in your workspace.'
                }
              />
            </Typography>

            {shouldShowCreateProviderButton ? (
              <Tooltip
                title={
                  canCreateProvider ? (
                    ''
                  ) : isDeveloper ? (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.contact.admin.tooltip"
                      defaultMessage={
                        'This is an admin task. Please contact your admin.'
                      }
                    />
                  ) : (
                    providerQuotaTooltip
                  )
                }
                disableHoverListener={canCreateProvider}
                disableFocusListener={canCreateProvider}
                disableTouchListener={canCreateProvider}
              >
                <span>
                  <Button
                    variant="contained"
                    onClick={() => navigate(newProviderPath)}
                    startIcon={<Plus size={18} />}
                    disabled={!canCreateProvider}
                    data-cyid="add-new-provider-button"
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.create.provider"
                      defaultMessage={'Create Provider'}
                    />
                  </Button>
                </span>
              </Tooltip>
            ) : (
              <Typography variant="body2" color="text.secondary">
                {emptyMessage}
              </Typography>
            )}
          </Stack>
        </Box>
      )}

      {!isLoading && !hasError && providers.length > 0 && (
        <>
          <Box sx={{ mb: 3 }}>
            <TextField
              fullWidth
              placeholder="Search Service Providers..."
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              data-cyid="provider-search-input"
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={20} />
                    </InputAdornment>
                  ),
                },
              }}
            />
          </Box>

          <Grid container spacing={2} sx={{ mt: 1 }}>
            {filteredProviders.map((provider) => {
              const providerId = provider.id ?? provider.displayName;
              const providerStatus = provider.status ?? 'Unknown';
              const lastUpdated =
                provider.lastUpdated ??
                provider.updatedAt ??
                provider.createdAt;
              const logoUrl = PROVIDER_LOGO_MAP[providerId];
              const hasLogo = Boolean(logoUrl);
              const descriptionText = provider.description?.trim() || '';
              const templateDisplayName = resolveTemplateDisplayName(
                provider.template,
                templatesResponse.list
              );
              const providerDisplayName = truncateProviderDisplayName(
                provider.displayName
              );
              const templateKey = (provider.template ?? '').toLowerCase();
              const templateLogo = PROVIDER_LOGO_MAP[templateKey];
              const hasTemplateLogo = Boolean(templateLogo);

              return (
                <Grid key={providerId} size={{ xs: 12, md: 4, lg: 3 }}>
                  <Card
                    data-cyid={`provider-card-${providerId}`}
                    sx={{
                      height: '100%',
                      width: '100%',
                      cursor: 'pointer',
                      transition: 'box-shadow 0.2s ease',
                      '&.MuiCard-root:hover': { boxShadow: 3 },
                      '&:focus-visible': {
                        outline: '2px solid',
                        outlineColor: 'primary.main',
                        outlineOffset: '2px',
                      },
                    }}
                    tabIndex={0}
                    role="button"
                    onClick={() =>
                      navigate(
                        isProjectLevel
                          ? buildProjectPath(
                              currentOrganization,
                              currentProject,
                              `/service-provider/${providerId}`
                            )
                          : buildOrgPath(
                              currentOrganization,
                              `/service-provider/${providerId}`
                            )
                      )
                    }
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        navigate(
                          isProjectLevel
                            ? buildProjectPath(
                                currentOrganization,
                                currentProject,
                                `/service-provider/${providerId}`
                              )
                            : buildOrgPath(
                                currentOrganization,
                                `/service-provider/${providerId}`
                              )
                        );
                      }
                    }}
                  >
                    <Box
                      sx={{
                        height: '100%',
                        width: '100%',
                        display: 'flex',
                        flexDirection: 'column',
                        padding: 2,
                      }}
                    >
                      <Stack spacing={1.5} sx={{ flex: 1 }}>
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'flex-start',
                            gap: 1.5,
                          }}
                        >
                          <Avatar
                            src={hasLogo ? logoUrl : undefined}
                            sx={{
                              width: 44,
                              height: 44,
                              fontWeight: 600,
                              bgcolor: hasLogo
                                ? 'common.white'
                                : 'primary.light',
                              color: hasLogo
                                ? 'text.primary'
                                : 'primary.contrastText',
                              border: hasLogo ? '1px solid' : 'none',
                              borderColor: 'divider',
                              p: hasLogo ? 0.5 : 0,
                              '& img': {
                                objectFit: 'contain',
                              },
                            }}
                          >
                            {!hasLogo ? getInitials(provider.displayName) : null}
                          </Avatar>

                          <Box sx={{ flex: 1, minWidth: 0 }}>
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                gap: 1,
                              }}
                            >
                              <Typography variant="h5" sx={{ fontWeight: 600 }}>
                                {providerDisplayName}
                              </Typography>

                              {/* <Stack
                                direction="row"
                                spacing={1}
                                alignItems="center"
                              >
                                <Chip
                                  size="small"
                                  label={providerStatus}
                                  color={
                                    statusChipColor[providerStatus] ?? 'default'
                                  }
                                />
                              </Stack> */}
                            </Box>

                            <Typography
                              variant="body2"
                              color="text.secondary"
                              fontSize="0.75rem"
                              sx={{ mb: 0.25, mt: 0.8 }}
                            >
                              {truncateText(descriptionText, 70)}
                            </Typography>

                            {templateDisplayName && (
                              <Stack
                                direction="row"
                                spacing={0.5}
                                alignItems="center"
                                sx={{ mt: 0.3 }}
                              >
                                <Typography
                                  variant="body2"
                                  color="text.secondary"
                                  fontSize="0.7rem"
                                >
                                  <FormattedMessage
                                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.template"
                                    defaultMessage={'Template:'}
                                  />{' '}
                                  {templateDisplayName}
                                </Typography>

                                {hasTemplateLogo ? (
                                  <Avatar
                                    src={templateLogo}
                                    variant="circular"
                                    sx={{
                                      width: 16,
                                      height: 16,
                                      '& img': { objectFit: 'contain' },
                                    }}
                                  />
                                ) : null}
                              </Stack>
                            )}
                          </Box>
                        </Box>
                      </Stack>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 2,
                        }}
                        mt="auto"
                        pt={2}
                      >
                        <Stack
                          direction="row"
                          spacing={0.5}
                          alignItems="center"
                        >
                          <Clock size={14} />
                          <Typography variant="body2" color="text.secondary">
                            {formatRelativeTime(lastUpdated)}
                          </Typography>
                        </Stack>

                        <Stack
                          direction="row"
                          spacing={1}
                          alignItems="center"
                          sx={{ ml: 'auto' }}
                        >
                          {/* <Button
                            size="small"
                            startIcon={<Plus size={16} />}
                            onClick={(event) => {
                              event.stopPropagation();
                              setAssignTarget({
                                id: providerId,
                                name: provider.displayName,
                              });
                            }}
                          >
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.add.genai.app"
                              defaultMessage={'Add GenAI app'}
                            />
                          </Button> */}
                          {canDelete ? (
                            <IconButton
                              size="small"
                              color="error"
                              disabled={checkingProviderId === providerId}
                              onClick={(event) => {
                                event.stopPropagation();
                                void checkProviderUsageAndConfirmDelete(
                                  providerId,
                                  provider.displayName
                                );
                              }}
                              aria-label={`Delete ${providerDisplayName}`}
                              data-cyid="delete-provider-button"
                            >
                              <Trash2 size={16} />
                            </IconButton>
                          ) : null}
                        </Stack>
                      </Box>
                    </Box>
                  </Card>
                </Grid>
              );
            })}
          </Grid>
        </>
      )}

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => {
          setDeleteTarget(null);
          setDeleteConfirmationInput('');
        }}
      >
        <DialogTitle>
          Are you sure you want to remove the LLM Provider{' '}
          <strong>'{deleteTarget?.name ?? ''}'</strong>?
        </DialogTitle>
        <DialogContent>
          <Typography sx={{ mt: 1 }} variant="body2" color="text.secondary">
            This action will be irreversible and all related details will be
            lost. Please type in the component name below to confirm.
          </Typography>
          <TextField
            fullWidth
            size="small"
            sx={{ mt: 2 }}
            value={deleteConfirmationInput}
            onChange={(event) => setDeleteConfirmationInput(event.target.value)}
            placeholder={deleteTarget?.name ?? 'Enter provider name'}
            data-cyid="delete-provider-confirm-input"
          />
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => {
              setDeleteTarget(null);
              setDeleteConfirmationInput('');
            }}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.cancel"
              defaultMessage={'Cancel'}
            />
          </Button>
          <Button
            color="error"
            onClick={handleDeleteConfirm}
            disabled={!isDeleteConfirmationValid}
            data-cyid="delete-provider-confirm-button"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.delete"
              defaultMessage={'Delete'}
            />
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(assignTarget)}
        onClose={() => {
          setAssignTarget(null);
          setSelectedApplicationIds([]);
        }}
      >
        <DialogTitle>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.add.provider.to.applications"
            defaultMessage={'Add provider to applications'}
          />
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.select.one.or.more.applications.for"
              defaultMessage={'Select one or more applications for'}
            />{' '}
            {assignTarget?.name}
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.message.2"
              defaultMessage={'.'}
            />
          </DialogContentText>
          <Stack spacing={0.5} sx={{ mt: 2 }}>
            {applications.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.no.applications.available"
                  defaultMessage={'No applications available.'}
                />
              </Typography>
            ) : (
              applications.map((application) => (
                <FormControlLabel
                  key={application.id}
                  control={
                    <Checkbox
                      checked={selectedApplicationIds.includes(application.id)}
                      onChange={(event) => {
                        const nextChecked = event.target.checked;
                        setSelectedApplicationIds((prev) =>
                          nextChecked
                            ? [...prev, application.id]
                            : prev.filter((id) => id !== application.id)
                        );
                      }}
                    />
                  }
                  label={`${application.displayName} • ${application.owner}`}
                />
              ))
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => {
              setAssignTarget(null);
              setSelectedApplicationIds([]);
            }}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.cancel"
              defaultMessage={'Cancel'}
            />
          </Button>
          <Button
            variant="contained"
            onClick={handleAssignConfirm}
            disabled={selectedApplicationIds.length === 0}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.add.to.app"
              defaultMessage={'Add to app'}
            />
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
