/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  CLOUD_INSIGHTS_EXTENSION_IDS,
  filterExtensionsForRuntime,
} from './runtimeFlags';

beforeEach(() => {
  vi.stubGlobal('window', {
    __RUNTIME_CONFIG__: {},
    config: {},
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('filterExtensionsForRuntime', () => {
  const extensions = [
    { id: 'organization-insights' },
    { id: 'project-insights' },
    { id: 'other-feature' },
  ] as const;

  it('drops cloud Insights entries when cloudProxyEnabled is absent', () => {
    const filtered = filterExtensionsForRuntime([...extensions]);

    expect(filtered.map((entry) => entry.id)).toEqual(['other-feature']);
    for (const id of CLOUD_INSIGHTS_EXTENSION_IDS) {
      expect(filtered.some((entry) => entry.id === id)).toBe(false);
    }
  });

  it('keeps cloud Insights entries when cloudProxyEnabled is true', () => {
    (window as Window & { __RUNTIME_CONFIG__?: Record<string, string> })
      .__RUNTIME_CONFIG__ = { cloudProxyEnabled: 'true' };

    const filtered = filterExtensionsForRuntime([...extensions]);

    expect(filtered.map((entry) => entry.id)).toEqual([
      'organization-insights',
      'project-insights',
      'other-feature',
    ]);
  });
});
