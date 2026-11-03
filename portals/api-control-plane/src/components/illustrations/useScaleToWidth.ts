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

import { useLayoutEffect, useRef, useState } from 'react';

/**
 * Scale factor that maps artwork drawn at a fixed `artWidth` onto the width of
 * the element the returned ref is attached to. Tracks resizes; stays at 1 where
 * `ResizeObserver` is unavailable (tests, very old browsers).
 */
export function useScaleToWidth(artWidth: number) {
  const ref = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element || typeof ResizeObserver === 'undefined') return undefined;

    // Measure up front so the first paint is already at the right size rather
    // than flashing at full size until the observer's first callback.
    if (element.clientWidth > 0) setScale(element.clientWidth / artWidth);

    const observer = new ResizeObserver(([entry]) => {
      if (entry) setScale(entry.contentRect.width / artWidth);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [artWidth]);

  return { ref, scale };
}
