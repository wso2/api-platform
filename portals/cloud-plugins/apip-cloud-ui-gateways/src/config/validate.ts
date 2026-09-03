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
  ConfigConstraint,
  ConfigValues,
  EditableField,
  GatewayConfiguration,
} from '../types';
import { parseDurationSeconds } from './duration';
import { parseQuantity } from './quantity';

/**
 * Client-side validation, which exists so a round trip is not the only way to
 * learn a value is out of range. THE SERVER IS THE AUTHORITY: every message
 * below deliberately repeats the sentence the platform returns for the same
 * value, so the user reads the same wording whichever side caught it.
 *
 * Two layers live here, both read from the response and neither from a
 * hardcoded copy of the field list:
 *
 *   1. per field, against its own `editable` entry;
 *   2. across the form, against `constraints`, evaluated on the MERGED document
 *      (loaded values overlaid with the dirty edits) rather than on the edits
 *      alone — which is what the server does, and the only way to catch
 *      "lower just the limit".
 */

/** Setting path → the message to show under that field. */
export type FieldErrors = Record<string, string>;

/**
 * Everything Go's `unicode.IsControl` refuses (Unicode Cc: U+0000–U+001F and
 * U+007F–U+009F) plus the two line separators, less tab, LF and CR, which the
 * platform allows. These are the characters CEL's `strings.quote()` does not
 * escape, so a value carrying one cannot be rendered into the gateway's values
 * document at all — hence a refusal rather than a warning.
 */
const UNRENDERABLE = /[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F-\u009F\u2028\u2029]/;

/** Bounds arrive as strings for every type; the platform parses this one with `%d`. */
const boundInteger = (text: string | undefined): number | null => {
  if (text === undefined) return null;
  const value = Number.parseInt(text.trim(), 10);
  return Number.isFinite(value) ? value : null;
};

/** Character count in CODE POINTS, matching the platform's `len([]rune(text))`. */
export const characterCount = (text: string): number => [...text].length;

/**
 * One value against one field declaration, or `null` when it passes.
 * `undefined` means the field was never edited and the stored value stands, so
 * there is nothing to check.
 *
 * Messages are FIELD-RELATIVE: they never name the setting path, because they
 * render directly under the control whose label already says which setting this
 * is. A path here would be both redundant and, for the deeper policy settings,
 * longer than the message it prefixes.
 */
export function validateFieldValue(
  field: EditableField,
  value: unknown
): string | null {
  if (value === undefined) return null;

  switch (field.type) {
    case 'enum': {
      const permitted = field.values ?? [];
      if (typeof value !== 'string' || !permitted.includes(value)) {
        return `Must be one of ${permitted.join(', ')}`;
      }
      return null;
    }

    case 'boolean':
      // A string "yes" is a 400 — the widget must produce a real boolean.
      return typeof value === 'boolean' ? null : 'Must be true or false';

    case 'integer': {
      if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
        // Also the guard for the `cost_scale_factor` ceiling of 1e12: a double
        // holds that exactly, but anything past 2^53 has silently stopped being
        // the number that was typed.
        return 'Must be a whole number';
      }
      const low = boundInteger(field.min);
      const high = boundInteger(field.max);
      if ((low !== null && value < low) || (high !== null && value > high)) {
        return `Must be between ${low} and ${high}`;
      }
      return null;
    }

    case 'quantity': {
      const example = 'Must be a quantity such as "500m" or "512Mi"';
      if (typeof value !== 'string') return example;
      const parsed = parseQuantity(value);
      if (parsed === null) return example;
      // Zero or negative is legal to Kubernetes and meaningless for a gateway;
      // a zero limit means "no limit", which the ceiling assumes away.
      if (parsed <= 0) return 'Must be greater than zero';
      const low = field.min === undefined ? null : parseQuantity(field.min);
      const high = field.max === undefined ? null : parseQuantity(field.max);
      if ((low !== null && parsed < low) || (high !== null && parsed > high)) {
        return `Must be between ${field.min} and ${field.max}`;
      }
      return null;
    }

    case 'duration': {
      const example = 'Must be a duration such as "5m"';
      if (typeof value !== 'string') return example;
      const parsed = parseDurationSeconds(value);
      if (parsed === null) return example;
      const low =
        field.min === undefined ? null : parseDurationSeconds(field.min);
      const high =
        field.max === undefined ? null : parseDurationSeconds(field.max);
      if ((low !== null && parsed < low) || (high !== null && parsed > high)) {
        return `Must be between ${field.min} and ${field.max}`;
      }
      return null;
    }

    case 'string': {
      if (typeof value !== 'string') return 'Must be a string';
      const length = characterCount(value);
      const low = boundInteger(field.min) ?? 0;
      const high = boundInteger(field.max);
      if (high !== null && (length < low || length > high)) {
        return `Must be between ${low} and ${high} characters`;
      }
      if (UNRENDERABLE.test(value)) {
        return 'Must not contain control characters or line separators';
      }
      return null;
    }

    default:
      return null;
  }
}

/**
 * Compares two values of a field's type. `null` when either side is
 * unparseable, which the caller must treat as "skip": such a value cannot have
 * come through the allowlist, so it is a pre-existing platform value the user
 * can do nothing about, and refusing their save for it would be blaming them
 * for someone else's write. The server reasons the same way.
 */
function compareByType(
  field: EditableField | undefined,
  left: unknown,
  right: unknown
): number | null {
  const asNumber = (value: unknown): number | null => {
    switch (field?.type) {
      case 'integer':
        return typeof value === 'number' && Number.isFinite(value)
          ? value
          : null;
      case 'quantity':
        return typeof value === 'string' ? parseQuantity(value) : null;
      case 'duration':
        return typeof value === 'string' ? parseDurationSeconds(value) : null;
      default:
        // Only ordered types can be compared. `enum`, `boolean` and `string`
        // have no ordering, so no constraint can be declared over them.
        return null;
    }
  };
  const a = asNumber(left);
  const b = asNumber(right);
  if (a === null || b === null) return null;
  if (a === b) return 0;
  return a < b ? -1 : 1;
}

/**
 * The declared constraints against a whole values document.
 *
 * A message lands on whichever side of the pair the user actually edited, so
 * "lower just the limit" reports on the limit rather than on the request they
 * never touched. When neither side is dirty the violation is pre-existing and
 * reports on the bounded side.
 */
export function validateConstraints(
  constraints: readonly ConfigConstraint[],
  merged: ConfigValues,
  fieldsByPath: ReadonlyMap<string, EditableField>,
  dirty: ConfigValues = {}
): FieldErrors {
  const errors: FieldErrors = {};
  for (const constraint of constraints) {
    if (constraint.type !== 'notGreaterThan') continue;
    const left = merged[constraint.path];
    const right = merged[constraint.than];
    // A document carrying only one side cannot violate the rule; the missing
    // side falls back to the chart's own default at render time.
    if (left === undefined || right === undefined) continue;
    const comparison = compareByType(
      fieldsByPath.get(constraint.path),
      left,
      right
    );
    if (comparison === null || comparison <= 0) continue;

    // Names the OTHER field by its label, not its path: this message renders
    // under a field whose own label is already on screen, so the only thing
    // worth spelling out is the one it is being compared against.
    const thanLabel =
      fieldsByPath.get(constraint.than)?.label ?? constraint.than;
    const message = constraint.message ?? `Must not exceed ${thanLabel}`;
    const edited = [constraint.path, constraint.than].filter(
      (path) => dirty[path] !== undefined
    );
    for (const path of edited.length > 0 ? edited : [constraint.path]) {
      errors[path] ??= message;
    }
  }
  return errors;
}

/**
 * Both client-side layers over a loaded configuration and the edits pending on
 * it. Empty means `Save` may be enabled — not that the write will succeed.
 */
export function validateForm(
  config: GatewayConfiguration,
  dirty: ConfigValues
): FieldErrors {
  const fieldsByPath = new Map(
    config.editable.map((field) => [field.path, field])
  );

  const errors: FieldErrors = {};
  for (const [path, value] of Object.entries(dirty)) {
    const field = fieldsByPath.get(path);
    if (!field) {
      // Never render, and never send, a setting the response did not list.
      errors[path] = `${path} is not an editable gateway setting`;
      continue;
    }
    const message = validateFieldValue(field, value);
    if (message) errors[path] = message;
  }

  // Constraints run over the document the write WOULD produce, and only once
  // every field in it is individually sound — comparing against a value already
  // known to be malformed says nothing.
  if (Object.keys(errors).length === 0) {
    Object.assign(
      errors,
      validateConstraints(
        config.constraints ?? [],
        { ...config.values, ...dirty },
        fieldsByPath,
        dirty
      )
    );
  }
  return errors;
}

/**
 * The field a server message belongs under, found by longest matching setting
 * path — a field-level message begins with the path it is about
 * (`…route_timeout_ms must be between 1000 and 300000`). `null` means it is
 * form-level and belongs in a banner.
 *
 * Longest wins because one path can prefix another: `gateway.config_toml` also
 * prefixes any message about a `gateway.config_toml.*` field.
 */
export function fieldForServerMessage(
  message: string,
  paths: readonly string[]
): string | null {
  let best: string | null = null;
  for (const path of paths) {
    if (!message.startsWith(path)) continue;
    if (best === null || path.length > best.length) best = path;
  }
  return best;
}

/**
 * A server message with its leading setting path removed, so it reads like the
 * client-side messages above when shown under the field it belongs to.
 *
 * The platform's own sentence is kept verbatim apart from that prefix — it is
 * user-presentable prose and may say things this client cannot derive. Only the
 * banner keeps the full text, where there is no field label to supply context.
 */
export function withoutPathPrefix(message: string, path: string): string {
  if (!message.startsWith(path)) return message;
  const rest = message.slice(path.length).trim();
  if (rest === '') return message;
  return rest.charAt(0).toUpperCase() + rest.slice(1);
}
