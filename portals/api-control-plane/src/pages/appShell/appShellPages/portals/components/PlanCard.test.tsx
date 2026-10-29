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

import { renderWithProviders, screen } from '@/test/utils';
import { PlanCard } from './PlanCard';

const CHIPS = ['Deleted', 'Inactive', 'Edited'];

const cases = [
  { deleted: false, edited: false, expected: undefined, inactive: false },
  { deleted: false, edited: true, expected: 'Edited', inactive: false },
  { deleted: false, edited: false, expected: 'Inactive', inactive: true },
  { deleted: false, edited: true, expected: 'Inactive', inactive: true },
  { deleted: true, edited: false, expected: 'Deleted', inactive: false },
  { deleted: true, edited: true, expected: 'Deleted', inactive: false },
  { deleted: true, edited: false, expected: 'Deleted', inactive: true },
  { deleted: true, edited: true, expected: 'Deleted', inactive: true },
];

describe('PlanCard status chip', () => {
  for (const readOnly of [false, true]) {
    for (const { expected, ...flags } of cases) {
      const label = Object.entries(flags)
        .filter(([, on]) => on)
        .map(([name]) => name)
        .join(' + ');
      it(`${readOnly ? 'published' : 'draft'}, ${label || 'no state'}: shows ${(!readOnly && expected === 'Edited' ? undefined : expected) ?? 'no chip'}`, () => {
        renderWithProviders(<PlanCard {...flags} displayName="Plan" readOnly={readOnly} selected />);

        const shown = CHIPS.filter((chip) => screen.queryByText(chip, { selector: '.MuiChip-label' }));
        // The draft reads the live plan, so it never says Edited; only the published view does.
        const visible = !readOnly && expected === 'Edited' ? undefined : expected;
        expect(shown).toEqual(visible ? [visible] : []);
      });
    }
  }
});
