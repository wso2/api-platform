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

/**
 * Recursive JSON-Schema-like model for policy parameters (matches the legacy
 * console's ParameterSchema). A policy definition's `parameters` is one of
 * these; the config form renders it recursively.
 */
export type ParameterSchema = {
  type: 'object' | 'array' | 'string' | 'boolean' | 'number' | 'integer';
  title?: string;
  description?: string;
  default?: unknown;
  enum?: string[];
  properties?: Record<string, ParameterSchema>;
  /** Value schema for a dynamic key→value map (JSON-Schema additionalProperties). */
  additionalProperties?: ParameterSchema;
  items?: ParameterSchema;
  required?: string[];
  /** From `x-wso2-policy-advanced-param`: force a param into Advanced Settings. */
  advanced?: boolean;
};

export type ParameterValues = Record<string, unknown>;

// --- path helpers (dot path with numeric array indices, e.g. "headers.0.name") ---

const segments = (path: string): (string | number)[] =>
  path
    .split('.')
    .filter((s) => s !== '')
    .map((s) => (/^\d+$/.test(s) ? Number(s) : s));

export const getByPath = (root: unknown, path: string): unknown => {
  let cur: unknown = root;
  for (const seg of segments(path)) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string | number, unknown>)[seg];
  }
  return cur;
};

/** Immutable deep set; creates intermediate objects/arrays as needed. */
export const setByPath = (
  root: ParameterValues,
  path: string,
  value: unknown
): ParameterValues => {
  const segs = segments(path);
  const clone = (node: unknown, depth: number): unknown => {
    if (depth === segs.length) return value;
    const seg = segs[depth];
    if (typeof seg === 'number') {
      const arr = Array.isArray(node) ? [...node] : [];
      arr[seg] = clone(arr[seg], depth + 1);
      return arr;
    }
    const obj = node && typeof node === 'object' && !Array.isArray(node)
      ? { ...(node as Record<string, unknown>) }
      : {};
    obj[seg] = clone(obj[seg], depth + 1);
    return obj;
  };
  return clone(root, 0) as ParameterValues;
};

/** A default value for a single schema node (used when adding array items). */
export const defaultForSchema = (schema: ParameterSchema): unknown => {
  if (schema.default !== undefined) return schema.default;
  switch (schema.type) {
    case 'object': {
      const out: ParameterValues = {};
      for (const [key, child] of Object.entries(schema.properties || {})) {
        const v = defaultForSchema(child);
        if (v !== undefined) out[key] = v;
      }
      return out;
    }
    case 'array':
      return [];
    case 'boolean':
      return false;
    default:
      return undefined;
  }
};

/** Builds initial form values from the schema, overlaying any existing values. */
export const initValues = (
  schema: ParameterSchema,
  existing?: ParameterValues
): ParameterValues => {
  if (existing && Object.keys(existing).length > 0) {
    // Edit mode: start from saved values; fall back to defaults per property.
    const base = (defaultForSchema(schema) as ParameterValues) || {};
    return { ...base, ...existing };
  }
  return (defaultForSchema(schema) as ParameterValues) || {};
};

/**
 * Whether any top-level required property is empty (drives the submit button).
 * Mirrors legacy's level-one required check.
 */
export const topLevelRequiredMissing = (
  schema: ParameterSchema,
  values: ParameterValues
): boolean => {
  const required = schema.required || [];
  return required.some((key) => {
    const v = values[key];
    return v === undefined || v === null || v === '';
  });
};
