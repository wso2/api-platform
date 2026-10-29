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

import { afterEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../test/utils';
import OverviewDescription from './OverviewDescription';

const LONG = 'Plans trips. '.repeat(60);

const measuredAs = (scrollHeight: number, clientHeight: number) => {
  vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(scrollHeight);
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(clientHeight);
};

describe('OverviewDescription', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the full text in a tooltip when it is cut off', async () => {
    measuredAs(200, 40);
    const { user } = renderWithProviders(<OverviewDescription text={LONG} />);

    await user.hover(screen.getAllByText(/Plans trips\./)[0]);

    expect(await screen.findByRole('tooltip')).toHaveTextContent(LONG.trim());
  });

  it('has no tooltip when it fits', async () => {
    measuredAs(40, 40);
    const { user } = renderWithProviders(<OverviewDescription text="Plans trips." />);

    await user.hover(screen.getByText('Plans trips.'));

    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('offers no button to expand it', () => {
    measuredAs(200, 40);

    renderWithProviders(<OverviewDescription text={LONG} />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
