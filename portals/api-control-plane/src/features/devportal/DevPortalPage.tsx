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

import { useMemo, useState } from 'react';
import {
  alpha,
  Box,
  Button,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  Clock,
  Copy,
  Globe,
  Plus,
  Search,
  ShieldCheck,
} from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import { useDevPortals } from '../../api/hooks/useMvpQueries';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../components/StateViews';
import { routes } from '../../routes/paths';
import { relativeTime } from '../../utils/relativeTime';
import type {
  DevPortal,
  DevPortalAuthType,
  DevPortalWorkflowStatus,
} from '../../types/domain';

const AUTH_LABEL: Record<DevPortalAuthType, string> = {
  local: 'Local',
  idp_client_credentials: 'IdP Client Credentials',
};

const STATUS_LABEL: Record<DevPortalWorkflowStatus, string> = {
  pending: 'Pending',
  active: 'Active',
  failed: 'Failed',
};

const STATUS_COLOR: Record<DevPortalWorkflowStatus, string> = {
  pending: 'warning.main',
  active: 'success.main',
  failed: 'error.main',
};

/** A small KPI summary tile. */
function StatCard({
  label,
  value,
  dotColor,
}: {
  label: string;
  value: number;
  dotColor: string;
}) {
  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        px: 2.5,
        py: 2,
      }}
    >
      <Stack alignItems="center" direction="row" spacing={1.25}>
        <Box
          sx={{ bgcolor: dotColor, borderRadius: '50%', height: 8, width: 8 }}
        />
        <Typography
          color="text.secondary"
          sx={{ fontWeight: 500 }}
          variant="body2"
        >
          {label}
        </Typography>
      </Stack>
      <Typography
        sx={{ fontWeight: 300, letterSpacing: '-.5px', mt: 1 }}
        variant="h4"
      >
        {value}
      </Typography>
    </Box>
  );
}

function DevPortalCard({ devPortal }: { devPortal: DevPortal }) {
  const [copied, setCopied] = useState(false);
  const statusColor = STATUS_COLOR[devPortal.workflowStatus];

  const copyUrl = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (!devPortal.url) return;
    navigator.clipboard?.writeText(devPortal.url).catch(() => undefined);
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const chipSx = {
    alignItems: 'center',
    bgcolor: 'action.hover',
    border: '1px solid',
    borderColor: 'divider',
    borderRadius: 1,
    color: 'text.secondary',
    display: 'inline-flex',
    fontSize: 12,
    fontWeight: 500,
    gap: 0.75,
    px: 1.25,
    py: 0.5,
  };

  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2.5,
      }}
    >
      <Stack direction="row" spacing={1.75}>
        <Box
          sx={{
            alignItems: 'center',
            bgcolor: 'action.hover',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1.5,
            color: 'text.secondary',
            display: 'flex',
            flex: 'none',
            height: 46,
            justifyContent: 'center',
            width: 46,
          }}
        >
          <Globe size={22} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
            {devPortal.name}
          </Typography>
          {devPortal.url && (
            <Stack
              alignItems="center"
              direction="row"
              spacing={0.5}
              sx={{ mt: 0.25 }}
            >
              <Typography
                noWrap
                sx={{
                  color: 'text.secondary',
                  fontFamily: 'monospace',
                  fontSize: 12.5,
                }}
              >
                {devPortal.url}
              </Typography>
              <Tooltip title={copied ? 'Copied' : 'Copy URL'}>
                <IconButton
                  onClick={copyUrl}
                  size="small"
                  sx={{ flex: 'none' }}
                >
                  {copied ? <Check size={14} /> : <Copy size={14} />}
                </IconButton>
              </Tooltip>
            </Stack>
          )}
        </Box>
      </Stack>

      {devPortal.description && (
        <Typography
          color="text.secondary"
          sx={{
            display: '-webkit-box',
            mt: 1.5,
            overflow: 'hidden',
            WebkitBoxOrient: 'vertical',
            WebkitLineClamp: 2,
          }}
          variant="body2"
        >
          {devPortal.description}
        </Typography>
      )}

      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mt: 2 }}>
        <Box
          sx={{
            ...chipSx,
            bgcolor: (t) => alpha(t.palette.primary.main, 0.14),
            borderColor: (t) => alpha(t.palette.primary.main, 0.3),
            color: 'primary.main',
            fontWeight: 600,
          }}
        >
          <ShieldCheck size={13} />
          {AUTH_LABEL[devPortal.authType]}
        </Box>
      </Stack>

      <Stack
        alignItems="center"
        direction="row"
        justifyContent="space-between"
        sx={{ borderColor: 'divider', borderTop: '1px solid', mt: 2, pt: 1.75 }}
      >
        <Stack
          alignItems="center"
          direction="row"
          spacing={0.875}
          sx={{ color: statusColor }}
        >
          <Box
            sx={{
              bgcolor: statusColor,
              borderRadius: '50%',
              height: 8,
              width: 8,
            }}
          />
          <Typography sx={{ fontSize: 12.5, fontWeight: 500 }}>
            {STATUS_LABEL[devPortal.workflowStatus]}
          </Typography>
        </Stack>
        {devPortal.createdAt && (
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.625}
            sx={{ color: 'text.disabled' }}
          >
            <Clock size={13} />
            <Typography sx={{ fontSize: 12 }}>
              {relativeTime(devPortal.createdAt)}
            </Typography>
          </Stack>
        )}
      </Stack>
    </Box>
  );
}

export function DevPortalPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const devPortalsQuery = useDevPortals();
  const [search, setSearch] = useState('');

  const provision = () => navigate(routes.newDevportal(orgHandle));

  const devPortals = useMemo(
    () => devPortalsQuery.data || [],
    [devPortalsQuery.data]
  );

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return devPortals;
    return devPortals.filter((devPortal) =>
      [devPortal.name, devPortal.handle, devPortal.url]
        .filter(Boolean)
        .some((field) => field!.toLowerCase().includes(term))
    );
  }, [devPortals, search]);

  const activeCount = devPortals.filter(
    (devPortal) => devPortal.workflowStatus === 'active'
  ).length;

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>Dev Portal</PageTitle.Header>
        <PageTitle.SubHeader>
          Provision and manage the developer portal for your organization.
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            onClick={provision}
            startIcon={<Plus />}
            sx={{ borderRadius: 5 }}
            variant="contained"
          >
            Provision Devportal
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {devPortalsQuery.isLoading ? (
        <LoadingState label="Loading dev portals" />
      ) : devPortalsQuery.error ? (
        <ErrorState message="Unable to load dev portals" />
      ) : devPortals.length === 0 ? (
        <EmptyState
          actionLabel="Provision Devportal"
          description="Provision a developer portal to publish APIs for external developers."
          onAction={provision}
          title="No Dev Portal provisioned yet"
        />
      ) : (
        <Stack spacing={4}>
          {/* KPI summary */}
          <Box
            sx={{
              display: 'grid',
              gap: 2,
              gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' },
            }}
          >
            <StatCard
              dotColor="primary.main"
              label="Total devportals"
              value={devPortals.length}
            />
            <StatCard
              dotColor="success.main"
              label="Active"
              value={activeCount}
            />
          </Box>

          {/* Toolbar */}
          <TextField
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search Dev Portal"
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
            sx={{ maxWidth: 420, minWidth: 240, width: '100%' }}
            value={search}
          />

          {filtered.length === 0 ? (
            <EmptyState
              description="Try a different search term."
              title="No matching dev portals"
            />
          ) : (
            <Box
              sx={{
                display: 'grid',
                gap: 2.5,
                gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))',
              }}
            >
              {filtered.map((devPortal) => (
                <DevPortalCard devPortal={devPortal} key={devPortal.id} />
              ))}
            </Box>
          )}
        </Stack>
      )}
    </PageContent>
  );
}
