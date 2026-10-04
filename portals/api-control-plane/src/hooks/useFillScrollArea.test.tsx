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

import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { APP_FOOTER_ID } from '../pages/appShell/appLayoutConstants';
import { useFillScrollArea } from './useFillScrollArea';

/**
 * jsdom has no layout, so geometry is declared on the elements themselves:
 * `data-top` for where an element starts on screen, `data-client-height` and
 * `data-offset-height` for their sizes.
 */
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    return { top: Number(this.dataset.top ?? 0) } as DOMRect;
  });
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
    configurable: true,
    get(this: HTMLElement) {
      return Number(this.dataset.clientHeight ?? 0);
    },
  });
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
    configurable: true,
    get(this: HTMLElement) {
      return Number(this.dataset.offsetHeight ?? 0);
    },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  Reflect.deleteProperty(HTMLElement.prototype, 'clientHeight');
  Reflect.deleteProperty(HTMLElement.prototype, 'offsetHeight');
});

function Probe({ minHeight }: { minHeight?: number }) {
  const { height, ref } = useFillScrollArea<HTMLDivElement>(minHeight);
  return <div data-testid="fill" data-top="164" ref={ref} style={{ height }} />;
}

const inScrollArea = (clientHeight: number, footer: 'inside' | 'outside', minHeight?: number) => (
  <>
    <div data-client-height={clientHeight} data-top="64" style={{ overflowY: 'auto' }}>
      <div style={{ paddingBottom: '40px' }}>
        <Probe minHeight={minHeight} />
      </div>
      {footer === 'inside' && <div data-offset-height="48" id={APP_FOOTER_ID} />}
    </div>
    {footer === 'outside' && <div data-offset-height="48" id={APP_FOOTER_ID} />}
  </>
);

describe('useFillScrollArea', () => {
  it('fills what is left of the scroll area once the space above, the padding below and the footer are set aside', () => {
    render(inScrollArea(800, 'inside'));

    // 800 tall area, element starts 100 into it, 40 of padding and a 48 footer follow.
    expect(screen.getByTestId('fill')).toHaveStyle({ height: '612px' });
  });

  it('ignores a footer that is not in the scroll area', () => {
    render(inScrollArea(800, 'outside'));

    expect(screen.getByTestId('fill')).toHaveStyle({ height: '660px' });
  });

  it('never goes below the minimum, so a very short window scrolls instead of crushing the page', () => {
    render(inScrollArea(300, 'inside', 420));

    expect(screen.getByTestId('fill')).toHaveStyle({ height: '420px' });
  });
});
