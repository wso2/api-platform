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

import type { ApiDocumentType } from '@/api/resources/apiDocuments';
import { asApiType, type FeatureDimensions } from '@/featureUsage';
import { isCustomDocumentType } from './documentTypes';

type DocumentationDimensions = FeatureDimensions<'apip.manage-apis.api-documentation'>;

const DOCUMENT_TYPE_DIMENSIONS: Record<
  ApiDocumentType,
  NonNullable<DocumentationDimensions['document_type']>
> = {
  HowTo: 'how-to',
  Samples: 'samples',
  PublicForum: 'public-forum',
  SupportForum: 'support-forum',
  Other: 'other',
};

/** The taxonomy's `api_type` for an `{apiType}` path segment; any other segment is not reported. */
export const documentationApiType = (apiType: string): DocumentationDimensions['api_type'] =>
  asApiType(apiType);

/**
 * The taxonomy's `document_type` for a stored document type. A custom type is
 * the user's own name, so it is reported only as `other`; an unknown (empty)
 * type is not reported.
 */
export const documentationDocumentType = (
  type: string | undefined,
): DocumentationDimensions['document_type'] => {
  if (!type) return undefined;
  return isCustomDocumentType(type) ? 'other' : DOCUMENT_TYPE_DIMENSIONS[type as ApiDocumentType];
};
