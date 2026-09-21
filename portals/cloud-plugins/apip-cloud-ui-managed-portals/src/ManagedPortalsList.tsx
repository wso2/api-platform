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
  DialogTitle,
  FormControl,
  FormLabel,
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
import { ExternalLink, Globe, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortalList } from './hooks';
import type { ManagedPortal } from './types';
import { portalHandleFromName, validatePortalName } from './utils/name';

export type ManagedPortalsListProps = {
  /** Invoked when a row's non-action area is clicked; the delete icon stops propagation to avoid double-firing. */
  onSelect: (id: string) => void;
};

export default function ManagedPortalsList({ onSelect }: ManagedPortalsListProps) {
  const { portals, isLoading, error, create, remove } = useManagedPortalList();

  // No loginEnvironment on create: server is authoritative on org bootstrap; Edit exposes the switch later.
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  // The handle is derived from the display name (same conversion the backend
  // applies) rather than typed by the user, so the two-field UX collapses to
  // one input with a live preview in the helper text - mirrors the gateway
  // create form. The handle is immutable after create, so we show what the
  // name will become before the user commits.
  const nameError = useMemo(() => validatePortalName(name), [name]);
  const derivedHandle = useMemo(() => portalHandleFromName(name), [name]);
  const [submitting, setSubmitting] = useState(false);

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

  const resetCreateForm = () => {
    setName('');
    setDescription('');
  };

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      await create({
        handle: derivedHandle,
        name: name.trim(),
        description: description.trim() || undefined,
        // loginEnvironment omitted; server picks the org's preferred env.
      });
      resetCreateForm();
      setCreateOpen(false);
    } catch {
      // Hook already notified; leave the dialog open with user input for retry.
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteConfirm = async () => {
    // Mirror gateways: keep the dialog open with a busy button until the delete settles.
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
              <PageTitle.Header>API Portals</PageTitle.Header>
              <PageTitle.SubHeader>Manage and monitor your API portals.</PageTitle.SubHeader>
            </PageTitle>

            <Stack direction="row" spacing={1.5} sx={{ ml: 'auto', flexShrink: 0 }}>
              {portals.length > 0 ? (
                <Button variant="contained" onClick={() => setCreateOpen(true)} startIcon={<Plus size={20} />}>
                  Create Portal
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
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6 }}>
              <Stack spacing={1.5} alignItems="center" justifyContent="center" sx={{ textAlign: 'center' }}>
                <Globe size={120} color="var(--mui-palette-action-disabled)" />
                <Typography variant="body1" color="text.secondary">
                  No API portals yet
                </Typography>
                <Button variant="contained" onClick={() => setCreateOpen(true)} startIcon={<Plus size={20} />}>
                  Create Portal
                </Button>
              </Stack>
            </Box>
          </Grid>
        ) : (
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                placeholder="Search API portals..."
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
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredPortals.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={4}>
                            <Typography variant="body2" color="text.secondary">
                              No portals found.
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        filteredPortals.map((portal) => (
                          <TableRow
                            key={portal.id}
                            hover
                            tabIndex={0}
                            role="button"
                            aria-label={`Open ${portal.name}`}
                            onClick={() => onSelect(portal.id)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' || event.key === ' ') {
                                event.preventDefault();
                                onSelect(portal.id);
                              }
                            }}
                            sx={{ cursor: 'pointer' }}
                          >
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
                                {portal.description || '—'}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              {portal.loginEnvironment ? (
                                <Chip label={portal.loginEnvironment} size="small" variant="outlined" />
                              ) : (
                                <Typography variant="body2" color="text.secondary">
                                  —
                                </Typography>
                              )}
                            </TableCell>
                            <TableCell
                              align="right"
                              // The row's onKeyDown reacts to Enter/Space and would fire on the
                              // Visit/Delete buttons too, opening the detail view on top of the
                              // button's own action. Neutralize keydown for the whole action cell.
                              onKeyDown={(event) => event.stopPropagation()}
                            >
                              {portal.url && (
                                <IconButton
                                  size="small"
                                  aria-label={`Visit ${portal.name}`}
                                  component="a"
                                  href={portal.url}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  onClick={(event) => event.stopPropagation()}
                                >
                                  <ExternalLink size={16} />
                                </IconButton>
                              )}
                              <IconButton
                                size="small"
                                color="error"
                                aria-label={`Delete ${portal.name}`}
                                onClick={(event) => {
                                  // Stop the row's onSelect from firing on delete.
                                  event.stopPropagation();
                                  setDeleteTarget(portal);
                                }}
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

      {/* Create portal */}
      <Dialog
        open={createOpen}
        onClose={() => {
          if (submitting) return;
          resetCreateForm();
          setCreateOpen(false);
        }}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Create Portal</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth error={Boolean(nameError)}>
              <FormLabel>Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
                placeholder="e.g. Acme Developer Portal"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                error={Boolean(nameError)}
                helperText={
                  nameError
                  ?? (derivedHandle
                    ? `Handle: ${derivedHandle}`
                    : 'The portal handle is derived from this name and cannot be changed later.')
                }
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>Description</FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={2}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                disabled={submitting}
              />
            </FormControl>
            {/* No Login-environment field on Create; the server picks the org's preferred env, and Edit exposes the picker later. */}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => {
              resetCreateForm();
              setCreateOpen(false);
            }}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={submitting || !name.trim() || Boolean(nameError) || !derivedHandle}
            onClick={handleCreate}
          >
            Create
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete portal */}
      <Dialog open={Boolean(deleteTarget)} onClose={deleting ? undefined : () => setDeleteTarget(null)}>
        <DialogTitle>Delete Portal</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete <strong>{deleteTarget?.name}</strong>? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setDeleteTarget(null)} disabled={deleting}>
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
