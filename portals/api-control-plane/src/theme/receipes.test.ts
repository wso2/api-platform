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

import type { Theme } from '@wso2/oxygen-ui';
import { describe, expect, it } from 'vitest';

import { iconTileSx, inheritedPillSx, tintedPillSx } from './receipes';

/** The block the recipes put their `color-mix()` styles in. */
const SUPPORTS = '@supports (background: color-mix(in srgb, red, blue))' as const;

/** Just enough of a theme for the recipes: palette, hairline and radius tokens. */
const theme = {
  border: { style: 'solid', width: '1px' },
  palette: {
    background: { paper: 'var(--paper)' },
    divider: 'var(--divider)',
    primary: { main: 'var(--primary)' },
    text: { primary: 'var(--text)', secondary: 'var(--muted)' },
  },
  shape: { borderRadius: 8 },
} as unknown as Theme;

describe('iconTileSx', () => {
  it('is a square of the requested size with a primary-tinted gradient', () => {
    const sx = iconTileSx(theme, 36);

    expect(sx.height).toBe(36);
    expect(sx.width).toBe(36);
    // A plain base, then the tint only where `color-mix()` exists.
    expect(sx.background).toBe('var(--paper)');
    const tinted = sx[SUPPORTS];
    expect(tinted.background).toContain('radial-gradient');
    expect(tinted.background).toContain('var(--primary)');
    expect(tinted.color).toContain('var(--muted)');
  });

  it('rounds its corners in proportion to its size', () => {
    expect(iconTileSx(theme, 50).borderRadius).toBe('13px');
    expect(iconTileSx(theme, 20).borderRadius).toBe('5px');
  });

  it('defaults to a 40px tile', () => {
    expect(iconTileSx(theme).height).toBe(40);
  });

  it('prefers the CSS-variable palette when the theme has one', () => {
    const withVars = {
      ...theme,
      vars: { palette: { ...theme.palette, primary: { main: 'var(--vars-primary)' } } },
    } as unknown as Theme;

    expect(iconTileSx(withVars)[SUPPORTS].background).toContain('var(--vars-primary)');
  });
});

describe('tintedPillSx', () => {
  it('is a solid, plain-surface chip with a divider edge before any tint is applied', () => {
    const sx = tintedPillSx(theme);

    expect(sx.border).toBe('1px solid');
    expect(sx.background).toBe('var(--paper)');
    expect(sx.borderColor).toBe('var(--divider)');
  });

  it('adds the orange glow and a primary-tinted edge only where color-mix() is supported', () => {
    const tinted = tintedPillSx(theme)[SUPPORTS];

    expect(tinted.background).toContain('radial-gradient');
    expect(tinted.background).toContain('var(--primary)');
    expect(tinted.borderColor).toContain('var(--primary)');
    expect(tinted.borderColor).toContain('var(--divider)');
  });

  it('switches the edge to the primary colour while it is a drop target', () => {
    const sx = tintedPillSx(theme, true);

    expect(sx.borderColor).toBe('var(--primary)');
    expect('borderColor' in sx[SUPPORTS]).toBe(false);
  });
});

describe('inheritedPillSx', () => {
  it('is a dashed hairline on the bare surface', () => {
    const sx = inheritedPillSx(theme);

    expect(sx.borderStyle).toBe('dashed');
    expect(sx.borderColor).toBe('var(--divider)');
    expect('background' in sx).toBe(false);
  });
});
