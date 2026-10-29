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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import { PublicationVersionToggle } from './PublicationVersionToggle';

describe('PublicationVersionToggle', () => {
  it('marks the version on screen as the selected one', () => {
    renderWithProviders(<PublicationVersionToggle onChange={vi.fn()} publishedAvailable value="draft" />);

    expect(screen.getByRole('button', { name: 'Draft' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Published' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('switches to the other version', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(
      <PublicationVersionToggle onChange={onChange} publishedAvailable value="draft" />
    );

    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(onChange).toHaveBeenCalledWith('published');
  });

  it('does not report a click on the version already shown', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(
      <PublicationVersionToggle onChange={onChange} publishedAvailable value="draft" />
    );

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(onChange).not.toHaveBeenCalled();
  });

  it('offers no Published version until the proxy is live', () => {
    renderWithProviders(<PublicationVersionToggle onChange={vi.fn()} publishedAvailable={false} value="draft" />);

    expect(screen.getByRole('button', { name: 'Published' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Draft' })).toBeEnabled();
  });

  it('locks both versions while an action runs', () => {
    renderWithProviders(<PublicationVersionToggle disabled onChange={vi.fn()} publishedAvailable value="draft" />);

    expect(screen.getByRole('button', { name: 'Draft' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Published' })).toBeDisabled();
  });
});
