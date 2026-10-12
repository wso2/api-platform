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

import type { BreadcrumbItem } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { routes } from '../routes/paths';
import { useConsoleScope } from '../scope/ConsoleScopeProvider';

const messages = defineMessages({
  // The ID predates this hook (it was declared in `AppLayout`); kept as-is so
  // existing translations are not discarded.
  home: { id: 'appLayout.breadcrumb.home', defaultMessage: 'Home' },
});

/**
 * The breadcrumb trail for the current route scope: Home → project → API, as
 * far as the URL carries handles. The final crumb is the current page and so
 * has no `onClick`. Every page shows the same trail because it is derived from
 * scope here, once, rather than written out by each page.
 */
export function useScopeBreadcrumbs(): BreadcrumbItem[] {
  const intl = useIntl();
  const navigate = useNavigate();
  const { project, component, params } = useConsoleScope();

  const crumbs: BreadcrumbItem[] = [];
  if (params.orgHandle) {
    crumbs.push({
      key: 'org',
      label: intl.formatMessage(messages.home),
      onClick: () => navigate(routes.organizationHome(params.orgHandle!)),
    });
  }
  if (params.orgHandle && params.projectHandler) {
    crumbs.push({
      key: 'project',
      label: project?.displayName || params.projectHandler,
      onClick: () => navigate(routes.projectHome(params.orgHandle!, params.projectHandler!)),
    });
  }
  if (params.orgHandle && params.projectHandler && params.apiHandler) {
    crumbs.push({
      key: 'api',
      label: component?.displayName || params.apiHandler,
      onClick: () =>
        navigate(routes.api(params.orgHandle!, params.projectHandler!, params.apiHandler!)),
    });
  }
  return crumbs.map((crumb, index) =>
    index === crumbs.length - 1 ? { ...crumb, onClick: undefined } : crumb,
  );
}
