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
  Drawer,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Clock, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useMyApiKeys, useRevokeApiKey } from '@/api/resources/apiKeys';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';

import { CreateApiKeyDialog } from './CreateApiKeyDialog';

const messages = defineMessages({
  add: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.add',
    defaultMessage: 'Add',
    description: 'Opens the dialog for issuing a new API key.',
  },
  addButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.addButton',
    defaultMessage: 'Add API Key',
    description: 'Opens the dialog for issuing a new API key.',
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
  createdMetadata: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.createdMetadata',
    defaultMessage: 'Created {time} by {creator}',
    description: 'Creation time and creator shown beside an API key.',
  },
  closeDrawer: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.closeDrawer',
    defaultMessage: 'Close API keys',
  },
  seeMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.seeMore',
    defaultMessage: 'See more',
  },
  separator: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ApiKeysPanel.separator',
    defaultMessage: '·',
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

/** Page size for fetching caller's keys. Uses endpoint maximum to avoid hiding keys due to client-side per-API filtering. */
const API_KEY_PAGE_SIZE = 100;

/** Stands in for a value the server did not send. Locale-independent, and one
 * definition so the table and the date formatter can't drift apart. */
const EMPTY_VALUE = '-';

/** The key the revoke dialog is armed for: its id addresses the request, its
 * name is what the dialog and the toast show. */
type RevokeTarget = { id: string; displayName: string };

/**
 * API Keys section of the Overview tab (ai-workspace layout). The server mints
 * the key, stores only a hash and broadcasts it to the gateways the API is
 * deployed on; the plaintext exists once, in `CreateApiKeyDialog`'s second
 * step, and is never readable again — hence the masked values listed here.
 */
export function ApiKeysPanel({ restApiId }: { restApiId: string }) {
  const intl = useIntl();
  const { dateTime, relativeTime } = useFormatters();
  const { notify } = useNotifications();
  // The spec has no per-API key listing — the only read is the caller's own
  // inventory across artifacts, so this narrows to REST API keys server-side
  // and to this API here. It therefore shows the signed-in user's keys only.
  //
  // `limit` is explicit because filtering to this API happens in the browser.
  // With the server default page size (20), relevant keys could be omitted.
  // `API_KEY_PAGE_SIZE` is the spec max and yields the widest single request.
  const keysQuery = useMyApiKeys({ limit: API_KEY_PAGE_SIZE, type: ['RestApi'] });
  const revokeMutation = useRevokeApiKey();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<RevokeTarget | null>(null);

  const keys = useMemo(
    () =>
      (keysQuery.data?.list ?? [])
        .filter((key) => key.artifactId === restApiId && key.status === 'active')
        .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()),
    [keysQuery.data, restApiId],
  );

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

  const recentKeys = keys.slice(0, 5);

  return (
    <Box>
      <Stack spacing={1.5}>
        <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={1}>
          <Box sx={{ flex: 1 }}>
            <Typography sx={{ fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.title} />
            </Typography>
            <Typography color="text.secondary" sx={{ lineHeight: 1.25 }} variant="caption">
              <FormattedMessage {...messages.description} />
            </Typography>
          </Box>
          <Button
            onClick={() => setDialogOpen(true)}
            size="small"
            startIcon={<Plus size={16} />}
            variant="outlined"
          >
            <FormattedMessage {...messages.add} />
          </Button>
        </Stack>

        {keysQuery.isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
            <CircularProgress />
          </Box>
        ) : keys.length > 0 ? (
          <Stack spacing={1}>
            {recentKeys.map((key) => (
              <Stack
                alignItems="center"
                direction="row"
                key={key.id ?? key.displayName}
                spacing={1}
                sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1, p: 1.25 }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography noWrap sx={{ fontWeight: 600 }} variant="body2">
                    {key.displayName || EMPTY_VALUE}
                  </Typography>
                  <Stack alignItems="center" direction="row" spacing={0.5}>
                    <Typography color="text.secondary" noWrap variant="caption">
                      {key.maskedApiKey || EMPTY_VALUE}
                    </Typography>
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...messages.separator} />
                    </Typography>
                    <Tooltip title={dateTime(key.createdAt)}>
                      <Stack alignItems="center" direction="row" spacing={0.5} sx={{ minWidth: 0 }}>
                        <Clock color="currentColor" size={13} />
                        <Typography color="text.secondary" noWrap variant="caption">
                          <FormattedMessage
                            {...messages.createdMetadata}
                            values={{
                              creator: key.createdBy || '—',
                              time: relativeTime(key.createdAt),
                            }}
                          />
                        </Typography>
                      </Stack>
                    </Tooltip>
                  </Stack>
                </Box>
                <Tooltip title={intl.formatMessage(messages.revokeTooltip)}>
                  <span>
                    <IconButton
                      disabled={revokeMutation.isPending || !key.id}
                      onClick={() =>
                        key.id && setRevokeTarget({ id: key.id, displayName: key.displayName })
                      }
                      size="small"
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </span>
                </Tooltip>
              </Stack>
            ))}
            {keys.length > 5 && (
              <Box sx={{ textAlign: 'center' }}>
                <Button onClick={() => setDrawerOpen(true)} size="small" variant="text">
                  <FormattedMessage {...messages.seeMore} />
                </Button>
              </Box>
            )}
          </Stack>
        ) : null}
      </Stack>

      <Drawer
        anchor="right"
        onClose={() => setDrawerOpen(false)}
        open={drawerOpen}
        sx={{ '& .MuiDrawer-paper': { width: { md: 560, xs: '100%' } } }}
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Stack
            alignItems="center"
            direction="row"
            spacing={1}
            sx={{ borderBottom: '1px solid', borderColor: 'divider', p: 2 }}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.closeDrawer)}
              onClick={() => setDrawerOpen(false)}
              size="small"
            >
              <ChevronLeft size={20} />
            </IconButton>
            <Typography sx={{ fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.title} />
            </Typography>
          </Stack>
          <Stack spacing={1} sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
            {keys.map((key) => (
              <Stack
                alignItems="center"
                direction="row"
                key={key.id ?? key.displayName}
                spacing={1}
                sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1, p: 1.25 }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography noWrap sx={{ fontWeight: 600 }} variant="body2">
                    {key.displayName || EMPTY_VALUE}
                  </Typography>
                  <Stack alignItems="center" direction="row" spacing={0.5}>
                    <Typography color="text.secondary" noWrap variant="caption">
                      {key.maskedApiKey || EMPTY_VALUE}
                    </Typography>
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...messages.separator} />
                    </Typography>
                    <Clock color="currentColor" size={13} />
                    <Typography color="text.secondary" noWrap variant="caption">
                      <FormattedMessage
                        {...messages.createdMetadata}
                        values={{
                          creator: key.createdBy || '—',
                          time: relativeTime(key.createdAt),
                        }}
                      />
                    </Typography>
                  </Stack>
                </Box>
                <Tooltip title={intl.formatMessage(messages.revokeTooltip)}>
                  <span>
                    <IconButton
                      disabled={revokeMutation.isPending || !key.id}
                      onClick={() =>
                        key.id && setRevokeTarget({ id: key.id, displayName: key.displayName })
                      }
                      size="small"
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </span>
                </Tooltip>
              </Stack>
            ))}
          </Stack>
        </Box>
      </Drawer>

      <CreateApiKeyDialog
        onClose={() => setDialogOpen(false)}
        open={dialogOpen}
        restApiId={restApiId}
      />

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
