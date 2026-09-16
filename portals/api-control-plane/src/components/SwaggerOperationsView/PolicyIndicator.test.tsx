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
import { PolicyIndicator, policyColor, policyInitials } from './PolicyIndicator';

describe('policyInitials', () => {
  it('initials the words after the namespace, not the namespace itself', () => {
    // Every `mediation.*` policy would otherwise read "ME".
    expect(policyInitials('mediation.rewrite_resource_path')).toBe('RR');
    expect(policyInitials('mediation.set_header')).toBe('SH');
  });

  it('takes two letters from a single-word name', () => {
    expect(policyInitials('cors')).toBe('CO');
  });

  it('splits on spaces, hyphens and camelCase alike', () => {
    expect(policyInitials('Rate Limiting')).toBe('RL');
    expect(policyInitials('rate-limiting')).toBe('RL');
    expect(policyInitials('rateLimiting')).toBe('RL');
  });

  it('degrades to a placeholder rather than throwing on an empty name', () => {
    expect(policyInitials('')).toBe('?');
  });
});

describe('policyColor', () => {
  it('is stable for a name, so one policy is one colour everywhere', () => {
    expect(policyColor('mediation.cors')).toBe(policyColor('mediation.cors'));
  });

  it('does not depend on position in a list', () => {
    // The regression this guards: colouring by index recoloured every circle
    // when a policy was attached above it.
    const before = ['a.one', 'b.two'].map(policyColor);
    const after = ['c.three', 'a.one', 'b.two'].map(policyColor);
    expect(after.slice(1)).toEqual(before);
  });
});

describe('PolicyIndicator', () => {
  it('renders nothing when no policies are attached', () => {
    const { container } = renderWithProviders(<PolicyIndicator policies={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('draws one initialled circle per attached policy', () => {
    renderWithProviders(
      <PolicyIndicator
        policies={[
          { name: 'mediation.cors', version: 'v1' },
          { name: 'mediation.rate_limit', version: 'v2' },
        ]}
      />,
    );

    expect(screen.getByText('CO')).toBeInTheDocument();
    expect(screen.getByText('RL')).toBeInTheDocument();
  });

  it('collapses the tail into one +N circle so a long list cannot crowd the row', () => {
    renderWithProviders(
      <PolicyIndicator
        policies={['one', 'two', 'three', 'four', 'five', 'six'].map((name) => ({
          name: `mediation.${name}`,
          version: 'v1',
        }))}
      />,
    );

    expect(screen.getByText('+2')).toBeInTheDocument();
    // The first four are drawn; the fifth is only named in the overflow tooltip.
    expect(screen.getByText('ON')).toBeInTheDocument();
    expect(screen.getByText('FO')).toBeInTheDocument();
    expect(screen.queryByText('FI')).not.toBeInTheDocument();
  });
});
