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

import { Card, CardContent, Typography } from '@wso2/oxygen-ui';
import { FormattedMessage, defineMessages } from 'react-intl';

const messages = defineMessages({
  mvpScope: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.mvpScope',
    defaultMessage:
      'Advanced organization admin settings, governance, marketplace, and developer portal configuration are intentionally excluded from the MVP replacement app.',
    description:
      'Body copy explaining which settings areas the MVP deliberately leaves out.',
  },
});

/**
 * Shared "General" tab content for both the organization- and project-level
 * Settings pages — the scope name in the copy is the only thing that differs,
 * and the surrounding `SettingsLayout` already names it, so one component
 * covers both rather than two near-duplicates.
 */
export function GeneralSettingsPage() {
  return (
    <Card variant="outlined">
      <CardContent>
        <Typography>
          <FormattedMessage {...messages.mvpScope} />
        </Typography>
      </CardContent>
    </Card>
  );
}
