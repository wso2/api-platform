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
  Card,
  Chip,
  Divider,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Stack,
  Typography,
  alpha,
  type Theme,
} from '@wso2/oxygen-ui';
import { ChevronDown, FileText } from '@wso2/oxygen-ui-icons-react';
import { Fragment, useMemo } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { useFormatters } from '@/i18n/useFormatters';
import { hairline } from '@/theme/receipes';
import { DOCUMENT_TYPES, documentTypeLabel } from './documentTypes';

const messages = defineMessages({
  heading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.heading',
    defaultMessage: 'All documents',
  },
  total: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.total',
    defaultMessage: '{count, plural, one {# document} other {# documents}}',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.updated',
    defaultMessage: 'Updated {when}',
    description: 'Secondary line of a document in the list. {when} is a relative time, e.g. "2 days ago".',
  },
  viewMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.viewMore',
    defaultMessage: 'View more',
    description: 'Loads the next page of documents into the list.',
  },
  loadingMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.loadingMore',
    defaultMessage: 'Loading documents…',
  },
  showing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.showing',
    defaultMessage: 'Showing {shown} of {total}',
    description: 'How many of the API’s documents are loaded into the list.',
  },
});

/**
 * The theme's `action.selected` fill is close to invisible on the dark acrylic
 * surface, so the selected document also gets a primary-coloured stroke. Every
 * item carries a transparent border of the same width, so selecting one never
 * shifts the list.
 */
const selectedItemSx = (theme: Theme) =>
  ({
    border: hairline(theme),
    borderColor: 'transparent',
    mb: 0.5,
    '&.Mui-selected, &.Mui-selected:hover': {
      backgroundColor: alpha(theme.palette.primary.main, 0.08),
      borderColor: 'primary.main',
    },
  }) as const;

type DocumentListProps = {
  documents: ApiDocumentMetadata[];
  /** Fixed panel height; the list scrolls inside it. */
  height: Readonly<Record<'md' | 'xs', string | number>>;
  selectedId?: string;
  /** Total across all pages, from `pagination.total`. */
  total: number;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onSelect: (docId: string) => void;
};

/**
 * The loaded documents, grouped by type, with "View more" appending the next
 * page. Groups are rebuilt from whatever has loaded so far — the server orders
 * by last update, so a later page can add to any group.
 */
export function DocumentList({
  documents,
  hasMore,
  height,
  loadingMore,
  onLoadMore,
  onSelect,
  selectedId,
  total,
}: DocumentListProps) {
  const intl = useIntl();
  const { relativeTime } = useFormatters();

  const groups = useMemo(() => {
    const known = new Set<string>(DOCUMENT_TYPES);
    return [...DOCUMENT_TYPES, 'UNKNOWN' as const]
      .map((type) => ({
        type,
        items: documents.filter((document) =>
          type === 'UNKNOWN' ? !known.has(document.type) : document.type === type
        ),
      }))
      .filter((group) => group.items.length > 0);
  }, [documents]);

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height }}>
      <Stack
        direction="row"
        sx={{ alignItems: 'baseline', flexShrink: 0, justifyContent: 'space-between', px: 2, py: 1.5 }}
      >
        <Typography component="h2" sx={{ fontWeight: 600 }} variant="subtitle1">
          <FormattedMessage {...messages.heading} />
        </Typography>
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage {...messages.total} values={{ count: total }} />
        </Typography>
      </Stack>
      <Divider />
      <List dense disablePadding sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 1, py: 0.5 }}>
        {groups.map((group, groupIndex) => (
          <Fragment key={group.type}>
            <ListSubheader
              disableSticky
              sx={{
                alignItems: 'center',
                bgcolor: 'transparent',
                display: 'flex',
                justifyContent: 'space-between',
                lineHeight: 2.5,
                mb: 1,
                mt: groupIndex === 0 ? 0.5 : 2,
                px: 1,
              }}
            >
              <Typography color="text.secondary" sx={{ fontWeight: 600, textTransform: 'uppercase' }} variant="caption">
                <FormattedMessage {...documentTypeLabel(group.type)} />
              </Typography>
              <Chip label={group.items.length} size="small" sx={{ typography: 'caption' }} />
            </ListSubheader>
            {group.items.map((document) => (
              <ListItemButton
                aria-current={document.id === selectedId ? 'true' : undefined}
                key={document.id}
                onClick={() => onSelect(document.id)}
                selected={document.id === selectedId}
                sx={(theme) => ({
                  ...selectedItemSx(theme),
                  alignItems: 'flex-start',
                })}
              >
                <ListItemIcon sx={{ minWidth: 32, pt: 0.5 }}>
                  <FileText size={16} />
                </ListItemIcon>
                <ListItemText
                  primary={document.displayName}
                  secondary={
                    document.updatedAt
                      ? intl.formatMessage(messages.updated, { when: relativeTime(document.updatedAt) })
                      : undefined
                  }
                  slotProps={{ primary: { noWrap: true } }}
                />
              </ListItemButton>
            ))}
          </Fragment>
        ))}
      </List>
      {(hasMore || documents.length < total) && (
        <>
          <Divider />
          <Stack spacing={1} sx={{ alignItems: 'center', flexShrink: 0, p: 1.5 }}>
            {hasMore && (
              <Button
                disabled={loadingMore}
                onClick={onLoadMore}
                size="small"
                startIcon={loadingMore ? undefined : <ChevronDown size={16} />}
                variant="outlined"
              >
                {loadingMore ? (
                  <FormattedMessage {...messages.loadingMore} />
                ) : (
                  <FormattedMessage {...messages.viewMore} />
                )}
              </Button>
            )}
            <Box aria-live="polite">
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.showing} values={{ shown: documents.length, total }} />
              </Typography>
            </Box>
          </Stack>
        </>
      )}
    </Card>
  );
}
