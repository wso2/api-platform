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

import { describe, expect, it } from 'vitest';
import type { MessageDescriptor } from 'react-intl';

import { renderWithProviders, screen } from '@/test/utils';
import { SelectionSummary } from './SelectionSummary';

// Plain descriptors, not `defineMessages`, so the extractor leaves these test strings out of the catalog.
const messages: Record<'published' | 'selected', MessageDescriptor> = {
  published: {
    id: 'test.SelectionSummary.published',
    defaultMessage: '{count, plural, one {<n>#</n> item published} other {<n>#</n> items published}}',
  },
  selected: { id: 'test.SelectionSummary.selected', defaultMessage: '{count} of {total} items selected' },
};

/** Matches text split across the summary's nested count element. */
const summaryText = (text: string) =>
  screen.getAllByText((_content, element) => element?.textContent === text)[0];

describe('SelectionSummary', () => {
  it('says how many of the offered items are selected in the draft', () => {
    renderWithProviders(<SelectionSummary count={2} published={messages.published} selected={messages.selected} total={5} />);

    expect(summaryText('2 of 5 items selected')).toBeInTheDocument();
  });

  it('says how many items the live listing holds in the published view, ignoring the total', () => {
    renderWithProviders(
      <SelectionSummary count={1} published={messages.published} readOnly selected={messages.selected} total={5} />,
    );
    expect(summaryText('1 item published')).toBeInTheDocument();
  });
});
