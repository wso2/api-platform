/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState } from 'react';
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ExternalLink, PanelTop, Pencil, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortalList } from './hooks';
import type { ManagedPortal } from './types';

export type ManagedPortalsListProps = {
  /** Switches parent to the create view; create is a full page, not a modal. */
  onCreate: () => void;
  /** Switches parent to the edit view for the given portal; edit is a full page, not a modal. */
  onEdit: (portal: ManagedPortal) => void;
};

/**
 * Short relative-time formatter local to this feature so the package stays
 * dependency-free. Picks the coarsest unit that fits a table cell ("3h ago",
 * "5d ago", "2mo ago"). Clock skew that puts the stamp in the future collapses
 * to "just now" rather than the misleading "3h ago".
 */
function shortRelative(iso: string | null | undefined): string {
  if (!iso) return '';
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return '';
  const seconds = Math.round((Date.now() - then) / 1000);
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
  if (seconds < 2592000) return `${Math.round(seconds / 86400)}d ago`;
  if (seconds < 31536000) return `${Math.round(seconds / 2592000)}mo ago`;
  return `${Math.round(seconds / 31536000)}y ago`;
}

export default function ManagedPortalsList({ onCreate, onEdit }: ManagedPortalsListProps) {
  const { portals, isLoading, error, remove } = useManagedPortalList();

  const [deleteTarget, setDeleteTarget] = useState<ManagedPortal | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [searchQuery, setSearchQuery] = useState('');

  const filteredPortals = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return portals;
    return portals.filter(
      (portal) =>
        portal.name.toLowerCase().includes(q) ||
        portal.handle.toLowerCase().includes(q) ||
        (portal.description?.toLowerCase().includes(q) ?? false) ||
        (portal.url?.toLowerCase().includes(q) ?? false),
    );
  }, [portals, searchQuery]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      setDeleteTarget(null);
    } catch {
      // Hook already notified.
    } finally {
      setDeleting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'nowrap', gap: 2 }}>
            <PageTitle sx={{ minWidth: 0, flex: 1 }}>
              <PageTitle.Header>Portals</PageTitle.Header>
              <PageTitle.SubHeader>Manage the portals for this organization.</PageTitle.SubHeader>
            </PageTitle>

            <Stack direction="row" spacing={1.5} sx={{ ml: 'auto', flexShrink: 0 }}>
              {portals.length > 0 ? (
                <Button variant="contained" onClick={onCreate} startIcon={<Plus size={20} />}>
                  Add Portal
                </Button>
              ) : null}
            </Stack>
          </Box>
        </Grid>

        {isLoading ? (
          <Grid size={{ xs: 12 }}>
            <Typography variant="body2" color="text.secondary">
              Loading portals…
            </Typography>
          </Grid>
        ) : error ? (
          <Grid size={{ xs: 12 }}>
            <Typography variant="body2" color="error">
              {error.message}
            </Typography>
          </Grid>
        ) : portals.length === 0 ? (
          <Grid size={{ xs: 12 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 8 }}>
              <Stack spacing={2} alignItems="center" justifyContent="center" sx={{ textAlign: 'center', maxWidth: 480 }}>
                <PanelTop size={64} color="var(--mui-palette-action-disabled)" />
                <Button variant="contained" onClick={onCreate} startIcon={<Plus size={20} />}>
                  Add Portal
                </Button>
              </Stack>
            </Box>
          </Grid>
        ) : (
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                placeholder="Search portals..."
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
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
            </Grid>

            <Grid size={{ xs: 12 }}>
              <Card>
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Name</TableCell>
                        <TableCell>Description</TableCell>
                        <TableCell>Login environment</TableCell>
                        <TableCell>Updated</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredPortals.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={5}>
                            <Typography variant="body2" color="text.secondary">
                              No portals match your search.
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        filteredPortals.map((portal) => (
                          <TableRow key={portal.id} hover>
                            <TableCell sx={{ minWidth: 220 }}>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Avatar
                                  sx={{
                                    width: 36,
                                    height: 36,
                                    backgroundColor: 'primary.light',
                                    color: 'primary.contrastText',
                                    fontSize: 16,
                                  }}
                                >
                                  {portal.name.trim().slice(0, 2).toUpperCase()}
                                </Avatar>
                                <Stack spacing={0.25}>
                                  <Typography variant="h6" sx={{ fontWeight: 600 }}>
                                    {portal.name}
                                  </Typography>
                                  <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                                    {portal.handle}
                                  </Typography>
                                </Stack>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}
                              >
                                {portal.description || '-'}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              {portal.loginEnvironment ? (
                                <Chip label={portal.loginEnvironment} size="small" variant="outlined" />
                              ) : (
                                <Typography variant="body2" color="text.secondary">
                                  -
                                </Typography>
                              )}
                            </TableCell>
                            <TableCell>
                              <Typography variant="body2" color="text.secondary">
                                {shortRelative(portal.updatedAt) || '-'}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              <IconButton
                                size="small"
                                aria-label={`Edit ${portal.name}`}
                                onClick={() => onEdit(portal)}
                              >
                                <Pencil size={16} />
                              </IconButton>
                              {portal.url && (
                                <IconButton
                                  size="small"
                                  aria-label={`Visit ${portal.name}`}
                                  component="a"
                                  href={portal.url}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                >
                                  <ExternalLink size={16} />
                                </IconButton>
                              )}
                              <IconButton
                                size="small"
                                color="error"
                                aria-label={`Delete ${portal.name}`}
                                onClick={() => setDeleteTarget(portal)}
                              >
                                <Trash2 size={16} />
                              </IconButton>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Card>
            </Grid>
          </>
        )}
      </Grid>

      <Dialog open={Boolean(deleteTarget)} onClose={deleting ? undefined : () => setDeleteTarget(null)}>
        <DialogTitle>Delete Portal</DialogTitle>
        <DialogContent>
          <DialogContentText>Are you sure you want to delete {deleteTarget?.name}?</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} variant="outlined" color="secondary" disabled={deleting}>
            Cancel
          </Button>
          <Button
            color="error"
            onClick={handleDeleteConfirm}
            disabled={deleting}
            startIcon={deleting ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {deleting ? 'Deleting…' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
