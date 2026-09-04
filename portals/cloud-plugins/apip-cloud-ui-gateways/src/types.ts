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
 * The gateway's functionality type. Mirrors platform-api's `functionalityType`
 * (`regular | ai | event`); this UI only offers `ai`/`event` on create, but a
 * gateway created elsewhere can come back as `regular`, so the type tolerates it.
 */
export type GatewayType = 'regular' | 'ai' | 'event';

export type GatewayStatus = 'active' | 'inactive';

export type Environment = {
  id: string;
  name: string;
};

export type Gateway = {
  id: string;
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
  /** The external host the gateway is exposed on — server-assigned, not a create input. */
  url: string;
  status: GatewayStatus;
  updatedAt: string;
};

/**
 * Fields the create/edit form collects. `id`/`url` (host) are server-assigned;
 * on edit only `name`/`description` are mutable (`type`/`environmentId` are
 * fixed at creation).
 */
export type GatewayInput = {
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
};

/* ── gateway configuration ─────────────────────────────────────────────────── */

/**
 * `applying` is the expected state immediately after ANY write and can persist
 * for minutes. It is not a failure and not an unfinished save.
 */
export type ConfigPhase = 'applying' | 'healthy' | 'failed';

export type ConfigStatus = {
  phase: ConfigPhase;
  /** Platform detail, present when the phase is not healthy. Prose, not a code. */
  message?: string;
};

export type ConfigFieldType =
  | 'enum'
  | 'boolean'
  | 'integer'
  | 'duration'
  | 'quantity'
  | 'string';

/**
 * One setting the organization may change. The platform reads its allowlist at
 * request time, so this array is the form definition — never a client-side copy
 * of it — and `label`/`description` are the platform's own user-facing copy.
 */
export type EditableField = {
  path: string;
  type: ConfigFieldType;
  label?: string;
  description?: string;
  /** Permitted values, present when `type` is `enum`. */
  values?: string[];
  /**
   * Inclusive bounds. STRINGS for every type — `Number('50m')` is `NaN`. For
   * `string`, `max` is a length in characters and `min` is absent.
   */
  min?: string;
  max?: string;
};

/**
 * A rule between two settings. Evaluated by the platform against the document a
 * write would PRODUCE, so a client checks it across the whole form.
 */
export type ConfigConstraint = {
  type: 'notGreaterThan';
  path: string;
  than: string;
  /** The platform's own sentence for the violation. Surface it verbatim. */
  message?: string;
};

export type ConfigValues = Record<string, unknown>;

export type GatewayConfiguration = {
  id: string;
  name?: string;
  environment?: string;
  /** Current value of every editable setting, keyed exactly as a request body is. */
  values: ConfigValues;
  editable: EditableField[];
  constraints?: ConfigConstraint[];
  status: ConfigStatus;
};
