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

// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getActiveColorScheme, subscribeToColorSchemeChanges } from './colorScheme';

const mockPrefersDark = (matches: boolean) => {
  const listeners = new Set<() => void>();
  const mediaQuery = {
    matches,
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
  };
  window.matchMedia = vi.fn(() => mediaQuery) as unknown as typeof window.matchMedia;
  return { mediaQuery, listeners };
};

const resetDom = () => {
  for (const el of [document.documentElement, document.body]) {
    el.removeAttribute('class');
    for (const attr of ['data-mui-color-scheme', 'data-color-scheme', 'data-theme']) {
      el.removeAttribute(attr);
    }
  }
};

beforeEach(() => {
  resetDom();
  mockPrefersDark(false);
});
afterEach(resetDom);

describe('getActiveColorScheme', () => {
  it('follows the OS preference when the DOM says nothing', () => {
    expect(getActiveColorScheme()).toBe('light');
    mockPrefersDark(true);
    expect(getActiveColorScheme()).toBe('dark');
  });

  it('reads each scheme attribute on <html>', () => {
    for (const attr of ['data-mui-color-scheme', 'data-color-scheme', 'data-theme']) {
      document.documentElement.setAttribute(attr, 'dark');
      expect(getActiveColorScheme()).toBe('dark');
      document.documentElement.removeAttribute(attr);
    }
  });

  it('reads the dark class', () => {
    document.documentElement.classList.add('dark');
    expect(getActiveColorScheme()).toBe('dark');
  });

  it('ignores attribute values that are not a scheme', () => {
    document.documentElement.setAttribute('data-theme', 'sepia');
    expect(getActiveColorScheme()).toBe('light');
  });

  it('prefers <html> over <body>, and <body> over the OS', () => {
    mockPrefersDark(true);
    document.body.setAttribute('data-theme', 'light');
    expect(getActiveColorScheme()).toBe('light');
    document.documentElement.setAttribute('data-theme', 'dark');
    expect(getActiveColorScheme()).toBe('dark');
  });
});

describe('subscribeToColorSchemeChanges', () => {
  it('reports the current scheme immediately', () => {
    const onChange = vi.fn();
    const unsubscribe = subscribeToColorSchemeChanges(onChange);
    expect(onChange).toHaveBeenCalledWith('light');
    unsubscribe();
  });

  it('reports a DOM toggle', async () => {
    const onChange = vi.fn();
    const unsubscribe = subscribeToColorSchemeChanges(onChange);
    document.documentElement.setAttribute('data-theme', 'dark');
    await vi.waitFor(() => expect(onChange).toHaveBeenLastCalledWith('dark'));
    unsubscribe();
  });

  it('reports an OS preference change', () => {
    const { listeners } = mockPrefersDark(false);
    const onChange = vi.fn();
    const unsubscribe = subscribeToColorSchemeChanges(onChange);
    expect(listeners.size).toBe(1);
    mockPrefersDark(true);
    listeners.forEach((fn) => fn());
    expect(onChange).toHaveBeenLastCalledWith('dark');
    unsubscribe();
  });

  it('stops reporting once unsubscribed', async () => {
    const { listeners } = mockPrefersDark(false);
    const onChange = vi.fn();
    subscribeToColorSchemeChanges(onChange)();
    expect(listeners.size).toBe(0);
    onChange.mockClear();
    document.documentElement.setAttribute('data-theme', 'dark');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onChange).not.toHaveBeenCalled();
  });
});
