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
 *
 * Slot & Hideable — the two primitives every host-injected extension point
 * in this app is built from. Deliberately generic: no dependency on this
 * portal's own scope/auth/routing types, so this file is meant to be copied
 * verbatim into any other host app that wants the same seam (see the
 * `apip-cloud-extensions` skill).
 *
 * - Slot: a named, additive extension point. A host exposes a slot name
 *   (e.g. "sidebar.project", "settings.organization.tabs"); anything can
 *   register an entry against that name without the host knowing what it
 *   is. New slot names cost nothing to introduce later — no enum to extend.
 * - Hideable: a named, suppressible region wrapping *built-in* UI, so an
 *   extension can eventually replace (not just add to) something the host
 *   ships by default. Orthogonal to Slot: Slot adds, Hideable suppresses;
 *   using both at the same conceptual position is how you override.
 */

import { createContext, useContext, useMemo, type ReactNode } from 'react';

/** The minimum shape any slot-registered entry must have. */
export type SlotEntry = {
  id: string;
  /** Which named extension point this entry attaches to. */
  slot: string;
  /** Merge-sort key among other entries in the same slot. */
  order: number;
};

const SlotEntriesContext = createContext<readonly SlotEntry[]>([]);

export function SlotEntriesProvider({
  entries,
  children,
}: {
  entries: readonly SlotEntry[];
  children: ReactNode;
}) {
  return (
    <SlotEntriesContext.Provider value={entries}>
      {children}
    </SlotEntriesContext.Provider>
  );
}

/** Every registered entry, unfiltered — for consumers that group by more than one slot name (e.g. a prefix match). */
export function useSlotEntries<T extends SlotEntry>(): readonly T[] {
  return useContext(SlotEntriesContext) as readonly T[];
}

/** Entries registered for exactly `slotName`, sorted by `order`. */
export function useSlot<T extends SlotEntry>(slotName: string): T[] {
  const entries = useSlotEntries<T>();
  return useMemo(
    () =>
      entries
        .filter((entry) => entry.slot === slotName)
        .sort((a, b) => a.order - b.order),
    [entries, slotName]
  );
}

const HiddenRegionsContext = createContext<ReadonlySet<string>>(new Set());

export function HiddenRegionsProvider({
  hidden,
  children,
}: {
  hidden: ReadonlySet<string> | readonly string[];
  children: ReactNode;
}) {
  const set = useMemo(
    () => (hidden instanceof Set ? hidden : new Set(hidden)),
    [hidden]
  );
  return (
    <HiddenRegionsContext.Provider value={set}>
      {children}
    </HiddenRegionsContext.Provider>
  );
}

/** Whether `name` has been suppressed by a `HiddenRegionsProvider` up the tree. */
export function useIsHidden(name: string): boolean {
  return useContext(HiddenRegionsContext).has(name);
}

/** Renders `children` unless `name` is currently hidden. */
export function Hideable({
  name,
  children,
}: {
  name: string;
  children: ReactNode;
}) {
  return useIsHidden(name) ? null : <>{children}</>;
}
