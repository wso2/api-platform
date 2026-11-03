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

import type { ReactNode } from 'react';
import { Avatar } from '@wso2/oxygen-ui';
import { Boxes } from '@wso2/oxygen-ui-icons-react';

import { hairline } from '@/theme/receipes';

import { useApiThumbnail } from '@/api/resources/apiThumbnail';
import { apiInitials } from '../utils/restApiDisplay';

type Props = {
  apiType: string;
  apiId: string | undefined;
  displayName: string | undefined;
  size: number;
  /** Font size for the initials fallback. Defaults to size/2.5. */
  fontSize?: number | string;
  /** Icon size for the empty-name fallback. Defaults to size/2. */
  iconSize?: number;
};

/**
 * Avatar that renders the API's uploaded thumbnail when one is set, falling
 * back to the two-letter name monogram (or a generic icon when no name is
 * available). While the fetch is in flight the initials/icon are shown — a
 * loading spinner here would flash on every row of a list.
 *
 * The underlying `useApiThumbnail` hook caches the Blob under
 * `(org, apiType, apiId)`, so rendering this component in multiple places on
 * the same page (listing + detail header, say) issues one network call.
 */
export function ApiThumbnailAvatar({
  apiType,
  apiId,
  displayName,
  size,
  fontSize,
  iconSize,
}: Props): ReactNode {
  const { url } = useApiThumbnail(apiType, apiId);

  const initials = apiInitials(displayName);
  const fallback: ReactNode = initials || <Boxes size={iconSize ?? Math.round(size / 2)} />;

  return (
    <Avatar
      alt={displayName ?? ''}
      imgProps={{ style: { objectFit: 'contain' } }}
      src={url}
      sx={(theme) => ({
        ...(url
          ? {
              bgcolor: 'background.paper',
              border: hairline(theme),
              borderColor: 'divider',
            }
          : { bgcolor: 'primary.light' }),
        color: 'primary.contrastText',
        flexShrink: 0,
        fontSize: fontSize ?? Math.round(size / 2.5),
        height: size,
        width: size,
      })}
      variant="rounded"
    >
      {fallback}
    </Avatar>
  );
}
