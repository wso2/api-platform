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
import { useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  Divider,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, ChevronRight, Clock, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  isBuiltInProviderTemplate,
  truncateProviderDisplayName,
} from '../../../../utils/providerTemplateDisplay';
import type { ProviderTemplate } from '../../../../utils/types';
import AnthropicLogo from '../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../assets/brands/openAI.png';

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

function resolveTemplateLogo(template: ProviderTemplate): string | undefined {
  const fromMeta = template.metadata?.logoUrl?.trim();
  if (fromMeta) return fromMeta;
  const id = (template.id ?? '').toLowerCase();
  return PROVIDER_LOGO_MAP[id];
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0 || words[0] === '') return '??';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

export default function ProviderTemplatesList({
  embedded = false,
}: {
  embedded?: boolean;
} = {}) {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();

  const {
    templatesResponse,
    isLoading,
    error,
    deleteTemplate,
    refreshTemplates,
  } = useProviderTemplates();

  const showSnackbar = useAIWorkspaceSnackbar();
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const templates = useMemo(
    () =>
      templatesResponse.list.filter(
        (template) => !isBuiltInProviderTemplate(template.id)
      ),
    [templatesResponse.list]
  );

  const templatesBase = buildOrgPath(
    currentOrganization,
    '/settings/llm-provider-templates'
  );
  const newTemplatePath = `${templatesBase}/new`;
  const filteredTemplates = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return templates;
    return templates.filter(
      (template) =>
        template.name.toLowerCase().includes(query) ||
        (template.description ?? '').toLowerCase().includes(query)
    );
  }, [templates, searchQuery]);

  // Paginate the custom templates ~2 rows at a time. Clamp the page so it stays
  // valid when the filtered list shrinks (e.g. while searching).
  const CUSTOM_PAGE_SIZE = 6;
  const [customPage, setCustomPage] = useState(1);
  const customTotalPages = Math.max(
    1,
    Math.ceil(filteredTemplates.length / CUSTOM_PAGE_SIZE)
  );
  const currentCustomPage = Math.min(customPage, customTotalPages);
  const pagedCustomTemplates = filteredTemplates.slice(
    (currentCustomPage - 1) * CUSTOM_PAGE_SIZE,
    currentCustomPage * CUSTOM_PAGE_SIZE
  );

  const builtInTemplates = useMemo(
    () =>
      templatesResponse.list.filter((template) =>
        isBuiltInProviderTemplate(template.id)
      ),
    [templatesResponse.list]
  );
  const filteredBuiltIn = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return builtInTemplates;
    return builtInTemplates.filter(
      (template) =>
        template.name.toLowerCase().includes(query) ||
        (template.description ?? '').toLowerCase().includes(query)
    );
  }, [builtInTemplates, searchQuery]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await deleteTemplate(deleteTarget.id);
      showSnackbar('Template deleted successfully.', 'success');
      setDeleteTarget(null);
    } catch {
      showSnackbar('Failed to delete template. Please try again.', 'error');
    }
  };

  const renderCard = (template: ProviderTemplate, deletable: boolean) => {
    const templateId = template.id ?? template.name;
    const overviewPath = `${templatesBase}/${templateId}`;
    const logoSrc = resolveTemplateLogo(template);
    const hasLogo = Boolean(logoSrc);
    return (
      <Card
        key={templateId}
        data-cyid={`provider-template-card-${templateId}`}
        tabIndex={0}
        role="button"
        onClick={() => navigate(overviewPath)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            navigate(overviewPath);
          }
        }}
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
      >
        <Box
          sx={{
            height: '100%',
            width: '100%',
            display: 'flex',
            flexDirection: 'column',
            p: 2,
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
            <Avatar
              src={hasLogo ? logoSrc : undefined}
              sx={{
                width: 44,
                height: 44,
                fontWeight: 600,
                bgcolor: hasLogo ? 'common.white' : 'primary.light',
                color: hasLogo ? 'text.primary' : 'primary.contrastText',
                border: hasLogo ? '1px solid' : 'none',
                borderColor: 'divider',
                p: hasLogo ? 0.5 : 0,
                '& img': { objectFit: 'contain' },
              }}
            >
              {!hasLogo ? getInitials(template.name) : null}
            </Avatar>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography
                variant="h5"
                noWrap
                sx={{ fontWeight: 600 }}
                title={template.name}
              >
                {truncateProviderDisplayName(template.name)}
              </Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                fontSize="0.75rem"
                noWrap
                sx={{ mt: 0.5 }}
                title={template.description?.trim() || undefined}
              >
                {template.description?.trim()
                  ? truncateProviderDisplayName(template.description, 70)
                  : 'No description'}
              </Typography>
            </Box>
          </Box>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 2,
              mt: 'auto',
              pt: 2,
            }}
          >
            <Stack direction="row" spacing={0.5} alignItems="center">
              <Clock size={14} />
              <Typography variant="body2" color="text.secondary">
                {formatRelativeTime(template.createdAt ?? template.updatedAt)}
              </Typography>
            </Stack>

            {deletable && (
              <IconButton
                size="small"
                color="error"
                sx={{ ml: 'auto' }}
                onClick={(event) => {
                  event.stopPropagation();
                  setDeleteTarget({ id: templateId, name: template.name });
                }}
                aria-label={`Delete ${template.name}`}
                data-cyid="delete-provider-template-button"
              >
                <Trash2 size={16} />
              </IconButton>
            )}
          </Box>
        </Box>
      </Card>
    );
  };

  const cardGridSx = {
    display: 'grid',
    gap: 2,
    gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
  } as const;

  const Wrapper: React.ElementType = embedded ? React.Fragment : PageContent;
  return (
    <Wrapper {...(embedded ? {} : { fullWidth: true })}>
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
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.title"
              defaultMessage={'LLM Provider Templates'}
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.subtitle"
              defaultMessage={
                'Define reusable templates for connecting LLM providers.'
              }
            />
          </PageTitle.SubHeader>
        </PageTitle>

        {templates.length > 0 && (
          <Button
            variant="contained"
            color="primary"
            onClick={() => navigate(newTemplatePath)}
            startIcon={<Plus size={20} />}
            data-cyid="add-provider-template-button"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.create"
              defaultMessage={'Create'}
            />
          </Button>
        )}
      </Box>

      {isLoading && (
        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
            mt: 1,
          }}
        >
          {[0, 1, 2].map((key) => (
            <Skeleton key={key} variant="rounded" height={120} />
          ))}
        </Box>
      )}

      {error && !isLoading && (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={error} onRetry={refreshTemplates} />
        </Box>
      )}

      {!isLoading &&
        !error &&
        templates.length === 0 &&
        builtInTemplates.length === 0 && (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              py: 6,
            }}
          >
            <Stack spacing={1.5} alignItems="center" sx={{ textAlign: 'center' }}>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.empty.title"
                  defaultMessage={'Create your first LLM Provider Template'}
                />
              </Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ maxWidth: 420 }}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.empty.subtitle"
                  defaultMessage={
                    'A template stores the endpoint and auth details so you can connect a custom LLM provider quickly.'
                  }
                />
              </Typography>
              <Button
                variant="contained"
                onClick={() => navigate(newTemplatePath)}
                startIcon={<Plus size={18} />}
                data-cyid="add-provider-template-button"
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.create"
                  defaultMessage={'Create'}
                />
              </Button>
            </Stack>
          </Box>
        )}

      {!isLoading &&
        !error &&
        (templates.length > 0 || builtInTemplates.length > 0) && (
          <>
            <Box sx={{ my: 3 }}>
              <TextField
                fullWidth
                placeholder="Search templates..."
                value={searchQuery}
                onChange={(event) => {
                  setSearchQuery(event.target.value);
                  setCustomPage(1);
                }}
                data-cyid="provider-template-search-input"
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

            <Typography variant="h6" sx={{ fontWeight: 700, mb: 0.5 }}>
              Custom LLM Providers
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Templates you create to connect your own LLM providers.
            </Typography>

            {filteredTemplates.length > 0 ? (
              <>
                <Box sx={cardGridSx}>
                  {pagedCustomTemplates.map((template) =>
                    renderCard(template, true)
                  )}
                </Box>
                {customTotalPages > 1 && (
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    justifyContent="center"
                    sx={{ mt: 3 }}
                  >
                    <IconButton
                      size="small"
                      disabled={currentCustomPage <= 1}
                      onClick={() => setCustomPage(currentCustomPage - 1)}
                      aria-label="Previous page"
                    >
                      <ChevronLeft size={18} />
                    </IconButton>
                    <Typography variant="body2" color="text.secondary">
                      Page {currentCustomPage} of {customTotalPages}
                    </Typography>
                    <IconButton
                      size="small"
                      disabled={currentCustomPage >= customTotalPages}
                      onClick={() => setCustomPage(currentCustomPage + 1)}
                      aria-label="Next page"
                    >
                      <ChevronRight size={18} />
                    </IconButton>
                  </Stack>
                )}
              </>
            ) : (
              <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                {templates.length === 0
                  ? 'No custom templates yet.'
                  : 'No custom templates match your search.'}
              </Typography>
            )}

            {builtInTemplates.length > 0 && (
              <>
                <Divider sx={{ my: 4 }} />
                <Typography variant="h6" sx={{ fontWeight: 700, mb: 0.5 }}>
                  Built-in templates
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Ready-made templates for popular LLM providers, included by
                  default.
                </Typography>
                {filteredBuiltIn.length > 0 ? (
                  <Box sx={cardGridSx}>
                    {filteredBuiltIn.map((template) => renderCard(template, false))}
                  </Box>
                ) : (
                  <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                    No built-in templates match your search.
                  </Typography>
                )}
              </>
            )}
          </>
        )}

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>
          Are you sure you want to delete the template{' '}
          <strong>'{deleteTarget?.name ?? ''}'</strong>?
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            This action is irreversible.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setDeleteTarget(null)}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.cancel"
              defaultMessage={'Cancel'}
            />
          </Button>
          <Button
            color="error"
            onClick={handleDeleteConfirm}
            data-cyid="delete-provider-template-confirm-button"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplatesList.delete"
              defaultMessage={'Delete'}
            />
          </Button>
        </DialogActions>
      </Dialog>
    </Wrapper>
  );
}
