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

/** How a pick on the Capabilities tab is written into the definition; anything else in it is kept as is. */

export type CapabilityKind = 'tools' | 'resources' | 'prompts';

export const CAPABILITY_KINDS: CapabilityKind[] = ['tools', 'resources', 'prompts'];

export type CapabilityItem = Record<string, unknown>;

export type CapabilityRow = {
  key: string;
  item: CapabilityItem;
  /** Whether the proxy itself offers it, as opposed to an item that only the definition holds. */
  onProxy: boolean;
  selected: boolean;
};

const isRecord = (value: unknown): value is CapabilityItem =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/** Tools and prompts are told apart by name, resources by URI. */
const keyOf = (kind: CapabilityKind, item: CapabilityItem, index: number): string => {
  const key = kind === 'resources' ? item.uri : item.name;
  return typeof key === 'string' && key !== '' ? key : `#${index}`;
};

const itemsOf = (definition: Record<string, unknown>, kind: CapabilityKind): CapabilityItem[] =>
  Array.isArray(definition[kind]) ? (definition[kind] as unknown[]).filter(isRecord) : [];

/**
 * The proxy's own items in its order, then any the definition holds that the proxy no longer
 * offers. An item the definition holds keeps the definition's version of it.
 */
export const capabilityRows = (
  kind: CapabilityKind,
  available: CapabilityItem[],
  definition: Record<string, unknown>
): CapabilityRow[] => {
  const chosen = new Map(itemsOf(definition, kind).map((item, index) => [keyOf(kind, item, index), item]));
  const rows: CapabilityRow[] = available.map((item, index) => {
    const key = keyOf(kind, item, index);
    const own = chosen.get(key);
    chosen.delete(key);
    return { key, item: own ?? item, onProxy: true, selected: own !== undefined };
  });
  chosen.forEach((item, key) => rows.push({ key, item, onProxy: false, selected: true }));
  return rows;
};

const withSelection = (
  definition: Record<string, unknown>,
  kind: CapabilityKind,
  available: CapabilityItem[],
  isSelected: (row: CapabilityRow) => boolean
): Record<string, unknown> => ({
  tools: [],
  resources: [],
  prompts: [],
  ...definition,
  [kind]: capabilityRows(kind, available, definition)
    .filter(isSelected)
    .map((row) => row.item),
});

export const toggleCapability = (
  definition: Record<string, unknown>,
  kind: CapabilityKind,
  available: CapabilityItem[],
  key: string
) => withSelection(definition, kind, available, (row) => (row.key === key ? !row.selected : row.selected));

export const setAllCapabilities = (
  definition: Record<string, unknown>,
  kind: CapabilityKind,
  available: CapabilityItem[],
  selected: boolean
) => withSelection(definition, kind, available, () => selected);

export const definitionToJson = (definition: Record<string, unknown>): string => JSON.stringify(definition, null, 2);
