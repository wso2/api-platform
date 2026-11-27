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
import { CodeView, languageForMediaType } from './CodeView';

describe('CodeView', () => {
  it('shows markup in the text as text, never as elements', () => {
    const hostile = '{"x": "<img src=x onerror=alert(1)>", "y": "<script>alert(2)</script>"}';

    for (const language of ['json', 'markup', 'bash'] as const) {
      const { container, unmount } = renderWithProviders(
        <CodeView label="Response body" language={language} text={hostile} />,
      );

      expect(container.querySelector('img')).toBeNull();
      expect(container.querySelector('script')).toBeNull();
      expect(screen.getByRole('region', { name: 'Response body' })).toHaveTextContent(
        '<img src=x onerror=alert(1)>',
      );
      unmount();
    }
  });

  it('keeps every character of the text once it is coloured', () => {
    const text = '{\n  "a": [1, 2.5, true, null],\n  "b": "x"\n}';

    renderWithProviders(<CodeView label="Body" language="json" text={text} />);

    expect(screen.getByRole('region', { name: 'Body' }).textContent).toBe(text);
  });

  it('shows a different text when it changes', () => {
    const { rerender } = renderWithProviders(
      <CodeView label="Body" language="json" text='{"a":1}' />,
    );

    rerender(<CodeView label="Body" language="json" text='{"b":2}' />);

    expect(screen.getByRole('region', { name: 'Body' })).toHaveTextContent('{"b":2}');
  });

  it('shows a very large text plain, without colouring it', () => {
    const big = `{"k":"${'x'.repeat(300 * 1024)}"}`;

    const { container } = renderWithProviders(<CodeView label="Body" language="json" text={big} />);

    expect(container.querySelector('.token')).toBeNull();
    expect(screen.getByRole('region', { name: 'Body' }).textContent).toHaveLength(big.length);
  });

  it('can be reached and scrolled from the keyboard', () => {
    renderWithProviders(<CodeView label="Body" language="json" text="{}" />);

    expect(screen.getByRole('region', { name: 'Body' })).toHaveAttribute('tabindex', '0');
  });
});

describe('languageForMediaType', () => {
  it('colours JSON as JSON and everything else as markup', () => {
    expect(languageForMediaType('application/json')).toBe('json');
    expect(languageForMediaType('application/problem+json; charset=utf-8')).toBe('json');
    expect(languageForMediaType('application/xml')).toBe('markup');
    expect(languageForMediaType(undefined)).toBe('markup');
  });
});
