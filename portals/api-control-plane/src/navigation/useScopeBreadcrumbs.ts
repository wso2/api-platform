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

import type { GraphQLApiDetail } from '../api/resources/graphqlApis';
import type { Project } from '../api/resources/projects';
import type { RestApi } from '../api/resources/restApis';
import { routes } from '../routes/paths';
import type { ConsoleRouteParams } from '../scope/ConsoleScopeContext';
import { useConsoleScope } from '../scope/ConsoleScopeProvider';

const messages = defineMessages({
  // The ID predates this hook (it was declared in `AppLayout`); kept as-is so
  // existing translations are not discarded.
  home: { id: 'appLayout.breadcrumb.home', defaultMessage: 'Home' },
});

/** One crumb's data — a `path` rather than an `onClick`, so this stays a pure function to unit test. */
type ScopeCrumb = { key: string; label: string; path: string };

/**
 * The breadcrumb trail for the current scope, org down to whichever kind of
 * API (if any) is open — a pure function so it can be unit tested without
 * rendering a page.
 *
 * REST and GraphQL APIs are mutually exclusive tiers here, exactly like
 * `params.apiHandler`/`params.graphqlApiHandler` themselves (see
 * `ConsoleRouteParams`): a GraphQL route never sets `apiHandler`, so without
 * its own branch reading `graphqlApiHandler`/`graphqlComponent`, the trail
 * silently stopped one level short, at the project.
 */
export const buildScopeCrumbs = (
  params: ConsoleRouteParams,
  homeLabel: string,
  project?: Project,
  component?: RestApi,
  graphqlComponent?: GraphQLApiDetail,
): ScopeCrumb[] => {
  const crumbs: ScopeCrumb[] = [];
  if (params.orgHandle) {
    crumbs.push({ key: 'org', label: homeLabel, path: routes.organizationHome(params.orgHandle) });
  }
  if (params.orgHandle && params.projectHandler) {
    crumbs.push({
      key: 'project',
      label: project?.displayName || params.projectHandler,
      path: routes.projectHome(params.orgHandle, params.projectHandler),
    });
  }
  if (params.orgHandle && params.projectHandler && params.apiHandler) {
    crumbs.push({
      key: 'api',
      label: component?.displayName || params.apiHandler,
      path: routes.api(params.orgHandle, params.projectHandler, params.apiHandler),
    });
  }
  if (params.orgHandle && params.projectHandler && params.graphqlApiHandler) {
    crumbs.push({
      key: 'api',
      label: graphqlComponent?.displayName || params.graphqlApiHandler,
      path: routes.graphqlApi(params.orgHandle, params.projectHandler, params.graphqlApiHandler),
    });
  }
  return crumbs;
};

/**
 * The breadcrumb trail for the current route scope: Home → project → API, as
 * far as the URL carries handles. The final crumb is the current page and so
 * has no `onClick`. Every page shows the same trail because it is derived from
 * scope here, once, rather than written out by each page.
 */
export function useScopeBreadcrumbs(): BreadcrumbItem[] {
  const intl = useIntl();
  const navigate = useNavigate();
  const { project, component, graphqlComponent, params } = useConsoleScope();

  const crumbs = buildScopeCrumbs(
    params,
    intl.formatMessage(messages.home),
    project,
    component,
    graphqlComponent,
  );
  return crumbs.map((crumb, index) => ({
    key: crumb.key,
    label: crumb.label,
    onClick: index === crumbs.length - 1 ? undefined : () => navigate(crumb.path),
  }));
}
