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

import { Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

import { useApiPublications } from '@/api/resources/apiPublications';
import { thumbnailApiType, type ListableApi } from './apiListItem';

const messages = defineMessages({
  unpublished: {
    id: 'apiListPage.delete.unpublished',
    defaultMessage:
      'It is also unpublished from {count, plural, one {# API Portal} other {# API Portals}}, and consumers there lose access to it.',
    description: 'Shown in the delete-API dialog when the API is live on one or more API Portals.',
  },
});

// Enough to read every portal of an organization in one page, as the overview's banner does.
const LIST_LIMIT = 100;

/**
 * The API Portals a delete takes the API off: every portal where it is published or
 * deprecated. Renders nothing while loading, on error, or when it is on no portal, so
 * the dialog's own message stands alone.
 */
export function DeleteApiImpact({ api }: { api: ListableApi }) {
  const publicationsQuery = useApiPublications(thumbnailApiType(api), api.id, { limit: LIST_LIMIT });
  if (publicationsQuery.isPlaceholderData) return null;
  const count =
    publicationsQuery.data?.list.filter(
      (publication) => publication.status === 'PUBLISHED' || publication.status === 'DEPRECATED',
    ).length ?? 0;
  if (count === 0) return null;
  return (
    <Typography color="text.secondary" variant="body2">
      <FormattedMessage {...messages.unpublished} values={{ count }} />
    </Typography>
  );
}
