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
import type { RequiredScope } from '../scope/ScopeGate';

export type NavigationLevel = 'organization' | 'project' | 'api';

export type NavigationDefinition = {
  featureKey?: string;
  /**
    * Divider-separated cluster for sidebar grouping.
   */
  group?: string;
  icon: ReactNode;
  id: string;
  isVisible?: (scope: ConsoleScope) => boolean;
  label: string;
  /**
   * Extensions only, where it decides the URL shape of the injected page (see
   * `buildScopedExtensionPath`). Built-in items leave it unset — a scope-adaptive
   * item spans several levels at once, and nothing else reads it.
   */
  level?: NavigationLevel;
  match?: (pathname: string) => boolean;
  order: number;
  /**
   * Pins this item to the sidebar's fixed bottom section (`Sidebar.Footer`)
   * instead of the scrolling main nav list. Used for Settings, which should
   * stay visible at the bottom regardless of how long the nav list gets.
   */
  pinned?: boolean;
  to: (scope: ConsoleScope) => string | undefined;
  /**
   * Sub-items, offered only once `requires` is satisfied.
   *
   * A parent with children showing is a disclosure, not a link — the sidebar
   * omits its `link` so a click expands instead of navigating. Out of scope the
   * children are withheld and the parent behaves as an ordinary link into its
   * first child's page, where `ScopeGate` asks for what's missing.
   */
  children?: NavigationDefinition[];
  /**
   * Scope this item's children need. Shares `ScopeGate`'s own union so the
   * sidebar and the page it opens can never disagree about what "in scope" means.
   */
  requires?: RequiredScope;
};

export type NavigationItem = {
  group?: string;
  icon: ReactNode;
  id: string;
  isActive: boolean;
  label: string;
  /**
   * Where the item leads. A parent rendering `children` keeps its first child's
   * target here but the sidebar does not link it — see `children` above.
   */
  pinned?: boolean;
  to: string;
  children?: NavigationItem[];
};
