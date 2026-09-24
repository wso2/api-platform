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

// api-portal's own app mount prefix (portals/api-portal/src/utils/constants.js:
// PORTAL_BASE_PATH) — hardcoded there too, not per-deployment configurable.
const PORTAL_APP_BASE_PATH = 'api-portal';
// Handle of the view every org is seeded with. Not a real per-API value: the
// publication summary carries no field for which view an API actually resolved
// into on the portal (that's driven by portal-side label matching), so this is
// a best-effort assumption, not a genuine parameter.
const DEFAULT_PORTAL_VIEW_HANDLE = 'default';
const PORTAL_VIEWS_SEGMENT = 'views';
const PORTAL_API_SEGMENT = 'api';

/**
 * api-portal serves an API's own page at
 * `/{PORTAL_APP_BASE_PATH}/{orgHandle}/views/{viewHandle}/api/{apiHandle}`.
 * The publication summary carries neither the portal-side view an API
 * resolved into nor confirmation that the portal's org handle matches this
 * console's — so this is a best-effort link: it assumes the seeded default
 * view, and that the portal reuses this console's API/org handles as its own,
 * which holds unless an operator has since renamed them on the portal. Good
 * enough to open the right page in the common case; a renamed view/org handle
 * would need surfacing a real field to fix properly.
 */
export const buildViewInPortalUrl = (portalUrl: string, orgHandle: string, apiHandle: string): string => {
  const url = new URL(portalUrl);
  url.pathname = [
    url.pathname.replace(/\/+$/, ''),
    PORTAL_APP_BASE_PATH,
    encodeURIComponent(orgHandle),
    PORTAL_VIEWS_SEGMENT,
    DEFAULT_PORTAL_VIEW_HANDLE,
    PORTAL_API_SEGMENT,
    encodeURIComponent(apiHandle),
  ].join('/');
  return url.toString();
};
