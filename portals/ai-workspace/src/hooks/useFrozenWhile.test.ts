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

import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { useFrozenWhile } from './useFrozenWhile';

describe('useFrozenWhile', () => {
  it('follows the value while it is not frozen', () => {
    const { result, rerender } = renderHook(({ value, frozen }) => useFrozenWhile(value, frozen), {
      initialProps: { value: 'a', frozen: false },
    });
    expect(result.current).toBe('a');

    rerender({ value: 'b', frozen: false });
    expect(result.current).toBe('b');
  });

  it('holds the last shown value while frozen, however the value changes', () => {
    const { result, rerender } = renderHook(({ value, frozen }) => useFrozenWhile(value, frozen), {
      initialProps: { value: 'a', frozen: false },
    });

    rerender({ value: 'a', frozen: true });
    rerender({ value: 'b', frozen: true });
    rerender({ value: 'c', frozen: true });

    expect(result.current).toBe('a');
  });

  it('shows the latest value once the freeze ends', () => {
    const { result, rerender } = renderHook(({ value, frozen }) => useFrozenWhile(value, frozen), {
      initialProps: { value: 'a', frozen: false },
    });

    rerender({ value: 'b', frozen: true });
    expect(result.current).toBe('a');

    rerender({ value: 'b', frozen: false });
    expect(result.current).toBe('b');
  });
});
