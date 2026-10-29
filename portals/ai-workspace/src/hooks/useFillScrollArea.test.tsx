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

import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useFillScrollArea } from './useFillScrollArea';

const observe = vi.fn();
const disconnect = vi.fn();

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = observe;
      disconnect = disconnect;
    }
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function Probe({ minHeight }: { minHeight?: number }) {
  const fill = useFillScrollArea<HTMLDivElement>(minHeight);
  return (
    <div data-testid="scroll" style={{ overflowY: 'auto', paddingBottom: '10px' }}>
      <div data-testid="fill" ref={fill.ref} style={{ height: fill.height }} />
    </div>
  );
}

/** Lays the scroll area out as `areaHeight` tall with the element `top` below its top edge. */
function layOut(areaHeight: number, top: number) {
  const scroll = screen.getByTestId('scroll');
  const fill = screen.getByTestId('fill');
  Object.defineProperty(scroll, 'clientHeight', { configurable: true, value: areaHeight });
  vi.spyOn(scroll, 'getBoundingClientRect').mockReturnValue({ top: 0 } as DOMRect);
  vi.spyOn(fill, 'getBoundingClientRect').mockReturnValue({ top } as DOMRect);
  act(() => {
    window.dispatchEvent(new Event('resize'));
  });
}

describe('useFillScrollArea', () => {
  it('fills what is left of the scroll area below the element', () => {
    render(<Probe />);

    layOut(800, 200);

    // 800 area - 200 above the element; the scroll area's own padding is not a
    // parent's, so nothing more is set aside.
    expect(screen.getByTestId('fill')).toHaveStyle({ height: '600px' });
  });

  it('never goes below the minimum height', () => {
    render(<Probe minHeight={420} />);

    layOut(500, 400);

    expect(screen.getByTestId('fill')).toHaveStyle({ height: '420px' });
  });

  it('watches the scroll area for resizing and lets go of it on unmount', () => {
    const { unmount } = render(<Probe />);

    expect(observe).toHaveBeenCalledWith(screen.getByTestId('scroll'));

    unmount();
    expect(disconnect).toHaveBeenCalled();
  });
});
