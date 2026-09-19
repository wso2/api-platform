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

import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { PublicationSummaryItem } from '@/api/resources/apiPublications';

const messages = defineMessages({
  draft: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationDisplay.draft',
    defaultMessage: 'Draft',
    description: 'Status chip: this API has unpublished changes saved as a draft on this portal.',
  },
  published: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationDisplay.published',
    defaultMessage: 'Published',
    description: 'Status chip: this API is live on this portal.',
  },
  deprecated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationDisplay.deprecated',
    defaultMessage: 'Deprecated',
    description: 'Status chip: this API is still listed on this portal but flagged as deprecated.',
  },
});

export type ChipColor = 'default' | 'error' | 'info' | 'secondary' | 'success' | 'warning';

/**
 * The one status mark shown per portal card. A pending draft outranks the
 * live publication state — "there are unpublished changes here" is what the
 * caller needs to notice first, even on an already-published portal — so it
 * is checked before `status`. `NOT_PUBLISHED` with no draft renders nothing:
 * an untouched portal needs no badge at all.
 */
export const publicationChipMeta = (
  item: Pick<PublicationSummaryItem, 'draftUpdatedAt' | 'status'>,
): { color: ChipColor; label: MessageDescriptor } | null => {
  if (item.draftUpdatedAt) return { color: 'secondary', label: messages.draft };
  if (item.status === 'PUBLISHED') return { color: 'success', label: messages.published };
  if (item.status === 'DEPRECATED') return { color: 'warning', label: messages.deprecated };
  return null;
};
