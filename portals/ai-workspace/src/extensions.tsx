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

import type { AIWorkspaceHostPort } from './hostPort';
import { SlotEntriesProvider, useSlotEntries, type SlotEntry } from './slots';

/**
 * The only slot this host currently exposes: a top-level sidebar item (plus
 * a matching route mounted at both the org- and project-scoped URL, mirroring
 * wherever the sidebar link currently points). New slot names — e.g. a
 * Settings sub-nav tab — can be introduced later without changing this type.
 */
export const AI_WORKSPACE_SIDEBAR_SLOT = 'sidebar.main';

/**
 * A host-injected feature: a sidebar item plus its route. `path` is relative
 * to the same route group the built-in pages live in (e.g. `"billing"`, not
 * `/organizations/:orgSlug/billing`) — resolved against the org or project
 * currently in view, the same way the built-in sidebar links are.
 *
 * `render` receives the small, portable `AIWorkspaceHostPort` (org/project
 * handle, navigate, notify) instead of a pre-built element, so the same
 * feature component can be reused by another host app without depending on
 * this portal's own hooks — see `hostPort.tsx`.
 */
export type AIWorkspaceExtension = SlotEntry & {
  path: string;
  label: string;
  icon?: ReactNode;
  render: (port: AIWorkspaceHostPort) => ReactNode;
};

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly AIWorkspaceExtension[];
  children: ReactNode;
}) {
  return (
    <SlotEntriesProvider entries={extensions}>{children}</SlotEntriesProvider>
  );
}

export function useAIWorkspaceExtensions(): readonly AIWorkspaceExtension[] {
  return useSlotEntries<AIWorkspaceExtension>();
}
