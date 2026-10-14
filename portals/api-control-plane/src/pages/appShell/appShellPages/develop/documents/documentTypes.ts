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
  'HowTo',
  'Samples',
  'PublicForum',
  'SupportForum',
  'Other',
] as const satisfies readonly ApiDocumentType[];

const LABELS: Record<ApiDocumentType, MessageDescriptor> = {
  HowTo: messages.howTo,
  Samples: messages.sampleSdk,
  PublicForum: messages.publicForum,
  SupportForum: messages.supportForum,
  Other: messages.other,
};

/** Label for a document type; an unknown value from a newer server falls back to "Other". */
export const documentTypeLabel = (type: string): MessageDescriptor =>
  LABELS[type as ApiDocumentType] ?? messages.other;

export const DEFAULT_DOCUMENT_TYPE: ApiDocumentType = 'HowTo';

/* -------------------------------------------------------------------------- */
/* Custom "Other" types                                                        */
/* -------------------------------------------------------------------------- */

/**
 * `api_documents.type` is VARCHAR(20), stored as `DOC_<name>`, so a custom
 * name has at most 20 − len("DOC_") = 16 **bytes** available.
 */
export const MAX_CUSTOM_TYPE_BYTES = 16;

/** Letters, digits, spaces, hyphens and underscores. */
const CUSTOM_TYPE_PATTERN = /^[\p{L}\p{N} _-]+$/u;

const RESERVED_CUSTOM_TYPE_NAMES = new Set<string>([
  ...DOCUMENT_TYPES.map((t) => t.toUpperCase()),
  'DEFINITION',
  'THUMBNAIL',
]);

const utf8ByteLength = (value: string): number => new TextEncoder().encode(value).length;

export type CustomTypeError = 'required' | 'tooLong' | 'invalid' | 'reserved';

/** Why a custom type name can't be saved, or `undefined` when it can. */
export const validateCustomType = (name: string): CustomTypeError | undefined => {
  const trimmed = name.trim();
  if (!trimmed) return 'required';
  if (!CUSTOM_TYPE_PATTERN.test(trimmed)) return 'invalid';
  if (utf8ByteLength(trimmed) > MAX_CUSTOM_TYPE_BYTES) return 'tooLong';
  if (RESERVED_CUSTOM_TYPE_NAMES.has(trimmed.toUpperCase())) return 'reserved';
  return undefined;
};

const FIXED_TYPES_SET = new Set<string>(DOCUMENT_TYPES);

/**
 * Whether a stored type is a custom one. The server stores a custom type as the
 * bare name the user typed ("FAQ"), so anything outside the fixed set is custom.
 */
export const isCustomDocumentType = (type: string): boolean => !FIXED_TYPES_SET.has(type);

/**
 * What to call a document's type on screen: the translated label of a fixed
 * type, or a custom type's own name exactly as stored.
 */
export const documentTypeName = (intl: IntlShape, type: string): string =>
  isCustomDocumentType(type) && type
    ? type
    : intl.formatMessage(documentTypeLabel(type));
