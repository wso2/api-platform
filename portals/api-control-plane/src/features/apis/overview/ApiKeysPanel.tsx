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

import { useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  IconButton,
  ListingTable,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Trash2 } from '@wso2/oxygen-ui-icons-react';

import {
  useApiKeys,
  useCreateApiKey,
  useRevokeApiKey,
} from '../../../api/hooks/useMvpQueries';
import { useNotifications } from '../../../components/Notifications';
import type { Api } from '../../../types/domain';

const formatDate = (value?: string): string => {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleDateString();
};

/**
 * API Keys section of the Overview tab (ai-workspace layout). platform-api's
 * model is key injection: the caller supplies the key value once, the server
 * stores a hash and broadcasts it to the gateways the API is deployed on.
 */
export function ApiKeysPanel({ api }: { api: Api }) {
  const { notify } = useNotifications();
  const keysQuery = useApiKeys(api);
  const createMutation = useCreateApiKey();
  const revokeMutation = useRevokeApiKey();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [displayName, setDisplayName] = useState('');
  const [keyValue, setKeyValue] = useState('');
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null);

  const keys = keysQuery.data || [];

  const closeDialog = () => {
    if (createMutation.isPending) return;
    setDialogOpen(false);
    setDisplayName('');
    setKeyValue('');
  };

  const submit = () => {
    createMutation.mutate(
      {
        api,
        input: { displayName: displayName.trim(), apiKey: keyValue.trim() },
      },
      {
        onSuccess: () => {
          notify(`API key "${displayName.trim()}" added.`, 'success');
          closeDialog();
        },
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Failed to add API key',
            'error'
          ),
      }
    );
  };

  const revoke = () => {
    if (!revokeTarget) return;
    revokeMutation.mutate(
      { api, keyName: revokeTarget },
      {
        onSuccess: () => {
          notify(`API key "${revokeTarget}" revoked.`, 'success');
          setRevokeTarget(null);
        },
        onError: (error) => {
          notify(
            error instanceof Error ? error.message : 'Failed to revoke key',
            'error'
          );
          setRevokeTarget(null);
        },
      }
    );
  };

  const canSubmit =
    Boolean(displayName.trim() && keyValue.trim()) && !createMutation.isPending;

  return (
    <Box>
      <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="h6">
        API Keys
      </Typography>
      <Stack spacing={2}>
        <Stack
          alignItems={{ sm: 'center', xs: 'flex-start' }}
          direction={{ sm: 'row', xs: 'column' }}
          spacing={2}
          sx={{
            bgcolor: 'background.paper',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            p: 2,
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Typography color="text.secondary" variant="body2">
              Add an API key to authenticate requests through the deployed
              gateways.
            </Typography>
          </Box>
          <Button
            onClick={() => setDialogOpen(true)}
            size="medium"
            variant="contained"
          >
            Add API Key
          </Button>
        </Stack>

        {keysQuery.isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
            <CircularProgress />
          </Box>
        ) : keys.length > 0 ? (
          <ListingTable.Container>
            <ListingTable>
              <ListingTable.Head>
                <ListingTable.Row>
                  <ListingTable.Cell>Name</ListingTable.Cell>
                  <ListingTable.Cell>API Key</ListingTable.Cell>
                  <ListingTable.Cell>Expires At</ListingTable.Cell>
                  <ListingTable.Cell align="right">Actions</ListingTable.Cell>
                </ListingTable.Row>
              </ListingTable.Head>
              <ListingTable.Body>
                {keys.map((key) => (
                  <ListingTable.Row key={key.name}>
                    <ListingTable.Cell>{key.name || '-'}</ListingTable.Cell>
                    <ListingTable.Cell>
                      {key.maskedApiKey || '-'}
                    </ListingTable.Cell>
                    <ListingTable.Cell>
                      <Tooltip
                        title={
                          key.expiresAt
                            ? new Date(key.expiresAt).toUTCString()
                            : ''
                        }
                      >
                        <span>{formatDate(key.expiresAt)}</span>
                      </Tooltip>
                    </ListingTable.Cell>
                    <ListingTable.Cell align="right">
                      <Tooltip title="Revoke API key">
                        <span>
                          <IconButton
                            color="error"
                            disabled={revokeMutation.isPending}
                            onClick={() => setRevokeTarget(key.name)}
                            size="small"
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </ListingTable.Cell>
                  </ListingTable.Row>
                ))}
              </ListingTable.Body>
            </ListingTable>
          </ListingTable.Container>
        ) : null}
      </Stack>

      {/* Add key dialog */}
      <Dialog fullWidth maxWidth="sm" onClose={closeDialog} open={dialogOpen}>
        <DialogTitle>Add API Key</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>Key Name</FormLabel>
              <TextField
                autoFocus
                fullWidth
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="Ex: Production Key"
                size="small"
                value={displayName}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>API Key Value</FormLabel>
              <TextField
                fullWidth
                helperText="Stored hashed and pushed to deployed gateways; you cannot read it back later."
                onChange={(event) => setKeyValue(event.target.value)}
                size="small"
                value={keyValue}
              />
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            color="secondary"
            disabled={createMutation.isPending}
            onClick={closeDialog}
            size="small"
            variant="outlined"
          >
            Cancel
          </Button>
          <Button
            disabled={!canSubmit}
            onClick={submit}
            size="small"
            variant="contained"
          >
            {createMutation.isPending ? (
              <>
                <CircularProgress size={16} sx={{ mr: 1 }} />
                Adding...
              </>
            ) : (
              'Add'
            )}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Revoke confirmation */}
      <Dialog
        fullWidth
        maxWidth="xs"
        onClose={() => !revokeMutation.isPending && setRevokeTarget(null)}
        open={Boolean(revokeTarget)}
      >
        <DialogTitle>Revoke API Key</DialogTitle>
        <DialogContent>
          <Typography color="text.secondary" variant="body2">
            Are you sure you want to revoke this API key? Requests using it will
            be rejected by the gateways.
          </Typography>
          <Typography sx={{ fontWeight: 600, mt: 1 }} variant="body2">
            {revokeTarget}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            color="secondary"
            disabled={revokeMutation.isPending}
            onClick={() => setRevokeTarget(null)}
            size="small"
            variant="outlined"
          >
            Cancel
          </Button>
          <Button
            color="error"
            disabled={revokeMutation.isPending}
            onClick={revoke}
            size="small"
            variant="contained"
          >
            {revokeMutation.isPending ? (
              <>
                <CircularProgress size={16} sx={{ mr: 1 }} />
                Revoking...
              </>
            ) : (
              'Revoke'
            )}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
