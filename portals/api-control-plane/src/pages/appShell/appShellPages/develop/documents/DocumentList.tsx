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
  Card,
  CircularProgress,
  Collapse,
  Divider,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { FileText } from '@wso2/oxygen-ui-icons-react';
import { Fragment, useEffect, useId, useMemo, useRef, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { useFormatters } from '@/i18n/useFormatters';
import { DocumentGroupHeader, documentItemSx } from './DocumentGroupHeader';
import { groupDocumentsByType } from './documentGroups';

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
    description:
      'Secondary line of a document in the list. {when} is a relative time, e.g. "2 days ago".',
  },
  loadingMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentList.loadingMore',
    defaultMessage: 'Loading documents…',
  },
});

/** Start loading the next page this far before the end of the list is reached. */
const LOAD_AHEAD = '120px';

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
 * The loaded documents, grouped by type. Scrolling to the end of the list
 * appends the next page. Groups are rebuilt from whatever has loaded so far — the server orders
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

  const scrollRef = useRef<HTMLUListElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const root = scrollRef.current;
    const sentinel = sentinelRef.current;
    if (!hasMore || loadingMore || !root || !sentinel) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) onLoadMore();
      },
      { root, rootMargin: `0px 0px ${LOAD_AHEAD} 0px` },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [documents.length, hasMore, loadingMore, onLoadMore]);
  // Groups the user has collapsed, by type. Everything starts expanded, and a
  // collapsed group stays collapsed as more pages load into it.
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  // A type name can hold spaces, which an id cannot, so each group's id is built from its position.
  const groupsId = useId();
  const toggleGroup = (type: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });

  const groups = useMemo(() => groupDocumentsByType(documents, intl), [documents, intl]);

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
      <List
        dense
        disablePadding
        ref={scrollRef}
        sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 1, py: 0.5 }}
      >
        {groups.map((group, groupIndex) => {
          const open = !collapsed.has(group.type);
          return (
            <Fragment key={group.type}>
              <DocumentGroupHeader
                controlsId={`${groupsId}-group-${groupIndex}`}
                count={group.items.length}
                first={groupIndex === 0}
                label={group.label}
                onToggle={() => toggleGroup(group.type)}
                open={open}
              />
              <Collapse id={`${groupsId}-group-${groupIndex}`} in={open} timeout="auto">
                {group.items.map((document) => (
                  <ListItemButton
                    aria-current={document.id === selectedId ? 'true' : undefined}
                    key={document.id}
                    onClick={() => onSelect(document.id)}
                    selected={document.id === selectedId}
                    sx={(theme) => ({
                      ...documentItemSx(theme),
                      // Icon centred on the name + updated-time block.
                      alignItems: 'center',
                    })}
                  >
                    <ListItemIcon sx={{ minWidth: 32 }}>
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
                      // Updated time a step below the name: caption, not the default body2.
                      slotProps={{ primary: { noWrap: true }, secondary: { variant: 'caption' } }}
                    />
                  </ListItemButton>
                ))}
              </Collapse>
            </Fragment>
          );
        })}
        {hasMore && <Box aria-hidden component="li" ref={sentinelRef} sx={{ height: 1 }} />}
        {loadingMore && (
          <Box
            component="li"
            sx={{
              alignItems: 'center',
              display: 'flex',
              gap: 1,
              justifyContent: 'center',
              py: 1.5,
            }}
          >
            <CircularProgress size={14} />
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage {...messages.loadingMore} />
            </Typography>
          </Box>
        )}
      </List>
    </Card>
  );
}
