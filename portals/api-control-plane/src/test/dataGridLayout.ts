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

import { vi } from 'vitest';

const WIDTH = 1000;
const HEIGHT = 600;

/**
 * jsdom lays nothing out, so a data grid measures itself as 0 x 0 and renders no columns or rows.
 * This gives every element a size and makes the resize observer report it, so the grid draws in full.
 * Call it once in a test file's `beforeAll`; the stubs last as long as the file, which vitest isolates.
 */
export function mockDataGridLayout(): void {
  const size = { bottom: HEIGHT, height: HEIGHT, left: 0, right: WIDTH, top: 0, width: WIDTH, x: 0, y: 0 };

  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(() => ({ ...size, toJSON: () => size }));
  for (const [property, value] of [
    ['clientHeight', HEIGHT],
    ['clientWidth', WIDTH],
    ['offsetHeight', HEIGHT],
    ['offsetWidth', WIDTH],
    ['scrollHeight', HEIGHT],
    ['scrollWidth', WIDTH],
  ] as const) {
    Object.defineProperty(HTMLElement.prototype, property, { configurable: true, get: () => value });
  }

  class MeasuringResizeObserver {
    constructor(private readonly callback: ResizeObserverCallback) {}
    observe(target: Element) {
      const box = [{ blockSize: HEIGHT, inlineSize: WIDTH }];
      this.callback(
        [
          {
            borderBoxSize: box,
            contentBoxSize: box,
            contentRect: { ...size, toJSON: () => size },
            devicePixelContentBoxSize: box,
            target,
          } as unknown as ResizeObserverEntry,
        ],
        this as unknown as ResizeObserver,
      );
    }
    unobserve = vi.fn();
    disconnect = vi.fn();
  }
  vi.stubGlobal('ResizeObserver', MeasuringResizeObserver);
}
