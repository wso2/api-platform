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
import { defineMessages, useIntl } from 'react-intl';
import { Settings } from '@wso2/oxygen-ui-icons-react';

import { useConsoleScope } from '../scope/ConsoleScopeProvider';
import {
  settingsTabExtensions,
  settingsTabSlot,
  useExtensions,
} from '../extensions';
import { useIsHidden } from '../slots';
import type { NavigationLevel } from './navigationTypes';

const messages = defineMessages({
  generalTab: {
    id: 'apiControlPlane.navigation.useSettingsTabs.generalTab',
    defaultMessage: 'General',
    description:
      'Label for the built-in first tab of the Settings page. A noun naming the section, not a command.',
  },
});

export type SettingsTab = {
  id: string;
  label: string;
  icon: ReactNode;
  /** Path segment relative to `/settings/`, e.g. `"general"`. */
  path: string;
  order: number;
};

/**
 * The sub-nav tabs rendered inside the Settings page for a given level: the
 * built-in "General" tab (unless suppressed via `Hideable`) plus any
 * host-injected extension registered against the matching
 * `settings.<level>.tabs` slot, sorted by `order`. Mirrors
 * `useNavigationItems`'s registry-plus-extensions merge, one level deeper
 * (inside Settings rather than the main sidebar).
 */
export const useSettingsTabs = (level: NavigationLevel): SettingsTab[] => {
  const intl = useIntl();
  const consoleScope = useConsoleScope();
  const extensions = useExtensions();
  const generalTabHidden = useIsHidden(`${settingsTabSlot(level)}.general`);

  const extensionTabs: SettingsTab[] = settingsTabExtensions(extensions, level)
    .filter((extension) => extension.isVisible?.(consoleScope) ?? true)
    .map((extension) => ({
      icon: extension.icon ?? <Settings size={18} />,
      id: extension.id,
      // An extension's label is host-supplied and already in the host's own
      // locale — it is passed through, never run through this app's catalog.
      label: extension.label,
      order: extension.order,
      path: extension.routePath.replace(/^settings\//, ''),
    }));

  const builtInTabs: SettingsTab[] = generalTabHidden
    ? []
    : [
        {
          icon: <Settings size={18} />,
          id: 'general',
          label: intl.formatMessage(messages.generalTab),
          order: 0,
          path: 'general',
        },
      ];

  return [...builtInTabs, ...extensionTabs].sort(
    (left, right) => left.order - right.order
  );
};
