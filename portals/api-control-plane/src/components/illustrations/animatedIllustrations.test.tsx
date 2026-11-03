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

import { act } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { ApiDesignerCanvasIllustration } from './ApiDesignerCanvasIllustration';
import { ContractFlowIllustration } from './ContractFlowIllustration';

type ObserverCallback = (entries: { contentRect: { width: number } }[]) => void;

describe('animated illustrations', () => {
  let callbacks: ObserverCallback[];
  let disconnect: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    callbacks = [];
    disconnect = vi.fn();
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(callback: ObserverCallback) {
          callbacks.push(callback);
        }
        observe() {}
        disconnect = disconnect;
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('ContractFlowIllustration', () => {
    it('is decorative and finds three sample resources', () => {
      const { container } = renderWithProviders(<ContractFlowIllustration />);

      expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true');
      expect(screen.getByText('3 resources found')).toBeInTheDocument();
      for (const method of ['GET', 'POST', 'PUT']) {
        expect(screen.getByText(method)).toBeInTheDocument();
      }
      expect(screen.getByText('/products/{id}')).toBeInTheDocument();
    });

    it('colours each verb badge from the shared method palette', () => {
      renderWithProviders(<ContractFlowIllustration />);

      const get = screen.getByText('GET').parentElement as HTMLElement;
      const post = screen.getByText('POST').parentElement as HTMLElement;
      expect(get.style.getPropertyValue('--apict-method')).not.toBe('');
      expect(get.style.getPropertyValue('--apict-method')).not.toBe(
        post.style.getPropertyValue('--apict-method'),
      );
    });

    it('scales the artwork to the width it is given and stops observing on unmount', () => {
      const { container, unmount } = renderWithProviders(<ContractFlowIllustration />);

      act(() => callbacks[0]?.([{ contentRect: { width: 210 } }]));

      const art = container.querySelector('.apict-anim') as HTMLElement;
      expect(art.style.transform || getComputedStyle(art).transform).toContain('0.5');

      unmount();
      expect(disconnect).toHaveBeenCalled();
    });

    it('measures the container on mount, before the observer reports', () => {
      const width = vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(210);

      const { container } = renderWithProviders(<ContractFlowIllustration />);

      const art = container.querySelector('.apict-anim') as HTMLElement;
      expect(art.style.transform || getComputedStyle(art).transform).toContain('0.5');
      width.mockRestore();
    });

    it('renders without a ResizeObserver, at natural scale', () => {
      vi.unstubAllGlobals();
      vi.stubGlobal('ResizeObserver', undefined);

      expect(() => renderWithProviders(<ContractFlowIllustration />)).not.toThrow();
    });
  });

  describe('ApiDesignerCanvasIllustration', () => {
    it('is decorative and shows the governance checks and score', () => {
      const { container } = renderWithProviders(<ApiDesignerCanvasIllustration />);

      expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true');
      expect(screen.getByText('OpenAPI 3.x valid')).toBeInTheDocument();
      expect(screen.getByText('Spectral rules passed')).toBeInTheDocument();
      expect(screen.getByText('AI-readiness')).toBeInTheDocument();
      expect(screen.getByText('92%')).toBeInTheDocument();
    });

    it('scales the artwork to its container', () => {
      const { container } = renderWithProviders(<ApiDesignerCanvasIllustration />);

      act(() => callbacks[0]?.([{ contentRect: { width: 105 } }]));

      const art = container.querySelector('.apid-anim') as HTMLElement;
      expect(art.style.transform || getComputedStyle(art).transform).toContain('0.25');
    });
  });
});
