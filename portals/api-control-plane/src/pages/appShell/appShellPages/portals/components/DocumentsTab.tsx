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
  alpha,
  Box,
  Button,
  Checkbox,
  Collapse,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { FileSearch, FileText, Minus, Plus, X } from '@wso2/oxygen-ui-icons-react';
import { Fragment, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useAllApiDocuments, type ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { DocumentBody } from '../../develop/documents/DocumentBody';
import { DocumentGroupHeader } from '../../develop/documents/DocumentGroupHeader';
import { groupDocumentsByType } from '../../develop/documents/documentGroups';
import type { DraftFormValues } from '../utils/publicationForm';

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

export type DocumentsTabProps = {
  apiHandle: string;
  /** The `{apiType}` path segment of the API that owns the documents. */
  apiType: string;
  disabled?: boolean;
  /** Shows the live selection read-only; `onChange` is never called. */
  readOnly?: boolean;
  onChange?: (values: DraftFormValues) => void;
  values: DraftFormValues;
};

/**
 * "Documentation": which of the API's documents are published with it, listed by type.
 *
 * The draft lists every document with a checkbox and a Preview button that opens its content.
 * The published view lists exactly the documents the live listing holds, with neither.
 */
export function DocumentsTab({ apiHandle, apiType, disabled, onChange, readOnly, values }: DocumentsTabProps) {
  const intl = useIntl();
  const documentsQuery = useAllApiDocuments(apiType, apiHandle);
  const selectedIds = values.docIds;
  const selectedSet = new Set(selectedIds);
  const [previewId, setPreviewId] = useState<string>();
  // Collapsed groups, by type.
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());

  const documents = documentsQuery.data?.list ?? [];
  const shownDocuments = readOnly ? documents.filter((doc) => selectedSet.has(doc.id)) : documents;

  if (documentsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (documentsQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (shownDocuments.length === 0) {
    return (
      <Box component="section" sx={{ p: 3 }}>
        <EmptyState
          description={readOnly ? undefined : intl.formatMessage(messages.emptyDescription)}
          title={intl.formatMessage(readOnly ? messages.publishedEmptyTitle : messages.emptyTitle)}
        />
      </Box>
    );
  }

  const groups = groupDocumentsByType(shownDocuments, intl);
  const selectedCount = documents.filter((doc) => selectedSet.has(doc.id)).length;
  const allSelected = selectedCount === documents.length;
  const previewed = documents.find((doc) => doc.id === previewId);

  const commitSelection = (nextIds: string[]) => onChange?.({ ...values, docIds: nextIds });

  const toggle = (docId: string) => {
    const next = new Set(selectedIds);
    if (next.has(docId)) next.delete(docId);
    else next.add(docId);
    commitSelection(Array.from(next));
  };

  const toggleAll = () => commitSelection(allSelected ? [] : documents.map((doc) => doc.id));

  const toggleGroup = (type: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });

  // Same summary row as the Subscription Plans tab.
  const countStyle = {
    color: readOnly ? 'success.main' : 'primary.main',
    fontSize: 20,
    fontWeight: 500,
  } as const;

  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 2, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <Typography color="text.secondary" variant="body2">
          {readOnly ? (
            <FormattedMessage
              {...messages.publishedSummary}
              values={{
                count: shownDocuments.length,
                n: (chunks) => (
                  <Typography component="span" sx={countStyle}>
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
                  <Typography component="span" sx={countStyle}>
                    {selectedCount}
                  </Typography>
                ),
                total: documents.length,
              }}
            />
          )}
        </Typography>
        {!readOnly && (
          <Button disabled={disabled} onClick={toggleAll} size="small" variant="text">
            <FormattedMessage {...(allSelected ? messages.clearAll : messages.selectAll)} />
          </Button>
        )}
      </Box>

      <List dense disablePadding>
        {groups.map((group, groupIndex) => {
          const open = !collapsed.has(group.type);
          return (
            <Fragment key={group.type}>
              <DocumentGroupHeader
                controlsId={`document-group-${group.type}`}
                count={group.items.length}
                first={groupIndex === 0}
                label={group.label}
                onToggle={() => toggleGroup(group.type)}
                open={open}
              />
              <Collapse id={`document-group-${group.type}`} in={open} timeout="auto">
                {group.items.map((doc) => (
                  <DocumentRow
                    disabled={disabled}
                    doc={doc}
                    key={doc.id}
                    onPreview={() => setPreviewId(doc.id)}
                    onToggle={() => toggle(doc.id)}
                    readOnly={readOnly}
                    selected={selectedSet.has(doc.id)}
                  />
                ))}
              </Collapse>
            </Fragment>
          );
        })}
      </List>

      <Drawer
        anchor="right"
        onClose={() => setPreviewId(undefined)}
        open={Boolean(previewed)}
        slotProps={{ paper: { sx: { maxWidth: '100%', width: 520 } } }}
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
              <DocumentBody apiHandle={apiHandle} apiType={apiType} docId={previewed.id} key={previewed.id} />
            </Box>
            <Divider />
            <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 2 }}>
              <Button
                color={selectedSet.has(previewed.id) ? 'inherit' : 'primary'}
                disabled={disabled}
                onClick={() => toggle(previewed.id)}
                startIcon={selectedSet.has(previewed.id) ? <Minus size={16} /> : <Plus size={16} />}
                variant={selectedSet.has(previewed.id) ? 'outlined' : 'contained'}
              >
                <FormattedMessage {...(selectedSet.has(previewed.id) ? messages.removeFromListing : messages.addToListing)} />
              </Button>
            </Box>
          </>
        )}
      </Drawer>
    </Box>
  );
}

type DocumentRowProps = {
  disabled?: boolean;
  doc: ApiDocumentMetadata;
  onPreview: () => void;
  onToggle: () => void;
  /** The published view: a plain row, no checkbox, no preview. */
  readOnly?: boolean;
  selected: boolean;
};

/** One document: its checkbox and name, and a button that opens its content. */
function DocumentRow({ disabled, doc, onPreview, onToggle, readOnly, selected }: DocumentRowProps) {
  const intl = useIntl();
  if (readOnly) {
    return (
      <ListItem component="div" dense>
        <ListItemIcon sx={{ minWidth: 32 }}>
          <FileText size={16} />
        </ListItemIcon>
        <ListItemText primary={doc.displayName} slotProps={{ primary: { noWrap: true } }} />
      </ListItem>
    );
  }
  return (
    <Stack
      direction="row"
      sx={(theme) => ({
        alignItems: 'center',
        borderRadius: 1,
        bgcolor: selected ? alpha(theme.palette.primary.main, 0.06) : undefined,
        mb: 0.5,
        pr: 1,
      })}
    >
      <Checkbox
        checked={selected}
        disabled={disabled}
        onChange={onToggle}
        slotProps={{ input: { 'aria-label': doc.displayName } }}
      />
      <ListItemText primary={doc.displayName} slotProps={{ primary: { noWrap: true } }} sx={{ flex: 1, minWidth: 0 }} />
      <Button
        aria-label={intl.formatMessage(messages.previewDocument, { name: doc.displayName })}
        onClick={onPreview}
        size="small"
        startIcon={<FileSearch size={16} />}
        variant="text"
      >
        <FormattedMessage {...messages.preview} />
      </Button>
    </Stack>
  );
}
