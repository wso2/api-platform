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

import { Box, ListingCard } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

import { thumbnailApiType, type ListableApi } from '../apiListItem';
import { ApiThumbnailAvatar } from '../../components/ApiThumbnailAvatar';
import { ApiKindChip } from './RestApiChips';
import { relativeTime } from '@/utils/relativeTime';
import { useCan } from '@/permissions/useCan';

type ApiCardProps = {
  api: ListableApi;
  onOpen: (api: ListableApi) => void;
  onDelete?: (api: ListableApi) => void;
};

// Fills the 44px icon slot `ListingCard` reserves.
const AVATAR_SIZE = 44;

const messages = defineMessages({
  deleteAriaLabel: {
    id: 'apiCard.deleteAriaLabel',
    defaultMessage: 'Delete {apiName}',
    description: 'Accessible label for the API delete button.',
  },
});

/**
 * API card for the grid view, on Oxygen's `ListingCard`. Renders either a REST
 * or a GraphQL API — see `ListableApi` for why the two need no per-type
 * branching here.
 */
export function ApiCard({ api, onOpen, onDelete }: ApiCardProps) {
  const intl = useIntl();
  const canDelete = useCan('DeleteRESTAPI');
  const updated = api.updatedAt || api.createdAt;

  return (
    <ListingCard
      deleteLabel={intl.formatMessage(messages.deleteAriaLabel, { apiName: api.displayName })}
      description={api.description || undefined}
      icon={
        <ApiThumbnailAvatar
          apiId={api.id}
          apiType={thumbnailApiType(api)}
          displayName={api.displayName}
          size={AVATAR_SIZE}
        />
      }
      onClick={() => onOpen(api)}
      onDelete={onDelete && canDelete ? () => onDelete(api) : undefined}
      // The name with its type chip beneath it.
      title={
        <Box component="span" sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
          <Box component="span" sx={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {api.displayName}
          </Box>
          <Box component="span" sx={{ display: 'flex', gap: 1 }}>
            <ApiKindChip kind={api.kind} />
          </Box>
        </Box>
      }
      updatedDateTime={updated || undefined}
      updatedTime={updated ? relativeTime(updated) : undefined}
    />
  );
}
