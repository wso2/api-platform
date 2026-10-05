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

import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { useNotifications } from '@/components/Notifications';

const messages = defineMessages({
  copyFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.useCopy.copyFailed',
    defaultMessage: 'Could not copy. Select the value and copy it manually.',
  },
});

/** Copies a value to the clipboard; `copied` flips once it lands. */
export const useCopy = () => {
  const intl = useIntl();
  const { notify } = useNotifications();
  const [copied, setCopied] = useState(false);

  const copy = (value: string) => {
    // Clipboard access may be unavailable outside secure contexts.
    const written = navigator.clipboard?.writeText(value);
    if (!written) {
      notify(intl.formatMessage(messages.copyFailed), 'error');
      return;
    }
    written
      .then(() => setCopied(true))
      .catch(() => notify(intl.formatMessage(messages.copyFailed), 'error'));
  };

  return { copied, copy };
};
