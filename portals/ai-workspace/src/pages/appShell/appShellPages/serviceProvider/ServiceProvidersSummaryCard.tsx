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

import React from 'react';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  Divider,
  Form,
  ParticleBackground,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Plus } from '@wso2/oxygen-ui-icons-react';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import NoProviders from '../../../../assets/images/NoProviders.svg';
import AnthropicLogo from '../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../assets/brands/openAI.png';
import { FormattedMessage } from 'react-intl';
import {
  getProviderTemplateDisplayName,
  truncateProviderDisplayName,
} from '../../../../utils/providerTemplateDisplay';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

export type ServiceProviderSummaryItem = {
  id: string;
  name: string;
  status?: string;
  lastUpdated?: string;
  modelCount?: number;
  description?: string;
  template?: string;
};

type ServiceProvidersSummaryCardProps = {
  providers: ServiceProviderSummaryItem[];
  totalCount?: number;
  onSeeMore?: () => void;
  onAddProvider?: () => void;
  onCreateProvider?: () => void;
  onProviderClick?: (providerId: string) => void;
  maxItems?: number;
  title?: string;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
  emptyMessage?: string;
  showSeeMore?: boolean;
};

export default function ServiceProvidersSummaryCard({
  providers,
  totalCount,
  onSeeMore,
  onAddProvider,
  onCreateProvider,
  onProviderClick,
  maxItems = 5,
  title = 'LLM Providers',
  isLoading = false,
  error,
  onRetry,
  emptyMessage = 'No Available LLM Providers',
  showSeeMore = true,
}: ServiceProvidersSummaryCardProps) {
  const { hasPermission } = useAppAuth();
  const displayCount = totalCount ?? providers.length;
  const visibleProviders = providers.slice(0, maxItems);
  const canClick = Boolean(onProviderClick);
  const hasProviders = visibleProviders.length > 0;
  const isEmptyState = !isLoading && !error && !hasProviders;
  const canCreateProvider = hasPermission(SCOPES.LLM_PROVIDER_CREATE) && Boolean(onCreateProvider);
  const shouldShowCreateProviderButton =
    Boolean(onCreateProvider) || !hasPermission(SCOPES.LLM_PROVIDER_CREATE);

  const templateLogoMap: Record<string, string> = {
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

  const truncateText = (text: string, maxLength: number) => {
    if (text.length <= maxLength) return text;
    return `${text.slice(0, maxLength).trim()}…`;
  };

  return (
    <Card
      sx={{
        height: '100%',
        width: '100%',
        minHeight: 300,
        ...(isEmptyState
          ? {
              display: 'flex',
              position: 'relative',
              overflow: 'hidden',
            }
          : {}),
      }}
    >
      {isEmptyState ? <ParticleBackground opacity={0.6} /> : null}
      {!isEmptyState ? (
        <CardHeader
          title={title}
          subheader={`Total: ${displayCount}`}
          slotProps={{
            title: {
              sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 },
            },
            subheader: { sx: { fontSize: '0.82rem' } },
          }}
          action={
            hasProviders ? (
              <Stack direction="row" spacing={1} alignItems="center">
                {hasPermission(SCOPES.LLM_PROVIDER_CREATE) && onAddProvider ? (
                  <Button size="small" onClick={onAddProvider} variant="text">
                    + Add New
                  </Button>
                ) : null}
                {onSeeMore && showSeeMore ? (
                  <Button size="small" onClick={onSeeMore}>
                    See more
                  </Button>
                ) : null}
              </Stack>
            ) : null
          }
        />
      ) : null}

      <CardContent
        sx={{
          position: 'relative',
          paddingTop: 0,
          zIndex: 1,
          ...(isEmptyState
            ? {
                flex: 1,
                minHeight: 300,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }
            : {}),
        }}
      >
        {isLoading ? (
          <Stack divider={<Divider />} spacing={1.5}>
            {[0, 1, 2].map((key) => (
              <Box
                key={key}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  width: '100%',
                  gap: 1.5,
                  borderRadius: 1,
                  px: 0.5,
                  py: 0.5,
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                  }}
                >
                  <Skeleton variant="circular" width={36} height={36} />
                  <Box sx={{ minWidth: 0 }}>
                    <Skeleton variant="text" width={140} height={20} />
                    <Skeleton variant="text" width={220} height={16} />
                  </Box>
                </Box>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Skeleton variant="rounded" width={70} height={22} />
                  <Skeleton variant="rounded" width={70} height={22} />
                </Stack>
              </Box>
            ))}
          </Stack>
        ) : error ? (
          <Box sx={{ py: 2 }}>
            {onRetry ? (
              <ErrorAlert error={error} onRetry={onRetry} />
            ) : (
              <ErrorAlert error={error} onRetry={() => window.location.reload()} />
            )}
          </Box>
        ) : !hasProviders ? (
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
                  ) : (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.contact.admin.tooltip"
                      defaultMessage={
                        'This is an admin task. Please contact your admin.'
                      }
                    />
                  )
                }
                disableHoverListener={canCreateProvider}
                disableFocusListener={canCreateProvider}
                disableTouchListener={canCreateProvider}
              >
                <span>
                  <Button
                    variant="contained"
                    onClick={() => onCreateProvider?.()}
                    startIcon={<Plus size={18} />}
                    disabled={!canCreateProvider}
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
        ) : (
          <Stack divider={<Divider />} spacing={1}>
            {visibleProviders.map((provider) => {
              const providerId = provider.id ?? provider.name;
              const lastUpdated = provider.lastUpdated;
              const templateKey = (provider.template ?? '').toLowerCase();
              const templateDisplayName = getProviderTemplateDisplayName(
                provider.template
              );
              const providerDisplayName = truncateProviderDisplayName(
                provider.name
              );
              const templateLogo = templateLogoMap[templateKey];
              const hasTemplateLogo = Boolean(templateLogo);
              const descriptionText =
                provider.description?.trim() ?? 'No description';

              return (
                <Form.CardButton
                  key={providerId}
                  onClick={() => onProviderClick?.(providerId)}
                  disabled={!canClick}
                  sx={{
                    width: '100%',
                    p: 1,
                    transition:
                      'background-color 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease',
                    '& .MuiCardHeader-root': {
                      width: '100%',
                      p: 0,
                      alignItems: 'center',
                    },
                    '& .MuiCardHeader-content': {
                      minWidth: 0,
                    },
                    '& .MuiCardHeader-action': {
                      m: 0,
                      pl: 1,
                      alignSelf: 'center',
                    },
                    ...(canClick
                      ? {
                          '&.MuiCard-root:hover': {
                            bgcolor: 'action.hover',
                            boxShadow: 1,
                            borderColor: 'action.selected',
                          },
                        }
                      : null),
                  }}
                >
                  <Form.CardHeader
                    disableTypography
                    avatar={
                      <Avatar
                        sx={{
                          width: 36,
                          height: 36,
                          fontSize: 16,
                          bgcolor: 'primary.light',
                          color: 'primary.contrastText',
                          border: 'none',
                          p: 0,
                          '& img': { objectFit: 'contain' },
                        }}
                      >
                        {getInitials(provider.name)}
                      </Avatar>
                    }
                    title={
                      <Typography
                        variant="body1"
                        sx={{ fontWeight: 600 }}
                        noWrap
                      >
                        {providerDisplayName}
                      </Typography>
                    }
                    subheader={
                      <Box sx={{ minWidth: 0, mt: 0.25 }}>
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          fontSize="0.75rem"
                          sx={{ mb: 0.25 }}
                        >
                          {truncateText(descriptionText, 70)}
                        </Typography>

                        {templateDisplayName && (
                          <Stack
                            direction="row"
                            spacing={0.5}
                            alignItems="center"
                          >
                            <Typography
                              variant="body2"
                              color="text.secondary"
                              fontSize="0.7rem"
                            >
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProvidersSummaryCard.template"
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
                    }
                    action={
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 0.5,
                          pr: 0.5,
                          whiteSpace: 'nowrap',
                          flexShrink: 0,
                        }}
                      >
                        <Clock size={14} />
                        <Typography variant="body2" fontSize="0.75rem">
                          {formatRelativeTime(lastUpdated)}
                        </Typography>
                      </Box>
                    }
                  />
                </Form.CardButton>
              );
            })}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}
