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
import { GraphiqlThemeStyles, graphiqlVariables, toHslChannels } from './GraphiqlThemeStyles';

describe('toHslChannels', () => {
  it('converts hex and rgb colours to GraphiQL\'s bare "h, s%, l%" channels', () => {
    expect(toHslChannels('#ff0000')).toBe('0, 100%, 50%');
    expect(toHslChannels('#fa7b3f')).toBe('19, 95%, 61%');
    expect(toHslChannels('rgb(0, 128, 0)')).toBe('120, 100%, 25%');
    expect(toHslChannels('#ffffff')).toBe('0, 0%, 100%');
  });

  it('drops alpha, since GraphiQL composes the channels into an opaque hsl()', () => {
    expect(toHslChannels('#ffffffc5')).toBe('0, 0%, 100%');
    expect(toHslChannels('rgba(0, 0, 0, 0.5)')).toBe('0, 0%, 0%');
  });

  it('passes hsl colours through as channels', () => {
    expect(toHslChannels('hsl(210, 50%, 40%)')).toBe('210, 50%, 40%');
  });
});

describe('graphiqlVariables', () => {
  it('maps the console palette onto the GraphiQL token roles the schema explorer uses', () => {
    const palette = {
      background: { paper: '#ffffff' },
      error: { main: '#d32f2f' },
      info: { main: '#0288d1' },
      primary: { main: '#fa7b3f' },
      success: { main: '#2e7d32' },
      text: { primary: '#40404b' },
      warning: { main: '#ed6c02' },
    } as unknown as Parameters<typeof graphiqlVariables>[0];

    const vars = graphiqlVariables(palette, '"Inter Variable", sans-serif');

    expect(vars['--color-primary']).toBe(toHslChannels('#fa7b3f'));
    // Field/type names in GraphiQL's docs use `info`, as the explorer colours types.
    expect(vars['--color-info']).toBe(toHslChannels('#0288d1'));
    // Argument names use plain text, as the explorer sets them.
    expect(vars['--color-secondary']).toBe(toHslChannels('#40404b'));
    expect(vars['--color-base']).toBe('0, 0%, 100%');
    expect(vars['--font-family']).toBe('"Inter Variable", sans-serif');
  });
});

describe('GraphiqlThemeStyles', () => {
  it('injects GraphiQL variables scoped to the app colour scheme, outranking GraphiQL\'s own', () => {
    renderWithProviders(<GraphiqlThemeStyles />);

    const css = [...document.querySelectorAll('style')].map((s) => s.textContent ?? '').join('\n');
    expect(css).toMatch(/html\[data-color-scheme='light'\] \.graphiql-container\.graphiql-container/);
    expect(css).toMatch(/\[data-radix-popper-content-wrapper\]\[data-radix-popper-content-wrapper\]/);
    expect(css).toMatch(/--color-primary:\s*\d+,\s*\d+%,\s*\d+%/);
    // Type links follow `info` (as the console explorer colours types), not
    // GraphiQL's `warning`, which it also uses for deprecation notices.
    expect(css).toMatch(/a\.graphiql-doc-explorer-type-name\{color:hsl\(var\(--color-info\)\);?\}/);
  });
});
