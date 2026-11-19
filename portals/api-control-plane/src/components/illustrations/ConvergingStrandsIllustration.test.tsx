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

import { afterEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, waitFor } from '@/test/utils';

import { ConvergingStrandsIllustration } from './ConvergingStrandsIllustration';

describe('ConvergingStrandsIllustration', () => {
  it('is decorative: hidden from assistive technology', () => {
    const { container } = renderWithProviders(<ConvergingStrandsIllustration />);

    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('draws strands from both sides', () => {
    const { container } = renderWithProviders(<ConvergingStrandsIllustration />);

    expect(container.querySelectorAll('.strand-left').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.strand-right').length).toBeGreaterThan(0);
  });

  it('gives each drawing its own gradient ids, so two on a page do not share them', () => {
    const { container } = renderWithProviders(
      <>
        <ConvergingStrandsIllustration />
        <ConvergingStrandsIllustration />
      </>,
    );

    const ids = [...container.querySelectorAll('linearGradient, radialGradient')].map(
      (gradient) => gradient.id,
    );
    expect(ids).toHaveLength(6);
    expect(new Set(ids).size).toBe(6);
  });

  describe('leaning toward the pointer', () => {
    afterEach(() => vi.unstubAllGlobals());

    const rect = {
      left: 0,
      top: 0,
      width: 400,
      height: 300,
      right: 400,
      bottom: 300,
      x: 0,
      y: 0,
    } as DOMRect;
    // jsdom has no PointerEvent: a mouse event under the pointer event's name.
    const move = (clientX: number, clientY: number, pointerType = 'mouse') => {
      const event = new MouseEvent('pointermove', { bubbles: true, clientX, clientY });
      Object.defineProperty(event, 'pointerType', { value: pointerType });
      window.dispatchEvent(event);
    };
    const place = (svg: SVGElement) => vi.spyOn(svg, 'getBoundingClientRect').mockReturnValue(rect);

    it('publishes where the pointer is, relative to the drawing', async () => {
      const { container } = renderWithProviders(<ConvergingStrandsIllustration />);
      const svg = container.querySelector('svg') as SVGElement;
      place(svg);

      move(400, 150, 'mouse');

      // Half a drawing-width to the right of its centre, level with it.
      await waitFor(() => expect(svg.style.getPropertyValue('--px')).toBe('0.500'));
      expect(svg.style.getPropertyValue('--py')).toBe('0.000');
    });

    it('never leans further than its limit, however far away the pointer is', async () => {
      const { container } = renderWithProviders(<ConvergingStrandsIllustration />);
      const svg = container.querySelector('svg') as SVGElement;
      place(svg);

      move(9000, -9000, 'mouse');

      await waitFor(() => expect(svg.style.getPropertyValue('--px')).toBe('1.000'));
      expect(svg.style.getPropertyValue('--py')).toBe('-1.000');
    });

    it('ignores touch', async () => {
      const { container } = renderWithProviders(<ConvergingStrandsIllustration />);
      const svg = container.querySelector('svg') as SVGElement;
      place(svg);

      move(400, 150, 'touch');
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(svg.style.getPropertyValue('--px')).toBe('');
    });

    it('stays still for a user who prefers reduced motion', async () => {
      vi.stubGlobal(
        'matchMedia',
        (query: string) =>
          ({
            matches: query.includes('prefers-reduced-motion'),
            media: query,
            addEventListener: () => {},
            removeEventListener: () => {},
            addListener: () => {},
            removeListener: () => {},
          }) as unknown as MediaQueryList,
      );
      const { container } = renderWithProviders(<ConvergingStrandsIllustration />);
      const svg = container.querySelector('svg') as SVGElement;
      place(svg);

      move(400, 150, 'mouse');
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(svg.style.getPropertyValue('--px')).toBe('');
    });
  });
});
