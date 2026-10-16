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

import React, { useSyncExternalStore } from 'react';
import { Stack, ToggleButton, ToggleButtonGroup, Tooltip, Typography } from '@wso2/oxygen-ui';
import { ArrowRight, Waypoints } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { TestCallMode } from '../utils/callMode';
import type { CallModeStore } from './utils/callModeStore';

const messages = defineMessages({
  callModeLabel: {
    id: 'apiControlPlane.pages.test.console.ExecuteModeSwitch.callModeLabel',
    defaultMessage: 'Send requests through the proxy or directly',
    description: 'Accessible label for the toggle between the two ways of sending a request.',
  },
  directHint: {
    id: 'apiControlPlane.pages.test.console.ExecuteModeSwitch.directHint',
    defaultMessage:
      'Requests go straight from your browser, so the gateway must be reachable from this machine and allow this origin with a CORS policy.',
    description:
      'Tooltip shown while Direct is selected. States what the user takes on by choosing it — the browser, not the portal, now has to reach the gateway.',
  },
  directMode: {
    id: 'apiControlPlane.pages.test.console.ExecuteModeSwitch.directMode',
    defaultMessage: 'Direct',
    description:
      'Toggle option sending the request from the browser straight to the gateway. An adverb describing how the request travels, not a command.',
  },
  proxyHint: {
    id: 'apiControlPlane.pages.test.console.ExecuteModeSwitch.proxyHint',
    defaultMessage: 'Requests go through a WSO2 Managed proxy, so the gateway does not need a CORS policy.',
    description:
      'Tooltip shown while Through proxy is selected. Explains the benefit of the default, so the user can tell what they would give up by switching.',
  },
  proxyMode: {
    id: 'apiControlPlane.pages.test.console.ExecuteModeSwitch.proxyMode',
    defaultMessage: 'Through proxy',
    description:
      'Toggle option relaying the request via the portal server. Describes the route a request takes, not a command.',
  },
});

export type ExecuteModeSwitchProps = { store: CallModeStore };

/**
 * The proxy/direct switch, rendered beside swagger's Execute button.
 *
 * It sits here rather than only on the Gateway card because the two modes fail
 * in opposite situations; the relay cannot reach a self-hosted gateway, the
 * browser cannot reach a cluster-internal one; and the moment someone learns
 * which is at the moment a request has just failed. Having to scroll away from
 * the response to retry the other way is the whole friction this removes.
 *
 * One setting, shown in as many places as there are open operations. Swagger
 * renders an Execute button per expanded operation, so flipping any one of
 * these flips all of them; they are views of a single page-level value, not
 * per-operation settings.
 */
export function ExecuteModeSwitch({ store }: ExecuteModeSwitchProps) {
  const intl = useIntl();
  const callMode = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot);

  return (
    <Tooltip
      arrow
      placement="top"
      title={
        <Stack spacing={0.5} sx={{ py: 0.5 }}>
          <Typography sx={{ fontWeight: 'fontWeightBold' }} variant="caption">
            <FormattedMessage
              {...(callMode === 'direct' ? messages.directMode : messages.proxyMode)}
            />
          </Typography>
          <Typography variant="caption">
            <FormattedMessage
              {...(callMode === 'direct' ? messages.directHint : messages.proxyHint)}
            />
          </Typography>
        </Stack>
      }
    >
      <ToggleButtonGroup
        aria-label={intl.formatMessage(messages.callModeLabel)}
        exclusive
        onChange={(_event, next) => next && store.requestMode(next as TestCallMode)}
        size="small"
        value={callMode}
      >
        <ToggleButton value="proxy">
          <Stack alignItems="center" direction="row" spacing={1}>
            <Waypoints size={16} />
            <span>
              <FormattedMessage {...messages.proxyMode} />
            </span>
          </Stack>
        </ToggleButton>
        <ToggleButton value="direct">
          <Stack alignItems="center" direction="row" spacing={1}>
            <ArrowRight size={16} />
            <span>
              <FormattedMessage {...messages.directMode} />
            </span>
          </Stack>
        </ToggleButton>
      </ToggleButtonGroup>
    </Tooltip>
  );
}

/**
 * Swagger plugin that places the switch above the Execute button.
 */
export const executeModeSwitchPlugin = (store: CallModeStore) => () => ({
  wrapComponents: {
    execute:
      (Original: React.ComponentType<Record<string, unknown>>) =>
      (props: Record<string, unknown>) => (
        // A fragment, not a wrapping element: swagger renders Execute and
        // Clear as siblings of one container and styles them as a joined pair
        // (`.btn-group .btn { flex: 1 }`, with the first and last child
        // carrying the outer radii). Wrapping Execute in a div takes it out of
        // that container, so Clear ends up beside the wrapper instead of
        // beside Execute. Staying a sibling keeps swagger's own pairing intact
        // and leaves this file responsible only for the switch's own line.
        <>
          <div className="test-console-mode-row">
            <ExecuteModeSwitch store={store} />
          </div>
          <Original {...props} />
        </>
      ),
  },
});
