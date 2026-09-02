/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

/**
 * The plugin speaks the platform-api pipeline shape directly — `promotionPaths`
 * and `defaultGateways` are the wire fields, read and written verbatim (no
 * intermediate model, no translation layer). Environments are referenced by
 * their name, exactly as the API does. `Environment`/`Gateway` are the only
 * derived types: reference data assembled from `/environments` + `/managed-gateways`
 * to render names, gateways, and the "Critical" badge.
 */

/**
 * A managed gateway available in an environment. `name` is its display label —
 * the gateway's display name, falling back to its resolved host, then its id.
 */
export type Gateway = {
  id: string;
  name: string;
};

/**
 * A deployment environment plus the managed gateways bound to it (sourced from
 * `/managed-gateways`, grouped by environment name). `critical` mirrors the
 * API's `isProduction` and drives the "Critical" badge.
 */
export type Environment = {
  id: string;
  name: string;
  gateways: Gateway[];
  critical?: boolean;
};

/**
 * One edge of the promotion graph: a source environment to one or more targets
 * (the API supports fan-out; the linear builder only ever writes a single
 * target). Environments are referenced by name.
 */
export type PromotionPath = {
  sourceEnvironment: string;
  targetEnvironments: string[];
};

/**
 * The default gateway for one environment of a pipeline. Only environments with
 * more than one gateway need an entry; a single-gateway environment defaults to
 * it implicitly (the API fills it in).
 */
export type DefaultGateway = {
  environment: string;
  gatewayId: string;
};

/**
 * One environment of a pipeline in promotion order, with the default gateway APIs
 * promote to there. A view projection of `promotionPaths` + `defaultGateways`,
 * assembled for the stage-card chain (see `buildStages`). `environmentId`/
 * `defaultGatewayId` reference the assembled `Environment`/`Gateway` ids.
 */
export type PipelineStage = {
  id: string;
  environmentId: string;
  defaultGatewayId: string;
};

export type Pipeline = {
  /** OpenChoreo's immutable resource name, used as the stable id. */
  id: string;
  name: string;
  promotionPaths: PromotionPath[];
  defaultGateways: DefaultGateway[];
  /** True for the organization's default pipeline (the one named `default`). */
  isDefault: boolean;
  /** Promotion-ordered stages, derived from `promotionPaths` + `defaultGateways`. */
  stages: PipelineStage[];
};

export type CreatePipelineInput = {
  name: string;
  promotionPaths: PromotionPath[];
  defaultGateways: DefaultGateway[];
};

export type UpdatePipelineInput = CreatePipelineInput & {
  id: string;
};
