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

import { Alert, Avatar, Box, Button, Chip, Form, Grid, Skeleton, Stack, ToggleButton, ToggleButtonGroup, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Handshake } from '@wso2/oxygen-ui-icons-react';
import type { ProviderTemplate, ProviderTemplatesResponse } from '../../../../../utils/types';
import McpMenuIcon from '../../../../../assets/icons/McpMenuIcon';
import { getProviderLogoForTemplate, getShortNameForTemplate, isComingSoonTemplate } from '../providerTemplateVisuals';
import type { ResourceType } from './types';

type SelectProviderTemplateStepProps = {
  role: string;
  resourceType: ResourceType;
  selectedTemplateId: string | null;
  sortedTemplates: ProviderTemplate[];
  templatesLoading: boolean;
  templatesError: Error | null;
  templatesResponse: ProviderTemplatesResponse;
  onSelectTemplate: (template: ProviderTemplate) => void;
  onResourceTypeChange: (resourceType: ResourceType) => void;
  onRetryTemplates: () => void | Promise<void>;
};

function ProviderTemplateCard({
  template,
  isSelected,
  onSelect,
}: {
  template: ProviderTemplate;
  isSelected: boolean;
  onSelect: (template: ProviderTemplate) => void;
}) {
  const logo = getProviderLogoForTemplate(template.displayName);
  const shortName = getShortNameForTemplate(template.displayName);
  const isComingSoon = isComingSoonTemplate(template.id);

  return (
    <Form.CardButton
      selected={isSelected}
      disabled={isComingSoon}
      onClick={() => { if (!isComingSoon) onSelect(template); }}
      sx={{
        width: '100%',
        minHeight: 72,
        minWidth: 140,
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        gap: 1.5,
        px: 2,
        py: 1.5,
        textAlign: 'left',
        borderRadius: 1.25,
        borderColor: isSelected ? 'warning.main' : 'divider',
        backgroundColor: isSelected ? 'rgba(234, 106, 51, 0.08)' : 'transparent',
        boxShadow: isSelected ? '0 8px 20px rgba(234, 106, 51, 0.12)' : 'none',
        transition: 'border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease',
        '&:hover': {
          borderColor: isComingSoon ? 'divider' : 'warning.main',
        },
      }}
    >
      <Box
        sx={{
          width: 42,
          height: 42,
          borderRadius: 1,
          border: '1px solid',
          borderColor: 'divider',
          backgroundColor: 'background.paper',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
          flexShrink: 0,
        }}
      >
        {logo ? (
          <Box
            component="img"
            src={logo}
            alt={`${template.displayName} logo`}
            sx={{ width: '88%', height: '88%', objectFit: 'contain' }}
          />
        ) : (
          <Avatar sx={{ width: 34, height: 34, fontSize: 13 }}>
            {shortName}
          </Avatar>
        )}
      </Box>

      <Typography variant="subtitle2" sx={{ fontWeight: 600, flex: 1, minWidth: 0 }}>
        {template.displayName}
      </Typography>

      {isComingSoon ? (
        <Chip
          label="Coming soon"
          size="small"
          sx={{
            bgcolor: '#EA6A33',
            color: '#FFFFFF',
            flexShrink: 0,
            '& .MuiChip-label': { px: 1 },
          }}
        />
      ) : null}
    </Form.CardButton>
  );
}

export default function SelectProviderTemplateStep({
  role,
  resourceType,
  selectedTemplateId,
  sortedTemplates,
  templatesLoading,
  templatesError,
  templatesResponse,
  onSelectTemplate,
  onResourceTypeChange,
  onRetryTemplates,
}: SelectProviderTemplateStepProps) {
  return (
    <Stack spacing={2.5}>
      <Box>
        <Typography variant="body2" sx={{ mb: 1.25, fontWeight: 600 }}>
          Resource Type
        </Typography>
        <ToggleButtonGroup
          size="medium"
          exclusive
          value={resourceType}
          onChange={(_, value: ResourceType | null) => {
            if (!value) return;
            onResourceTypeChange(value);
          }}
          sx={{
            borderRadius: '6px',
            '& .MuiToggleButtonGroup-grouped': {
              '&:first-of-type': { borderRadius: '6px 0 0 6px' },
              '&:last-of-type': { borderRadius: '0 6px 6px 0' },
            },
            '& .MuiToggleButton-root.Mui-selected': {
              backgroundColor: 'rgba(234, 106, 51, 0.1)',
              color: 'warning.main',
              '&:hover': { backgroundColor: 'rgba(234, 106, 51, 0.16)' },
            },
          }}
        >
          <Tooltip
            title={
              role === 'developer'
                ? 'Switch to the admin role to add LLM providers.'
                : ''
            }
            disableHoverListener={role !== 'developer'}
          >
            <Box component="span">
              <ToggleButton
                value="provider"
                disabled={role === 'developer'}
                sx={{
                  gap: 1,
                  px: 2,
                  textTransform: 'none',
                  justifyContent: 'flex-start',
                  borderTopLeftRadius: '6px !important',
                  borderBottomLeftRadius: '6px !important',
                  borderTopRightRadius: '0 !important',
                  borderBottomRightRadius: '0 !important',
                }}
              >
                <Handshake size={18} />
                LLM Provider
              </ToggleButton>
            </Box>
          </Tooltip>
          <ToggleButton
            value="mcp"
            sx={{
              gap: 1,
              px: 2,
              textTransform: 'none',
              justifyContent: 'flex-start',
            }}
          >
            <McpMenuIcon size={18} aria-hidden />
            MCP Server
          </ToggleButton>
        </ToggleButtonGroup>
      </Box>

      <Stack spacing={2}>
        <Box>
          <Typography variant="body2" sx={{ mb: 1.25, fontWeight: 600 }}>
            LLM Provider
          </Typography>

          {templatesLoading ? (
            <Grid container spacing={2}>
              {Array.from({ length: 4 }).map((_, i) => (
                <Grid key={i} size={{ xs: 12, sm: 6, lg: 3 }}>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1.5,
                      px: 2,
                      py: 1.5,
                      minHeight: 72,
                      minWidth: 140,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1.25,
                    }}
                  >
                    <Skeleton variant="rounded" width={42} height={42} sx={{ flexShrink: 0 }} />
                    <Skeleton variant="text" sx={{ flex: 1 }} height={20} />
                  </Box>
                </Grid>
              ))}
            </Grid>
          ) : templatesError ? (
            <Alert
              severity="error"
              action={
                <Button
                  color="inherit"
                  size="small"
                  onClick={() => {
                    void onRetryTemplates();
                  }}
                >
                  Retry
                </Button>
              }
            >
              Failed to load provider templates.
            </Alert>
          ) : (
            <>
              {sortedTemplates.length > 0 || templatesResponse.list.length > 0 ? (
                <Grid container spacing={2}>
                  {sortedTemplates.map((template) => (
                    <Grid key={template.id ?? template.displayName} size={{ xs: 12, sm: 6, lg: 3 }}>
                      <ProviderTemplateCard
                        template={template}
                        isSelected={selectedTemplateId === template.id}
                        onSelect={onSelectTemplate}
                      />
                    </Grid>
                  ))}
                </Grid>
              ) : (
                <Typography variant="body2" color="text.secondary">
                  No provider templates are available for this organization yet.
                </Typography>
              )}
            </>
          )}
        </Box>
      </Stack>
    </Stack>
  );
}
