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

import type { ReactNode } from 'react';

import type { ConsoleScope } from '../scope/ConsoleScopeProvider';

export type NavigationLevel = 'organization' | 'project' | 'api';

export type NavigationDefinition = {
  featureKey?: string;
  /** Sidebar section heading this item is grouped under. */
  group?: string;
  icon: ReactNode;
  id: string;
  isVisible?: (scope: ConsoleScope) => boolean;
  label: string;
  level: NavigationLevel;
  match?: (pathname: string) => boolean;
  order: number;
  /**
   * Pins this item to the sidebar's fixed bottom section (`Sidebar.Footer`)
   * instead of the scrolling main nav list. Used for Settings, which should
   * stay visible at the bottom regardless of how long the nav list gets.
   */
  pinned?: boolean;
  to: (scope: ConsoleScope) => string | undefined;
};

export type NavigationItem = {
  group: string;
  icon: ReactNode;
  id: string;
  isActive: boolean;
  label: string;
  pinned?: boolean;
  to: string;
};

/** A sidebar section: a heading plus the nav items under it (order preserved). */
export type NavigationGroup = {
  label: string;
  items: NavigationItem[];
};

export const NAVIGATION_GROUP_BY_LEVEL: Record<NavigationLevel, string> = {
  organization: 'Organization',
  project: 'Project',
  api: 'API',
};
