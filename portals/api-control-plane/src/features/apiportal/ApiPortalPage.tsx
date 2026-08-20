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

import { useMemo, useRef, useState } from 'react';
import {
  alpha,
  Box,
  Button,
  Card,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  Clock,
  Copy,
  Globe,
  LayoutGrid,
  List,
  MoreVertical,
  Plus,
  Search,
  ShieldCheck,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import { useDeleteApiPortal, useApiPortals } from '../../api/hooks/useMvpQueries';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useNotifications } from '../../components/Notifications';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../components/StateViews';
import { routes } from '../../routes/paths';
import { relativeTime } from '../../utils/relativeTime';
import type { ApiPortal } from '../../types/domain';
import { AUTH_LABEL, STATUS_COLOR, STATUS_LABEL } from './apiPortalDisplay';

type ViewMode = 'grid' | 'list';

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

function ApiPortalCard({
  apiPortal,
  onOpen,
  onDelete,
  openMenuId,
  onMenuOpenChange,
}: {
  apiPortal: ApiPortal;
  onOpen: (apiPortal: ApiPortal) => void;
  onDelete?: (apiPortal: ApiPortal) => void;
  openMenuId: string | null;
  onMenuOpenChange: (id: string | null) => void;
}) {
  const [copied, setCopied] = useState(false);
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const statusColor = STATUS_COLOR[apiPortal.workflowStatus];
  // MUI's Menu closes on an outside click via a document-level listener that
  // doesn't reliably block the same click from also reaching a card's
  // onClick underneath it — dismissing a menu by clicking elsewhere would
  // otherwise also open whichever API Portal was clicked, including a
  // different card than the one whose menu was open. mousedown always fires
  // before click, so capturing the page-level "was some menu open" state
  // there is a deterministic guard regardless of exactly when the menu's own
  // close logic runs.
  const wasMenuOpenRef = useRef(false);

  const copyUrl = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (!apiPortal.url) return;
    navigator.clipboard
      ?.writeText(apiPortal.url)
      .then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 1400);
      })
      .catch(() => undefined);
  };

  const closeMenu = (event?: React.MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
    onMenuOpenChange(null);
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
      aria-label={`Open ${apiPortal.name}`}
      onClick={() => {
        if (wasMenuOpenRef.current) {
          wasMenuOpenRef.current = false;
          return;
        }
        onOpen(apiPortal);
      }}
      onKeyDown={(event) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen(apiPortal);
        }
      }}
      onMouseDown={() => {
        wasMenuOpenRef.current = openMenuId !== null;
      }}
      role="button"
      tabIndex={0}
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        cursor: 'pointer',
        p: 2.5,
        transition: 'border-color .2s, box-shadow .2s, transform .2s',
        '&:hover': {
          borderColor: 'primary.main',
          boxShadow: 3,
          transform: 'translateY(-2px)',
        },
        '&:focus-visible': {
          outline: (t) => `2px solid ${t.palette.primary.main}`,
          outlineOffset: 2,
        },
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
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
            {apiPortal.name}
          </Typography>
          {apiPortal.url && (
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
                {apiPortal.url}
              </Typography>
              <Tooltip title={copied ? 'Copied' : 'Copy URL'}>
                <IconButton
                  aria-label="Copy URL"
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
        {onDelete && (
          <>
            <Tooltip title="API Portal actions">
              <IconButton
                aria-label="API Portal actions"
                onClick={(event) => {
                  event.stopPropagation();
                  setMenuAnchor(event.currentTarget);
                  onMenuOpenChange(apiPortal.id);
                }}
                size="small"
                sx={{ alignSelf: 'flex-start', flex: 'none' }}
              >
                <MoreVertical size={16} />
              </IconButton>
            </Tooltip>
            <Menu
              anchorEl={menuAnchor}
              onClose={() => closeMenu()}
              open={Boolean(menuAnchor)}
            >
              <MenuItem
                onClick={(event) => {
                  closeMenu(event);
                  onDelete(apiPortal);
                }}
                sx={{ color: 'error.main' }}
              >
                <ListItemIcon sx={{ color: 'inherit' }}>
                  <Trash2 size={16} />
                </ListItemIcon>
                <ListItemText>Delete</ListItemText>
              </MenuItem>
            </Menu>
          </>
        )}
      </Stack>

      {apiPortal.description && (
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
          {apiPortal.description}
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
          {AUTH_LABEL[apiPortal.authType]}
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
            {STATUS_LABEL[apiPortal.workflowStatus]}
          </Typography>
        </Stack>
        {apiPortal.createdAt && (
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.625}
            sx={{ color: 'text.disabled' }}
          >
            <Clock size={13} />
            <Typography sx={{ fontSize: 12 }}>
              {relativeTime(apiPortal.createdAt)}
            </Typography>
          </Stack>
        )}
      </Stack>
    </Box>
  );
}

function ApiPortalRow({
  apiPortal,
  onOpen,
  onDelete,
  openMenuId,
  onMenuOpenChange,
}: {
  apiPortal: ApiPortal;
  onOpen: (apiPortal: ApiPortal) => void;
  onDelete?: (apiPortal: ApiPortal) => void;
  openMenuId: string | null;
  onMenuOpenChange: (id: string | null) => void;
}) {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const statusColor = STATUS_COLOR[apiPortal.workflowStatus];
  // Same click-away guard as ApiPortalCard — see the comment there.
  const wasMenuOpenRef = useRef(false);

  const closeMenu = (event?: React.MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
    onMenuOpenChange(null);
  };

  return (
    <Box
      aria-label={`Open ${apiPortal.name}`}
      onClick={() => {
        if (wasMenuOpenRef.current) {
          wasMenuOpenRef.current = false;
          return;
        }
        onOpen(apiPortal);
      }}
      onKeyDown={(event) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen(apiPortal);
        }
      }}
      onMouseDown={() => {
        wasMenuOpenRef.current = openMenuId !== null;
      }}
      role="button"
      tabIndex={0}
      sx={{
        alignItems: 'center',
        borderBottom: '1px solid',
        borderColor: 'divider',
        cursor: 'pointer',
        display: 'flex',
        gap: 2,
        px: 2.5,
        py: 1.75,
        transition: 'background-color 250ms',
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
        '&:focus-visible': {
          outline: (t) => `2px solid ${t.palette.primary.main}`,
          outlineOffset: 2,
        },
      }}
    >
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
          height: 36,
          justifyContent: 'center',
          width: 36,
        }}
      >
        <Globe size={18} />
      </Box>
      <Box sx={{ flex: 1, minWidth: 140 }}>
        <Typography noWrap sx={{ fontWeight: 500 }} variant="subtitle2">
          {apiPortal.name}
        </Typography>
        <Typography
          color="text.secondary"
          component="div"
          noWrap
          sx={{ fontFamily: 'monospace' }}
          variant="caption"
        >
          {apiPortal.url || apiPortal.handle}
        </Typography>
      </Box>
      <Box
        sx={{
          alignItems: 'center',
          color: 'text.secondary',
          display: { sm: 'flex', xs: 'none' },
          flexShrink: 0,
          gap: 0.75,
        }}
      >
        <ShieldCheck size={13} />
        <Typography noWrap variant="caption">
          {AUTH_LABEL[apiPortal.authType]}
        </Typography>
      </Box>
      <Box
        sx={{
          alignItems: 'center',
          color: statusColor,
          display: 'flex',
          flexShrink: 0,
          gap: 0.75,
        }}
      >
        <Box
          sx={{ bgcolor: statusColor, borderRadius: '50%', height: 8, width: 8 }}
        />
        <Typography noWrap sx={{ fontWeight: 500 }} variant="caption">
          {STATUS_LABEL[apiPortal.workflowStatus]}
        </Typography>
      </Box>
      <Box
        sx={{
          alignItems: 'center',
          color: 'text.secondary',
          display: { md: 'flex', xs: 'none' },
          flexShrink: 0,
          gap: 0.75,
          ml: 'auto',
        }}
      >
        <Clock size={12} />
        <Typography color="text.secondary" noWrap variant="caption">
          {apiPortal.createdAt ? relativeTime(apiPortal.createdAt) : '—'}
        </Typography>
      </Box>
      {onDelete && (
        <>
          <IconButton
            aria-label="API Portal actions"
            onClick={(event) => {
              event.stopPropagation();
              setMenuAnchor(event.currentTarget);
              onMenuOpenChange(apiPortal.id);
            }}
            size="small"
            sx={{ ml: { md: 0, xs: 'auto' } }}
          >
            <MoreVertical size={18} />
          </IconButton>
          <Menu
            anchorEl={menuAnchor}
            onClose={() => closeMenu()}
            open={Boolean(menuAnchor)}
          >
            <MenuItem
              onClick={(event) => {
                closeMenu(event);
                onDelete(apiPortal);
              }}
              sx={{ color: 'error.main' }}
            >
              <ListItemIcon sx={{ color: 'inherit' }}>
                <Trash2 size={16} />
              </ListItemIcon>
              <ListItemText>Delete</ListItemText>
            </MenuItem>
          </Menu>
        </>
      )}
    </Box>
  );
}

/** Compact row layout for API Portals — the list-view counterpart of the card grid. */
function ApiPortalListView({
  apiPortals,
  onOpen,
  onDelete,
  openMenuId,
  onMenuOpenChange,
}: {
  apiPortals: ApiPortal[];
  onOpen: (apiPortal: ApiPortal) => void;
  onDelete?: (apiPortal: ApiPortal) => void;
  openMenuId: string | null;
  onMenuOpenChange: (id: string | null) => void;
}) {
  return (
    <Card variant="outlined">
      {apiPortals.map((apiPortal) => (
        <ApiPortalRow
          apiPortal={apiPortal}
          key={apiPortal.id}
          onDelete={onDelete}
          onMenuOpenChange={onMenuOpenChange}
          onOpen={onOpen}
          openMenuId={openMenuId}
        />
      ))}
    </Card>
  );
}

export function ApiPortalPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const apiPortalsQuery = useApiPortals();
  const deleteApiPortalMutation = useDeleteApiPortal();
  const [search, setSearch] = useState('');
  const [view, setView] = useState<ViewMode>('grid');
  const [toDelete, setToDelete] = useState<ApiPortal | null>(null);
  // Shared across every card/row so opening one item's actions menu, then
  // clicking a DIFFERENT item, dismisses the menu instead of also opening
  // that item — see the click-away guard comment in ApiPortalCard.
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const provision = () => navigate(routes.newApiPortal(orgHandle));
  const openApiPortal = (apiPortal: ApiPortal) =>
    navigate(routes.apiPortalDetail(orgHandle, apiPortal.id));

  const confirmDelete = () => {
    if (!toDelete) return;
    deleteApiPortalMutation.mutate(toDelete, {
      onSuccess: () => {
        notify(`Deleted "${toDelete.name}".`, 'success');
        setToDelete(null);
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Delete failed',
          'error'
        ),
    });
  };

  const apiPortals = useMemo(
    () => apiPortalsQuery.data || [],
    [apiPortalsQuery.data]
  );

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return apiPortals;
    return apiPortals.filter((apiPortal) =>
      [apiPortal.name, apiPortal.handle, apiPortal.url]
        .filter(Boolean)
        .some((field) => field!.toLowerCase().includes(term))
    );
  }, [apiPortals, search]);

  const activeCount = apiPortals.filter(
    (apiPortal) => apiPortal.workflowStatus === 'active'
  ).length;

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>API Portal</PageTitle.Header>
        <PageTitle.SubHeader>
          Provision and manage the API Portal for your organization.
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            onClick={provision}
            startIcon={<Plus />}
            sx={{ borderRadius: 5 }}
            variant="contained"
          >
            Provision API Portal
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {apiPortalsQuery.isLoading ? (
        <LoadingState label="Loading API Portals" />
      ) : apiPortalsQuery.error ? (
        <ErrorState message="Unable to load API Portals" />
      ) : apiPortals.length === 0 ? (
        <EmptyState
          actionLabel="Provision API Portal"
          description="Provision an API Portal to publish APIs for external developers."
          onAction={provision}
          title="No API Portal provisioned yet"
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
              label="Total API Portals"
              value={apiPortals.length}
            />
            <StatCard
              dotColor="success.main"
              label="Active"
              value={activeCount}
            />
          </Box>

          {/* Toolbar */}
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 1.5,
            }}
          >
            <TextField
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search API Portal"
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
              sx={{ flex: 1, maxWidth: 420, minWidth: 240 }}
              value={search}
            />
            <ToggleButtonGroup
              exclusive
              onChange={(_event, value: ViewMode | null) => {
                if (value) setView(value);
              }}
              size="small"
              sx={{ ml: 'auto' }}
              value={view}
            >
              <ToggleButton aria-label="Grid view" value="grid">
                <LayoutGrid size={16} />
              </ToggleButton>
              <ToggleButton aria-label="List view" value="list">
                <List size={16} />
              </ToggleButton>
            </ToggleButtonGroup>
          </Box>

          {filtered.length === 0 ? (
            <EmptyState
              description="Try a different search term."
              title="No matching API Portals"
            />
          ) : view === 'grid' ? (
            <Box
              sx={{
                display: 'grid',
                gap: 2.5,
                gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))',
              }}
            >
              {filtered.map((apiPortal) => (
                <ApiPortalCard
                  apiPortal={apiPortal}
                  key={apiPortal.id}
                  onDelete={setToDelete}
                  onMenuOpenChange={setOpenMenuId}
                  onOpen={openApiPortal}
                  openMenuId={openMenuId}
                />
              ))}
            </Box>
          ) : (
            <ApiPortalListView
              apiPortals={filtered}
              onDelete={setToDelete}
              onMenuOpenChange={setOpenMenuId}
              onOpen={openApiPortal}
              openMenuId={openMenuId}
            />
          )}
        </Stack>
      )}

      <ConfirmDialog
        confirmInputLabel={`Type "${toDelete?.name ?? ''}" to confirm`}
        confirmLabel="Delete"
        confirmPhrase={toDelete?.name ?? ''}
        destructive
        loading={deleteApiPortalMutation.isPending}
        message={
          toDelete
            ? `This permanently deletes the API Portal "${toDelete.name}". This action is irreversible.`
            : ''
        }
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
        open={toDelete !== null}
        title="Delete API Portal"
      />
    </PageContent>
  );
}
