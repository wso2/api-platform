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

import type { JsonSchema, Path, ValidationError } from './types';
import { deepEqual, formatValue, humanize, typeOf } from './utils';

/**
 * A small JSON Schema validator covering every keyword the gateway policy definitions use
 * (draft-07 subset: type, enum, const, string/number/array/object constraints, format uri,
 * allOf/anyOf/oneOf/not, if/then/else, contains, plus WSO2's non-standard `validation` block).
 *
 * Messages are written for people filling in a form, using field labels from `label()`.
 */
export interface ValidateOptions {
  /** label for a config path; defaults to a humanized last key */
  label?: (path: Path) => string;
}

interface Ctx extends ValidateOptions {
  reason?: string;
}

const TEMPLATED = /\{\{[\s\S]*\}\}|\$\{[^}]*\}/;

export function validate(
  schema: unknown,
  value: unknown,
  options: ValidateOptions = {},
): ValidationError[] {
  return run(schema, value, [], options);
}

export function matches(schema: unknown, value: unknown): boolean {
  return run(schema, value, [], {}).length === 0;
}

function labelFor(ctx: Ctx, path: Path): string {
  if (ctx.label) return ctx.label(path);
  const last = [...path].reverse().find((p) => typeof p === 'string');
  return humanize(String(last ?? 'value'));
}

function typeMessage(t: string): string {
  switch (t) {
    case 'integer':
      return 'Enter a whole number';
    case 'number':
      return 'Enter a number';
    case 'boolean':
      return 'Must be true or false';
    case 'string':
      return 'Enter text';
    case 'array':
      return 'Must be a list';
    case 'object':
      return 'Must be a group of settings';
    default:
      return `Must be ${t}`;
  }
}

function typeMatches(t: string, v: unknown): boolean {
  const a = typeOf(v);
  if (t === 'number') return a === 'number' || a === 'integer';
  return a === t;
}

function run(schema: unknown, value: unknown, path: Path, ctx: Ctx): ValidationError[] {
  if (schema === true || schema === undefined || schema === null) return [];
  if (schema === false) return [{ path, keyword: 'false', message: 'Not allowed' }];
  if (typeof schema !== 'object') return [];
  const raw = schema as JsonSchema;
  const s: JsonSchema =
    raw.validation && typeof raw.validation === 'object' ? { ...raw.validation, ...raw } : raw;
  const errs: ValidationError[] = [];
  const push = (keyword: string, message: string, at: Path = path) =>
    errs.push({ path: at, keyword, message });

  // Template expressions ({{ env "LIMIT" }}, {{ secret "x" }}) are resolved by the gateway at
  // deploy time, so their final value can't be checked here — not even its type.
  if (typeof value === 'string' && TEMPLATED.test(value)) return errs;

  if (s.type !== undefined) {
    const types: string[] = (Array.isArray(s.type) ? s.type : [s.type]).filter((t: string) =>
      ['string', 'number', 'integer', 'boolean', 'object', 'array', 'null'].includes(t),
    );
    if (types.length && !types.some((t) => typeMatches(t, value))) {
      push('type', typeMessage(types[0]));
      return errs;
    }
  }
  if (Array.isArray(s.enum) && !s.enum.some((e: unknown) => deepEqual(e, value))) {
    push('enum', `Choose one of: ${s.enum.map(formatValue).join(', ')}`);
  }
  if ('const' in s && !deepEqual(s.const, value)) push('const', `Must be ${formatValue(s.const)}`);

  const t = typeOf(value);
  if (t === 'string') {
    const str = value as string;
    const len = [...str].length;
    if (s.minLength !== undefined && len < s.minLength) {
      push('minLength', s.minLength === 1 ? 'Required' : `Use at least ${s.minLength} characters`);
    }
    if (s.maxLength !== undefined && len > s.maxLength)
      push('maxLength', `Use at most ${s.maxLength} characters`);
    if (typeof s.pattern === 'string' && !TEMPLATED.test(str)) {
      let re: RegExp | null = null;
      try {
        re = new RegExp(s.pattern, 'u');
      } catch {
        try {
          re = new RegExp(s.pattern);
        } catch {
          re = null;
        }
      }
      if (re && !re.test(str)) push('pattern', `Doesn't match the expected format (${s.pattern})`);
    }
    if ((s.format === 'uri' || s.format === 'url') && str && !TEMPLATED.test(str)) {
      if (!/^[a-z][a-z0-9+.-]*:\/?\/?[^\s]+$/i.test(str))
        push('format', 'Enter a full URL, e.g. https://example.com/path');
    }
  }
  if (t === 'number' || t === 'integer') {
    const n = value as number;
    if (s.minimum !== undefined && n < s.minimum) push('minimum', `Must be ${s.minimum} or more`);
    if (s.maximum !== undefined && n > s.maximum) push('maximum', `Must be ${s.maximum} or less`);
    if (typeof s.exclusiveMinimum === 'number' && n <= s.exclusiveMinimum)
      push('exclusiveMinimum', `Must be more than ${s.exclusiveMinimum}`);
    if (typeof s.exclusiveMaximum === 'number' && n >= s.exclusiveMaximum)
      push('exclusiveMaximum', `Must be less than ${s.exclusiveMaximum}`);
    if (
      typeof s.multipleOf === 'number' &&
      s.multipleOf > 0 &&
      Math.abs(n / s.multipleOf - Math.round(n / s.multipleOf)) > 1e-9
    ) {
      push('multipleOf', `Must be a multiple of ${s.multipleOf}`);
    }
  }
  if (t === 'array') {
    const arr = value as unknown[];
    if (s.minItems !== undefined && arr.length < s.minItems)
      push(
        'minItems',
        s.minItems === 1 ? 'Add at least one item' : `Add at least ${s.minItems} items`,
      );
    if (s.maxItems !== undefined && arr.length > s.maxItems)
      push('maxItems', `Use at most ${s.maxItems} items`);
    if (s.uniqueItems === true) {
      arr.forEach((x, i) => {
        if (arr.findIndex((y) => deepEqual(x, y)) < i)
          push('uniqueItems', 'Duplicate item', [...path, i]);
      });
    }
    if (Array.isArray(s.items)) {
      s.items.forEach((it: unknown, i: number) => {
        if (i < arr.length) errs.push(...run(it, arr[i], [...path, i], ctx));
      });
    } else if (s.items !== undefined) {
      arr.forEach((x, i) => errs.push(...run(s.items, x, [...path, i], ctx)));
    }
    if (s.contains !== undefined) {
      const count = arr.filter((x) => run(s.contains, x, [], {}).length === 0).length;
      const min = typeof s.minContains === 'number' ? s.minContains : 1;
      if (count < min)
        push('contains', `Include an item where ${describe(s.contains, [...path, 0], ctx)}`);
    }
  }
  if (t === 'object') {
    const obj = value as Record<string, unknown>;
    const props: JsonSchema = s.properties ?? {};
    if (Array.isArray(s.required)) {
      for (const k of s.required) {
        if (!(k in obj) || obj[k] === undefined) {
          push('required', ctx.reason ? `Required when ${ctx.reason}` : 'Required', [...path, k]);
        }
      }
    }
    const keys = Object.keys(obj);
    if (s.minProperties !== undefined && keys.length < s.minProperties)
      push(
        'minProperties',
        s.minProperties === 1
          ? 'Add at least one entry'
          : `Add at least ${s.minProperties} entries`,
      );
    if (s.maxProperties !== undefined && keys.length > s.maxProperties)
      push('maxProperties', `Use at most ${s.maxProperties} entries`);
    for (const k of keys) {
      const childPath = [...path, k];
      let matched = false;
      if (k in props) {
        matched = true;
        errs.push(...run(props[k], obj[k], childPath, { label: ctx.label }));
      }
      if (s.patternProperties) {
        for (const [p, ps] of Object.entries(s.patternProperties)) {
          try {
            if (new RegExp(p, 'u').test(k)) {
              matched = true;
              errs.push(...run(ps, obj[k], childPath, { label: ctx.label }));
            }
          } catch {
            /* bad pattern */
          }
        }
      }
      if (!matched && s.additionalProperties !== undefined) {
        if (s.additionalProperties === false)
          push('additionalProperties', `"${k}" isn't a supported setting`, childPath);
        else errs.push(...run(s.additionalProperties, obj[k], childPath, { label: ctx.label }));
      }
    }
  }

  // --- combinators ------------------------------------------------------------------------
  if (Array.isArray(s.allOf))
    s.allOf.forEach((sub: unknown) => errs.push(...run(sub, value, path, ctx)));

  if (Array.isArray(s.anyOf)) {
    const ok = s.anyOf.some(
      (sub: unknown) => run(sub, value, path, { label: ctx.label }).length === 0,
    );
    if (!ok)
      push('anyOf', `Either ${s.anyOf.map((b: unknown) => describe(b, path, ctx)).join(', or ')}`);
  }
  if (Array.isArray(s.oneOf)) {
    const passing = s.oneOf.filter(
      (sub: unknown) => run(sub, value, path, { label: ctx.label }).length === 0,
    );
    if (passing.length === 0) {
      push(
        'oneOf',
        `Set exactly one of these: ${s.oneOf.map((b: unknown) => describe(b, path, ctx)).join('; or ')}`,
      );
    } else if (passing.length > 1) {
      push(
        'oneOf',
        `Only one of these may apply: ${passing.map((b: unknown) => describe(b, path, ctx)).join('; ')}`,
      );
    }
  }
  if (s.not !== undefined && run(s.not, value, path, { label: ctx.label }).length === 0) {
    push('not', `This combination isn't allowed: ${describe(s.not, path, ctx)}`);
  }
  if (s.if !== undefined && (s.then !== undefined || s.else !== undefined)) {
    const cond = run(s.if, value, path, { label: ctx.label }).length === 0;
    const reason = describe(s.if, path, ctx);
    if (cond && s.then !== undefined)
      errs.push(...run(s.then, value, path, { label: ctx.label, reason }));
    if (!cond && s.else !== undefined)
      errs.push(...run(s.else, value, path, { label: ctx.label, reason: `not (${reason})` }));
  }
  return errs;
}

/** Plain-language description of a sub-schema used as a condition or branch. */
export function describe(schema: unknown, path: Path, ctx: ValidateOptions = {}): string {
  if (typeof schema !== 'object' || schema === null) return String(schema);
  const s = schema as JsonSchema;
  const lab = (k: string) => labelFor(ctx, [...path, k]);
  const parts: string[] = [];
  const props: JsonSchema = s.properties ?? {};
  const described = new Set<string>();

  for (const [k, ps] of Object.entries(props)) {
    if (typeof ps !== 'object' || ps === null) continue;
    const p = ps as JsonSchema;
    if ('const' in p) {
      parts.push(
        p.const === true
          ? `${lab(k)} is on`
          : p.const === false
            ? `${lab(k)} is off`
            : `${lab(k)} is ${formatValue(p.const)}`,
      );
      described.add(k);
    } else if (Array.isArray(p.enum)) {
      parts.push(`${lab(k)} is ${p.enum.map(formatValue).join(' or ')}`);
      described.add(k);
    } else if (p.minItems || p.minProperties || p.minLength) {
      parts.push(`${lab(k)} is not empty`);
      described.add(k);
    }
  }
  if (Array.isArray(s.required)) {
    const rest = s.required.filter((k: string) => !described.has(k));
    if (rest.length)
      parts.unshift(`${listOf(rest.map(lab))} ${rest.length > 1 ? 'are' : 'is'} set`);
  }
  if (s.not !== undefined) {
    const n = s.not as JsonSchema;
    if (Array.isArray(n.required) && Object.keys(n).length === 1) {
      parts.push(`${listOf(n.required.map(lab))} ${n.required.length > 1 ? 'are' : 'is'} not set`);
    } else if (
      Array.isArray(n.anyOf) &&
      n.anyOf.every((b: JsonSchema) => Array.isArray(b?.required) && Object.keys(b).length === 1)
    ) {
      parts.push(
        `none of ${listOf(n.anyOf.flatMap((b: JsonSchema) => b.required).map(lab), 'or')} is set`,
      );
    } else {
      parts.push(`not (${describe(n, path, ctx)})`);
    }
  }
  if (Array.isArray(s.anyOf))
    parts.push(`(${s.anyOf.map((b: unknown) => describe(b, path, ctx)).join(' or ')})`);
  if (Array.isArray(s.allOf)) parts.push(...s.allOf.map((b: unknown) => describe(b, path, ctx)));
  return parts.length ? parts.join(' and ') : 'the expected settings are given';
}

function listOf(items: string[], conj = 'and'): string {
  if (items.length <= 1) return items[0] ?? '';
  return `${items.slice(0, -1).join(', ')} ${conj} ${items[items.length - 1]}`;
}
