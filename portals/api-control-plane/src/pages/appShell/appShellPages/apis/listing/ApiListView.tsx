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

import { Box, RelativeTime, Stack, Typography, type Theme } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

import { thumbnailApiType, type ListableApi } from './apiListItem';
import { ListingCardTable, type ListingCardColumn } from '@/components/common/ListingCardTable';
import { apiDescriptionSx, ApiDeleteLabelButton } from './components/RestApiChips';
import { apiKindLabel } from '../utils/restApiDisplay';
import { ApiThumbnailAvatar } from '../components/ApiThumbnailAvatar';
import { relativeTime } from '@/utils/relativeTime';
import { useCan } from '@/permissions/useCan';

const AVATAR_SIZE = 40;

const messages = defineMessages({
  api: { id: 'apiListView.column.api', defaultMessage: 'Name' },
  type: { id: 'apiListView.column.type', defaultMessage: 'Type' },
  updated: { id: 'apiListView.column.updated', defaultMessage: 'Last Updated' },
  description: { id: 'apiListView.column.description', defaultMessage: 'Description' },
});

/**
 * Where the Delete button replaces the time on row hover: wide enough for a
 * pointer layout and a device that can hover. Elsewhere (phones, touch
 * tablets) the time and the button sit side by side, both visible.
 */
const hoverMedia = (theme: Theme) => `${theme.breakpoints.up('md')} and (hover: hover)`;

type ApiListViewProps = {
  apis: ListableApi[];
  onOpen: (api: ListableApi) => void;
  onDelete?: (api: ListableApi) => void;
};

/** Card-row layout for APIs, the list-view counterpart of ApiCardGrid. */
export function ApiListView({ apis, onOpen, onDelete }: ApiListViewProps) {
  const intl = useIntl();
  const canDelete = useCan('DeleteRESTAPI');

  const columns: ListingCardColumn<ListableApi>[] = [
    {
      id: 'name',
      header: intl.formatMessage(messages.api),
      render: (api) => (
        // `minWidth: 0` lets long names truncate.
        <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
          <ApiThumbnailAvatar
            apiId={api.id}
            apiType={thumbnailApiType(api)}
            displayName={api.displayName}
            size={AVATAR_SIZE}
          />
          <Box sx={{ minWidth: 0 }}>
            <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
              {api.displayName}
            </Typography>
            <Typography
              color="text.secondary"
              sx={{ ...apiDescriptionSx(1), display: { xs: '-webkit-box', md: 'none' } }}
              variant="caption"
            >
              {api.description || ''}
            </Typography>
          </Box>
        </Stack>
      ),
    },
    {
      id: 'description',
      header: intl.formatMessage(messages.description),
      hideOnMobile: true,
      render: (api) => (
        <Typography color="text.secondary" sx={apiDescriptionSx(1)} variant="body2">
          {api.description || ''}
        </Typography>
      ),
    },
    {
      id: 'type',
      header: intl.formatMessage(messages.type),
      hideOnMobile: true,
      width: 120,
      render: (api) => (
        <Typography color="text.secondary" noWrap variant="body2">
          {apiKindLabel(api.kind)}
        </Typography>
      ),
    },
    {
      id: 'updated',
      header: intl.formatMessage(messages.updated),
      // Hug the content: the column is as wide as its widest cell (the Delete
      // button), so the button sits at the same edge at every screen width.
      // A fixed width narrower than the button let it overflow to the left.
      width: '1%',
      render: (api) => {
        const updated = api.updatedAt || api.createdAt;
        return (
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              gap: 1,
              justifyContent: 'flex-start',
              position: 'relative',
            }}
          >
            {/* The time sizes the cell; from md up the Delete button is laid over it and
                the time fades out on row hover. */}
            <Box
              sx={(theme) => ({
                // Keep "7 hours ago" on one line; a squeezed column should shrink the
                // description instead of wrapping the time.
                whiteSpace: 'nowrap',
                transition: 'opacity 150ms ease',
                ...(onDelete && canDelete && {
                  [hoverMedia(theme)]: {
                    'tr:focus-within &, tr:hover &': { opacity: 0 },
                  },
                }),
              })}
            >
              {updated && <RelativeTime time={relativeTime(updated)} dateTime={updated} />}
            </Box>
            {onDelete && canDelete && (
              // Pinned to the cell's right edge, not placed by a grid track, so it
              // lands in the same spot whatever the time's width or the screen size.
              <Box
                sx={(theme) => ({
                  [hoverMedia(theme)]: {
                    position: 'absolute',
                    right: -8,
                    top: '50%',
                    transform: 'translateY(-50%)',
                  },
                })}
              >
                <ApiDeleteLabelButton apiName={api.displayName} onDelete={() => onDelete(api)} />
              </Box>
            )}
          </Box>
        );
      },
    },
  ];

  return (
    <ListingCardTable
      columns={columns}
      data-testid="api-list-view"
      getRowId={(api) => api.id ?? api.displayName}
      getRowLabel={(api) => api.displayName}
      onRowOpen={onOpen}
      revealOnHoverClassName="api-delete-action"
      rows={apis}
    />
  );
}
