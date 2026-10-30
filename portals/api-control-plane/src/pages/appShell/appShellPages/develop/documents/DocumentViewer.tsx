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

import { Card, IconButton, Stack, Tooltip } from '@wso2/oxygen-ui';
import { Pencil, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { REST_API_TYPE } from '@/api/resources/apiPublications';
import {
  useApiDocument,
  useApiDocumentContent,
  useDeleteApiDocument,
} from '@/api/resources/apiDocuments';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { isErrorCode } from '@/api/core/errors';
import { DocumentBody } from './DocumentBody';
import { DocumentCard } from './DocumentCard';

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
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.edit',
    defaultMessage: 'Edit',
  },
  delete: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentViewer.delete',
    defaultMessage: 'Delete',
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
  /** The `{apiType}` path segment — `rest-api` unless another artifact kind owns the documents. */
  apiType?: string;
  docId: string;
  onEdit: () => void;
  onDeleted: () => void;
};

/** One document's metadata, its rendered Markdown, and the edit/delete actions. */
export function DocumentViewer({
  apiHandle,
  apiType = REST_API_TYPE,
  docId,
  onDeleted,
  onEdit,
}: DocumentViewerProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const documentQuery = useApiDocument(apiType, apiHandle, docId);
  // Starts the body request with the metadata's; `DocumentBody` reads the same cache entry.
  useApiDocumentContent(apiType, apiHandle, docId);
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
          isErrorCode(documentQuery.error, 'NOT_FOUND') ? messages.notFound : messages.loadError,
        )}
      />
    );
  }

  const document = documentQuery.data;

  const confirmDelete = () =>
    deleteMutation.mutate(
      { apiId: apiHandle, apiType, docId },
      {
        onSuccess: () => {
          setConfirmOpen(false);
          notify(intl.formatMessage(messages.deleted, { name: document.displayName }), 'success');
          onDeleted();
        },
      },
    );

  return (
    <>
      <DocumentCard
        actions={
          <Stack direction="row" spacing={0.5} sx={{ flexShrink: 0 }}>
            <Can do="UpdateAPIDocument" denied="hide">
              <Tooltip title={intl.formatMessage(messages.edit)}>
                <IconButton aria-label={intl.formatMessage(messages.edit)} onClick={onEdit} size="small">
                  <Pencil size={18} />
                </IconButton>
              </Tooltip>
            </Can>
            <Can do="DeleteAPIDocument" denied="hide">
              <Tooltip title={intl.formatMessage(messages.delete)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.delete)}
                  color="error"
                  onClick={() => setConfirmOpen(true)}
                  size="small"
                >
                  <Trash2 size={18} />
                </IconButton>
              </Tooltip>
            </Can>
          </Stack>
        }
        title={document.displayName}
      >
        <DocumentBody apiHandle={apiHandle} apiType={apiType} docId={docId} />
      </DocumentCard>

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
    </>
  );
}
