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

import type { RestApi } from '@/api/resources/restApis';
import {
  apiDescriptionSx,
  ApiKindAvatar,
  DeleteApiButton,
  GatewayManagedChip,
  revealApiDeleteOnHoverSx,
  TransportChips,
  UpdatedLabel,
  VersionChip,
} from './components/RestApiChips';

const AVATAR_SIZE = 40;

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
  const transports = api.transport ?? [];

  return (
    <Box
      onClick={() => onOpen(api)}
      sx={(theme) => ({
        ...revealApiDeleteOnHoverSx,
        alignItems: 'center',
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        display: 'flex',
        gap: 2,
        px: 2.5,
        py: 1.75,
        transition: theme.transitions.create('background-color'),
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      <ApiKindAvatar kind={api.kind} size={AVATAR_SIZE} />

      {/* `minWidth: 0` lets long names truncate. */}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack alignItems="center" direction="row" spacing={1} sx={{ minWidth: 0 }}>
          <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
            {api.displayName}
          </Typography>
          <VersionChip version={api.version} />
        </Stack>
        {api.description && (
          <Typography color="text.secondary" sx={apiDescriptionSx(1)} variant="body2">
            {api.description}
          </Typography>
        )}
      </Box>

      {/* Dropped first as the row narrows: the card's own subheader marks. */}
      <Stack
        direction="row"
        spacing={1}
        sx={{
          display: { sm: 'flex', xs: 'none' },
          flexShrink: 0,
          flexWrap: 'wrap',
          gap: 1,
          justifyContent: 'flex-end',
        }}
        useFlexGap
      >
        <TransportChips transports={transports} />
        {api.readOnly && <GatewayManagedChip />}
      </Stack>

      <Box sx={{ display: { md: 'flex', xs: 'none' }, ml: 'auto' }}>
        <UpdatedLabel timestamp={updated} />
      </Box>

      {onDelete && <DeleteApiButton apiName={api.displayName} onDelete={() => onDelete(api)} />}
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
      {apis.map((api) => (
        <ApiRow api={api} key={api.id} onDelete={onDelete} onOpen={onOpen} />
      ))}
    </Card>
  );
}
