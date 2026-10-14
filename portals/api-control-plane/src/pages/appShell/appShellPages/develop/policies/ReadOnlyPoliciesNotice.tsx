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

import { Alert, AlertTitle } from '@wso2/oxygen-ui';
import { Lock } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

const messages = defineMessages({
  body: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.ReadOnlyPoliciesNotice.body',
    defaultMessage:
      'This API was discovered from a data-plane gateway, so its policies are read-only in this console. Change them on the gateway.',
    description:
      'Explains why the policies of a gateway-managed API cannot be added, edited or removed.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.ReadOnlyPoliciesNotice.title',
    defaultMessage: 'Policies cannot be changed here',
    description: 'Title of the notice shown on the policies page of a gateway-managed API.',
  },
});

/**
 * Shown in place of the policy editor's controls for an API synced from a
 * data-plane gateway (`readOnly`). The control plane rejects any change to
 * such an API, so the page lists its policies without offering edits — shared
 * by the REST and GraphQL policy panels so both say the same thing.
 */
export function ReadOnlyPoliciesNotice() {
  return (
    <Alert icon={<Lock size={18} />} severity="info">
      <AlertTitle>
        <FormattedMessage {...messages.title} />
      </AlertTitle>
      <FormattedMessage {...messages.body} />
    </Alert>
  );
}
