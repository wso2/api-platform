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

import { Box, Card, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

import type { RestApi } from '@/api/resources/restApis';
import {
  apiDescriptionSx,
  ApiDeleteButton,
  ApiKindAvatar,
  ApiKindChip,
  UpdatedLabel,
} from './components/RestApiChips';

const AVATAR_SIZE = 40;

const messages = defineMessages({
  api: { id: 'apiListView.column.api', defaultMessage: 'API' },
  type: { id: 'apiListView.column.type', defaultMessage: 'Type' },
  updated: { id: 'apiListView.column.updated', defaultMessage: 'Updated' },
  version: { id: 'apiListView.column.version', defaultMessage: 'Version' },
});

const rowGridSx = {
  alignItems: 'center',
  display: 'grid',
  gap: 2,
  gridTemplateColumns: {
    xs: 'minmax(0, 1fr) auto',
    md: 'minmax(0, 1fr) 120px 100px 180px',
  },
} as const;

type ApiRowProps = {
  api: RestApi;
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

/**
 * One API as a row.
 */
function ApiRow({ api, onOpen, onDelete }: ApiRowProps) {
  const updated = api.updatedAt || api.createdAt;

  return (
    <Box
      onClick={() => onOpen(api)}
      sx={(theme) => ({
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        px: 2.5,
        py: 1.75,
        transition: theme.transitions.create('background-color'),
        ...rowGridSx,
        '&:focus-within .api-delete-action, &:hover .api-delete-action': {
          opacity: 1,
        },
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      {/* `minWidth: 0` lets long names truncate. */}
      <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
        <ApiKindAvatar kind={api.kind} size={AVATAR_SIZE} />
        <Box sx={{ minWidth: 0 }}>
          <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
            {api.displayName}
          </Typography>
          <Typography color="text.secondary" sx={apiDescriptionSx(1)} variant="caption">
            {api.description || ''}
          </Typography>
        </Box>
      </Stack>

      <Box sx={{ display: { md: 'block', xs: 'none' } }}>
        <Typography sx={{ fontFamily: 'monospace' }} variant="body2">
          {api.version}
        </Typography>
      </Box>
      <Box sx={{ display: { md: 'block', xs: 'none' } }}>
        <ApiKindChip kind={api.kind} />
      </Box>
      <Stack
        alignItems="center"
        direction="row"
        justifyContent="flex-end"
        spacing={0.5}
        sx={{ display: { md: 'flex', xs: 'none' } }}
      >
        <UpdatedLabel timestamp={updated} />
        {onDelete && (
          <Box sx={{ mr: -1 }}>
            <ApiDeleteButton apiName={api.displayName} onDelete={() => onDelete(api)} />
          </Box>
        )}
      </Stack>
      <Stack
        alignItems="center"
        direction="row"
        spacing={0.5}
        sx={{ display: { md: 'none', xs: 'flex' } }}
      >
        <UpdatedLabel timestamp={updated} />
        {onDelete && <ApiDeleteButton apiName={api.displayName} onDelete={() => onDelete(api)} />}
      </Stack>
    </Box>
  );
}

type ApiListViewProps = {
  apis: RestApi[];
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

/** Compact row layout for APIs, the list-view counterpart of ApiCardGrid. */
export function ApiListView({ apis, onOpen, onDelete }: ApiListViewProps) {
  return (
    <Card data-testid="api-list-view" variant="outlined">
      <Box
        sx={{
          ...rowGridSx,
          bgcolor: 'action.hover',
          px: 2.5,
          py: 1.25,
        }}
      >
        <Typography color="text.secondary" sx={{ fontWeight: 700 }} variant="caption">
          <FormattedMessage {...messages.api} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ display: { md: 'block', xs: 'none' }, fontWeight: 700 }}
          variant="caption"
        >
          <FormattedMessage {...messages.version} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ display: { md: 'block', xs: 'none' }, fontWeight: 700 }}
          variant="caption"
        >
          <FormattedMessage {...messages.type} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ display: { md: 'block', xs: 'none' }, fontWeight: 700, textAlign: 'right' }}
          variant="caption"
        >
          <FormattedMessage {...messages.updated} />
        </Typography>
      </Box>
      {apis.map((api) => (
        <ApiRow api={api} key={api.id} onDelete={onDelete} onOpen={onOpen} />
      ))}
    </Card>
  );
}
