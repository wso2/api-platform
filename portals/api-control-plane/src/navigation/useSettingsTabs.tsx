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
import { Settings as SettingsIcon } from '@wso2/oxygen-ui-icons-react';

import { useConsoleScope } from '../scope/ConsoleScopeProvider';
import { useExtensions } from '../extensions';
import { useIsHidden } from '../slots';
import type { NavigationLevel } from './navigationTypes';

export type SettingsTab = {
  id: string;
  label: string;
  icon: ReactNode;
  /** Path segment relative to `/settings/`, e.g. `"general"`. */
  path: string;
  order: number;
};

const BUILT_IN_GENERAL_TAB: SettingsTab = {
  id: 'general',
  label: 'General',
  icon: <SettingsIcon />,
  path: 'general',
  order: 0,
};

/**
 * The sub-nav tabs rendered inside the Settings page for a given scope: the
 * built-in "General" tab (unless suppressed via `Hideable`) plus any
 * cloud-injected extension registered against the matching
 * `settings.<scope>.tabs` slot, sorted by order. Mirrors
 * `useNavigationItems`'s registry-plus-extensions merge, one level deeper
 * (inside Settings rather than the main sidebar).
 */
export const useSettingsTabs = (scope: NavigationLevel): SettingsTab[] => {
  const consoleScope = useConsoleScope();
  const extensions = useExtensions();
  const generalTabHidden = useIsHidden(`settings.${scope}.tabs.general`);

  const extensionTabs: SettingsTab[] = extensions
    .filter((extension) => extension.slot === `settings.${scope}.tabs`)
    .filter((extension) => extension.isVisible?.(consoleScope) ?? true)
    .map((extension) => ({
      id: extension.id,
      label: extension.label,
      icon: extension.icon ?? <SettingsIcon />,
      path: extension.routePath.replace(/^settings\//, ''),
      order: extension.order,
    }));

  const builtInTabs = generalTabHidden ? [] : [BUILT_IN_GENERAL_TAB];

  return [...builtInTabs, ...extensionTabs].sort(
    (left, right) => left.order - right.order
  );
};
