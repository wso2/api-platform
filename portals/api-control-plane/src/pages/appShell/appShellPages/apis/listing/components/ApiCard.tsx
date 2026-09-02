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
  Card,
  CardActions,
  CardContent,
  CardHeader,
  Divider,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';

import type { RestApi } from '@/api/resources/restApis';
import { interactiveCardSx } from '@/theme';
import {
  apiDescriptionSx,
  ApiKindAvatar,
  DeleteApiButton,
  GatewayManagedChip,
  revealApiDeleteOnHoverSx,
  TransportChips,
  UpdatedLabel,
  VersionChip,
} from './RestApiChips';

type ApiCardProps = {
  api: RestApi;
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

/** Edge of the square kind tile; the icon inside scales with it. */
const AVATAR_SIZE = 56;

/**
 * API card for the grid view, rendering the spec's `RESTAPI` shape.
 */
export function ApiCard({ api, onOpen, onDelete }: ApiCardProps) {
  const updated = api.updatedAt || api.createdAt;
  const transports = api.transport ?? [];

  return (
    <Card
      onClick={() => onOpen(api)}
      sx={{
        ...interactiveCardSx,
        ...revealApiDeleteOnHoverSx,
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      <CardHeader
        avatar={<ApiKindAvatar kind={api.kind} size={AVATAR_SIZE} />}
        slotProps={{
          // `content` has no min-width of its own, so a long name would widen
          // the card instead of truncating. Both slots render a Stack, which
          // cannot legally sit inside the default `span`.
          content: { sx: { minWidth: 0 } },
          subheader: { component: 'div' },
          title: { component: 'div', sx: { mb: 1 } },
        }}
        subheader={
          <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }} useFlexGap>
            <TransportChips transports={transports} />
            {api.readOnly && <GatewayManagedChip />}
          </Stack>
        }
        sx={{ alignItems: 'flex-start' }}
        title={
          <Stack alignItems="center" direction="row" spacing={1} sx={{ minWidth: 0 }}>
            <Typography component="span" noWrap sx={{ fontWeight: 600 }} variant="h5">
              {api.displayName}
            </Typography>
            <VersionChip version={api.version} />
          </Stack>
        }
      />

      {/* Two-line clamped description; the flex grow is what keeps every
          card's footer on the same line across the grid. */}
      <CardContent sx={{ flex: 1, pt: 0 }}>
        <Typography color="text.secondary" sx={apiDescriptionSx(2)} variant="body2">
          {api.description || ''}
        </Typography>
      </CardContent>

      <Divider />

      {/* Footer: when it last changed, and the one destructive action. */}
      <CardActions sx={{ justifyContent: 'space-between', px: 2 }}>
        <UpdatedLabel timestamp={updated} />
        {onDelete && <DeleteApiButton apiName={api.displayName} onDelete={() => onDelete(api)} />}
      </CardActions>
    </Card>
  );
}
