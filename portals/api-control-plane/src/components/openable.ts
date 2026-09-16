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

import type { KeyboardEvent } from 'react';
import { defineMessages, type IntlShape } from 'react-intl';

const messages = defineMessages({
  openLabel: {
    id: 'apiControlPlane.components.openable.openLabel',
    defaultMessage: 'Open {name}',
    description: 'Accessible name of a card or row that opens what it describes.',
  },
});

/**
 * Props that make a clickable card or row keyboard accessible.
 *
 * A `div` with only `onClick` is not keyboard accessible. In listings with
 * nested controls, this ensures the row itself can also be opened. Reuse these
 * props across views to keep the interaction consistent.
 *
 * Pair with `focusRingSx` from `@/theme` to provide a visible focus state.
 */
export function openableProps(intl: IntlShape, name: string, open: () => void) {
  return {
    'aria-label': intl.formatMessage(messages.openLabel, { name }),
    onClick: open,
    onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
      // Ignore bubbled events from nested controls; only the element itself opens.
      if (event.target !== event.currentTarget) return;
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault(); // Space would otherwise scroll the page.
      open();
    },
    role: 'button',
    tabIndex: 0,
  } as const;
}
