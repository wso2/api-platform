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

/**
 * Slot for overriding the built-in AI Gateways page (list + create/edit)
 * without changing anything under `pages/appShell/appShellPages/gateways`.
 * Pairs with the `Hideable name={AI_WORKSPACE_GATEWAYS_SLOT}` wrapping that
 * page's built-in element in `App.tsx` — Slot supplies the replacement,
 * Hideable suppresses the built-in, same two-primitive split the header
 * comment in `slots/index.tsx` describes.
 */
export const AI_WORKSPACE_GATEWAYS_SLOT = 'page.gateways';

/**
 * A host-injected replacement for a specific built-in page. Unlike
 * `AIWorkspaceExtension`, this isn't a new sidebar item — the built-in
 * page's own route and sidebar entry stay in place; only what renders at
 * that route changes.
 */
export type AIWorkspacePageOverride = SlotEntry & {
  render: (port: AIWorkspaceHostPort) => ReactNode;
  /**
   * Optional nav placement for the built-in item this override replaces. When
   * `label` is given the sidebar renders the override alongside the sidebar
   * extensions, ordered by `order` — so a cloud build can move the entry into
   * the extension cluster instead of leaving it where the built-in sits. Pair
   * it with `hides` naming the built-in item's own region, or both appear.
   */
  label?: string;
  icon?: ReactNode;
  path?: string;
  /** Built-in `Hideable` regions this entry suppresses (see `slots/index.tsx`). */
  hides?: readonly string[];
};

/**
 * `Hideable` region wrapping the built-in AI Gateways *sidebar item* (the page
 * itself is `AI_WORKSPACE_GATEWAYS_SLOT`). Separate name because a cloud build
 * may want to reposition the nav entry while still rendering at the built-in
 * route.
 */
export const AI_WORKSPACE_GATEWAYS_NAV_REGION = 'nav.gateways';

/** Every `Hideable` region the registered entries suppress. */
export const hiddenRegionsOf = (
  entries: readonly AIWorkspaceCloudEntry[]
): readonly string[] =>
  entries.flatMap((entry) => ('hides' in entry ? (entry.hides ?? []) : []));

/** Every registered cloud entry — sidebar items and page overrides share one slot registry (see `slots/index.tsx`), filtered by `slot` at each consumption site. */
export type AIWorkspaceCloudEntry = AIWorkspaceExtension | AIWorkspacePageOverride;

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly AIWorkspaceCloudEntry[];
  children: ReactNode;
}) {
  return (
    <SlotEntriesProvider entries={extensions}>{children}</SlotEntriesProvider>
  );
}

export function useAIWorkspaceExtensions(): readonly AIWorkspaceExtension[] {
  return useSlotEntries<AIWorkspaceExtension>();
}
