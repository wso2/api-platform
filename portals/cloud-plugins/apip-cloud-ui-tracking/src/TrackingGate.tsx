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

import { useEffect, useMemo, useRef } from 'react';
import { matchPath, useLocation } from 'react-router-dom';

import { readTrackingConfig } from './config';
import { isTrackingStarted, startTracking } from './loader';
import { pushPageView, routePattern, sortPatterns } from './pageView';

export type TrackingGateProps = {
  /** Every route pattern the console serves, as `:param` paths (`/organizations/:orgHandle/home`). */
  routePatterns: readonly string[];
  /** Patterns nobody sees as a page (redirects, sign-in steps): matched, but not counted. */
  nonPagePatterns?: readonly string[];
};

/**
 * Headless `app.gate` entry: loads CookiePro and GTM when the deployment
 * configures both, then reports one `virtual_page_view` per route change.
 * Renders nothing, and does nothing at all when they are not configured.
 */
export default function TrackingGate({ routePatterns, nonPagePatterns = [] }: TrackingGateProps) {
  const { pathname } = useLocation();
  const patterns = useMemo(() => sortPatterns(routePatterns), [routePatterns]);
  const lastPushed = useRef<string | null>(null);

  useEffect(() => {
    const config = readTrackingConfig();
    if (config) startTracking(config);
  }, []);

  useEffect(() => {
    if (!isTrackingStarted() || lastPushed.current === pathname) return;
    lastPushed.current = pathname;
    const pattern = routePattern(
      pathname,
      patterns,
      (candidate, path) => matchPath({ path: candidate, end: true }, path) !== null
    );
    if (nonPagePatterns.includes(pattern)) return;
    pushPageView(window, pattern);
  }, [pathname, patterns, nonPagePatterns]);

  return null;
}
