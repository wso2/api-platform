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

import type { ReactNode } from 'react';

import type { ConsoleScope } from './scope/ConsoleScopeProvider';
import type { NavigationLevel } from './navigation/navigationTypes';
import type { CloudHostPort } from './hostPort';
import {
  SlotEntriesProvider,
  useSlotEntries,
  type SlotEntry,
} from './slots';

/**
 * A host-injected feature. `routePath` is relative to the same route group
 * the built-in pages live in (e.g. `"billing"` or `"settings/environments"`,
 * never an absolute `/organizations/...` path), and `scope` decides the URL
 * shape (organization/project/api) the same way the built-in pages' own
 * `level` does.
 *
 * `slot` is the named extension point this entry attaches to (see
 * `slots/index.tsx`) — e.g. `"sidebar.project"` for a top-level project nav
 * item, or `"settings.project.tabs"` to appear as a Settings sub-nav tab.
 * New slot names can be introduced by core without changing this type.
 *
 * `render` receives the small, portable `CloudHostPort` (org/project handle,
 * navigate, notify) instead of a pre-built element, so the same feature
 * component can be reused by another host app without depending on this
 * portal's own hooks — see `hostPort.tsx`.
 */
export type ApiControlPlaneExtension = SlotEntry & {
  routePath: string;
  render: (port: CloudHostPort) => ReactNode;
  label: string;
  icon?: ReactNode;
  scope: NavigationLevel;
  isVisible?: (scope: ConsoleScope) => boolean;
};

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly ApiControlPlaneExtension[];
  children: ReactNode;
}) {
  return (
    <SlotEntriesProvider entries={extensions}>{children}</SlotEntriesProvider>
  );
}

export function useExtensions(): readonly ApiControlPlaneExtension[] {
  return useSlotEntries<ApiControlPlaneExtension>();
}

/**
 * Prefixes an extension's `routePath` with the URL shape for its `scope`
 * (organization/project/api), so both `AppRoutes` (route patterns, `orgHandle`
 * etc. as `:param` placeholders) and the nav pipeline (concrete scope values)
 * build the same URL shape from one place.
 */
export function buildScopedExtensionPath(
  scope: NavigationLevel,
  routeSuffix: string,
  params: { orgHandle: string; projectHandler?: string; apiHandler?: string }
): string {
  if (scope === 'organization') {
    return `/organizations/${params.orgHandle}/${routeSuffix}`;
  }
  if (scope === 'project') {
    return `/organizations/${params.orgHandle}/projects/${params.projectHandler}/${routeSuffix}`;
  }
  return `/organizations/${params.orgHandle}/projects/${params.projectHandler}/apis/${params.apiHandler}/${routeSuffix}`;
}
