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

import { Alert, Box, Typography } from '@wso2/oxygen-ui';
import { TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

const messages = defineMessages({
  attach: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.RequiredPoliciesNote.attach',
    defaultMessage:
      '<b>Attach {count, plural, one {this policy} other {these policies}} to {apiName} and redeploy it to the gateway</b>, so the plans are enforced:',
    description:
      'Lead-in to a list of policy names. {apiName} is the API display name, user-supplied. <b> sets the instruction in bold.',
  },
  publishAnyway: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.RequiredPoliciesNote.publishAnyway',
    defaultMessage:
      'You can publish now. Nothing breaks: subscribers can see the plans and subscribe, and the plan conditions apply once the API is deployed with {count, plural, one {this policy} other {these policies}}.',
  },
});

const bold = (chunks: ReactNode[]) => <strong>{chunks}</strong>;

/**
 * The policies the selected plans need but the API does not have yet. Publishing does not wait
 * for them: the person who deploys the API may not be the one who publishes it. Rendered only
 * when something is missing; a complete API gets no note at all.
 */
export function RequiredPoliciesNote({
  apiName,
  missing,
}: {
  apiName: string;
  missing: readonly string[];
}) {
  if (missing.length === 0) return null;
  const count = missing.length;
  return (
    <Alert icon={<TriangleAlert size={18} />} severity="warning">
      <Typography variant="body2">
        <FormattedMessage {...messages.attach} values={{ apiName, b: bold, count }} />
      </Typography>
      <Box component="ul" sx={{ my: 0.75, pl: 2.5 }}>
        {missing.map((name) => (
          <li key={name}>
            <Box component="code" sx={{ fontFamily: 'monospace' }}>
              {name}
            </Box>
          </li>
        ))}
      </Box>
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.publishAnyway} values={{ count }} />
      </Typography>
    </Alert>
  );
}
