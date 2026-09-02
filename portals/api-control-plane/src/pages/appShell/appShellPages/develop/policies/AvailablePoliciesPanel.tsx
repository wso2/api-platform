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

import {
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ExternalLink, GripVertical, Search, Shield } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  usePolicyHubCategories,
  usePolicyHubPolicies,
  type PolicySummary,
} from '@/api/resources/policyHub';
import { EmptyState, ErrorState } from '@/components/StateViews';
import { runtimeConfig } from '@/config/runtime';
import { POLICY_DND_MIME, setDraggedPolicy } from './policyDnd';

const messages = defineMessages({
  heading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.heading',
    defaultMessage: 'Available policies',
    description: 'Heading over the Policy Hub catalog that policies are dragged from.',
  },
  policyHub: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.policyHub',
    defaultMessage: 'Policy Hub',
    description: 'Link opening the Policy Hub website. Product name — leave untranslated.',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.searchPlaceholder',
    defaultMessage: 'Search policies',
  },
  allCategories: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.allCategories',
    defaultMessage: 'All',
    description: 'Filter chip clearing the category selection so every policy is listed.',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.loadError',
    defaultMessage: 'Unable to load policies from the Policy Hub.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.emptyTitle',
    defaultMessage: 'No policies',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.emptyDescription',
    defaultMessage: 'No policies match the filter.',
  },
  version: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.version',
    defaultMessage: 'v{version}',
    description:
      'Version chip on a catalog entry. {version} is backend data; keep the "v" prefix if it reads naturally.',
  },
  previous: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.previous',
    defaultMessage: 'Previous',
    description: 'Pagination button for the previous page of the catalog.',
  },
  next: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.next',
    defaultMessage: 'Next',
    description: 'Pagination button for the next page of the catalog.',
  },
});

const PAGE_SIZE = 20;

/**
 * The "Available Policies" panel: the Policy Hub catalog rendered inline, with
 * each policy draggable onto a drop zone in the Policies panel. Also supports a
 * click fallback (onSelect) for non-DnD attach.
 */
export function AvailablePoliciesPanel({
  onSelect,
}: {
  onSelect: (policy: PolicySummary) => void;
}) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const [activeCategories, setActiveCategories] = useState<string[]>([]);
  const [page, setPage] = useState(1);

  const categoriesQuery = usePolicyHubCategories();
  const policiesQuery = usePolicyHubPolicies(page, PAGE_SIZE, activeCategories);

  const toggleCategory = (cat: string) => {
    setActiveCategories((prev) =>
      prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat],
    );
    setPage(1);
  };

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    const list = policiesQuery.data?.policies || [];
    if (!term) return list;
    return list.filter((p) =>
      [p.displayName, p.name, p.description, ...p.tags]
        .filter(Boolean)
        .some((v) => v!.toLowerCase().includes(term)),
    );
  }, [policiesQuery.data, search]);

  const total = policiesQuery.data?.total ?? 0;
  const hasMore = page * PAGE_SIZE < total;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'space-between',
          mb: 1.5,
        }}
      >
        <Typography variant="subtitle1">
          <FormattedMessage {...messages.heading} />
        </Typography>
        {runtimeConfig.policyHubWebUrl && (
          <Button
            component="a"
            endIcon={<ExternalLink size={14} />}
            href={runtimeConfig.policyHubWebUrl}
            rel="noreferrer"
            size="small"
            target="_blank"
          >
            <FormattedMessage {...messages.policyHub} />
          </Button>
        )}
      </Box>

      <TextField
        fullWidth
        onChange={(event) => setSearch(event.target.value)}
        placeholder={intl.formatMessage(messages.searchPlaceholder)}
        size="small"
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={18} />
              </InputAdornment>
            ),
          },
        }}
        value={search}
      />

      {(categoriesQuery.data?.length || 0) > 0 && (
        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75, mt: 1.5 }}>
          <Chip
            color={activeCategories.length === 0 ? 'primary' : 'default'}
            label={intl.formatMessage(messages.allCategories)}
            onClick={() => {
              setActiveCategories([]);
              setPage(1);
            }}
            size="small"
          />
          {categoriesQuery.data!.map((cat) => {
            const selected = activeCategories.includes(cat);
            return (
              <Chip
                color={selected ? 'primary' : 'default'}
                key={cat}
                label={cat}
                onClick={() => toggleCategory(cat)}
                size="small"
                variant={selected ? 'filled' : 'outlined'}
              />
            );
          })}
        </Stack>
      )}

      <Box sx={{ flex: 1, mt: 1.5, overflowY: 'auto' }}>
        {policiesQuery.isPending ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={24} />
          </Box>
        ) : policiesQuery.error ? (
          // The hub's own wording is deliberately not shown: `ApiError.message`
          // for a hub failure is a sterile client-side string, and echoing an
          // upstream message risks naming internal hosts.
          <ErrorState message={intl.formatMessage(messages.loadError)} />
        ) : filtered.length === 0 ? (
          <EmptyState
            title={intl.formatMessage(messages.emptyTitle)}
            description={intl.formatMessage(messages.emptyDescription)}
          />
        ) : (
          <Stack spacing={1}>
            {filtered.map((policy) => (
              <Box
                draggable
                key={`${policy.name}@${policy.version}`}
                onClick={() => onSelect(policy)}
                onDragEnd={() => setDraggedPolicy(null)}
                onDragStart={(event) => {
                  setDraggedPolicy({
                    name: policy.name,
                    version: policy.version,
                    displayName: policy.displayName,
                  });
                  event.dataTransfer.effectAllowed = 'copy';
                  event.dataTransfer.setData(POLICY_DND_MIME, policy.name);
                }}
                sx={{
                  alignItems: 'center',
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1.5,
                  cursor: 'grab',
                  display: 'flex',
                  gap: 1,
                  p: 1.25,
                  '&:active': { cursor: 'grabbing' },
                  '&:hover': { bgcolor: 'action.hover', borderColor: 'primary.light' },
                }}
              >
                <Box sx={{ color: 'text.disabled', display: 'flex' }}>
                  <GripVertical size={16} />
                </Box>
                <Avatar src={policy.iconUrl} sx={{ height: 28, width: 28 }} variant="rounded">
                  <Shield size={14} />
                </Avatar>
                <Typography noWrap sx={{ flex: 1, fontWeight: 600, minWidth: 0 }} variant="body2">
                  {policy.displayName}
                </Typography>
                <Chip
                  label={intl.formatMessage(messages.version, { version: policy.version })}
                  size="small"
                  variant="outlined"
                />
              </Box>
            ))}
          </Stack>
        )}
      </Box>

      {(page > 1 || hasMore) && (
        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'space-between', pt: 1.5 }}>
          <Button disabled={page <= 1} onClick={() => setPage((p) => p - 1)} size="small">
            <FormattedMessage {...messages.previous} />
          </Button>
          <Button disabled={!hasMore} onClick={() => setPage((p) => p + 1)} size="small">
            <FormattedMessage {...messages.next} />
          </Button>
        </Box>
      )}
    </Box>
  );
}
