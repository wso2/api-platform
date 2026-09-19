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

import { useLayoutEffect, useState } from 'react';

import { APP_FOOTER_ID } from '../pages/appShell/appLayoutConstants';

/** The nearest ancestor that scrolls, or null when the document itself does. */
const scrollParentOf = (element: HTMLElement): HTMLElement | null => {
  for (let node = element.parentElement; node && node !== document.body; node = node.parentElement) {
    const { overflowY } = getComputedStyle(node);
    if (overflowY === 'auto' || overflowY === 'scroll') return node;
  }
  return null;
};

const px = (value: string): number => Number.parseFloat(value) || 0;

/**
 * Sizes an element to fill the rest of the scroll area it sits in, so the page
 * around it stays put instead of growing and scrolling.
 *
 * The height is what is left below the element's own top edge once everything
 * that follows it in the scroll area (the padding and margins of its ancestors,
 * and the app footer when that lives in the same area) is set aside. Everything
 * is measured, not assumed, so it holds when the shell's spacing or the footer's
 * wrapping changes. `minHeight` keeps it usable in a very short window, where
 * the page scrolls after all.
 *
 * Returns `undefined` until the first measurement, so callers render at their
 * natural height for that one frame rather than at a guessed one.
 */
export function useFillScrollArea<T extends HTMLElement>(minHeight = 0) {
  // Held in state, not a ref object: a page that starts on a loading view mounts
  // the element later, and the measurement has to follow it.
  const [element, setElement] = useState<T | null>(null);
  const [height, setHeight] = useState<number>();

  useLayoutEffect(() => {
    if (!element) return;
    const scrollArea = scrollParentOf(element);

    const measure = () => {
      const areaHeight = scrollArea ? scrollArea.clientHeight : window.innerHeight;
      const areaTop = scrollArea ? scrollArea.getBoundingClientRect().top : 0;
      const scrolled = scrollArea ? scrollArea.scrollTop : window.scrollY;
      const top = element.getBoundingClientRect().top - areaTop + scrolled;

      let below = 0;
      for (let node = element.parentElement; node && node !== scrollArea && node !== document.body; node = node.parentElement) {
        const style = getComputedStyle(node);
        below += px(style.paddingBottom) + px(style.marginBottom) + px(style.borderBottomWidth);
      }
      const footer = document.getElementById(APP_FOOTER_ID);
      if (footer && (!scrollArea || scrollArea.contains(footer))) below += footer.offsetHeight;

      setHeight(Math.max(minHeight, Math.floor(areaHeight - top - below)));
    };

    measure();
    window.addEventListener('resize', measure);
    const observer = new ResizeObserver(measure);
    if (scrollArea) observer.observe(scrollArea);
    const footer = document.getElementById(APP_FOOTER_ID);
    if (footer) observer.observe(footer);

    return () => {
      window.removeEventListener('resize', measure);
      observer.disconnect();
    };
  }, [element, minHeight]);

  return { height, ref: setElement };
}
