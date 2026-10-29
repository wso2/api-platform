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

import { createIntl } from 'react-intl';
import { describe, expect, it } from 'vitest';
import type { MCPProxyDocument } from '../../../../utils/types';
import { groupDocumentsByType } from './documentGroups';

const intl = createIntl({ locale: 'en', messages: {} });

const doc = (id: string, type: string): MCPProxyDocument => ({ displayName: id, id, type });

describe('groupDocumentsByType', () => {
  it('lists fixed types in their usual order, then custom types alphabetically, then Other', () => {
    const groups = groupDocumentsByType(
      [doc('a', 'Other'), doc('b', 'Release notes'), doc('c', 'HowTo'), doc('d', 'FAQ'), doc('e', 'Samples')],
      intl
    );
    expect(groups.map((group) => group.label)).toEqual(['How To', 'Samples & SDK', 'FAQ', 'Release notes', 'Other']);
  });

  it("keeps a type's documents in the order given and leaves out empty groups", () => {
    const groups = groupDocumentsByType([doc('second', 'HowTo'), doc('first', 'HowTo')], intl);
    expect(groups).toHaveLength(1);
    expect(groups[0].items.map((item) => item.id)).toEqual(['second', 'first']);
  });

  it('puts a document with no type under Other', () => {
    expect(groupDocumentsByType([doc('a', '')], intl).map((group) => group.type)).toEqual(['Other']);
  });

  it('labels every fixed type', () => {
    const types = ['HowTo', 'Samples', 'PublicForum', 'SupportForum', 'Other'];
    expect(groupDocumentsByType(types.map((type) => doc(type, type)), intl).map((group) => group.label)).toEqual([
      'How To',
      'Samples & SDK',
      'Public Forum',
      'Support Forum',
      'Other',
    ]);
  });
});
