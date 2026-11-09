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
  Card,
  CardContent,
  Checkbox,
  Chip,
  Divider,
  InputAdornment,
  MenuItem,
  Select,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ExternalLink, GripVertical, Library, Search, Shield } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  usePolicyHubCategories,
  usePolicyHubInfinitePolicies,
  type PolicySummary,
} from '@/api/resources/policyHub';
import { SecondaryText } from '@/components/common/SecondaryText';
import { EmptyState, ErrorState } from '@/components/StateViews';
import { runtimeConfig } from '@/config/runtime';
import { iconTileSx } from '@/theme/receipes';
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
    description: 'Placeholder shown when no category filters are selected.',
  },
  categories: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.categories',
    defaultMessage: 'Categories',
    description: 'Accessible name of the category filter dropdown. Plural noun.',
  },
  categoriesSelected: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.categoriesSelected',
    defaultMessage: '{count} categories',
    description:
      'Shown in the category dropdown when several categories are selected. {count} is a number.',
  },
  retry: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.retry',
    defaultMessage: 'Retry',
    description: 'Loads the policy catalog again after a failed request. Verb.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AvailablePoliciesPanel.subtitle',
    defaultMessage: 'Drag onto Global policies or a resource',
    description: 'Hint under the catalog heading about how to attach a policy.',
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
});

const PAGE_SIZE = 20;

/** How long typing must pause before the catalog is searched. */
const SEARCH_DEBOUNCE_MS = 300;

/** Placeholder cards while the first page, or the next one, is on its way. */
const INITIAL_SKELETONS = 8;
const NEXT_PAGE_SKELETONS = 3;

/** A catalog card's outline while its policy is still loading. */
function PolicySkeletonCard() {
  return (
    <Card>
      <CardContent
        sx={{
          alignItems: 'center',
          display: 'flex',
          gap: 1,
          p: 1.25,
          '&:last-child': { pb: 1.25 },
        }}
      >
        <Skeleton height={16} variant="rounded" width={16} />
        <Skeleton height={28} variant="rounded" width={28} />
        <Skeleton sx={{ flex: 1 }} variant="text" />
        <Skeleton height={22} variant="rounded" width={44} />
      </CardContent>
    </Card>
  );
}

/**
 * The "Available Policies" panel: the Policy Hub catalog rendered inline, with
 * each policy draggable onto a drop zone in the Policies panel. Also supports a
 * click fallback (onSelect) for non-DnD attach.
 */
export function AvailablePoliciesPanel({
  onSelect,
  disabled = false,
}: {
  onSelect: (policy: PolicySummary) => void;
  /** Catalog stays browsable, but its policies can be neither dragged nor clicked to attach. */
  disabled?: boolean;
}) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const [activeCategories, setActiveCategories] = useState<string[]>([]);
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  // The hub does the searching, so wait for the typing to settle before asking it.
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedSearch(search.trim()), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [search]);

  const categoriesQuery = usePolicyHubCategories();
  const policiesQuery = usePolicyHubInfinitePolicies(PAGE_SIZE, debouncedSearch, activeCategories);
  const { fetchNextPage, hasNextPage, isFetchingNextPage, isFetchNextPageError, refetch } =
    policiesQuery;

  const policies = useMemo(
    () => policiesQuery.data?.pages.flatMap((page) => page.policies) ?? [],
    [policiesQuery.data],
  );

  // Load the next page when the end of the list scrolls into view. Re-observing after
  // each page keeps loading when a short page leaves the end still on screen.
  useEffect(() => {
    const sentinel = sentinelRef.current;
    // A failed page stops the loading: retrying on every re-render would hammer the hub.
    if (
      !sentinel ||
      !hasNextPage ||
      isFetchNextPageError ||
      typeof IntersectionObserver === 'undefined'
    ) {
      return undefined;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting) && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { root: scrollRef.current, rootMargin: '120px' },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage, isFetchNextPageError, policies.length]);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Stack alignItems="center" direction="row" spacing={1.5} sx={{ mb: 1.5 }}>
        <Box sx={(theme) => iconTileSx(theme, 36)}>
          <Library size={18} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontWeight: 700, lineHeight: 1.3 }} variant="subtitle2">
            <FormattedMessage {...messages.heading} />
          </Typography>
          <SecondaryText sx={{ display: 'block', lineHeight: 1.3 }} variant="caption">
            <FormattedMessage {...messages.subtitle} />
          </SecondaryText>
        </Box>
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
      </Stack>

      {/* One line: the search box with the category filter docked on its right. */}
      <TextField
        fullWidth
        onChange={(event) => setSearch(event.target.value)}
        placeholder={intl.formatMessage(messages.searchPlaceholder)}
        size="small"
        slotProps={{
          input: {
            endAdornment:
              (categoriesQuery.data?.length || 0) > 0 ? (
                <>
                  <Divider flexItem orientation="vertical" />
                  <Select
                    disableUnderline
                    displayEmpty
                    inputProps={{ 'aria-label': intl.formatMessage(messages.categories) }}
                    multiple
                    onChange={(event) => {
                      const next = event.target.value;
                      setActiveCategories(typeof next === 'string' ? next.split(',') : next);
                    }}
                    renderValue={(selected) =>
                      selected.length === 0
                        ? intl.formatMessage(messages.allCategories)
                        : selected.length === 1
                          ? selected[0]
                          : intl.formatMessage(messages.categoriesSelected, {
                              count: selected.length,
                            })
                    }
                    sx={{ maxWidth: 170, minWidth: 120, px: 1 }}
                    value={activeCategories}
                    variant="standard"
                  >
                    {categoriesQuery.data!.map((category) => (
                      <MenuItem key={category} value={category}>
                        <Checkbox
                          checked={activeCategories.includes(category)}
                          size="small"
                          sx={{ mr: 1 }}
                        />
                        {category}
                      </MenuItem>
                    ))}
                  </Select>
                </>
              ) : null,
            startAdornment: (
              <InputAdornment position="start">
                <Search size={18} />
              </InputAdornment>
            ),
          },
        }}
        sx={{ '& .MuiInputBase-root': { pr: 0 } }}
        value={search}
      />

      <Box ref={scrollRef} sx={{ flex: 1, mt: 1.5, overflowY: 'auto' }}>
        {policiesQuery.isPending ? (
          <Stack aria-busy spacing={1}>
            {Array.from({ length: INITIAL_SKELETONS }, (_unused, index) => (
              <PolicySkeletonCard key={index} />
            ))}
          </Stack>
        ) : policiesQuery.error && policies.length === 0 ? (
          // The hub's own wording is deliberately not shown: `ApiError.message`
          // for a hub failure is a sterile client-side string, and echoing an
          // upstream message risks naming internal hosts.
          <Stack spacing={1}>
            <ErrorState message={intl.formatMessage(messages.loadError)} />
            <Button
              disabled={policiesQuery.isFetching}
              onClick={() => void refetch()}
              size="small"
              sx={{ alignSelf: 'center' }}
            >
              <FormattedMessage {...messages.retry} />
            </Button>
          </Stack>
        ) : policies.length === 0 ? (
          <EmptyState
            title={intl.formatMessage(messages.emptyTitle)}
            description={intl.formatMessage(messages.emptyDescription)}
          />
        ) : (
          <Stack spacing={1}>
            {policies.map((policy) => (
              <Card
                aria-disabled={disabled || undefined}
                draggable={!disabled}
                key={`${policy.name}@${policy.version}`}
                onClick={disabled ? undefined : () => onSelect(policy)}
                onDragEnd={() => setDraggedPolicy(null)}
                onDragStart={(event) => {
                  if (disabled) {
                    event.preventDefault();
                    return;
                  }
                  setDraggedPolicy({
                    name: policy.name,
                    version: policy.version,
                    displayName: policy.displayName,
                  });
                  event.dataTransfer.effectAllowed = 'copy';
                  event.dataTransfer.setData(POLICY_DND_MIME, policy.name);
                }}
                sx={{
                  cursor: disabled ? 'default' : 'grab',
                  opacity: disabled ? 0.6 : 1,
                  ...(!disabled && {
                    '&:active': { cursor: 'grabbing' },
                    '&:hover': { bgcolor: 'action.hover', borderColor: 'primary.light' },
                  }),
                }}
              >
                <CardContent
                  sx={{
                    alignItems: 'center',
                    display: 'flex',
                    gap: 1,
                    p: 1.25,
                    '&:last-child': { pb: 1.25 },
                  }}
                >
                  <Box sx={{ color: 'text.disabled', display: 'flex' }}>
                    <GripVertical size={16} />
                  </Box>
                  <Avatar
                    src={policy.iconUrl}
                    sx={{ borderRadius: 0.75, height: 28, width: 28 }}
                    variant="rounded"
                  >
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
                </CardContent>
              </Card>
            ))}
            {isFetchingNextPage &&
              Array.from({ length: NEXT_PAGE_SKELETONS }, (_unused, index) => (
                <PolicySkeletonCard key={`next-${index}`} />
              ))}
            {isFetchNextPageError && (
              // The policies already loaded stay usable; only the next page is missing.
              <Stack spacing={1}>
                <ErrorState message={intl.formatMessage(messages.loadError)} />
                <Button
                  disabled={isFetchingNextPage}
                  onClick={() => void fetchNextPage()}
                  size="small"
                  sx={{ alignSelf: 'center' }}
                >
                  <FormattedMessage {...messages.retry} />
                </Button>
              </Stack>
            )}
            <Box aria-hidden ref={sentinelRef} sx={{ height: 1 }} />
          </Stack>
        )}
      </Box>
    </Box>
  );
}
