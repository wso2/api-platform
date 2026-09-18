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

import { renderHook, act } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useNow } from './useNow';

describe('useNow', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts at the current time', () => {
    const { result } = renderHook(() => useNow(1000));

    expect(result.current).toBe(Date.now());
  });

  it('advances as time passes, which is the whole point', () => {
    const { result } = renderHook(() => useNow(1000));
    const start = result.current;

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current).toBe(start + 3000);
  });

  it('does not tick when the interval is zero, so an idle page keeps still', () => {
    const { result } = renderHook(() => useNow(0));
    const start = result.current;

    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    expect(result.current).toBe(start);
  });

  it('clears its timer on unmount', () => {
    const clear = vi.spyOn(window, 'clearInterval');
    const { unmount } = renderHook(() => useNow(1000));

    unmount();

    expect(clear).toHaveBeenCalled();
  });
});
