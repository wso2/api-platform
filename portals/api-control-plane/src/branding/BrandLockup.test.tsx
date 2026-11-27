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

import { BrandLockup } from './BrandLockup';

describe('BrandLockup', () => {
  it('shows only the logo, named "Choreo API Management" for assistive technology', () => {
    renderWithProviders(<BrandLockup height={32} />);

    expect(screen.getByRole('img', { name: 'Choreo API Management' })).toBeInTheDocument();
    // The artwork carries the name: no second, typeset copy beside it.
    expect(screen.queryByText('Choreo API Management')).not.toBeInTheDocument();
  });

  it('takes the logo height as a CSS length', () => {
    renderWithProviders(<BrandLockup height="1.75rem" />);

    expect(document.querySelector('img')).toHaveStyle({ height: '1.75rem' });
  });
});
