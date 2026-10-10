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
import { MarkdownView } from './MarkdownView';

describe('MarkdownView', () => {
  it('renders document structure as elements', () => {
    renderWithProviders(<MarkdownView source={'# Guide\n\nRead the [docs](https://example.com).\n\n- step'} />);

    expect(screen.getByRole('heading', { name: 'Guide' })).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'docs' });
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.getByRole('listitem')).toHaveTextContent('step');
  });

  it('shows embedded HTML as text instead of injecting it', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'<img src=x onerror="alert(1)"><script>alert(1)</script>'} />
    );

    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('script')).toBeNull();
    expect(screen.getByText(/<script>alert\(1\)<\/script>/)).toBeInTheDocument();
  });

  it('renders the fallback for an empty document', () => {
    renderWithProviders(<MarkdownView emptyFallback={<p>Nothing here</p>} source="" />);

    expect(screen.getByText('Nothing here')).toBeInTheDocument();
  });
});
