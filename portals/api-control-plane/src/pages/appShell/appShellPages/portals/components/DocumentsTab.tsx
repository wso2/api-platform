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

import {
  Box,
  Button,
  Chip,
  DataGrid,
  Divider,
  Drawer,
  IconButton,
  ListingTable,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { FileSearch, Minus, Plus, X } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useAllApiDocuments, type ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { DocumentBody } from '../../develop/documents/DocumentBody';
import { groupDocumentsByType } from '../../develop/documents/documentGroups';
import { toggleSelection, type DeletedSelection, type DraftFormValues } from '../utils/publicationForm';
import { SelectionSummary } from './SelectionSummary';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.loading',
    defaultMessage: 'Loading documents',
  },
  error: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.error',
    defaultMessage: 'Unable to load documents.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.emptyTitle',
    defaultMessage: 'No documents',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.emptyDescription',
    defaultMessage: 'Add documents to this API in Develop to choose from here.',
  },
  publishedEmptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.publishedEmptyTitle',
    defaultMessage: 'This listing has no documents.',
  },
  selectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.selectedSummary',
    defaultMessage: '{count} of {total} documents selected',
    description: '{count} is a styled number rendered as a React element, not plain text; {total} is a plain number.',
  },
  publishedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.publishedSummary',
    defaultMessage: '{count, plural, one {<n>#</n> document published} other {<n>#</n> documents published}}',
    description: '<n> wraps the number, which is styled.',
  },
  selectAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.selectAll',
    defaultMessage: 'Select all',
  },
  clearAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.clearAll',
    defaultMessage: 'Clear all',
  },
  selectRow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.selectRow',
    defaultMessage: 'Select row',
    description: 'Accessible name of the checkbox that adds a document to the listing; the row it sits in names the document.',
  },
  unselectRow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.unselectRow',
    defaultMessage: 'Unselect row',
    description: 'Accessible name of the checkbox that takes a document out of the listing; the row it sits in names the document.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.columnName',
    defaultMessage: 'Name',
  },
  columnType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.columnType',
    defaultMessage: 'Type',
  },
  typeUnknown: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.typeUnknown',
    defaultMessage: '—',
    description: 'Type cell of a deleted document, whose type is no longer known.',
  },
  edited: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.edited',
    defaultMessage: 'Edited',
    description: 'Chip after the name of a published document that was changed after it was last published.',
  },
  deletedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.deletedPublishedHint',
    defaultMessage: 'This document was deleted. Republish to remove it from the listing.',
    description: 'Tooltip on the name of a published document whose source no longer exists.',
  },
  editedPublishedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.editedPublishedHint',
    defaultMessage: 'Document was edited after publishing. Republish to apply changes.',
    description: 'Tooltip on the Edited chip of a published document.',
  },
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.columnActions',
    defaultMessage: 'Actions',
  },
  preview: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.preview',
    defaultMessage: 'Preview',
  },
  previewDocument: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.previewDocument',
    defaultMessage: 'Preview {name}',
    description: 'Accessible name of the button that opens a document\'s content; {name} is the document\'s name.',
  },
  addToListing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.addToListing',
    defaultMessage: 'Add to listing',
  },
  removeFromListing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.removeFromListing',
    defaultMessage: 'Remove from listing',
  },
  closePreview: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.DocumentsTab.closePreview',
    defaultMessage: 'Close',
  },
});

// The community grid shows at most 100 rows a page; past that the footer's pager is the only way to the rest.
const MAX_PAGE_ROWS = 100;

/** One table row: a live document, or a deleted one that only has its stored name. */
type DocumentRow = {
  deleted: boolean;
  edited: boolean;
  id: string;
  name: string;
  typeLabel?: string;
};

export type DocumentsTabProps = {
  apiHandle: string;
  /** The `{apiType}` path segment of the API that owns the documents. */
  apiType: string;
  /** Selected documents that were deleted. Only the published view lists them; the draft has dropped them. */
  deletedDocuments?: readonly DeletedSelection[];
  disabled?: boolean;
  /** Ids of selected documents changed since the listing was published; the published view tags them "Edited". */
  editedDocIds?: readonly string[];
  /** Shows the live selection read-only, its checkboxes ticked and disabled; `onChange` is never called. */
  readOnly?: boolean;
  onChange?: (values: DraftFormValues) => void;
  /** The names stored at the last publish, by id. The published view shows these rather than the document's current name. */
  storedNames?: Readonly<Record<string, string>>;
  values: DraftFormValues;
};

/** What needs attention leads, as on the plan cards: deleted rows, then edited ones, then the rest. Each tier keeps its order. */
const inAttentionOrder = (deleted: DocumentRow[], live: DocumentRow[]): DocumentRow[] => [
  ...deleted,
  ...live.filter((row) => row.edited),
  ...live.filter((row) => !row.edited),
];

/** The side panel that reads a document's content and adds it to the listing, or takes it out. */
function DocumentPreviewDrawer({
  apiHandle,
  apiType,
  disabled,
  document,
  onClose,
  onToggle,
  selected,
}: {
  apiHandle: string;
  apiType: string;
  disabled?: boolean;
  /** The document to read; the panel is closed while there is none. */
  document?: ApiDocumentMetadata;
  onClose: () => void;
  onToggle: (docId: string) => void;
  selected: boolean;
}) {
  const intl = useIntl();

  return (
    <Drawer
      anchor="right"
      onClose={onClose}
      open={Boolean(document)}
      slotProps={{ paper: { sx: { maxWidth: '100%', width: 520 } } }}
    >
      {document && (
        <>
          <Stack direction="row" sx={{ alignItems: 'center', gap: 1, px: 2, py: 1.5 }}>
            <Typography component="h2" noWrap sx={{ flex: 1, fontWeight: 600 }} variant="h6">
              {document.displayName}
            </Typography>
            <IconButton aria-label={intl.formatMessage(messages.closePreview)} onClick={onClose} size="small">
              <X size={18} />
            </IconButton>
          </Stack>
          <Divider />
          <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', p: 2 }}>
            <DocumentBody apiHandle={apiHandle} apiType={apiType} docId={document.id} key={document.id} />
          </Box>
          <Divider />
          <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 2 }}>
            <Button
              color={selected ? 'inherit' : 'primary'}
              disabled={disabled}
              onClick={() => onToggle(document.id)}
              startIcon={selected ? <Minus size={16} /> : <Plus size={16} />}
              variant={selected ? 'outlined' : 'contained'}
            >
              <FormattedMessage {...(selected ? messages.removeFromListing : messages.addToListing)} />
            </Button>
          </Box>
        </>
      )}
    </Drawer>
  );
}

/**
 * "Documentation": which of the API's documents are published with it, as an Oxygen `ListingTable.DataGrid` ordered by type.
 * The draft lists existing documents only, with a Preview button per row. The published view lists exactly what the
 * live listing holds, under the names stored at publish, ticked and unchangeable: deleted rows first (struck through),
 * then edited ones (outlined chip), then the rest.
 */
export function DocumentsTab({
  apiHandle,
  apiType,
  deletedDocuments = [],
  disabled,
  editedDocIds = [],
  onChange,
  readOnly,
  storedNames,
  values,
}: DocumentsTabProps) {
  const intl = useIntl();
  const documentsQuery = useAllApiDocuments(apiType, apiHandle);
  const selectedIds = values.docIds;
  const selectedSet = new Set(selectedIds);
  const editedSet = new Set(editedDocIds);
  const [previewId, setPreviewId] = useState<string>();

  const documents = documentsQuery.data?.list ?? [];
  // Grouped once per list; the published view then keeps just the documents it lists.
  const groups = useMemo(() => groupDocumentsByType(documents, intl), [documents, intl]);
  const shownDeleted = readOnly ? deletedDocuments : [];

  const deletedRows: DocumentRow[] = shownDeleted.map((doc) => ({
    deleted: true,
    edited: false,
    id: doc.id,
    name: doc.displayName,
  }));
  const liveRows: DocumentRow[] = groups.flatMap((group) =>
    group.items
      .filter((doc) => !readOnly || selectedSet.has(doc.id))
      .map((doc) => ({
        deleted: false,
        edited: Boolean(readOnly) && editedSet.has(doc.id),
        id: doc.id,
        name: (readOnly && storedNames?.[doc.id]) || doc.displayName,
        typeLabel: group.label,
      })),
  );
  const rows = inAttentionOrder(deletedRows, liveRows);

  // The published view ticks every row it lists; the draft ticks the selected documents that still exist.
  // Held by what it ticks, so the grid is handed a new model only when that changes.
  const tickedIds = (readOnly ? rows.map((row) => row.id) : documents.filter((doc) => selectedSet.has(doc.id)).map((doc) => doc.id)).join('\n');
  const selection = useMemo<DataGrid.GridRowSelectionModel>(
    () => ({ ids: new Set(tickedIds ? tickedIds.split('\n') : []), type: 'include' }),
    [tickedIds],
  );

  if (documentsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (documentsQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (rows.length === 0) {
    return (
      <Box component="section" sx={{ p: 3 }}>
        <EmptyState
          description={readOnly ? undefined : intl.formatMessage(messages.emptyDescription)}
          title={intl.formatMessage(readOnly ? messages.publishedEmptyTitle : messages.emptyTitle)}
        />
      </Box>
    );
  }

  const selectedCount = documents.filter((doc) => selectedSet.has(doc.id)).length;
  const previewed = documents.find((doc) => doc.id === previewId);

  const toggle = (docId: string) => onChange?.({ ...values, docIds: toggleSelection(selectedIds, docId) });

  /** The grid reports a select all as "every row but these", so it is read back as the ids that end up ticked. */
  const handleSelection = (model: DataGrid.GridRowSelectionModel) => {
    if (readOnly) return;
    const ticked = (doc: { id: string }) => (model.type === 'exclude' ? !model.ids.has(doc.id) : model.ids.has(doc.id));
    const loaded = new Set(documents.map((doc) => doc.id));
    // A selected id the list never loaded is not the grid's to change.
    const unloaded = selectedIds.filter((id) => !loaded.has(id));
    onChange?.({ ...values, docIds: [...documents.filter(ticked).map((doc) => doc.id), ...unloaded] });
  };

  const columns: DataGrid.GridColDef<DocumentRow>[] = [
    {
      field: 'name',
      flex: 1,
      headerName: intl.formatMessage(messages.columnName),
      minWidth: 200,
      renderCell: ({ row }) => (
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, minWidth: 0 }}>
          {row.deleted ? (
            <Tooltip describeChild title={intl.formatMessage(messages.deletedPublishedHint)}>
              <Typography component="del" variant="body2">
                {row.name}
              </Typography>
            </Tooltip>
          ) : (
            <Typography variant="body2">{row.name}</Typography>
          )}
          {row.edited && (
            // describeChild keeps "Edited" as the chip's name; the tooltip only describes it.
            <Tooltip describeChild title={intl.formatMessage(messages.editedPublishedHint)}>
              <Chip color="warning" label={<FormattedMessage {...messages.edited} />} size="small" variant="outlined" />
            </Tooltip>
          )}
        </Box>
      ),
      sortable: false,
    },
    {
      align: 'left',
      field: 'typeLabel',
      flex: 0.5,
      headerAlign: 'left',
      headerName: intl.formatMessage(messages.columnType),
      minWidth: 140,
      renderCell: ({ row }) =>
        row.typeLabel ? (
          <Chip label={row.typeLabel} size="small" variant="outlined" />
        ) : (
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.typeUnknown} />
          </Typography>
        ),
      sortable: false,
    },
    ...(readOnly
      ? []
      : [
          {
            field: 'actions',
            headerName: intl.formatMessage(messages.columnActions),
            renderCell: ({ row }) => (
              <Button
                aria-label={intl.formatMessage(messages.previewDocument, { name: row.name })}
                onClick={() => setPreviewId(row.id)}
                size="small"
                startIcon={<FileSearch size={16} />}
                variant="text"
              >
                <FormattedMessage {...messages.preview} />
              </Button>
            ),
            sortable: false,
            width: 140,
          } satisfies DataGrid.GridColDef<DocumentRow>,
        ]),
  ];

  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 2, height: '100%', minHeight: 0, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', flexShrink: 0, gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <SelectionSummary
          count={readOnly ? rows.length : selectedCount}
          published={messages.publishedSummary}
          readOnly={readOnly}
          selected={messages.selectedSummary}
          total={documents.length}
        />
      </Box>

      <ListingTable.Container sx={{ flex: 1, minHeight: 0 }}>
        <ListingTable.DataGrid
          checkboxSelection
          columns={columns}
          density="compact"
          disableColumnMenu
          disableColumnResize
          disableRowSelectionOnClick
          getRowHeight={() => 'auto'}
          hideFooter={rows.length <= MAX_PAGE_ROWS}
          hideFooterSelectedRowCount
          // A deleted document is no longer available: the grid draws its checkbox greyed, still ticked.
          isRowSelectable={({ row }) => !disabled && !row.deleted}
          localeText={{
            checkboxSelectionSelectAllRows: intl.formatMessage(messages.selectAll),
            checkboxSelectionSelectRow: intl.formatMessage(messages.selectRow),
            checkboxSelectionUnselectAllRows: intl.formatMessage(messages.clearAll),
            checkboxSelectionUnselectRow: intl.formatMessage(messages.unselectRow),
          }}
          onRowSelectionModelChange={handleSelection}
          rows={rows}
          rowSelectionModel={selection}
          slotProps={{ baseCheckbox: { material: { color: readOnly ? 'success' : 'primary' } } }}
        />
      </ListingTable.Container>

      <DocumentPreviewDrawer
        apiHandle={apiHandle}
        apiType={apiType}
        disabled={disabled}
        document={previewed}
        onClose={() => setPreviewId(undefined)}
        onToggle={toggle}
        selected={previewed !== undefined && selectedSet.has(previewed.id)}
      />
    </Box>
  );
}
