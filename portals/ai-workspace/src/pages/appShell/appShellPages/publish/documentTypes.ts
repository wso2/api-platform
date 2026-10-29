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

import { defineMessages, type IntlShape, type MessageDescriptor } from 'react-intl';

const messages = defineMessages({
  howTo: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.type.howTo',
    defaultMessage: 'How To',
  },
  samples: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.type.samples',
    defaultMessage: 'Samples & SDK',
  },
  publicForum: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.type.publicForum',
    defaultMessage: 'Public Forum',
  },
  supportForum: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.type.supportForum',
    defaultMessage: 'Support Forum',
  },
  other: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.documents.type.other',
    defaultMessage: 'Other',
  },
});

/** The fixed document types, in the order they are grouped. */
export const DOCUMENT_TYPES = ['HowTo', 'Samples', 'PublicForum', 'SupportForum', 'Other'] as const;

const LABELS: Record<(typeof DOCUMENT_TYPES)[number], MessageDescriptor> = {
  HowTo: messages.howTo,
  Samples: messages.samples,
  PublicForum: messages.publicForum,
  SupportForum: messages.supportForum,
  Other: messages.other,
};

const FIXED_TYPES = new Set<string>(DOCUMENT_TYPES);

/** The server stores a custom type as the bare name typed ("FAQ"), so anything outside the fixed set is custom. */
export const isCustomDocumentType = (type: string): boolean => !FIXED_TYPES.has(type);

/** A fixed type's translated label, or a custom type's name as stored. */
export const documentTypeName = (intl: IntlShape, type: string): string =>
  isCustomDocumentType(type) && type
    ? type
    : intl.formatMessage(LABELS[type as keyof typeof LABELS] ?? messages.other);
