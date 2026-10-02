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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import type { DraftFormValues } from '../utils/publicationForm';
import { ApiDetailsTab } from './ApiDetailsTab';

const VALUES: DraftFormValues = {
  displayName: 'Loans',
  version: '1.0.0',
  description: 'Manage loans.',
  productionUrl: 'https://api.example.com',
  sandboxUrl: 'https://sandbox.example.com',
  subscriptionPlanIds: [],
};

describe('ApiDetailsTab', () => {
  it('reports each edit with every other value kept, and a blur with the field it left', async () => {
    const onChange = vi.fn();
    const onBlurField = vi.fn();
    const { user } = renderWithProviders(
      <ApiDetailsTab onBlurField={onBlurField} onChange={onChange} values={VALUES} />,
    );

    const name = screen.getByDisplayValue('Loans');
    await user.type(name, 'x');
    await user.tab();

    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, displayName: 'Loansx' });
    expect(onBlurField).toHaveBeenCalledWith('displayName');
  });

  it('shows every value as read-only when asked to, and never reports an edit', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(<ApiDetailsTab onChange={onChange} readOnly values={VALUES} />);

    // `ApiDetailsTab` only ever renders the string fields; `subscriptionPlanIds`
    // belongs to a different tab and has no input here.
    const stringValues = Object.values(VALUES).filter((value): value is string => typeof value === 'string');
    for (const value of stringValues) {
      expect(screen.getByDisplayValue(value)).toHaveAttribute('readonly');
    }
    await user.type(screen.getByDisplayValue('Loans'), 'x');

    expect(onChange).not.toHaveBeenCalled();
  });

  it('offers a description hint only while the field can be edited', () => {
    const empty = { ...VALUES, description: '' };
    const { rerender } = renderWithProviders(<ApiDetailsTab values={empty} />);
    expect(screen.getByRole('textbox', { name: 'Description' })).toHaveAttribute('placeholder');

    rerender(<ApiDetailsTab readOnly values={empty} />);
    expect(screen.getByRole('textbox', { name: 'Description' })).not.toHaveAttribute('placeholder');
  });
});
