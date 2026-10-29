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

import { cleanup, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useObjectUrl } from './useObjectUrl';

const created: Blob[] = [];
const revoked: string[] = [];
const original = { create: URL.createObjectURL, revoke: URL.revokeObjectURL };

beforeEach(() => {
  created.length = 0;
  revoked.length = 0;
  URL.createObjectURL = vi.fn((blob: Blob) => {
    created.push(blob);
    return `blob:test/${created.length}`;
  });
  URL.revokeObjectURL = vi.fn((url: string) => {
    revoked.push(url);
  });
});

afterEach(() => {
  // Unmounts while the stubs are still in place, so the hook's cleanup can call them.
  cleanup();
  URL.createObjectURL = original.create;
  URL.revokeObjectURL = original.revoke;
});

describe('useObjectUrl', () => {
  it('has no URL without a blob', () => {
    const { result } = renderHook(() => useObjectUrl(undefined));

    expect(result.current).toBeUndefined();
    expect(created).toHaveLength(0);
  });

  it('makes a URL for the blob and releases it when the blob changes or the component goes', () => {
    const first = new Blob(['a']);
    const second = new Blob(['b']);
    const { rerender, result, unmount } = renderHook(({ blob }) => useObjectUrl(blob), {
      initialProps: { blob: first as Blob | undefined },
    });
    expect(result.current).toBe('blob:test/1');

    rerender({ blob: second });
    expect(result.current).toBe('blob:test/2');
    expect(revoked).toEqual(['blob:test/1']);

    unmount();
    expect(revoked).toEqual(['blob:test/1', 'blob:test/2']);
  });

  it('drops the URL, releasing it, when the blob goes away', () => {
    const { rerender, result } = renderHook(({ blob }) => useObjectUrl(blob), {
      initialProps: { blob: new Blob(['a']) as Blob | undefined },
    });

    rerender({ blob: undefined });

    expect(result.current).toBeUndefined();
    expect(revoked).toEqual(['blob:test/1']);
  });

  it('keeps the URL for as long as the same blob is held, however often it renders', () => {
    const blob = new Blob(['a']);
    const { rerender } = renderHook(() => useObjectUrl(blob));

    rerender();
    rerender();

    expect(created).toHaveLength(1);
    expect(revoked).toHaveLength(0);
  });
});
