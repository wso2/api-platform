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

import type { ApiDocumentType } from '@/api/resources/apiDocuments';

const messages = defineMessages({
  howTo: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentTypes.howTo',
    defaultMessage: 'How To',
    description: 'Document type: a step-by-step guide.',
  },
  sampleSdk: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentTypes.sampleSdk',
    defaultMessage: 'Samples & SDK',
    description: 'Document type: code samples and client SDKs.',
  },
  publicForum: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentTypes.publicForum',
    defaultMessage: 'Public Forum',
    description: 'Document type: a link to or notes about a public community forum.',
  },
  supportForum: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentTypes.supportForum',
    defaultMessage: 'Support Forum',
    description: 'Document type: a link to or notes about a support channel.',
  },
  other: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentTypes.other',
    defaultMessage: 'Other',
    description: 'Document type: anything that fits no other type.',
  },
});

/**
 * The user-authored document types, in the order they are offered and grouped.
 * `satisfies` keeps this list in step with the spec's enum: a type added to the
 * spec without a label here fails the type check rather than rendering blank.
 */
export const DOCUMENT_TYPES = [
  'HOW_TO',
  'SAMPLE_SDK',
  'PUBLIC_FORUM',
  'SUPPORT_FORUM',
  'OTHER',
] as const satisfies readonly ApiDocumentType[];

const LABELS: Record<ApiDocumentType, MessageDescriptor> = {
  HOW_TO: messages.howTo,
  SAMPLE_SDK: messages.sampleSdk,
  PUBLIC_FORUM: messages.publicForum,
  SUPPORT_FORUM: messages.supportForum,
  OTHER: messages.other,
};

/** Label for a document type; an unknown value from a newer server falls back to "Other". */
export const documentTypeLabel = (type: string): MessageDescriptor =>
  LABELS[type as ApiDocumentType] ?? messages.other;

export const DEFAULT_DOCUMENT_TYPE: ApiDocumentType = 'HOW_TO';
