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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import {
  alpha,
  Box,
  Button,
  Checkbox,
  Chip,
  Divider,
  Drawer,
  IconButton,
  ListingTable,
  Stack,
  Tooltip,
  Typography,
  type Theme,
} from '@wso2/oxygen-ui';
import { FileSearch, Minus, Plus, X } from '@wso2/oxygen-ui-icons-react';

import type { MCPProxyDocument } from '../../../../utils/types';
import { groupDocumentsByType } from './documentGroups';
import { DocumentPreview } from './DocumentPreview';
import type { DeletedSelection, DraftFormValues } from './mcpPublishForm';
import { EmptyState, ErrorState, LoadingState } from './PublishStates';

const messages = defineMessages({
  loading: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.loading',
    defaultMessage: 'Loading documents',
  },
  error: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.error',
    defaultMessage: 'Unable to load documents.',
  },
  emptyTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.emptyTitle',
    defaultMessage: 'No documents',
  },
  emptyDescription: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.emptyDescription',
    defaultMessage: 'This MCP proxy has no documents to choose from.',
  },
  publishedEmptyTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.publishedEmptyTitle',
    defaultMessage: 'This listing has no documents.',
  },
  selectedSummary: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.selectedSummary',
    defaultMessage: '{count} of {total} documents selected',
    description: '{count} is a styled number rendered as a React element, not plain text; {total} is a plain number.',
  },
  publishedSummary: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.publishedSummary',
    defaultMessage: '{count, plural, one {<n>#</n> document published} other {<n>#</n> documents published}}',
    description: '<n> wraps the number, which is styled.',
  },
  selectAll: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.selectAll',
    defaultMessage: 'Select all',
  },
  clearAll: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.clearAll',
    defaultMessage: 'Clear all',
  },
  columnName: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.columnName',
    defaultMessage: 'Name',
  },
  columnType: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.columnType',
    defaultMessage: 'Type',
  },
  columnActions: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.columnActions',
    defaultMessage: 'Actions',
  },
  typeUnknown: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.typeUnknown',
    defaultMessage: '—',
  },
  edited: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.edited',
    defaultMessage: 'Edited',
  },
  deletedPublishedHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.deletedPublishedHint',
    defaultMessage: 'This document was deleted. Republish to remove it from the listing.',
  },
  editedPublishedHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.editedPublishedHint',
    defaultMessage: 'Document was edited after publishing. Republish to apply changes.',
  },
  preview: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.preview',
    defaultMessage: 'Preview',
  },
  previewDocument: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.previewDocument',
    defaultMessage: 'Preview {name}',
    description: "Accessible name of the button that opens a document's content; {name} is the document's name.",
  },
  addToListing: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.addToListing',
    defaultMessage: 'Add to listing',
  },
  removeFromListing: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.removeFromListing',
    defaultMessage: 'Remove from listing',
  },
  closePreview: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.closePreview',
    defaultMessage: 'Close',
  },
});

const PREVIEW_WIDTH = 520;

/** The control plane's selected grid row: neutral, taking the primary tint only on hover. */
const selectedRowSx = (theme: Theme) => ({
  '&.Mui-selected': {
    backgroundColor: theme.palette.action.selected,
    '&:hover': {
      backgroundColor: alpha(
        theme.palette.primary.main,
        theme.palette.action.selectedOpacity + theme.palette.action.hoverOpacity
      ),
    },
  },
});

/** One table row: a live document, or a deleted one that only has its stored name. */
type DocumentRow = {
  deleted: boolean;
  edited: boolean;
  id: string;
  name: string;
  /** Unknown for a deleted document. */
  typeLabel?: string;
};

/** Deleted rows first, then edited ones, then the rest, as on the plan cards. */
const inAttentionOrder = (deleted: DocumentRow[], live: DocumentRow[]): DocumentRow[] => [
  ...deleted,
  ...live.filter((row) => row.edited),
  ...live.filter((row) => !row.edited),
];

export type DocumentsTabProps = {
  /** Published view only; the draft has already dropped them. */
  deletedDocuments?: readonly DeletedSelection[];
  disabled?: boolean;
  /** Tagged "Edited" in the published view. */
  editedDocIds?: readonly string[];
  /** `undefined` while they load. */
  documents?: MCPProxyDocument[];
  failed?: boolean;
  mcpProxyId: string;
  onChange?: (values: DraftFormValues) => void;
  /** Lists only the selected documents, ticked and unchangeable, without the Actions column. */
  readOnly?: boolean;
  /** Names as stored at the last publish, shown in the published view. */
  storedNames?: Readonly<Record<string, string>>;
  values: DraftFormValues;
};

/** "Documentation": which of the proxy's documents are published with it, in a table ordered by type. */
export function DocumentsTab({
  deletedDocuments = [],
  disabled,
  documents,
  editedDocIds = [],
  failed,
  mcpProxyId,
  onChange,
  readOnly,
  storedNames,
  values,
}: DocumentsTabProps) {
  const intl = useIntl();
  const [previewId, setPreviewId] = useState<string>();
  // Grouped once per list; the published view then filters it.
  const groups = useMemo(() => (documents ? groupDocumentsByType(documents, intl) : []), [documents, intl]);

  if (failed) return <ErrorState message={intl.formatMessage(messages.error)} />;
  if (!documents) return <LoadingState label={intl.formatMessage(messages.loading)} />;

  const selected = new Set(values.docIds);
  const edited = new Set(editedDocIds);
  const shownDeleted = readOnly ? deletedDocuments : [];

  const deletedRows: DocumentRow[] = shownDeleted.map((doc) => ({
    deleted: true,
    edited: false,
    id: doc.id,
    name: doc.displayName,
  }));
  const liveRows: DocumentRow[] = groups.flatMap((group) =>
    group.items
      .filter((doc) => !readOnly || selected.has(doc.id))
      .map((doc) => ({
        deleted: false,
        edited: readOnly === true && edited.has(doc.id),
        id: doc.id,
        name: (readOnly && storedNames?.[doc.id]) || doc.displayName,
        typeLabel: group.label,
      }))
  );
  const rows = inAttentionOrder(deletedRows, liveRows);

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

  const selectedCount = documents.filter((doc) => selected.has(doc.id)).length;
  const allSelected = selectedCount === documents.length;
  const someSelected = selectedCount > 0 && !allSelected;
  const previewed = documents.find((doc) => doc.id === previewId);

  const toggle = (docId: string) =>
    onChange?.({
      ...values,
      docIds: selected.has(docId) ? values.docIds.filter((id) => id !== docId) : [...values.docIds, docId],
    });

  // A selected id the list never loaded is not the table's to change.
  const loaded = new Set(documents.map((doc) => doc.id));
  const unloaded = values.docIds.filter((id) => !loaded.has(id));
  const toggleAll = () =>
    onChange?.({ ...values, docIds: [...(allSelected ? [] : documents.map((doc) => doc.id)), ...unloaded] });

  const countSx = { color: readOnly ? 'success.main' : 'primary.main', fontSize: 20, fontWeight: 500 } as const;

  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 2, height: '100%', minHeight: 0, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', flexShrink: 0, gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <Typography color="text.secondary" variant="body2">
          {readOnly ? (
            <FormattedMessage
              {...messages.publishedSummary}
              values={{
                count: rows.length,
                n: (chunks) => (
                  <Typography component="span" sx={countSx}>
                    {chunks}
                  </Typography>
                ),
              }}
            />
          ) : (
            <FormattedMessage
              {...messages.selectedSummary}
              values={{
                count: (
                  <Typography component="span" sx={countSx}>
                    {selectedCount}
                  </Typography>
                ),
                total: documents.length,
              }}
            />
          )}
        </Typography>
      </Box>

      <ListingTable.Provider>
        <ListingTable.Container sx={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>
          {/* Type 35% and Actions 15% of the row; Name takes the rest and truncates. */}
          <ListingTable
            density="compact"
            stickyHeader
            sx={(theme) => ({
              minWidth: 640,
              tableLayout: 'fixed',
              // Oxygen drops the line under each section's last row, the header's only row included.
              '& .MuiTableRow-root:last-child > .MuiTableCell-root': {
                borderBottom: `1px solid ${theme.vars ? theme.vars.palette.TableCell.border : theme.palette.divider}`,
              },
            })}
          >
            <ListingTable.Head
              // The theme's header fill is see-through in dark mode, so rows would show through a sticky one.
              sx={(theme) => {
                const { palette } = theme.vars ?? theme;
                return {
                  '& .MuiTableCell-head': {
                    backgroundColor: palette.background.paper,
                    backgroundImage: `linear-gradient(${palette.action.hover}, ${palette.action.hover})`,
                  },
                };
              }}
            >
              <ListingTable.Row>
                <ListingTable.Cell padding="checkbox" sx={{ width: 48 }}>
                  {readOnly ? (
                    // Shown like the draft's select-all box, but only marks every row as published.
                    <Checkbox
                      checked
                      color="success"
                      size="small"
                      slotProps={{ input: { 'aria-hidden': true, readOnly: true } }}
                      sx={{ pointerEvents: 'none' }}
                      tabIndex={-1}
                    />
                  ) : (
                    <Checkbox
                      checked={allSelected}
                      disabled={disabled}
                      indeterminate={someSelected}
                      size="small"
                      onChange={toggleAll}
                      slotProps={{
                        input: { 'aria-label': intl.formatMessage(allSelected ? messages.clearAll : messages.selectAll) },
                      }}
                    />
                  )}
                </ListingTable.Cell>
                <ListingTable.Cell>
                  <FormattedMessage {...messages.columnName} />
                </ListingTable.Cell>
                <ListingTable.Cell sx={{ width: '35%' }}>
                  <FormattedMessage {...messages.columnType} />
                </ListingTable.Cell>
                {!readOnly && (
                  <ListingTable.Cell sx={{ width: '15%' }}>
                    <FormattedMessage {...messages.columnActions} />
                  </ListingTable.Cell>
                )}
              </ListingTable.Row>
            </ListingTable.Head>
            <ListingTable.Body>
              {rows.map((row) => {
                const isSelected = readOnly || selected.has(row.id);
                return (
                  <ListingTable.Row hover key={row.id} selected={isSelected} sx={selectedRowSx}>
                    <ListingTable.Cell padding="checkbox">
                      {readOnly ? (
                        // The listing holds it; a deleted document's box is greyed.
                        <Checkbox
                          checked
                          color="success"
                          disabled={row.deleted}
                          size="small"
                          slotProps={{ input: { 'aria-label': row.name, readOnly: true } }}
                          sx={{ pointerEvents: 'none' }}
                          tabIndex={-1}
                        />
                      ) : (
                        <Checkbox
                          checked={isSelected}
                          disabled={disabled}
                          size="small"
                          onChange={() => toggle(row.id)}
                          slotProps={{ input: { 'aria-label': row.name } }}
                        />
                      )}
                    </ListingTable.Cell>
                    <ListingTable.Cell truncate>
                      <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, minWidth: 0 }}>
                        {row.deleted ? (
                          <Tooltip describeChild title={intl.formatMessage(messages.deletedPublishedHint)}>
                            <Typography color="text.secondary" component="del" noWrap variant="body2">
                              {row.name}
                            </Typography>
                          </Tooltip>
                        ) : (
                          <Typography noWrap variant="body2">
                            {row.name}
                          </Typography>
                        )}
                        {row.edited && (
                          // describeChild keeps "Edited" as the chip's name; the tooltip only describes it.
                          <Tooltip describeChild title={intl.formatMessage(messages.editedPublishedHint)}>
                            <Chip
                              color="warning"
                              label={<FormattedMessage {...messages.edited} />}
                              size="small"
                              sx={{ flexShrink: 0 }}
                              variant="outlined"
                            />
                          </Tooltip>
                        )}
                      </Box>
                    </ListingTable.Cell>
                    <ListingTable.Cell>
                      {row.typeLabel ? (
                        <Chip label={row.typeLabel} size="small" variant="outlined" />
                      ) : (
                        <Typography color="text.secondary" variant="body2">
                          <FormattedMessage {...messages.typeUnknown} />
                        </Typography>
                      )}
                    </ListingTable.Cell>
                    {!readOnly && (
                      <ListingTable.Cell>
                        <Button
                          aria-label={intl.formatMessage(messages.previewDocument, { name: row.name })}
                          onClick={() => setPreviewId(row.id)}
                          size="small"
                          startIcon={<FileSearch size={16} />}
                          variant="text"
                        >
                          <FormattedMessage {...messages.preview} />
                        </Button>
                      </ListingTable.Cell>
                    )}
                  </ListingTable.Row>
                );
              })}
            </ListingTable.Body>
          </ListingTable>
        </ListingTable.Container>
      </ListingTable.Provider>

      <Drawer
        anchor="right"
        onClose={() => setPreviewId(undefined)}
        open={Boolean(previewed)}
        slotProps={{ paper: { sx: { maxWidth: '100%', width: PREVIEW_WIDTH } } }}
      >
        {previewed && (
          <>
            <Stack direction="row" sx={{ alignItems: 'center', gap: 1, px: 2, py: 1.5 }}>
              <Typography component="h2" noWrap sx={{ flex: 1, fontWeight: 600 }} variant="h6">
                {previewed.displayName}
              </Typography>
              <IconButton
                aria-label={intl.formatMessage(messages.closePreview)}
                onClick={() => setPreviewId(undefined)}
                size="small"
              >
                <X size={18} />
              </IconButton>
            </Stack>
            <Divider />
            <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', p: 2 }}>
              <DocumentPreview docId={previewed.id} key={previewed.id} mcpProxyId={mcpProxyId} />
            </Box>
            <Divider />
            <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 2 }}>
              <Button
                color={selected.has(previewed.id) ? 'inherit' : 'primary'}
                disabled={disabled}
                onClick={() => toggle(previewed.id)}
                startIcon={selected.has(previewed.id) ? <Minus size={16} /> : <Plus size={16} />}
                variant={selected.has(previewed.id) ? 'outlined' : 'contained'}
              >
                <FormattedMessage
                  {...(selected.has(previewed.id) ? messages.removeFromListing : messages.addToListing)}
                />
              </Button>
            </Box>
          </>
        )}
      </Drawer>
    </Box>
  );
}
