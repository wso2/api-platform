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

import { Box, Button, Grid, PageTitle } from '@wso2/oxygen-ui';
import { FileText, Plus } from '@wso2/oxygen-ui-icons-react';
import { useEffect } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { REST_API_TYPE } from '@/api/resources/apiPublications';
import { useApiDocumentPages } from '@/api/resources/apiDocuments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { DocumentList } from './DocumentList';
import { DocumentViewer } from './DocumentViewer';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.title',
    defaultMessage: 'Documents',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.subtitle',
    defaultMessage: 'Guides, samples and support resources that describe this API.',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.create',
    defaultMessage: 'Create Document',
    description: 'Button that opens the form for a new API document. Verb phrase.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.loading',
    defaultMessage: 'Loading documents',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.loadError',
    defaultMessage: 'Unable to load the documents for this API.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentsBrowser.empty',
    defaultMessage: 'No Documents available for this API',
  },
});

/** Documents loaded per "View more". */
export const DOCUMENTS_PAGE_SIZE = 10;

/**
 * Both panels share one fixed height and scroll inside it, so neither the
 * number of documents nor the length of one grows the page. Sized to the
 * viewport below the page title, with a floor for short windows.
 */
export const DOCUMENTS_PANEL_HEIGHT = { md: 'max(480px, calc(100vh - 300px))', xs: 560 } as const;

type DocumentsBrowserProps = {
  apiHandle: string;
  /** The document in the URL; the first loaded one is shown when absent. */
  selectedId?: string;
  onCreate: () => void;
  onDeleted: () => void;
  onEdit: (docId: string) => void;
  onSelect: (docId: string, options?: { replace?: boolean }) => void;
};

/** The document list beside the selected document's content. */
export function DocumentsBrowser({
  apiHandle,
  onCreate,
  onDeleted,
  onEdit,
  onSelect,
  selectedId,
}: DocumentsBrowserProps) {
  const intl = useIntl();
  const pagesQuery = useApiDocumentPages(REST_API_TYPE, apiHandle, { limit: DOCUMENTS_PAGE_SIZE });
  const firstId = pagesQuery.data?.pages[0]?.list[0]?.id;

  // Arriving without a document in the URL selects the first one, and records
  // it (replacing, not pushing) so the highlight, the preview and the URL agree.
  useEffect(() => {
    if (!selectedId && firstId) onSelect(firstId, { replace: true });
  }, [firstId, onSelect, selectedId]);

  // `isPending`, not `isLoading`: a disabled query has no data and would flash the empty state.
  if (pagesQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (pagesQuery.error) return <ErrorState message={intl.formatMessage(messages.loadError)} />;

  const pages = pagesQuery.data.pages;
  const documents = pages.flatMap((page) => page.list);
  // Lists read `pagination.total`, never `list.length`; the latest page has the freshest count.
  const total = pages[pages.length - 1]?.pagination.total ?? 0;
  const activeId = selectedId ?? documents[0]?.id;
  const isEmpty = total === 0 && documents.length === 0;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {/* The empty state carries its own create action; one way out is enough. */}
        {!isEmpty && (
          <PageTitle.Actions>
            <Can do="CreateAPIDocument" denied="hide">
              <Button onClick={onCreate} startIcon={<Plus size={18} />} variant="contained">
                <FormattedMessage {...messages.create} />
              </Button>
            </Can>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isEmpty ? (
        <EmptyState
          actionIcon={<Plus size={18} />}
          actionLabel={intl.formatMessage(messages.create)}
          illustration={<FileText size={48} />}
          onAction={onCreate}
          operationId="CreateAPIDocument"
          title={intl.formatMessage(messages.empty)}
        />
      ) : (
        <Grid container spacing={2}>
          <Grid size={{ md: 4, xs: 12 }}>
            <DocumentList
              documents={documents}
              height={DOCUMENTS_PANEL_HEIGHT}
              hasMore={pagesQuery.hasNextPage}
              loadingMore={pagesQuery.isFetchingNextPage}
              onLoadMore={() => void pagesQuery.fetchNextPage()}
              onSelect={onSelect}
              selectedId={activeId}
              total={total}
            />
          </Grid>
          <Grid size={{ md: 8, xs: 12 }}>
            <Box sx={{ height: DOCUMENTS_PANEL_HEIGHT, minWidth: 0 }}>
              {activeId && (
                <DocumentViewer
                  apiHandle={apiHandle}
                  docId={activeId}
                  key={activeId}
                  onDeleted={onDeleted}
                  onEdit={() => onEdit(activeId)}
                />
              )}
            </Box>
          </Grid>
        </Grid>
      )}
    </>
  );
}
