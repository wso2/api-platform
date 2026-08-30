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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useCreateApiKey, useMyApiKeys, useRevokeApiKey } from '@/api/resources/apiKeys';
import { useNotifications } from '@/components/Notifications';

const messages = defineMessages({
  add: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.add',
    defaultMessage: 'Add',
    description: 'Commits the new API key in the add dialog.',
  },
  addButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.addButton',
    defaultMessage: 'Add API Key',
    description: 'Opens the dialog for issuing a new API key.',
  },
  addDialogTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.addDialogTitle',
    defaultMessage: 'Add API Key',
    description: 'Title of the dialog for issuing a new API key.',
  },
  addFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.addFailed',
    defaultMessage: 'Failed to add API key',
    description: 'Fallback toast when the server gives no reason for the failure.',
  },
  adding: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.adding',
    defaultMessage: 'Adding...',
    description: 'Label of the add button while the key is being issued.',
  },
  addSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.addSucceeded',
    defaultMessage: 'API key "{name}" added.',
    description: 'Toast confirming a new key; {name} is the name the user typed.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.cancel',
    defaultMessage: 'Cancel',
    description: 'Closes a dialog without applying it.',
  },
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.column.actions',
    defaultMessage: 'Actions',
    description: 'Header of the column holding the per-key buttons.',
  },
  columnExpiresAt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.column.expiresAt',
    defaultMessage: 'Expires At',
    description: "Header of the column holding each key's expiry date.",
  },
  columnKey: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.column.key',
    defaultMessage: 'API Key',
    description: 'Header of the column holding the masked key value.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.column.name',
    defaultMessage: 'Name',
    description: "Header of the column holding each key's name. A noun, not a command.",
  },
  description: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.description',
    defaultMessage: 'Add an API key to authenticate requests through the deployed gateways.',
    description: 'Explains what an API key is for, above the button that adds one.',
  },
  keyNameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.keyNameLabel',
    defaultMessage: 'Key Name',
    description: 'Label of the field naming the new key.',
  },
  keyNamePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.keyNamePlaceholder',
    defaultMessage: 'Ex: Production Key',
    description: 'Example name shown in the empty key-name field.',
  },
  keyValueHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.keyValueHelper',
    defaultMessage: 'Stored hashed and pushed to deployed gateways; you cannot read it back later.',
    description: 'Warns that the key value is write-only once submitted.',
  },
  keyValueLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.keyValueLabel',
    defaultMessage: 'API Key Value',
    description: 'Label of the field holding the secret the gateways will accept.',
  },
  revoke: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revoke',
    defaultMessage: 'Revoke',
    description: 'Confirms the irreversible revocation of a key.',
  },
  revokeDialogMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revokeDialogMessage',
    defaultMessage:
      'Are you sure you want to revoke this API key? Requests using it will be rejected by the gateways.',
    description: 'Body of the revoke confirmation dialog.',
  },
  revokeDialogTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revokeDialogTitle',
    defaultMessage: 'Revoke API Key',
    description: 'Title of the revoke confirmation dialog.',
  },
  revokeFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revokeFailed',
    defaultMessage: 'Failed to revoke key',
    description: 'Fallback toast when the server gives no reason for the failure.',
  },
  revoking: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revoking',
    defaultMessage: 'Revoking...',
    description: 'Label of the revoke button while the request is in flight.',
  },
  revokeSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revokeSucceeded',
    defaultMessage: 'API key "{name}" revoked.',
    description: 'Toast confirming a revocation; {name} is the key that was revoked.',
  },
  revokeTooltip: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.revokeTooltip',
    defaultMessage: 'Revoke API key',
    description: 'Tooltip on the button that revokes one key from the table.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.title',
    defaultMessage: 'API Keys',
    description: 'Heading of the section listing the keys accepted for this API.',
  },
});

/** Stands in for a value the server did not send. Locale-independent, and one
 * definition so the table and the date formatter can't drift apart. */
const EMPTY_VALUE = '-';

const formatDate = (value?: string): string => {
  if (!value) return EMPTY_VALUE;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? EMPTY_VALUE : date.toLocaleDateString();
};

/** The key the revoke dialog is armed for: its id addresses the request, its
 * name is what the dialog and the toast show. */
type RevokeTarget = { id: string; displayName: string };

/**
 * API Keys section of the Overview tab (ai-workspace layout). platform-api's
 * model is key injection: the caller supplies the key value once, the server
 * stores a hash and broadcasts it to the gateways the API is deployed on.
 */
export function ApiKeysPanel({ restApiId }: { restApiId: string }) {
  const intl = useIntl();
  const { notify } = useNotifications();
  // The spec has no per-API key listing — the only read is the caller's own
  // inventory across artifacts, so this narrows to REST API keys server-side
  // and to this API here. It therefore shows the signed-in user's keys only.
  const keysQuery = useMyApiKeys({ type: ['RestApi'] });
  const createMutation = useCreateApiKey();
  const revokeMutation = useRevokeApiKey();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [displayName, setDisplayName] = useState('');
  const [keyValue, setKeyValue] = useState('');
  const [revokeTarget, setRevokeTarget] = useState<RevokeTarget | null>(null);

  const keys = useMemo(
    () => (keysQuery.data?.list ?? []).filter((key) => key.artifactId === restApiId),
    [keysQuery.data, restApiId],
  );

  const closeDialog = () => {
    if (createMutation.isPending) return;
    setDialogOpen(false);
    setDisplayName('');
    setKeyValue('');
  };

  const submit = () => {
    const name = displayName.trim();
    createMutation.mutate(
      { restApiId, body: { displayName: name, apiKey: keyValue.trim() } },
      {
        onSuccess: () => {
          notify(intl.formatMessage(messages.addSucceeded, { name }), 'success');
          closeDialog();
        },
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.addFailed), 'error'),
      },
    );
  };

  const revoke = () => {
    if (!revokeTarget) return;
    const { displayName: name } = revokeTarget;
    revokeMutation.mutate(
      { restApiId, apiKeyId: revokeTarget.id },
      {
        onSuccess: () => {
          notify(intl.formatMessage(messages.revokeSucceeded, { name }), 'success');
          setRevokeTarget(null);
        },
        onError: (error) => {
          notify(error.message || intl.formatMessage(messages.revokeFailed), 'error');
          setRevokeTarget(null);
        },
      },
    );
  };

  const canSubmit = Boolean(displayName.trim() && keyValue.trim()) && !createMutation.isPending;

  return (
    <Box>
      <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="h6">
        <FormattedMessage {...messages.title} />
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
              <FormattedMessage {...messages.description} />
            </Typography>
          </Box>
          <Button onClick={() => setDialogOpen(true)} size="medium" variant="contained">
            <FormattedMessage {...messages.addButton} />
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
                  <ListingTable.Cell>
                    <FormattedMessage {...messages.columnName} />
                  </ListingTable.Cell>
                  <ListingTable.Cell>
                    <FormattedMessage {...messages.columnKey} />
                  </ListingTable.Cell>
                  <ListingTable.Cell>
                    <FormattedMessage {...messages.columnExpiresAt} />
                  </ListingTable.Cell>
                  <ListingTable.Cell align="right">
                    <FormattedMessage {...messages.columnActions} />
                  </ListingTable.Cell>
                </ListingTable.Row>
              </ListingTable.Head>
              <ListingTable.Body>
                {keys.map((key) => (
                  <ListingTable.Row key={key.id ?? key.displayName}>
                    <ListingTable.Cell>{key.displayName || EMPTY_VALUE}</ListingTable.Cell>
                    <ListingTable.Cell>{key.maskedApiKey || EMPTY_VALUE}</ListingTable.Cell>
                    <ListingTable.Cell>
                      <Tooltip title={key.expiresAt ? new Date(key.expiresAt).toUTCString() : ''}>
                        <span>{formatDate(key.expiresAt)}</span>
                      </Tooltip>
                    </ListingTable.Cell>
                    <ListingTable.Cell align="right">
                      <Tooltip title={intl.formatMessage(messages.revokeTooltip)}>
                        <span>
                          <IconButton
                            color="error"
                            disabled={revokeMutation.isPending || !key.id}
                            onClick={() =>
                              key.id &&
                              setRevokeTarget({
                                id: key.id,
                                displayName: key.displayName,
                              })
                            }
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
        <DialogTitle>
          <FormattedMessage {...messages.addDialogTitle} />
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage {...messages.keyNameLabel} />
              </FormLabel>
              <TextField
                autoFocus
                fullWidth
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder={intl.formatMessage(messages.keyNamePlaceholder)}
                size="small"
                value={displayName}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage {...messages.keyValueLabel} />
              </FormLabel>
              <TextField
                fullWidth
                helperText={intl.formatMessage(messages.keyValueHelper)}
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
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!canSubmit} onClick={submit} size="small" variant="contained">
            {createMutation.isPending ? (
              <>
                <CircularProgress size={16} sx={{ mr: 1 }} />
                <FormattedMessage {...messages.adding} />
              </>
            ) : (
              <FormattedMessage {...messages.add} />
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
        <DialogTitle>
          <FormattedMessage {...messages.revokeDialogTitle} />
        </DialogTitle>
        <DialogContent>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.revokeDialogMessage} />
          </Typography>
          <Typography sx={{ fontWeight: 600, mt: 1 }} variant="body2">
            {revokeTarget?.displayName}
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
            <FormattedMessage {...messages.cancel} />
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
                <FormattedMessage {...messages.revoking} />
              </>
            ) : (
              <FormattedMessage {...messages.revoke} />
            )}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
