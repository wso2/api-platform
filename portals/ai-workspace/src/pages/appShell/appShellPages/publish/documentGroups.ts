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

import type { IntlShape } from 'react-intl';
import type { MCPProxyDocument } from '../../../../utils/types';
import { DOCUMENT_TYPES, documentTypeName, isCustomDocumentType } from './documentTypes';

/** The fixed catch-all type; documents with no more specific type group here, last. */
const PLAIN_OTHER = 'Other';

type DocumentGroup = {
  items: MCPProxyDocument[];
  label: string;
  type: string;
};

/** Groups documents by type: fixed types in their usual order, custom types alphabetically, then "Other". Empty groups are omitted. */
export const groupDocumentsByType = (documents: readonly MCPProxyDocument[], intl: IntlShape): DocumentGroup[] => {
  const fixed = DOCUMENT_TYPES.filter((type) => type !== PLAIN_OTHER);
  const buckets = new Map<string, MCPProxyDocument[]>();
  for (const document of documents) {
    const key =
      (fixed as readonly string[]).includes(document.type) || (document.type && isCustomDocumentType(document.type))
        ? document.type
        : PLAIN_OTHER;
    const bucket = buckets.get(key);
    if (bucket) bucket.push(document);
    else buckets.set(key, [document]);
  }
  const custom = [...buckets.keys()]
    .filter((type) => type !== PLAIN_OTHER && isCustomDocumentType(type))
    .sort((a, b) => documentTypeName(intl, a).localeCompare(documentTypeName(intl, b)));
  return [...fixed, ...custom, PLAIN_OTHER]
    .filter((type) => buckets.has(type))
    .map((type) => ({ items: buckets.get(type)!, label: documentTypeName(intl, type), type }));
};
