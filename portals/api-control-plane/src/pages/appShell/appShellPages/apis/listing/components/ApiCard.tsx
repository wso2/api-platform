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

import { Box, Card, CardContent, Divider, Stack, Typography } from '@wso2/oxygen-ui';

import type { RestApi } from '@/api/resources/restApis';
import { interactiveCardSx } from '@/theme';
import {
  apiDescriptionSx,
  ApiActionsMenu,
  ApiKindAvatar,
  ApiKindChip,
  LifecycleStatusLabel,
  UpdatedLabel,
  VersionChip,
} from './RestApiChips';

type ApiCardProps = {
  api: RestApi;
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

const AVATAR_SIZE = 42;

/**
 * API card for the grid view, rendering the spec's `RESTAPI` shape.
 */
export function ApiCard({ api, onOpen, onDelete }: ApiCardProps) {
  const updated = api.updatedAt || api.createdAt;

  return (
    <Card
      onClick={() => onOpen(api)}
      sx={{
        ...interactiveCardSx,
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      <CardContent sx={{ flex: 1 }}>
        <Stack spacing={2}>
          <Stack alignItems="flex-start" direction="row" spacing={1.5}>
            <ApiKindAvatar kind={api.kind} size={AVATAR_SIZE} />
            <Box sx={{ minWidth: 0 }}>
              <Typography noWrap sx={{ fontWeight: 700 }} variant="h6">
                {api.displayName}
              </Typography>
              <Stack direction="row" spacing={1} sx={{ mt: 0.5 }}>
                <ApiKindChip kind={api.kind} />
                <VersionChip version={api.version} />
              </Stack>
            </Box>
          </Stack>
          <Typography color="text.secondary" sx={apiDescriptionSx(2)} variant="body2">
            {api.description || ''}
          </Typography>
        </Stack>
      </CardContent>

      <Divider />

      <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, px: 2, py: 1.25 }}>
        <LifecycleStatusLabel status={api.lifeCycleStatus} />
        <Stack alignItems="center" direction="row" spacing={0.5} sx={{ ml: 'auto' }}>
          <UpdatedLabel timestamp={updated} />
          {onDelete && (
            <Box sx={{ mr: -1.5 }}>
              <ApiActionsMenu apiName={api.displayName} onDelete={() => onDelete(api)} />
            </Box>
          )}
        </Stack>
      </Box>
    </Card>
  );
}
