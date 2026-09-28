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

import { defineMessages, useIntl } from 'react-intl';

import { isApiError } from '@/api/core/errors';
import { ErrorState } from '@/components/StateViews';

const messages = defineMessages({
  forbiddenTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublicationLoadError.forbiddenTitle',
    defaultMessage: 'You don’t have permission',
  },
  forbiddenMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublicationLoadError.forbiddenMessage',
    defaultMessage:
      'You don’t have permission to publish APIs to API portals. Contact your organization admin to request access.',
  },
});

type PublicationLoadErrorProps = {
  error: unknown;
  /** Shown for every failure that isn't a permission problem. */
  fallbackMessage: string;
};

/**
 * The error screen for a publish surface that failed to load. The server
 * enforces the `ap:api_portal:*` and `ap:api_publication:*` scopes and answers
 * 403 when one is missing, so that case says so instead of showing the
 * generic failure, which would read as something the user could retry.
 */
export function PublicationLoadError({ error, fallbackMessage }: PublicationLoadErrorProps) {
  const intl = useIntl();

  if (isApiError(error) && error.isForbidden) {
    return (
      <ErrorState
        message={intl.formatMessage(messages.forbiddenMessage)}
        title={intl.formatMessage(messages.forbiddenTitle)}
      />
    );
  }
  return <ErrorState message={fallbackMessage} />;
}
