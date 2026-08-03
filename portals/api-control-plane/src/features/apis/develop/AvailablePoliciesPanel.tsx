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

import type { PolicySummary } from '../../../api/policyHub/policyHubClient';
import {
  usePolicyHubCategories,
  usePolicyHubPolicies,
} from '../../../api/policyHub/usePolicyHub';
import { EmptyState, ErrorState } from '../../../components/StateViews';
import { runtimeConfig } from '../../../config/runtime';
import { POLICY_DND_MIME, setDraggedPolicy } from './policyDnd';

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
  const [search, setSearch] = useState('');
  const [activeCategories, setActiveCategories] = useState<string[]>([]);
  const [page, setPage] = useState(1);

  const categoriesQuery = usePolicyHubCategories();
  const policiesQuery = usePolicyHubPolicies(page, PAGE_SIZE, activeCategories);

  const toggleCategory = (cat: string) => {
    setActiveCategories((prev) =>
      prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat]
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
        .some((v) => v!.toLowerCase().includes(term))
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
        <Typography variant="subtitle1">Available policies</Typography>
        {runtimeConfig.policyHubWebUrl && (
          <Button
            component="a"
            endIcon={<ExternalLink size={14} />}
            href={runtimeConfig.policyHubWebUrl}
            rel="noreferrer"
            size="small"
            target="_blank"
          >
            Policy Hub
          </Button>
        )}
      </Box>

      <TextField
        fullWidth
        onChange={(event) => setSearch(event.target.value)}
        placeholder="Search policies"
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
            label="All"
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
        {policiesQuery.isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={24} />
          </Box>
        ) : policiesQuery.error ? (
          <ErrorState
            message={
              policiesQuery.error instanceof Error
                ? policiesQuery.error.message
                : 'Unable to load policies from the Policy Hub.'
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState title="No policies" description="No policies match the filter." />
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
                <Chip label={`v${policy.version}`} size="small" variant="outlined" />
              </Box>
            ))}
          </Stack>
        )}
      </Box>

      {(page > 1 || hasMore) && (
        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'space-between', pt: 1.5 }}>
          <Button disabled={page <= 1} onClick={() => setPage((p) => p - 1)} size="small">
            Previous
          </Button>
          <Button disabled={!hasMore} onClick={() => setPage((p) => p + 1)} size="small">
            Next
          </Button>
        </Box>
      )}
    </Box>
  );
}
