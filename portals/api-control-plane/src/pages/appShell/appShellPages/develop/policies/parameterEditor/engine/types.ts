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
 * Shared types for the policy form library.
 *
 * A policy definition (policy-definition.yaml) is parsed by the caller (e.g. with js-yaml)
 * and handed to `buildModel`, which turns its `parameters` JSON Schema into a tree of
 * FieldNodes. The resolver then combines that tree with the user's current FormState to
 * work out what is visible, what is required, the config to save, and validation errors.
 */

// A JSON Schema fragment is open-ended (keywords of any shape, nested to any depth), so
// the engine reads it dynamically and narrows each keyword at its use site.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type JsonSchema = { [key: string]: any };

export interface PolicyDefinition {
  name: string;
  version: string;
  displayName?: string;
  description?: string;
  parameters?: JsonSchema;
  systemParameters?: JsonSchema;
  [key: string]: unknown;
}

/** A path into the form state or into the config. Strings are object keys, numbers are array indexes. */
export type Path = (string | number)[];

export type FieldKind =
  | 'text' // single-line string
  | 'textarea' // multi-line string (templates, PEM, CEL, schemas…)
  | 'secret' // string that should be masked
  | 'url' // string with format: uri
  | 'integer'
  | 'number'
  | 'boolean'
  | 'select' // string/number with enum
  | 'constant' // field with a const value (shown read-only)
  | 'object' // nested group of fields
  | 'map' // object with additionalProperties: {schema}  → key/value editor
  | 'list' // array of primitives
  | 'multiselect' // array of enum values with uniqueItems
  | 'objectList' // array of objects → repeatable group
  | 'json'; // anything the library can't model → raw JSON editor

export interface FieldOption {
  value: unknown;
  label: string;
}

export interface Constraints {
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  minimum?: number;
  maximum?: number;
  exclusiveMinimum?: number;
  exclusiveMaximum?: number;
  multipleOf?: number;
  minItems?: number;
  maxItems?: number;
  uniqueItems?: boolean;
  minProperties?: number;
  maxProperties?: number;
}

/**
 * A choice group comes from a `oneOf` / `anyOf` whose branches only say which sibling
 * keys must (or must not) be present. Instead of showing every field and failing
 * validation, the UI shows a selector and only the chosen branch's fields.
 *
 *  - single: exactly one option at a time (oauth2-generator: endpoint vs bearer token)
 *  - multi:  any non-empty combination (token-based-ratelimit, request-rewrite)
 */
export interface ChoiceGroup {
  id: string;
  label: string;
  multi: boolean;
  /** minimum number of options that must be selected (multi groups) */
  min: number;
  options: ChoiceOption[];
  /** every property key controlled by this group */
  members: string[];
  source: 'oneOf' | 'anyOf';
}

export interface ChoiceOption {
  id: string;
  label: string;
  /** keys that become visible (and required) when the option is selected */
  keys: string[];
}

/**
 * An `if/then/else` rule on an object. `requires` lists keys the `then` makes required.
 * When the `if` tests a sibling's value (const/enum) the rule has `discriminators`, and
 * the keys it requires are treated as *dependent*: shown only when the rule matches.
 */
export interface ConditionalRule {
  if: JsonSchema;
  then?: JsonSchema;
  else?: JsonSchema;
  requires: string[];
  elseRequires: string[];
  discriminators: string[];
}

export interface FieldNode {
  /** Stable schema id, e.g. "quotas[].keyExtraction[].type". Root is "". */
  id: string;
  key: string;
  kind: FieldKind;
  label: string;
  description?: string;
  schema: JsonSchema;
  /** statically required by the parent's `required` list */
  required: boolean;
  advanced: boolean;
  deprecated: boolean;
  readOnly: boolean;
  hasDefault: boolean;
  default?: unknown;
  /** `wso2/defaultValue`: value resolved from gateway config when the field is left empty */
  gatewayDefault?: string;
  options?: FieldOption[];
  constraints: Constraints;
  placeholder?: string;

  /** object */
  children?: FieldNode[];
  choiceGroups?: ChoiceGroup[];
  rules?: ConditionalRule[];
  /** key → indexes into `rules` that reveal it */
  dependentKeys?: Record<string, number[]>;
  /** optional object with required children → rendered with an on/off toggle */
  toggleable?: boolean;

  /** list / multiselect / objectList */
  item?: FieldNode;
  /** map */
  valueField?: FieldNode;

  /** escape hatch from overrides: field shown only when the parent object matches this schema */
  visibleWhen?: JsonSchema;
}

export interface PolicyFormModel {
  name: string;
  version: string;
  displayName: string;
  description?: string;
  /** the `parameters` schema as given (used for validation) */
  schema: JsonSchema;
  root: FieldNode;
  /** `systemParameters`, modelled for read-only display (operator-level settings) */
  system?: FieldNode;
  /** every field by id */
  fields: Record<string, FieldNode>;
}

/** Per-field and per-group overrides, keyed by field id. Lets a console refine labels or
 *  behaviour without editing the policy definition. */
export interface ModelOverrides {
  fields?: Record<
    string,
    Partial<
      Pick<
        FieldNode,
        | 'label'
        | 'description'
        | 'kind'
        | 'placeholder'
        | 'advanced'
        | 'readOnly'
        | 'visibleWhen'
        | 'options'
      >
    >
  >;
  /** keyed by `${objectFieldId}#${groupIndex}` (see ChoiceGroup.id) */
  groups?: Record<string, { label?: string; optionLabels?: string[] }>;
}

export interface BuildOptions {
  overrides?: ModelOverrides;
}

/**
 * What the UI edits. `values` mirrors the config, with two differences that make editing easier:
 *   - maps are arrays of { key, value } entries (so a row can have an empty or duplicate key while typing)
 *   - json fields are raw text
 * `choices` holds the selected option ids per choice-group instance (see groupKey()).
 */
export interface FormState {
  values: Record<string, unknown>;
  choices: Record<string, string[]>;
}

export interface MapEntry {
  key: string;
  value: unknown;
}

export interface ValidationError {
  /** config path */
  path: Path;
  keyword: string;
  message: string;
}

export interface GroupView {
  group: ChoiceGroup;
  /** state key for setChoice */
  instanceKey: string;
  selected: string[];
  errors: string[];
}

export interface MapEntryView {
  index: number;
  key: string;
  keyPath: Path;
  keyErrors: string[];
  node: ViewNode;
}

export interface ViewNode {
  field: FieldNode;
  /** path into FormState.values */
  path: Path;
  pathKey: string;
  value: unknown;
  visible: boolean;
  required: boolean;
  errors: string[];
  /** toggleable objects: whether it's switched on */
  enabled?: boolean;
  children?: ViewNode[];
  groups?: GroupView[];
  items?: ViewNode[];
  entries?: MapEntryView[];
}

export interface Resolution {
  view: ViewNode;
  /** the full config (defaults included). This is what gets validated. */
  config: Record<string, unknown>;
  /** the same config without values that equal their schema default */
  compactConfig: Record<string, unknown>;
  errors: { pathKey: string; path: Path; message: string }[];
  valid: boolean;
}
