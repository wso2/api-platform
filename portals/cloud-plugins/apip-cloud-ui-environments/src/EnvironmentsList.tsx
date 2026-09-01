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
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
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
import { Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { useEnvironmentList } from './hooks';
import type { CloudEnvironment } from './types';

export default function EnvironmentsList() {
  const { environments, isLoading, error, create, remove } = useEnvironmentList();

  const [searchQuery, setSearchQuery] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [displayName, setDisplayName] = useState('');
  const [isProduction, setIsProduction] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CloudEnvironment | null>(null);

  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return environments;
    return environments.filter((e) =>
      [e.displayName, e.name].filter(Boolean).join(' ').toLowerCase().includes(q)
    );
  }, [environments, searchQuery]);

  const resetCreateForm = () => {
    setDisplayName('');
    setIsProduction(false);
  };

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      await create({ displayName, isProduction });
      setCreateOpen(false);
      resetCreateForm();
    } catch {
      // error surfaced via host.notify in the hook
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await remove(deleteTarget.id);
    } catch {
      // error surfaced via host.notify in the hook
    } finally {
      setDeleteTarget(null);
    }
  };

  return (
    <Stack spacing={2}>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2 }}>
        <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="h5">Environments</Typography>
          <Typography color="text.secondary">
            Manage deployment environments. Gateways are assigned to an environment when
            they are created.
          </Typography>
        </Stack>
        <Button
          variant="contained"
          startIcon={<Plus size={20} />}
          onClick={() => setCreateOpen(true)}
          sx={{ flexShrink: 0 }}
        >
          Create Environment
        </Button>
      </Box>

      <TextField
        fullWidth
        placeholder="Search environments..."
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

      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Type</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={3}>
                  <Typography variant="body2" color="text.secondary">
                    Loading environments…
                  </Typography>
                </TableCell>
              </TableRow>
            ) : error ? (
              <TableRow>
                <TableCell colSpan={3}>
                  <Typography variant="body2" color="error">
                    {error.message}
                  </Typography>
                </TableCell>
              </TableRow>
            ) : filtered.length === 0 ? (
              <TableRow>
                <TableCell colSpan={3}>
                  <Typography variant="body2" color="text.secondary">
                    No environments found.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              filtered.map((env) => (
                <TableRow key={env.id} hover>
                  <TableCell sx={{ minWidth: 220 }}>
                    <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                      {env.displayName}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {env.name}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      variant="outlined"
                      color={env.isProduction ? 'warning' : 'default'}
                      label={env.isProduction ? 'Production' : 'Non-production'}
                    />
                  </TableCell>
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={`Delete ${env.displayName}`}
                      onClick={() => setDeleteTarget(env)}
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

      {/* Create environment */}
      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Create Environment</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>Display Name</FormLabel>
              <TextField
                fullWidth
                autoFocus
                placeholder="e.g. Staging"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>Type</FormLabel>
              <Select
                value={isProduction ? 'production' : 'non-production'}
                onChange={(event) => setIsProduction(event.target.value === 'production')}
              >
                <MenuItem value="non-production">Non-production</MenuItem>
                <MenuItem value="production">Production</MenuItem>
              </Select>
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setCreateOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={submitting || !displayName.trim()}
            onClick={handleCreate}
          >
            Create
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete environment */}
      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>Delete Environment</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete <strong>{deleteTarget?.displayName}</strong>? Any
            gateway bindings to this environment will be removed.
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
    </Stack>
  );
}
