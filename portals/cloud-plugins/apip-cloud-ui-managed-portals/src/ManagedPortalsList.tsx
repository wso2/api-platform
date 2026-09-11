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
  /** Invoked when a row's non-action area is clicked; the delete icon stops propagation to avoid double-firing. */
  onSelect: (id: string) => void;
};

export default function ManagedPortalsList({ onSelect }: ManagedPortalsListProps) {
  const { portals, isLoading, error, create, remove } = useManagedPortalList();

  // No loginEnvironment on create: server is authoritative on org bootstrap; Edit exposes the switch later.
  const [createOpen, setCreateOpen] = useState(false);
  const [handle, setHandle] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);

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
    if (!deleteTarget) return;
    try {
      await remove(deleteTarget.id);
    } catch {
      // Hook already notified.
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
                          // Stop the row's onSelect from firing on delete.
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
