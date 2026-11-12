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

import { defineMessages } from 'react-intl';

/** Wording for transport failures. Shown as plain text, never interpreted. */
export const transportMessages = defineMessages({
  invalidTarget: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.invalidTarget',
    defaultMessage:
      'This request cannot be sent as written. Check the path, headers and parameters.',
    description:
      'Shown when the console refused to send a request it could not prove stays on the selected gateway.',
  },
  invalidHeader: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.invalidHeader',
    defaultMessage: 'A header on this request is not allowed. Remove it and try again.',
  },
  bodyTooLarge: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.bodyTooLarge',
    defaultMessage:
      'This request body is too large to send from the console. Use the cURL command instead.',
  },
  timeout: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.timeout',
    defaultMessage: 'The gateway did not respond in time.',
  },
  redirect: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.redirect',
    defaultMessage: 'The gateway answered with a redirect. The console does not follow redirects.',
    description: 'Redirects are not followed so a request can never be steered to another host.',
  },
  directUnreachable: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.directUnreachable',
    defaultMessage:
      'The browser could not reach the gateway. Check that it is running and allows cross-origin requests (CORS), or switch to “Through proxy”.',
  },
  unexpected: {
    id: 'apiControlPlane.components.ApiTestConsole.transport.unexpected',
    defaultMessage: 'The request could not be sent. Try again.',
  },
});
