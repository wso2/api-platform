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

import { useEffect, useState } from 'react';

import { APP_FOOTER_ID } from './appLayoutConstants';

/**
 * Measures the app footer's height.
 *
 * The footer sits at the bottom of the same scroll area a sticky bottom bar
 * (`stickyBottomBarSx`) sticks to, so `bottom: 0` would leave the bar underneath
 * it. Every such bar offsets by this value instead of a hardcoded one, because
 * the footer's height depends on the theme's spacing and on whether its links
 * wrap on a narrow viewport.
 *
 * Returns 0 when the footer is absent — a page rendered outside the app shell,
 * or a test — which degrades to `bottom: 0` rather than throwing.
 */
export function useFooterHeight(): number {
  const [height, setHeight] = useState(0);

  useEffect(() => {
    const element = document.getElementById(APP_FOOTER_ID);
    if (!element) return;

    const update = () => setHeight(element.offsetHeight);
    update();

    // The footer reflows on viewport resize (its links wrap), so a one-off
    // measurement would leave the bar overlapping it at that width.
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return height;
}
