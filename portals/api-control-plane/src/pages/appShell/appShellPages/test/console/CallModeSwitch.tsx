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

import { Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { ArrowRight, Waypoints } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SegmentedControl } from '@/components/common/SegmentedControl';
import type { TestCallMode } from '../utils/callMode';

const messages = defineMessages({
  callModeLabel: {
    id: 'apiControlPlane.pages.test.console.CallModeSwitch.callModeLabel',
    defaultMessage: 'Send requests through the proxy or directly',
    description: 'Accessible label for the toggle between the two ways of sending a request.',
  },
  directHint: {
    id: 'apiControlPlane.pages.test.console.CallModeSwitch.directHint',
    defaultMessage:
      'Requests go straight from your browser, so the gateway must be reachable from this machine and allow this origin with a CORS policy.',
    description:
      'Tooltip shown while Direct is selected. States what the user takes on by choosing it — the browser, not the portal, now has to reach the gateway.',
  },
  directMode: {
    id: 'apiControlPlane.pages.test.console.CallModeSwitch.directMode',
    defaultMessage: 'Direct',
    description:
      'Toggle option sending the request from the browser straight to the gateway. An adverb describing how the request travels, not a command.',
  },
  proxyHint: {
    id: 'apiControlPlane.pages.test.console.CallModeSwitch.proxyHint',
    defaultMessage:
      'Requests go through the portal server, so the gateway does not need a CORS policy.',
    description:
      'Tooltip shown while Through proxy is selected. Explains the benefit of the default, so the user can tell what they would give up by switching.',
  },
  proxyMode: {
    id: 'apiControlPlane.pages.test.console.CallModeSwitch.proxyMode',
    defaultMessage: 'Through proxy',
    description:
      'Toggle option relaying the request via the portal server. Describes the route a request takes, not a command.',
  },
});

export type CallModeSwitchProps = {
  mode: TestCallMode;
  onChange: (mode: TestCallMode) => void;
};

/**
 * The proxy/direct switch. What the selected mode means is in its tooltip.
 *
 * It sits beside the endpoint it affects rather than inside the request panel,
 * because the two modes fail in opposite situations — the relay cannot reach a
 * self-hosted gateway, the browser cannot reach a cluster-internal one — and
 * the explanation is what tells someone which to try when a request fails.
 */
export function CallModeSwitch({ mode, onChange }: CallModeSwitchProps) {
  const intl = useIntl();

  return (
    <Tooltip
      arrow
      placement="top"
      title={
        <Stack spacing={0.5} sx={{ py: 0.5 }}>
          <Typography fontWeight="fontWeightBold" variant="caption">
            <FormattedMessage {...(mode === 'direct' ? messages.directMode : messages.proxyMode)} />
          </Typography>
          <Typography variant="caption">
            <FormattedMessage {...(mode === 'direct' ? messages.directHint : messages.proxyHint)} />
          </Typography>
        </Stack>
      }
    >
      <SegmentedControl
        aria-label={intl.formatMessage(messages.callModeLabel)}
        onChange={onChange}
        options={[
          {
            icon: <Waypoints size={16} />,
            label: <FormattedMessage {...messages.proxyMode} />,
            value: 'proxy',
          },
          {
            icon: <ArrowRight size={16} />,
            label: <FormattedMessage {...messages.directMode} />,
            value: 'direct',
          },
        ]}
        value={mode}
      />
    </Tooltip>
  );
}
