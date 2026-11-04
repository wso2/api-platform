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

import type { FieldNode, FormState, MapEntry, Path, PolicyFormModel } from './types';
import { clone, isEmpty } from './utils';

/**
 * Initial form state. With no config, defaults are pre-filled. With an existing config
 * (editing an attached policy), its values are loaded and missing defaults are filled in.
 */
export function initialState(model: PolicyFormModel, config?: Record<string, unknown>): FormState {
  const values = (fill(
    model.root,
    config === undefined ? undefined : toState(model.root, config),
  ) ?? {}) as Record<string, unknown>;
  return { values, choices: {} };
}

/** The starting value for a field (used for new forms, enabling a toggle, adding a list item). */
export function emptyValue(field: FieldNode): unknown {
  return fill(field, undefined, true);
}

function fill(field: FieldNode, value: unknown, force = false): unknown {
  switch (field.kind) {
    case 'object': {
      if (value === undefined && field.toggleable && !force) return undefined;
      const obj: Record<string, unknown> = { ...((value as Record<string, unknown>) ?? {}) };
      const grouped = new Set((field.choiceGroups ?? []).flatMap((g) => g.members));
      for (const c of field.children ?? []) {
        // A section inside a choice group stays empty until chosen, so it isn't mistaken for a choice.
        if (grouped.has(c.key) && obj[c.key] === undefined && c.kind === 'object') continue;
        const next = fill(c, obj[c.key]);
        if (next === undefined) delete obj[c.key];
        else obj[c.key] = next;
      }
      return obj;
    }
    case 'objectList':
      if (Array.isArray(value)) return value.map((v) => fill(field.item!, v, true));
      return field.hasDefault ? toState(field, clone(field.default)) : [];
    case 'list':
    case 'multiselect':
      if (Array.isArray(value)) return value;
      return field.hasDefault && Array.isArray(field.default) ? clone(field.default) : [];
    case 'map':
      if (Array.isArray(value)) return value;
      return field.hasDefault ? toState(field, clone(field.default)) : [];
    case 'json':
      if (value !== undefined) return value;
      return field.hasDefault ? JSON.stringify(field.default, null, 2) : '';
    case 'boolean':
      if (value !== undefined) return value;
      return field.hasDefault ? field.default : field.required ? false : undefined;
    default:
      if (value !== undefined) return value;
      return field.hasDefault ? clone(field.default) : undefined;
  }
}

/** A value with its missing defaults filled in (used when a chosen section has no value yet). */
export function withDefaults(field: FieldNode, value: unknown): unknown {
  return fill(field, value, true);
}

/** Config value → state value (maps become entry arrays, json becomes text). */
export function toState(field: FieldNode, value: unknown): unknown {
  if (value === undefined || value === null) return value;
  switch (field.kind) {
    case 'object': {
      if (typeof value !== 'object' || Array.isArray(value)) return value;
      const out: Record<string, unknown> = { ...(value as Record<string, unknown>) };
      for (const c of field.children ?? []) if (c.key in out) out[c.key] = toState(c, out[c.key]);
      return out;
    }
    case 'objectList':
    case 'list':
    case 'multiselect':
      return Array.isArray(value) ? value.map((v) => toState(field.item!, v)) : value;
    case 'map':
      if (typeof value !== 'object' || Array.isArray(value)) return [];
      return Object.entries(value as Record<string, unknown>).map(([key, v]): MapEntry => ({
        key,
        value: toState(field.valueField!, v),
      }));
    case 'json':
      // Always encoded, so a saved string `hello` shows as the JSON text `"hello"` that parses back.
      return JSON.stringify(value, null, 2);
    default:
      return value;
  }
}

/** Which field a state path points at. Map entries are addressed as [...mapPath, index, 'key' | 'value']. */
export function fieldAt(model: PolicyFormModel, path: Path): FieldNode | undefined {
  let f: FieldNode | undefined = model.root;
  for (let i = 0; i < path.length && f; i++) {
    const p = path[i];
    if (f.kind === 'object') f = f.children?.find((c) => c.key === p);
    else if (f.kind === 'objectList' || f.kind === 'list' || f.kind === 'multiselect')
      f = typeof p === 'number' ? f.item : undefined;
    else if (f.kind === 'map') {
      if (typeof p !== 'number') return undefined;
      const part = path[i + 1];
      if (part === undefined) return f.valueField;
      i++;
      f = part === 'value' ? f.valueField : undefined;
      if (part === 'key') return undefined;
    } else return undefined;
  }
  return f;
}

/** Which field a *config* path points at (map keys are arbitrary strings). */
export function fieldAtConfigPath(root: FieldNode, path: Path): FieldNode | undefined {
  let f: FieldNode | undefined = root;
  for (const p of path) {
    if (!f) return undefined;
    if (f.kind === 'object') f = f.children?.find((c) => c.key === p);
    else if (f.kind === 'objectList' || f.kind === 'list' || f.kind === 'multiselect') f = f.item;
    else if (f.kind === 'map') f = f.valueField;
    else return undefined;
  }
  return f;
}

/** Coerce what an input gives back into the field's type. Empty input → undefined. */
export function coerceInput(field: FieldNode, raw: unknown): unknown {
  if (field.kind === 'integer' || field.kind === 'number') {
    if (typeof raw === 'number') return raw;
    if (isEmpty(raw)) return undefined;
    const n = Number(String(raw).trim());
    return Number.isFinite(n) ? n : raw;
  }
  if (field.kind === 'select' && field.options) {
    if (isEmpty(raw)) return undefined;
    const hit = field.options.find((o) => String(o.value) === String(raw));
    return hit ? hit.value : raw;
  }
  if (typeof raw === 'string' && raw === '' && field.kind !== 'json') return undefined;
  return raw;
}
