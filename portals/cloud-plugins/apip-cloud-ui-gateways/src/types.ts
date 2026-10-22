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
 * The gateway's functionality type, mirroring platform-api's
 * `functionalityType`: `regular` is an API gateway, `event` an event gateway,
 * `ai` an AI gateway. Which of them a host offers on create is the host's own
 * choice (see `GatewaysFeature`'s `gatewayTypes`), but any of the three can come
 * back from the API, so the type covers them all.
 */
export type GatewayType = 'regular' | 'ai' | 'event';

/**
 * Where the gateway is between being asked for and serving traffic.
 *
 * `provisioning` and `failed` are about building it; `active` and `inactive` are
 * about its controller being connected to the control plane right now. They are
 * separate questions, and a gateway still being built has nothing to connect yet
 * — which is why `provisioning` is its own status rather than another way of
 * reading `inactive`. `inactive` is routine (a restart, a brief drop); `failed`
 * is not, and comes with a reason.
 */
export type GatewayStatus = 'provisioning' | 'failed' | 'active' | 'inactive';

export type Environment = {
  id: string;
  name: string;
};

/**
 * Who runs the gateway's infrastructure. `managed` is provisioned and operated
 * by WSO2 on `/managed-gateways`; `self-hosted` is a plain platform-api gateway
 * record the organization runs and connects itself (see `selfHostedGatewaysApi`)
 * — there is no cloud provisioning step for it, so it starts `inactive` and
 * stays that way until its own controller dials in with a registration token.
 */
export type GatewayHosting = 'managed' | 'self-hosted';

export type Gateway = {
  id: string;
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
  hosting: GatewayHosting;
  /** The external host the gateway is exposed on — server-assigned, not a create input. */
  url: string;
  status: GatewayStatus;
  /** Why provisioning failed, in plain words. Present only when `status` is `failed`. */
  statusReason?: string;
  isCritical: boolean;
  /**
   * Whether this gateway is the one its environment resolves to for its type.
   * One gateway per environment and type carries it, so an AI gateway being the
   * default says nothing about the regular ones in the same environment.
   *
   * For a self-hosted gateway this is a best-effort client-side marker (stashed
   * in the native record's `properties`, see `selfHostedGatewaysApi`), not a
   * server-enforced invariant the way it is for a managed one.
   */
  isDefault: boolean;
  version?: string;
  createdAt: string;
  updatedAt: string;
};

/**
 * Fields the create/edit form collects. `id`/`url` (host) are server-assigned;
 * on edit only `name`/`description`/`isDefault` are mutable (`type` and
 * `environmentId` are fixed at creation).
 */
export type GatewayInput = {
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
  /**
   * Ask for this gateway to be its environment's default for its type. Only a
   * true value acts: the default is handed over by marking another gateway,
   * never by clearing this one, so an environment always keeps a default for a
   * type it has gateways of.
   */
  isDefault?: boolean;
};

/**
 * Fields the create form collects for a self-hosted gateway. Shaped like
 * `GatewayInput` — same name/description/type/environment/default — but kept
 * as its own type since it flows to `selfHostedGatewaysApi`, a different
 * client hitting platform-api's native `/gateways`, not `/managed-gateways`,
 * and carries two fields a managed gateway has no use for: unlike a managed
 * gateway (cloud-provisioned, so its host and version are discovered once
 * live), a self-hosted one has nothing to provision — there is no automatic
 * way to know either until its own controller dials in, so the admin
 * declares them up front instead.
 */
export type SelfHostedGatewayInput = GatewayInput & {
  /**
   * The gateway's own endpoint. Optional — left blank, the gateway starts
   * with a pending placeholder URL until a controller connects and reports
   * its real one (see `PENDING_ENDPOINT` in `selfHostedGatewaysApi`).
   */
  url?: string;
  /**
   * Which release this gateway record represents, `major.minor` (e.g.
   * `1.2`) — platform-api's own format for `POST /gateways`. Purely a
   * declared hint (nothing provisions from it): it drives which Quick Start
   * commands the overview page offers until the real controller connects
   * and may report a different one.
   */
  version?: string;
};

/**
 * Returned once, right after a self-hosted gateway is created or its token is
 * regenerated. There is no second way to read it back — losing it means
 * regenerating (which revokes the one before it).
 */
export type GatewayRegistrationToken = {
  gatewayId: string;
  token: string;
};

/* ── gateway configuration ─────────────────────────────────────────────────── */

/**
 * `applying` is the expected state immediately after ANY write and can persist
 * for minutes. It is not a failure and not an unfinished save.
 *
 * `unknown` is the platform reporting no phase at all: whether the last change
 * has landed cannot be said, and it will not settle on its own — so it is a
 * fourth case to render, not a variant of `applying` to keep waiting on.
 */
export type ConfigPhase = 'applying' | 'healthy' | 'failed' | 'unknown';

export type ConfigStatus = {
  phase: ConfigPhase;
  /** Platform detail, present when the phase is not healthy. Prose, not a code. */
  message?: string;
  /**
   * RFC 3339 — when the gateway ENTERED this phase, already validated as a
   * timestamp by platform-api (a bad one is dropped rather than forwarded).
   *
   * Read it with `phase`, never instead of it: `applying` reads the same five
   * minutes after a write and stalled an hour later, and this is the only thing
   * that separates them. Absent on the response to a write — the phase there is
   * `applying` before the data plane has seen the change — and on a gateway
   * whose resources have not reported yet.
   */
  lastTransitionTime?: string;
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
