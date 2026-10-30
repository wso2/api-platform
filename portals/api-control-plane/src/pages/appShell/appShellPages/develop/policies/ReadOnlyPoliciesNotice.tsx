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

import { Alert, Box } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

const messages = defineMessages({
  body: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.ReadOnlyPoliciesNotice.body',
    defaultMessage:
      'This API was created from a gateway. You can view its policies, but attach, edit, remove and reorder actions are managed by the gateway and are unavailable in the API Control Plane.',
    description:
      'Info banner on the Policies page of a read-only API, i.e. one that a gateway pushed to the control plane.',
  },
});

/**
 * Shown above the policy editor of an API synced from a data-plane gateway
 * (`readOnly`). The control plane rejects any change to such an API, so the
 * page lists its policies without offering edits — shared by the REST and
 * GraphQL policy panels so both say the same thing, and styled like the
 * Deploy page's gateway-managed banner.
 */
export function ReadOnlyPoliciesNotice() {
  return (
    <Box sx={{ pt: 2 }}>
      <Alert severity="info">
        <FormattedMessage {...messages.body} />
      </Alert>
    </Box>
  );
}
