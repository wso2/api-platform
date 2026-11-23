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
import { afterEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';

import { ApiTypeTicker } from './ApiTypeTicker';

describe('ApiTypeTicker', () => {
  it('names every API type of the create wizard, after "Supports"', () => {
    renderWithProviders(<ApiTypeTicker />);

    expect(screen.getByText('Supports')).toBeInTheDocument();
    for (const label of [
      'REST APIs',
      'GraphQL APIs',
      'WebSocket APIs',
      'WebSub APIs',
      'gRPC APIs',
    ]) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
  });

  it('repeats the first type at the end of the strip, hidden from assistive technology', () => {
    renderWithProviders(<ApiTypeTicker />);

    const rest = screen.getAllByText('REST APIs');
    expect(rest).toHaveLength(2);
    expect(rest[0].closest('[aria-hidden="true"]')).toBeNull();
    expect(rest[1].closest('[aria-hidden="true"]')).not.toBeNull();
  });

  describe('scrolling', () => {
    // The strip is the rows' parent; how far it has moved is its translateY.
    const offset = () =>
      (screen.getAllByText('REST APIs')[0].parentElement!.parentElement as HTMLElement).style
        .transform;

    afterEach(() => {
      vi.useRealTimers();
      vi.unstubAllGlobals();
    });

    // One second at a time: React commits between steps, as it does for a user.
    const tickUntil = async (done: () => boolean) => {
      for (let second = 0; second < 40 && !done(); second += 1) {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1000);
        });
      }
    };

    it('moves up one row at a time, then wraps round to the first', async () => {
      vi.useFakeTimers();
      renderWithProviders(<ApiTypeTicker />);
      const seen = [offset()];

      await tickUntil(() => {
        if (offset() !== seen[seen.length - 1]) seen.push(offset());
        // Five types plus the repeated first row: the strip ends on the sixth.
        return offset() === 'translateY(-10rem)';
      });
      await tickUntil(() => offset() === 'translateY(-0rem)');

      expect(seen).toEqual([
        'translateY(-0rem)',
        'translateY(-2rem)',
        'translateY(-4rem)',
        'translateY(-6rem)',
        'translateY(-8rem)',
        'translateY(-10rem)',
      ]);
      expect(offset()).toBe('translateY(-0rem)');
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
      vi.useFakeTimers();
      renderWithProviders(<ApiTypeTicker />);
      const start = offset();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(10_000);
      });

      expect(offset()).toBe(start);
    });
  });
});
