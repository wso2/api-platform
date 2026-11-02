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

import type {
  BuildOptions,
  ChoiceGroup,
  ConditionalRule,
  Constraints,
  FieldKind,
  FieldNode,
  JsonSchema,
  PolicyDefinition,
  PolicyFormModel,
} from './types';
import { humanize, MULTILINE_KEY, SECRET_KEY } from './utils';

const ADVANCED = 'x-wso2-policy-advanced-param';
const DEPRECATED = 'x-wso2-policy-deprecated-param';
const ORDER = 'x-wso2-policy-property-order';
const GATEWAY_DEFAULT = 'wso2/defaultValue';

const CONSTRAINT_KEYS: (keyof Constraints)[] = [
  'minLength',
  'maxLength',
  'pattern',
  'format',
  'minimum',
  'maximum',
  'exclusiveMinimum',
  'exclusiveMaximum',
  'multipleOf',
  'minItems',
  'maxItems',
  'uniqueItems',
  'minProperties',
  'maxProperties',
];

/** Turn a parsed policy definition into a form model. */
export function buildModel(def: PolicyDefinition, options: BuildOptions = {}): PolicyFormModel {
  const fields: Record<string, FieldNode> = {};
  const schema = def.parameters ?? { type: 'object', properties: {} };
  const root = buildField(schema, '', '', true, false, fields);
  if (root.kind !== 'object') {
    root.kind = 'object';
    root.children = [];
  } // policies with no parameters
  root.label = def.displayName ?? humanize(def.name);
  let system: FieldNode | undefined;
  if (def.systemParameters) {
    const sysFields: Record<string, FieldNode> = {};
    system = buildField(def.systemParameters, '', '', true, false, sysFields);
    markReadOnly(system);
  }
  applyOverrides(fields, options);
  return {
    name: def.name,
    version: def.version,
    displayName: def.displayName ?? humanize(def.name),
    description: typeof def.description === 'string' ? def.description.trim() : undefined,
    schema,
    root,
    system,
    fields,
  };
}

/** Effective type of a schema node. */
export function schemaType(s: JsonSchema): string | undefined {
  let t = s.type;
  if (Array.isArray(t)) t = t.find((x: string) => x !== 'null');
  if (typeof t === 'string') return t;
  if (s.properties || s.additionalProperties) return 'object';
  if (s.items) return 'array';
  if (Array.isArray(s.enum) && s.enum.length)
    return typeof s.enum[0] === 'number' ? 'number' : typeof s.enum[0];
  if ('const' in s) return typeof s.const;
  return undefined;
}

function constraintsOf(s: JsonSchema): Constraints {
  // Some definitions put constraints under a non-standard `validation:` block (api-key-auth).
  const merged: JsonSchema = { ...(isObj(s.validation) ? s.validation : {}), ...s };
  const c: Constraints = {};
  for (const k of CONSTRAINT_KEYS)
    if (merged[k] !== undefined) (c as Record<string, unknown>)[k] = merged[k];
  return c;
}

function isObj(v: unknown): v is JsonSchema {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function kindOf(s: JsonSchema, key: string): FieldKind {
  const t = schemaType(s);
  if ('const' in s && t !== 'object' && t !== 'array') return 'constant';
  switch (t) {
    case 'string': {
      if (Array.isArray(s.enum)) return 'select';
      if (s.format === 'password' || SECRET_KEY.test(key)) return 'secret';
      if (s.format === 'uri' || s.format === 'url' || s.format === 'uri-reference') return 'url';
      const max = constraintsOf(s).maxLength;
      const desc = typeof s.description === 'string' ? s.description : '';
      if (MULTILINE_KEY.test(key) || (max !== undefined && max > 1024) || /\bPEM\b/.test(desc))
        return 'textarea';
      return 'text';
    }
    case 'integer':
      return Array.isArray(s.enum) ? 'select' : 'integer';
    case 'number':
      return Array.isArray(s.enum) ? 'select' : 'number';
    case 'boolean':
      return 'boolean';
    case 'object':
      if (isObj(s.properties) && Object.keys(s.properties).length) return 'object';
      if (isObj(s.additionalProperties)) return 'map';
      return 'json';
    case 'array': {
      const it = isObj(s.items) ? s.items : undefined;
      if (!it) return 'json';
      const itType = schemaType(it);
      if (itType === 'object') return isObj(it.properties) ? 'objectList' : 'json';
      if (itType === 'array' || itType === undefined) return 'json';
      if (Array.isArray(it.enum)) return 'multiselect';
      return 'list';
    }
    default:
      return 'json';
  }
}

function orderedKeys(s: JsonSchema): string[] {
  const keys = Object.keys(s.properties ?? {});
  const order: string[] = Array.isArray(s[ORDER])
    ? s[ORDER].filter((k: string) => keys.includes(k))
    : [];
  return [...order, ...keys.filter((k) => !order.includes(k))];
}

function buildField(
  s: JsonSchema,
  key: string,
  id: string,
  required: boolean,
  parentAdvanced: boolean,
  fields: Record<string, FieldNode>,
): FieldNode {
  s = isObj(s) ? s : {};
  const kind = kindOf(s, key);
  const node: FieldNode = {
    id,
    key,
    kind,
    label: typeof s.title === 'string' ? s.title : humanize(key),
    description: typeof s.description === 'string' ? s.description.trim() : undefined,
    schema: s,
    required,
    advanced: s[ADVANCED] === true || parentAdvanced,
    deprecated: s[DEPRECATED] === true || /^\s*DEPRECATED\b/.test(s.description ?? ''),
    readOnly: s.readOnly === true || kind === 'constant',
    hasDefault: 'default' in s,
    default: s.default,
    gatewayDefault: typeof s[GATEWAY_DEFAULT] === 'string' ? s[GATEWAY_DEFAULT] : undefined,
    constraints: constraintsOf(s),
  };
  if (kind === 'constant') {
    node.hasDefault = true;
    node.default = s.const;
  }
  if (Array.isArray(s.enum))
    node.options = s.enum.map((v: unknown) => ({ value: v, label: String(v) }));
  if (Array.isArray(s.examples) && s.examples.length) node.placeholder = String(s.examples[0]);
  else if (s.example !== undefined && typeof s.example !== 'object')
    node.placeholder = String(s.example);
  fields[id] = node;

  if (kind === 'object') {
    const req: string[] = Array.isArray(s.required) ? s.required : [];
    node.children = orderedKeys(s).map((k) =>
      buildField(s.properties[k], k, id ? `${id}.${k}` : k, req.includes(k), false, fields),
    );
    const { groups, rules } = analyseCombinators(s, node);
    if (groups.length) node.choiceGroups = groups;
    if (rules.length) {
      node.rules = rules;
      node.dependentKeys = dependentKeys(rules, req, groups);
    }
    node.toggleable = !required && id !== '' && hasRequiredChildren(node);
    // a section controlled by a choice group is switched on and off by the group, not its own toggle
    for (const g of groups)
      for (const m of g.members) {
        const c = node.children.find((x) => x.key === m);
        if (c) c.toggleable = false;
      }
  } else if (kind === 'list' || kind === 'multiselect' || kind === 'objectList') {
    node.item = buildField(s.items, key, `${id}[]`, true, false, fields);
    if (kind === 'multiselect') node.options = node.item.options;
  } else if (kind === 'map') {
    node.valueField = buildField(s.additionalProperties, key, `${id}{}`, true, false, fields);
  }
  return node;
}

function hasRequiredChildren(node: FieldNode): boolean {
  return (
    (node.children ?? []).some((c) => c.required) ||
    (node.choiceGroups ?? []).some((g) => !g.multi || g.min > 0)
  );
}

function markReadOnly(node: FieldNode) {
  node.readOnly = true;
  node.children?.forEach(markReadOnly);
  if (node.item) markReadOnly(node.item);
  if (node.valueField) markReadOnly(node.valueField);
}

// ---------------------------------------------------------------------------------------
// Combinators → choice groups and conditional rules
// ---------------------------------------------------------------------------------------

/** A branch that only states which keys are present/absent, e.g. { required: [a], not: { required: [b] } }. */
function presenceBranch(b: unknown, props: string[]): string[] | null {
  if (!isObj(b)) return null;
  const allowed = new Set(['required', 'not', 'properties', 'description', 'title']);
  if (Object.keys(b).some((k) => !allowed.has(k))) return null;
  if (!Array.isArray(b.required) || b.required.length === 0) return null;
  if (!b.required.every((k: unknown) => typeof k === 'string' && props.includes(k))) return null;
  // `properties` inside the branch may only tighten "non-empty" (minItems / minProperties / minLength)
  if (b.properties) {
    for (const v of Object.values(b.properties)) {
      if (
        !isObj(v) ||
        Object.keys(v).some((k) => !['minItems', 'minProperties', 'minLength'].includes(k))
      )
        return null;
    }
  }
  return b.required as string[];
}

function analyseCombinators(
  s: JsonSchema,
  node: FieldNode,
): { groups: ChoiceGroup[]; rules: ConditionalRule[] } {
  const props = Object.keys(s.properties ?? {});
  const groups: ChoiceGroup[] = [];
  const rules: ConditionalRule[] = [];
  const labelOf = (k: string) => node.children?.find((c) => c.key === k)?.label ?? humanize(k);

  const tryGroup = (branches: unknown, source: 'oneOf' | 'anyOf') => {
    if (!Array.isArray(branches) || branches.length < 2) return;
    const sets = branches.map((b) => presenceBranch(b, props));
    if (sets.some((x) => x === null)) return;
    const keySets = sets as string[][];
    const members = [...new Set(keySets.flat())];
    if (members.some((m) => groups.some((g) => g.members.includes(m)))) return; // already grouped
    let multi = false;
    let options = keySets;
    if (source === 'anyOf' && keySets.every((k) => k.length === 1)) {
      multi = true; // "at least one of"
    } else if (source === 'oneOf' && isPowerSet(keySets, members)) {
      multi = true; // branches enumerate every non-empty combination
      options = members.map((m) => [m]);
    }
    const id = `${node.id}#${groups.length}`;
    groups.push({
      id,
      label: multi ? 'Configure' : 'Choose one',
      multi,
      min: 1,
      members,
      source,
      options: options.map((keys, i) => ({
        id: String(i),
        keys,
        label: joinLabels(keys.map(labelOf)),
      })),
    });
  };

  tryGroup(s.oneOf, 'oneOf');
  tryGroup(s.anyOf, 'anyOf');

  const addRule = (r: JsonSchema) => {
    if (!isObj(r) || !isObj(r.if)) return;
    const thenReq = isObj(r.then) && Array.isArray(r.then.required) ? r.then.required : [];
    const elseReq = isObj(r.else) && Array.isArray(r.else.required) ? r.else.required : [];
    const discriminators = Object.entries(r.if.properties ?? {})
      .filter(([, v]) => isObj(v) && ('const' in v || Array.isArray(v.enum)))
      .map(([k]) => k);
    rules.push({
      if: r.if,
      then: r.then,
      else: r.else,
      requires: thenReq,
      elseRequires: elseReq,
      discriminators,
    });
  };
  addRule(s);
  if (Array.isArray(s.allOf)) s.allOf.forEach(addRule);
  return { groups, rules };
}

function isPowerSet(sets: string[][], members: string[]): boolean {
  if (members.length < 2 || members.length > 6) return false;
  if (sets.length !== 2 ** members.length - 1) return false;
  const seen = new Set(sets.map((k) => [...k].sort().join('|')));
  return seen.size === sets.length;
}

function joinLabels(labels: string[]): string {
  if (labels.length <= 1) return labels[0] ?? '';
  return `${labels.slice(0, -1).join(', ')} and ${labels[labels.length - 1]}`;
}

/** Keys revealed by discriminator rules (e.g. `expression` only when `type` is "cel"). */
function dependentKeys(
  rules: ConditionalRule[],
  required: string[],
  groups: ChoiceGroup[],
): Record<string, number[]> {
  const grouped = new Set(groups.flatMap((g) => g.members));
  const out: Record<string, number[]> = {};
  rules.forEach((r, i) => {
    if (!r.discriminators.length) return;
    for (const k of r.requires) {
      if (required.includes(k) || grouped.has(k) || r.discriminators.includes(k)) continue;
      (out[k] ??= []).push(i);
    }
  });
  return out;
}

function applyOverrides(fields: Record<string, FieldNode>, options: BuildOptions) {
  const o = options.overrides;
  if (!o) return;
  for (const [id, patch] of Object.entries(o.fields ?? {})) {
    if (fields[id]) Object.assign(fields[id], patch);
  }
  for (const [gid, patch] of Object.entries(o.groups ?? {})) {
    const [fid] = gid.split('#');
    const g = fields[fid]?.choiceGroups?.find((x) => x.id === gid);
    if (!g) continue;
    if (patch.label) g.label = patch.label;
    patch.optionLabels?.forEach((l, i) => {
      if (g.options[i]) g.options[i].label = l;
    });
  }
}
