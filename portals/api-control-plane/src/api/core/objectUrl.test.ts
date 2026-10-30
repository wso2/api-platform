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

import { renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useObjectUrl } from './objectUrl';

const created: Blob[] = [];
const revoked: string[] = [];

beforeEach(() => {
  vi.useFakeTimers();
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
  vi.useRealTimers();
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
    vi.runAllTimers();
    expect(revoked).toEqual(['blob:test/1']);

    unmount();
    vi.runAllTimers();
    expect(revoked).toEqual(['blob:test/1', 'blob:test/2']);
  });

  it('never hands back the previous blob\'s URL for a new blob', () => {
    const first = new Blob(['a']);
    const second = new Blob(['b']);
    const seen: Array<{ blob: Blob; url: string | undefined }> = [];
    const { rerender } = renderHook(
      ({ blob }) => {
        const url = useObjectUrl(blob);
        seen.push({ blob, url });
        return url;
      },
      { initialProps: { blob: first } },
    );

    rerender({ blob: second });

    expect(seen.filter((render) => render.blob === second).every((render) => render.url !== 'blob:test/1')).toBe(true);
  });

  it('gives every holder of the same blob one URL, revoked only after the last lets go', () => {
    const blob = new Blob(['a']);
    const one = renderHook(() => useObjectUrl(blob));
    const two = renderHook(() => useObjectUrl(blob));

    expect(one.result.current).toBe('blob:test/1');
    expect(two.result.current).toBe('blob:test/1');
    expect(created).toHaveLength(1);

    one.unmount();
    vi.runAllTimers();
    expect(revoked).toEqual([]);

    two.unmount();
    vi.runAllTimers();
    expect(revoked).toEqual(['blob:test/1']);
  });

  it('keeps the URL through an immediate remount', () => {
    const blob = new Blob(['a']);
    const first = renderHook(() => useObjectUrl(blob));
    first.unmount();
    const second = renderHook(() => useObjectUrl(blob));
    vi.runAllTimers();

    expect(second.result.current).toBe('blob:test/1');
    expect(revoked).toEqual([]);
  });
});
