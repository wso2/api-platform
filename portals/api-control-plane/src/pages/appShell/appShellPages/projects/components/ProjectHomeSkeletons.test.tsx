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

import { ProjectHomeSkeleton } from './ProjectHomeSkeletons';

describe('ProjectHomeSkeleton', () => {
  it('is marked busy and renders a title, a subtitle and a content block', () => {
    const { container } = renderWithProviders(<ProjectHomeSkeleton />);

    const busy = container.querySelector('[aria-busy="true"]');
    expect(busy).not.toBeNull();
    expect(busy?.children).toHaveLength(3);
  });

  it('carries no text, so it never reads as a second loading label', () => {
    const { container } = renderWithProviders(<ProjectHomeSkeleton />);

    expect(container.textContent).toBe('');
  });
});
