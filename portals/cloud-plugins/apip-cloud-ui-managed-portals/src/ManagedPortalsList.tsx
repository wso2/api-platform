/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useState } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  PageContent,
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
import { Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortalList } from './hooks';
import type { ManagedPortal } from './types';

export type ManagedPortalsListProps = {
  /**
   * Called when a row's non-action area is clicked, so the page shell can
   * open the detail view. The delete icon stops event propagation so it does
   * not double-fire into onSelect.
   */
  onSelect: (id: string) => void;
};

export default function ManagedPortalsList({ onSelect }: ManagedPortalsListProps) {
  const { portals, isLoading, error, create, remove } = useManagedPortalList();

  // Create dialog state.
  //
  // No loginEnvironment field in the Create form: the backend picks the org's
  // preferred login env from environments.Service.List (see the plugin's
  // Service.Create). Rationale — the "which env authenticates consumers" pick
  // is server authority (matches the operator's org bootstrap), not the
  // portal creator's choice on first-create. Editing switches later via the
  // Edit form's env dropdown.
  const [createOpen, setCreateOpen] = useState(false);
  const [handle, setHandle] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Delete confirmation state.
  const [deleteTarget, setDeleteTarget] = useState<ManagedPortal | null>(null);

  const resetCreateForm = () => {
    setHandle('');
    setName('');
    setDescription('');
  };

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      await create({
        handle: handle.trim(),
        name: name.trim(),
        description: description.trim() || undefined,
        // loginEnvironment omitted — backend picks from environments.Service.List
      });
      resetCreateForm();
      setCreateOpen(false);
    } catch {
      // Notification already handled inside the hook; keep dialog open so the
      // caller can fix and retry without losing typed values.
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await remove(deleteTarget.id);
    } catch {
      // notified by hook
    } finally {
      setDeleteTarget(null);
    }
  };

  return (
    <PageContent fullWidth>
      <Stack spacing={3}>
        <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2 }}>
          <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="h5">Managed API Portals</Typography>
            <Typography color="text.secondary">
              WSO2-managed developer portals for your organization.
            </Typography>
          </Stack>
          <Button
            variant="contained"
            startIcon={<Plus size={20} />}
            onClick={() => setCreateOpen(true)}
            sx={{ flexShrink: 0 }}
          >
            Create Portal
          </Button>
        </Box>

        <Divider />

        {isLoading ? (
          <Typography variant="body2" color="text.secondary">
            Loading portals…
          </Typography>
        ) : error ? (
          <Typography variant="body2" color="error">
            {error.message}
          </Typography>
        ) : portals.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No managed portals yet.
          </Typography>
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Portal</TableCell>
                  <TableCell>URL</TableCell>
                  <TableCell>Login environment</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {portals.map((portal) => (
                  <TableRow
                    key={portal.id}
                    hover
                    onClick={() => onSelect(portal.id)}
                    sx={{ cursor: 'pointer' }}
                  >
                    <TableCell sx={{ minWidth: 240 }}>
                      <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                        {portal.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                        {portal.handle}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                        {portal.url ?? '—'}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {portal.loginEnvironment ?? '—'}
                      </Typography>
                    </TableCell>
                    <TableCell align="right">
                      <IconButton
                        size="small"
                        color="error"
                        aria-label={`Delete ${portal.name}`}
                        onClick={(event) => {
                          // Prevent the row click from also firing onSelect —
                          // deleting is an explicit action, not a navigation.
                          event.stopPropagation();
                          setDeleteTarget(portal);
                        }}
                      >
                        <Trash2 size={16} />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Stack>

      {/* Create portal */}
      <Dialog
        open={createOpen}
        onClose={() => (submitting ? undefined : setCreateOpen(false))}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Create Portal</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>Handle</FormLabel>
              <TextField
                fullWidth
                autoFocus
                placeholder="e.g. acme-portal"
                value={handle}
                onChange={(event) => setHandle(event.target.value)}
                disabled={submitting}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>Name</FormLabel>
              <TextField
                fullWidth
                placeholder="e.g. Acme Developer Portal"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
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
            {/*
             * No Login-environment field on Create. The backend picks the
             * org's preferred login env from the environments list (see the
             * plugin's Service.Create). The Edit form (ManagedPortalDetail)
             * exposes the picker for switching later.
             */}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setCreateOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={submitting || !handle.trim() || !name.trim()}
            onClick={handleCreate}
          >
            Create
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete portal */}
      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>Delete Portal</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete <strong>{deleteTarget?.name}</strong>? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setDeleteTarget(null)}>
            Cancel
          </Button>
          <Button color="error" onClick={handleDeleteConfirm}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
