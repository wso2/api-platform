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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { readStoredCallMode, writeStoredCallMode } from './callMode';

const STORAGE_KEY = 'apicp.test.callMode';

beforeEach(() => {
  window.localStorage.clear();
  vi.restoreAllMocks();
});

describe('readStoredCallMode', () => {
  it('relays when nothing has been stored', () => {
    // The relay is what makes a cloud-managed gateway testable at all, so a
    // first visit must not land on the mode that needs a CORS policy.
    expect(readStoredCallMode()).toBe('proxy');
  });

  it('restores a stored choice', () => {
    writeStoredCallMode('direct');

    expect(readStoredCallMode()).toBe('direct');
  });

  it('falls back to the relay for a value it does not recognise', () => {
    // Anything could be under this key — an older build, a hand-edited value.
    // Trusting it would hand swagger a mode the plugin cannot act on.
    window.localStorage.setItem(STORAGE_KEY, 'tunnel');

    expect(readStoredCallMode()).toBe('proxy');
  });

  it('falls back to the relay when storage itself throws', () => {
    // Private mode and blocked site data both make getItem throw rather than
    // return null; the console still has to render.
    vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('access denied');
    });

    expect(readStoredCallMode()).toBe('proxy');
  });
});

describe('writeStoredCallMode', () => {
  it('swallows a storage failure rather than breaking the switch', () => {
    // The in-session choice still applies; only its persistence is lost.
    vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });

    expect(() => writeStoredCallMode('direct')).not.toThrow();
  });
});
