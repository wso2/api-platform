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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { FormattedMessage } from 'react-intl';

import { ComingSoon } from '@/components/ComingSoon';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiPortalPublicationsList } from './ApiPortalPublicationsList';

/**
 * Mounted at all three scope levels (see `AppRoutes.tsx`). Only the API level
 * has a real page today — publishing an API to a portal is inherently
 * API-scoped. Portal registration/management at the organization and project
 * levels is a separate, not-yet-built feature.
 */
export function PortalsPage() {
  const { isApiScope } = useConsoleScope();

  if (isApiScope) return <ApiPortalPublicationsList />;

  return (
    <ComingSoon
      feature={
        <FormattedMessage
          id="apiControlPlane.pages.appShell.appShellPages.portals.PortalsPage.feature"
          defaultMessage="Portals"
        />
      }
    />
  );
}
