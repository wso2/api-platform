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
  Collapse,
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
import { ChevronDown, ChevronRight, FileText } from '@wso2/oxygen-ui-icons-react';
import { Fragment, useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { useFormatters } from '@/i18n/useFormatters';
import { hairline } from '@/theme/receipes';
import { DOCUMENT_TYPES, documentTypeName, isCustomDocumentType } from './documentTypes';

const messages = defineMessages({
  toggleGroup: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.toggleGroup',
    defaultMessage: '{type} ({count})',
    description:
      'Accessible name of a document-type group header that expands or collapses the group.',
  },
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
    description:
      'Secondary line of a document in the list. {when} is a relative time, e.g. "2 days ago".',
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
  // Groups the user has collapsed, by type. Everything starts expanded, and a
  // collapsed group stays collapsed as more pages load into it.
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  const toggleGroup = (type: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });

  // Fixed types first, in their usual order; then each custom type (stored as
  // the name the user typed) as its own group, alphabetically; then plain
  // "Other".
  const groups = useMemo(() => {
    const fixed = DOCUMENT_TYPES.filter((type) => type !== 'OTHER');
    const buckets = new Map<string, ApiDocumentMetadata[]>();
    for (const document of documents) {
      const key =
        (fixed as readonly string[]).includes(document.type) || isCustomDocumentType(document.type)
          ? document.type
          : 'OTHER';
      buckets.set(key, [...(buckets.get(key) ?? []), document]);
    }
    const custom = [...buckets.keys()]
      .filter(isCustomDocumentType)
      .sort((a, b) => documentTypeName(intl, a).localeCompare(documentTypeName(intl, b)));
    return [...fixed, ...custom, 'OTHER']
      .filter((type) => buckets.has(type))
      .map((type) => ({ items: buckets.get(type)!, label: documentTypeName(intl, type), type }));
  }, [documents, intl]);

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height }}>
      <Stack
        direction="row"
        sx={{
          alignItems: 'baseline',
          flexShrink: 0,
          justifyContent: 'space-between',
          px: 2,
          py: 1.5,
        }}
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
        {groups.map((group, groupIndex) => {
          const open = !collapsed.has(group.type);
          return (
            <Fragment key={group.type}>
              <ListSubheader
                disableGutters
                disableSticky
                sx={{
                  bgcolor: 'transparent',
                  lineHeight: 'normal',
                  mb: open ? 1 : 0,
                  mt: groupIndex === 0 ? 0.5 : 1.5,
                }}
              >
                {/* The header is the toggle: a real button, so it is reachable by keyboard. */}
                <ListItemButton
                  aria-controls={`document-group-${group.type}`}
                  aria-expanded={open}
                  aria-label={intl.formatMessage(messages.toggleGroup, {
                    count: group.items.length,
                    type: group.label,
                  })}
                  dense
                  onClick={() => toggleGroup(group.type)}
                  sx={{ gap: 1, px: 1, py: 0.5 }}
                >
                  <Box sx={{ color: 'text.secondary', display: 'flex' }}>
                    {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  </Box>
                  {/* Shown exactly as stored — no case transform on custom type names. */}
                  <Typography
                    color="text.secondary"
                    sx={{ flex: 1, fontWeight: 600, minWidth: 0, textTransform: 'none' }}
                    noWrap
                    variant="caption"
                  >
                    {group.label}
                  </Typography>
                  <Chip label={group.items.length} size="small" sx={{ typography: 'caption' }} />
                </ListItemButton>
              </ListSubheader>
              <Collapse id={`document-group-${group.type}`} in={open} timeout="auto">
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
                          ? intl.formatMessage(messages.updated, {
                              when: relativeTime(document.updatedAt),
                            })
                          : undefined
                      }
                      slotProps={{ primary: { noWrap: true } }}
                    />
                  </ListItemButton>
                ))}
              </Collapse>
            </Fragment>
          );
        })}
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
                <FormattedMessage
                  {...messages.showing}
                  values={{ shown: documents.length, total }}
                />
              </Typography>
            </Box>
          </Stack>
        </>
      )}
    </Card>
  );
}
