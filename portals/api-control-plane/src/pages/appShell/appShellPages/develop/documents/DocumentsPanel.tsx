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

import { defineMessages, FormattedMessage } from 'react-intl';

import { ComingSoon } from '@/components/ComingSoon';

const messages = defineMessages({
  feature: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.DocumentsTab.feature',
    defaultMessage: 'Documents for this API',
  },
  detail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.DocumentsTab.detail',
    defaultMessage:
      'You will be able to publish guides, references, and release notes alongside the API so consumers can read them in the Developer Portal.',
  },
});

export function DocumentsPanel() {
  return (
    <ComingSoon
      detail={<FormattedMessage {...messages.detail} />}
      feature={<FormattedMessage {...messages.feature} />}
    />
  );
}
