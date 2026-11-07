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
import { DocumentsIllustration } from './DocumentsIllustration';
import { PortalPublishIllustration } from './PortalPublishIllustration';
import { ReadyToConnectIllustration } from './ReadyToConnectIllustration';

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

  describe('ReadyToConnectIllustration', () => {
    it('is decorative and shows three sample resources feeding a backend card', () => {
      renderWithProviders(<ReadyToConnectIllustration />);

      expect(screen.getByTestId('ready-to-connect')).toHaveAttribute('aria-hidden', 'true');
      expect(screen.getAllByText('GET')).toHaveLength(2);
      expect(screen.getByText('POST')).toBeInTheDocument();
      expect(screen.getByText('/orders/{id}')).toBeInTheDocument();
      expect(screen.getByText('Backend')).toBeInTheDocument();
      expect(screen.getByText('https://')).toBeInTheDocument();
    });

    it('shrinks to a narrower container and keeps its height in proportion', () => {
      renderWithProviders(<ReadyToConnectIllustration />);

      act(() => callbacks[0]?.([{ contentRect: { width: 234 } }]));

      const frame = screen.getByTestId('ready-to-connect');
      expect(getComputedStyle(frame).height).toBe('84px');
      expect(getComputedStyle(frame.firstElementChild as Element).transform).toBe('scale(0.5)');
    });

    it('draws at natural size without a ResizeObserver', () => {
      vi.unstubAllGlobals();
      vi.stubGlobal('ResizeObserver', undefined);

      renderWithProviders(<ReadyToConnectIllustration />);

      expect(
        getComputedStyle(screen.getByTestId('ready-to-connect').firstElementChild as Element)
          .transform,
      ).toBe('scale(1)');
    });
  });

  describe('PortalPublishIllustration', () => {
    it('is decorative and walks a portal from empty to an API published', () => {
      const { container } = renderWithProviders(<PortalPublishIllustration />);

      expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true');
      for (const text of [
        'developer.example.com',
        'Live',
        'API catalog',
        'Discover and subscribe to APIs',
        'Payments API',
        'Untitled API',
        'Published',
        'Visible to developers',
        '1 of 1 portals',
      ]) {
        expect(screen.getByText(text)).toBeInTheDocument();
      }
    });
  });

  describe('DocumentsIllustration', () => {
    it('is decorative and walks the documents list from empty to a document added', () => {
      const { container } = renderWithProviders(<DocumentsIllustration />);

      expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true');
      for (const text of [
        'docs / untitled-api',
        'API documents',
        'Guides, samples and support',
        'Getting started',
        'Added',
        'Visible to developers',
        '2 documents',
      ]) {
        expect(screen.getByText(text)).toBeInTheDocument();
      }
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
