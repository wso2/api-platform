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

import { Box, Button, Card, Chip, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Pencil, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { REST_API_TYPE } from '@/api/resources/apiPublications';
import {
  useApiDocument,
  useApiDocumentContent,
  useDeleteApiDocument,
} from '@/api/resources/apiDocuments';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { MarkdownView } from '@/components/MarkdownView';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useFormatters } from '@/i18n/useFormatters';
import { Can } from '@/permissions';
import { isErrorCode } from '@/api/core/errors';
import { isTextContent } from './documentContent';
import { documentTypeLabel } from './documentTypes';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.loading',
    defaultMessage: 'Loading document',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.loadError',
    defaultMessage: 'Unable to load this document.',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.notFound',
    defaultMessage: 'This document no longer exists. It may have been deleted.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.updated',
    defaultMessage: 'Updated {when}',
    description: '{when} is a relative time, e.g. "2 days ago".',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.edit',
    defaultMessage: 'Edit',
  },
  delete: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.delete',
    defaultMessage: 'Delete',
  },
  contentLoading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.contentLoading',
    defaultMessage: 'Loading content',
  },
  contentError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.contentError',
    defaultMessage: 'Unable to load the content of this document.',
  },
  unsupported: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.unsupported',
    defaultMessage: 'This document’s format can’t be previewed here.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.empty',
    defaultMessage: 'This document has no content.',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.deleteTitle',
    defaultMessage: 'Delete document?',
  },
  deleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.deleteMessage',
    defaultMessage: '"{name}" will be permanently removed from this API. This can’t be undone.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.cancel',
    defaultMessage: 'Cancel',
  },
  deleted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.deleted',
    defaultMessage: 'Deleted "{name}".',
  },
});

type DocumentViewerProps = {
  apiHandle: string;
  docId: string;
  onEdit: () => void;
  onDeleted: () => void;
};

/** One document's metadata, its rendered Markdown, and the edit/delete actions. */
export function DocumentViewer({ apiHandle, docId, onDeleted, onEdit }: DocumentViewerProps) {
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const { notify } = useNotifications();
  // Metadata and body load in parallel: the header renders as soon as the
  // metadata arrives, the body fills in below it.
  const documentQuery = useApiDocument(REST_API_TYPE, apiHandle, docId);
  const contentQuery = useApiDocumentContent(REST_API_TYPE, apiHandle, docId);
  const deleteMutation = useDeleteApiDocument();
  const [confirmOpen, setConfirmOpen] = useState(false);

  if (documentQuery.isPending) {
    return (
      <Card sx={{ height: '100%', p: 2 }}>
        <LoadingState label={intl.formatMessage(messages.loading)} />
      </Card>
    );
  }
  if (documentQuery.error) {
    return (
      <ErrorState
        message={intl.formatMessage(
          isErrorCode(documentQuery.error, 'NOT_FOUND') ? messages.notFound : messages.loadError
        )}
      />
    );
  }

  const document = documentQuery.data;
  const when = relativeTime(document.updatedAt ?? document.createdAt);

  const confirmDelete = () =>
    deleteMutation.mutate(
      { apiId: apiHandle, apiType: REST_API_TYPE, docId },
      {
        onSuccess: () => {
          setConfirmOpen(false);
          notify(intl.formatMessage(messages.deleted, { name: document.displayName }), 'success');
          onDeleted();
        },
      }
    );

  return (
    <Card component="article" sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Stack
        direction={{ sm: 'row', xs: 'column' }}
        spacing={2}
        // A tinted band sets the document's metadata apart from its content.
        sx={{
          alignItems: { sm: 'flex-start' },
          bgcolor: 'action.hover',
          flexShrink: 0,
          justifyContent: 'space-between',
          px: 3,
          py: 2.5,
        }}
      >
        <Stack spacing={1} sx={{ minWidth: 0 }}>
          <Box>
            <Chip
              color="primary"
              label={intl.formatMessage(documentTypeLabel(document.type))}
              size="small"
              variant="outlined"
            />
          </Box>
          <Typography component="h2" sx={{ fontWeight: 600, overflowWrap: 'anywhere' }} variant="h5">
            {document.displayName}
          </Typography>
          {when && (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.updated} values={{ when }} />
            </Typography>
          )}
        </Stack>
        <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
          <Can do="UpdateAPIDocument" denied="hide">
            <Button
              color="secondary"
              onClick={onEdit}
              startIcon={<Pencil size={16} />}
              variant="outlined"
            >
              <FormattedMessage {...messages.edit} />
            </Button>
          </Can>
          <Can do="DeleteAPIDocument" denied="hide">
            <Button
              color="error"
              onClick={() => setConfirmOpen(true)}
              startIcon={<Trash2 size={16} />}
              variant="outlined"
            >
              <FormattedMessage {...messages.delete} />
            </Button>
          </Can>
        </Stack>
      </Stack>
      <Divider />
      {/* Only the content scrolls; the title and actions stay in view. */}
      <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 3, py: 3 }}>
        <Box sx={{ maxWidth: 820 }}>
          {contentQuery.isPending ? (
            <LoadingState label={intl.formatMessage(messages.contentLoading)} />
          ) : contentQuery.error ? (
            <Typography color="error" variant="body2">
              <FormattedMessage {...messages.contentError} />
            </Typography>
          ) : !isTextContent(contentQuery.data.contentType) ? (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.unsupported} />
            </Typography>
          ) : (
            <MarkdownView
              emptyFallback={
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.empty} />
                </Typography>
              }
              source={contentQuery.data.text}
            />
          )}
        </Box>
      </Box>

      <ConfirmDialog
        cancelLabel={intl.formatMessage(messages.cancel)}
        confirmLabel={intl.formatMessage(messages.delete)}
        destructive
        loading={deleteMutation.isPending}
        message={intl.formatMessage(messages.deleteMessage, { name: document.displayName })}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={confirmDelete}
        open={confirmOpen}
        title={intl.formatMessage(messages.deleteTitle)}
      />
    </Card>
  );
}
