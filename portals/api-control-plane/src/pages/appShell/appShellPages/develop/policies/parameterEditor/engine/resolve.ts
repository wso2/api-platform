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
  ChoiceGroup,
  FieldNode,
  FormState,
  GroupView,
  MapEntry,
  MapEntryView,
  Path,
  PolicyFormModel,
  Resolution,
  ViewNode,
} from './types';
import { fieldAtConfigPath, withDefaults } from './state';
import { matches, validate } from './validate';
import { configKey, deepEqual, isEmpty, pathKey, setOwn } from './utils';

interface Out {
  has: boolean;
  full?: unknown;
  compact?: unknown;
  compactHas: boolean;
}

interface Walk {
  state: FormState;
  /** config path key → state path key */
  pathMap: Map<string, string>;
  nodes: Map<string, ViewNode>;
  groupNodes: Map<string, GroupView>;
  local: { pathKey: string; path: Path; message: string }[];
  /** visible, required, but empty */
  missing: ViewNode[];
}

/** State key for a choice group instance (the same group appears once per array item). */
export function groupKey(objectPath: Path, group: ChoiceGroup): string {
  return `${pathKey(objectPath)}#${group.id.split('#')[1]}`;
}

/**
 * Work out everything the UI needs for the current state: which fields are visible and
 * required, the config to save (full and compact), and validation errors placed on fields.
 */
export function resolve(model: PolicyFormModel, state: FormState): Resolution {
  const w: Walk = {
    state,
    pathMap: new Map(),
    nodes: new Map(),
    groupNodes: new Map(),
    local: [],
    missing: [],
  };
  const [view, out] = walkField(model.root, state.values, [], [], true, true, w);
  const config = (out.full ?? {}) as Record<string, unknown>;
  const compactConfig = (out.compact ?? {}) as Record<string, unknown>;

  const label = (p: Path) => {
    const f = fieldAtConfigPath(model.root, p);
    return f?.label ?? String(p[p.length - 1] ?? '');
  };
  const schemaErrors = validate(model.schema, config, { label });

  const errors: Resolution['errors'] = [...w.local];
  for (const e of schemaErrors) {
    // find the closest visible node for the error's config path
    let p = e.path.slice();
    let key: string | undefined;
    while (true) {
      key = w.pathMap.get(configKey(p));
      if (key !== undefined || p.length === 0) break;
      p = p.slice(0, -1);
    }
    key = key ?? '';
    errors.push({ pathKey: key, path: e.path, message: e.message });
  }
  for (const e of errors) {
    const groupView = w.groupNodes.get(e.pathKey);
    const node = w.nodes.get(e.pathKey);
    // object-level combinator errors are most useful next to the group selector
    if (
      groupView &&
      node?.field.kind === 'object' &&
      /^(Either|Set exactly|Only one|Select at least)/.test(e.message)
    ) {
      if (!groupView.errors.includes(e.message)) groupView.errors.push(e.message);
    } else if (node && !node.errors.includes(e.message)) {
      node.errors.push(e.message);
    }
  }
  // Fields made required by a choice or a rule get their own "Required", unless the
  // validator already said something more specific ("Required when …").
  for (const n of w.missing) {
    if (n.errors.some((m) => m.startsWith('Required'))) continue;
    n.errors.push('Required');
    errors.push({ pathKey: n.pathKey, path: n.path, message: 'Required' });
  }
  // A group whose chosen fields are simply empty doesn't need a second, vaguer message.
  for (const g of w.groupNodes.values()) {
    const owner = [...w.nodes.values()].find((n) => n.groups?.includes(g));
    const memberMissing = owner?.children?.some(
      (c) => g.group.members.includes(c.field.key) && c.visible && c.errors.length > 0,
    );
    // "Select at least one" already says it; drop the vaguer schema message.
    if (owner && g.errors.some((m) => m.startsWith('Select at least'))) {
      g.errors = g.errors.filter((m) => !/^(Either|Set exactly)/.test(m));
      for (let i = errors.length - 1; i >= 0; i--) {
        if (errors[i].pathKey === owner.pathKey && /^(Either|Set exactly)/.test(errors[i].message))
          errors.splice(i, 1);
      }
    }
    if (memberMissing && owner) {
      g.errors = g.errors.filter((m) => !/^(Either|Set exactly)/.test(m));
      for (let i = errors.length - 1; i >= 0; i--) {
        if (errors[i].pathKey === owner.pathKey && /^(Either|Set exactly)/.test(errors[i].message))
          errors.splice(i, 1);
      }
    }
  }
  return { view, config, compactConfig, errors, valid: errors.length === 0 };
}

function nodeOf(
  field: FieldNode,
  path: Path,
  value: unknown,
  visible: boolean,
  required: boolean,
): ViewNode {
  return { field, path, pathKey: pathKey(path), value, visible, required, errors: [] };
}

function walkField(
  field: FieldNode,
  value: unknown,
  path: Path,
  cfgPath: Path,
  required: boolean,
  visible: boolean,
  w: Walk,
): [ViewNode, Out] {
  const node = nodeOf(field, path, value, visible, required);
  if (!visible) return [node, { has: false, compactHas: false }];
  w.nodes.set(node.pathKey, node);
  w.pathMap.set(configKey(cfgPath), node.pathKey);

  switch (field.kind) {
    case 'object':
      return walkObject(field, node, value, path, cfgPath, required, w);
    case 'objectList':
    case 'list':
    case 'multiselect':
      return walkList(field, node, value, path, cfgPath, required, w);
    case 'map':
      return walkMap(field, node, value, path, cfgPath, required, w);
    case 'json': {
      const text =
        typeof value === 'string' ? value : value === undefined ? '' : JSON.stringify(value);
      if (!text.trim()) {
        if (required) w.missing.push(node);
        return [node, { has: false, compactHas: false }];
      }
      try {
        const parsed = JSON.parse(text);
        const isDefault = field.hasDefault && !required && deepEqual(parsed, field.default);
        return [node, { has: true, full: parsed, compact: parsed, compactHas: !isDefault }];
      } catch (err) {
        w.local.push({
          pathKey: node.pathKey,
          path,
          message: `Invalid JSON: ${(err as Error).message}`,
        });
        return [node, { has: false, compactHas: false }];
      }
    }
    default: {
      if (isEmpty(value)) {
        if (required && path.length) w.missing.push(node);
        return [node, { has: false, compactHas: false }];
      }
      const isDefault = field.hasDefault && !required && deepEqual(value, field.default);
      return [node, { has: true, full: value, compact: value, compactHas: !isDefault }];
    }
  }
}

function walkObject(
  field: FieldNode,
  node: ViewNode,
  value: unknown,
  path: Path,
  cfgPath: Path,
  required: boolean,
  w: Walk,
): [ViewNode, Out] {
  const enabled = value !== undefined && value !== null;
  if (field.toggleable) node.enabled = enabled;
  if (!enabled && !required) return [node, { has: false, compactHas: false }];
  const obj = (
    typeof value === 'object' && value !== null && !Array.isArray(value) ? value : {}
  ) as Record<string, unknown>;

  // What the object currently "says" — used to evaluate if-conditions and infer choices.
  const probe: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) {
    if (isEmpty(v) || (Array.isArray(v) && v.length === 0)) continue;
    probe[k] = v;
  }

  const req = new Set((field.children ?? []).filter((c) => c.required).map((c) => c.key));
  const hidden = new Set<string>();

  // choice groups
  const groups: GroupView[] = [];
  for (const g of field.choiceGroups ?? []) {
    const instanceKey = groupKey(path, g);
    let selected = w.state.choices[instanceKey];
    if (!selected) selected = inferChoice(g, probe);
    selected = selected.filter((id) => g.options.some((o) => o.id === id));
    if (!g.multi && selected.length === 0) selected = [g.options[0].id];
    const chosenKeys = new Set(
      g.options.filter((o) => selected.includes(o.id)).flatMap((o) => o.keys),
    );
    for (const m of g.members) {
      if (chosenKeys.has(m)) req.add(m);
      else hidden.add(m);
    }
    const gv: GroupView = { group: g, instanceKey, selected, errors: [] };
    if (g.multi && selected.length < g.min) {
      const message = g.min === 1 ? 'Select at least one' : `Select at least ${g.min}`;
      gv.errors.push(message);
      w.local.push({ pathKey: node.pathKey, path, message });
    }
    groups.push(gv);
    w.groupNodes.set(node.pathKey, gv);
  }
  if (groups.length) node.groups = groups;

  // conditional rules
  const ruleMatch = (field.rules ?? []).map((r) => matches(r.if, probe));
  (field.rules ?? []).forEach((r, i) => {
    (ruleMatch[i] ? r.requires : r.elseRequires).forEach((k) => req.add(k));
  });

  const children: ViewNode[] = [];
  const full: Record<string, unknown> = {};
  const compact: Record<string, unknown> = {};
  for (const c of field.children ?? []) {
    let visible = !hidden.has(c.key);
    const deps = field.dependentKeys?.[c.key];
    if (visible && deps) visible = deps.some((i) => ruleMatch[i]);
    if (visible && c.visibleWhen) visible = matches(c.visibleWhen, probe);
    // a chosen section shows (and outputs) its defaults, even before it's edited
    const chosenSection =
      visible && c.kind === 'object' && groups.some((g) => g.group.members.includes(c.key));
    const childValue = chosenSection ? withDefaults(c, obj[c.key]) : obj[c.key];
    const [child, out] = walkField(
      c,
      childValue,
      [...path, c.key],
      [...cfgPath, c.key],
      req.has(c.key),
      visible,
      w,
    );
    children.push(child);
    if (out.has) full[c.key] = out.full;
    if (out.compactHas) compact[c.key] = out.compact;
  }
  node.children = children;
  // An optional section that only holds its defaults stays out of the config, so a section
  // the user never touched can't fail validation (e.g. a guardrail's response settings).
  const touched = Object.keys(compact).length > 0;
  return [
    node,
    {
      has: required || (field.toggleable === true && enabled) || touched,
      full,
      compact,
      compactHas:
        Object.keys(compact).length > 0 || required || (field.toggleable === true && enabled),
    },
  ];
}

function inferChoice(g: ChoiceGroup, probe: Record<string, unknown>): string[] {
  const present = (k: string) => k in probe;
  if (g.multi) {
    const sel = g.options.filter((o) => o.keys.some(present)).map((o) => o.id);
    return sel.length ? sel : [g.options[0].id];
  }
  const full = g.options.find((o) => o.keys.every(present));
  if (full) return [full.id];
  const partial = g.options.find((o) => o.keys.some(present));
  return [(partial ?? g.options[0]).id];
}

function walkList(
  field: FieldNode,
  node: ViewNode,
  value: unknown,
  path: Path,
  cfgPath: Path,
  required: boolean,
  w: Walk,
): [ViewNode, Out] {
  const arr = Array.isArray(value) ? value : [];
  const items: ViewNode[] = [];
  const full: unknown[] = [];
  const compact: unknown[] = [];
  arr.forEach((v, i) => {
    const isObject = field.kind === 'objectList';
    const cfgIndex = full.length;
    const [child, out] = walkField(
      field.item!,
      v,
      [...path, i],
      [...cfgPath, cfgIndex],
      true,
      true,
      w,
    );
    items.push(child);
    if (out.has || isObject) {
      full.push(out.full ?? {});
      compact.push(out.compact ?? out.full ?? {});
    } else {
      // blank row: not part of the config, so don't let it claim this config index
      w.pathMap.delete(configKey([...cfgPath, cfgIndex]));
    }
  });
  node.items = items;
  const isDefault = field.hasDefault && !required && deepEqual(full, field.default);
  const has = full.length > 0 || required;
  return [node, { has, full, compact, compactHas: has && !isDefault }];
}

function walkMap(
  field: FieldNode,
  node: ViewNode,
  value: unknown,
  path: Path,
  cfgPath: Path,
  required: boolean,
  w: Walk,
): [ViewNode, Out] {
  const entries = (Array.isArray(value) ? value : []) as MapEntry[];
  const views: MapEntryView[] = [];
  const full: Record<string, unknown> = {};
  const compact: Record<string, unknown> = {};
  const seen = new Set<string>();
  entries.forEach((e, i) => {
    const key = (e?.key ?? '').trim();
    const keyPath = [...path, i, 'key'];
    const keyErrors: string[] = [];
    const [child, out] = walkField(
      field.valueField!,
      e?.value,
      [...path, i, 'value'],
      key ? [...cfgPath, key] : [...cfgPath, `\u0000${i}`],
      true,
      true,
      w,
    );
    if (!key) {
      if (out.has) keyErrors.push('Enter a name');
    } else if (seen.has(key)) {
      keyErrors.push('Duplicate name');
    } else {
      seen.add(key);
      setOwn(full, key, out.has ? out.full : '');
      setOwn(compact, key, out.has ? out.compact : '');
    }
    keyErrors.forEach((m) =>
      w.local.push({ pathKey: pathKey(keyPath), path: keyPath, message: m }),
    );
    views.push({ index: i, key: e?.key ?? '', keyPath, keyErrors, node: child });
  });
  node.entries = views;
  const has = Object.keys(full).length > 0 || required;
  const isDefault = field.hasDefault && !required && deepEqual(full, field.default);
  return [node, { has, full, compact, compactHas: has && !isDefault }];
}
