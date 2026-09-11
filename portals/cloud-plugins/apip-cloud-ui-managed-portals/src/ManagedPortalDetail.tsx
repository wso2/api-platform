/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useState } from 'react';
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
  MenuItem,
  PageContent,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowLeft, ArrowUpRight, Pencil, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { useManagedPortal, useOrgEnvironments } from './hooks';

export type ManagedPortalDetailProps = {
  id: string;
  /** Invoked on back button or after a successful delete; the page shell owns list/detail switching. */
  onBack: () => void;
};

/** Labelled read-only field for the detail summary. */
function Field({ label, value }: { label: string; value: string }) {
  return (
    <Stack spacing={0.5}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: 'uppercase', letterSpacing: 0.6 }}>
        {label}
      </Typography>
      <Typography variant="body2" sx={{ fontFamily: value.startsWith('http') || /^[a-z0-9-]+$/.test(value) ? 'monospace' : undefined }}>
        {value || '—'}
      </Typography>
    </Stack>
  );
}

export default function ManagedPortalDetail({ id, onBack }: ManagedPortalDetailProps) {
  const { portal, isLoading, error, update, remove } = useManagedPortal(id);
  // Populates the login-environment picker with the org's real envs so the operator can't type an env that fails at provision-time.
  const { environments, isLoading: envsLoading, error: envsError } = useOrgEnvironments();

  const [editOpen, setEditOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loginEnvironment, setLoginEnvironment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  // Reseed on each open so Cancel-then-reopen reflects the latest server-side values.
  useEffect(() => {
    if (editOpen && portal) {
      setName(portal.name);
      setDescription(portal.description ?? '');
      setLoginEnvironment(portal.loginEnvironment ?? '');
    }
  }, [editOpen, portal]);

  const handleSave = async () => {
    if (!portal) return;
    setSubmitting(true);
    try {
      // Send only changed fields so a no-op save doesn't restamp values and untouched fields aren't cleared.
      const patch = {
        ...(name.trim() !== portal.name ? { name: name.trim() } : {}),
        ...(description !== (portal.description ?? '') ? { description: description.trim() } : {}),
        ...(loginEnvironment !== (portal.loginEnvironment ?? '')
          ? { loginEnvironment: loginEnvironment.trim() }
          : {}),
      };
      if (Object.keys(patch).length === 0) {
        setEditOpen(false);
        return;
      }
      await update(patch);
      setEditOpen(false);
    } catch {
      // Hook already notified; leave the dialog open with user input for retry.
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteConfirm = async () => {
    try {
      await remove();
      setDeleteOpen(false);
      onBack();
    } catch {
      setDeleteOpen(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Stack spacing={3}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <IconButton onClick={onBack} aria-label="Back to portals list" size="small">
            <ArrowLeft size={20} />
          </IconButton>
          <Typography variant="body2" color="text.secondary">
            Managed API Portals
          </Typography>
        </Stack>

        {isLoading ? (
          <Typography variant="body2" color="text.secondary">
            Loading portal…
          </Typography>
        ) : error ? (
          <Typography variant="body2" color="error">
            {error.message}
          </Typography>
        ) : !portal ? (
          <Typography variant="body2" color="text.secondary">
            Portal not found.
          </Typography>
        ) : (
          <>
            <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2 }}>
              <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
                <Typography variant="h5">{portal.name}</Typography>
                <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                  {portal.handle}
                </Typography>
              </Stack>
              <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
                {portal.url && (
                  <Button
                    variant="contained"
                    startIcon={<ArrowUpRight size={18} />}
                    // New tab preserves the console session; noreferrer for external navigation.
                    href={portal.url}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Visit Portal
                  </Button>
                )}
                <Button
                  variant="outlined"
                  startIcon={<Pencil size={18} />}
                  onClick={() => setEditOpen(true)}
                >
                  Edit
                </Button>
                <Button
                  variant="outlined"
                  color="error"
                  startIcon={<Trash2 size={18} />}
                  onClick={() => setDeleteOpen(true)}
                >
                  Delete
                </Button>
              </Stack>
            </Box>

            <Divider />

            <Stack spacing={2.5} sx={{ maxWidth: 640 }}>
              <Field label="Handle" value={portal.handle} />
              <Field label="Description" value={portal.description ?? ''} />
              <Field label="URL" value={portal.url ?? ''} />
              <Field label="Login environment" value={portal.loginEnvironment ?? ''} />
              {portal.updatedAt && <Field label="Last updated" value={portal.updatedAt} />}
            </Stack>
          </>
        )}
      </Stack>

      {/* Edit portal */}
      <Dialog
        open={editOpen}
        onClose={() => (submitting ? undefined : setEditOpen(false))}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Edit Portal</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>Handle</FormLabel>
              <TextField fullWidth value={portal?.handle ?? ''} disabled />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
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
            <FormControl fullWidth>
              <FormLabel>Login environment</FormLabel>
              {/* Current env is added as a synthetic option if missing from the list, so an out-of-band deletion shows as a mismatch rather than a silent swap. */}
              <Select
                fullWidth
                value={loginEnvironment}
                onChange={(event) => setLoginEnvironment(String(event.target.value))}
                disabled={submitting || envsLoading}
                displayEmpty
              >
                {loginEnvironment && !environments.some((e) => e.name === loginEnvironment) && (
                  <MenuItem value={loginEnvironment}>
                    {loginEnvironment} (not in current environment list)
                  </MenuItem>
                )}
                {environments.map((env) => (
                  <MenuItem key={env.name} value={env.name}>
                    {env.displayName ? `${env.displayName} (${env.name})` : env.name}
                  </MenuItem>
                ))}
                {environments.length === 0 && !envsLoading && (
                  <MenuItem value="" disabled>
                    No environments — provision one first
                  </MenuItem>
                )}
              </Select>
              {envsError && (
                <Typography variant="caption" color="error" sx={{ mt: 0.5 }}>
                  Failed to load environments: {envsError.message}
                </Typography>
              )}
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setEditOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={submitting || !name.trim()}
            onClick={handleSave}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete portal */}
      <Dialog open={deleteOpen} onClose={() => setDeleteOpen(false)}>
        <DialogTitle>Delete Portal</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete <strong>{portal?.name}</strong>? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setDeleteOpen(false)}>
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
