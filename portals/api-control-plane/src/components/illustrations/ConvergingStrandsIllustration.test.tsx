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

import { renderWithProviders } from '@/test/utils';

import { ConvergingStrandsIllustration } from './ConvergingStrandsIllustration';

describe('ConvergingStrandsIllustration', () => {
  it('is decorative: hidden from assistive technology', () => {
    const { container } = renderWithProviders(<ConvergingStrandsIllustration />);

    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('draws strands from both sides', () => {
    const { container } = renderWithProviders(<ConvergingStrandsIllustration />);

    expect(container.querySelectorAll('.strand-left').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.strand-right').length).toBeGreaterThan(0);
  });

  it('gives each drawing its own gradient ids, so two on a page do not share them', () => {
    const { container } = renderWithProviders(
      <>
        <ConvergingStrandsIllustration />
        <ConvergingStrandsIllustration />
      </>,
    );

    const ids = [...container.querySelectorAll('linearGradient, radialGradient')].map(
      (gradient) => gradient.id,
    );
    expect(ids).toHaveLength(6);
    expect(new Set(ids).size).toBe(6);
  });
});
