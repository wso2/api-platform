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

import type { Theme } from '@wso2/oxygen-ui';
import { describe, expect, it } from 'vitest';

import { withControlPlaneTypography } from './publishTypography';

const outer = {
  palette: { mode: 'light' },
  typography: {
    fontFamily: 'Inter',
    body1: { fontFamily: 'Inter', fontSize: '1rem', fontWeight: 400, letterSpacing: '0.01em' },
    caption: { fontFamily: 'Inter', fontSize: '0.79rem' },
    button: { textTransform: 'none' },
  },
} as unknown as Theme;

describe('withControlPlaneTypography', () => {
  it('sets the control plane size, weight and line height on each variant', () => {
    const { typography } = withControlPlaneTypography(outer);

    expect(typography.body1).toMatchObject({ fontSize: '0.875rem', fontWeight: 400, lineHeight: 1.5 });
    expect(typography.caption).toMatchObject({ fontSize: '0.6875rem', lineHeight: 1.66 });
  });

  it('keeps what the workspace theme sets besides those three', () => {
    const { typography } = withControlPlaneTypography(outer);

    expect(typography.body1).toMatchObject({ fontFamily: 'Inter', letterSpacing: '0.01em' });
  });

  it('leaves variants and settings it does not scale alone', () => {
    const { typography, palette } = withControlPlaneTypography(outer);

    expect(typography.button).toEqual({ textTransform: 'none' });
    expect(typography.fontFamily).toBe('Inter');
    expect(palette).toBe(outer.palette);
  });

  it('does not change the theme it is given', () => {
    withControlPlaneTypography(outer);

    expect((outer.typography as unknown as Record<string, { fontSize: string }>).body1.fontSize).toBe('1rem');
  });
});
