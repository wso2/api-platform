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
 * is the wire field, read and written verbatim (no intermediate model, no
 * translation layer). Environments are referenced by their name, exactly as the
 * API does. `Environment` is the only derived type: reference data from
 * `/environments` to render names and the "Critical" badge.
 *
 * A pipeline says nothing about gateways. Which gateway an environment deploys
 * to is the default marked on the gateway itself when it is onboarded, so it is
 * the managed-gateways feature that owns it, not this one.
 */

/**
 * A deployment environment. `critical` mirrors the API's `isProduction` and
 * drives the "Critical" badge.
 */
export type Environment = {
  id: string;
  name: string;
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
 * One environment of a pipeline in promotion order. A view projection of
 * `promotionPaths`, assembled for the stage-card chain (see `buildStages`).
 */
export type PipelineStage = {
  id: string;
  environmentId: string;
};

export type Pipeline = {
  /** OpenChoreo's immutable resource name, used as the stable id. */
  id: string;
  name: string;
  promotionPaths: PromotionPath[];
  /** True for the organization's default pipeline (the one named `default`). */
  isDefault: boolean;
  /** Promotion-ordered stages, derived from `promotionPaths`. */
  stages: PipelineStage[];
};

export type CreatePipelineInput = {
  name: string;
  promotionPaths: PromotionPath[];
};

export type UpdatePipelineInput = CreatePipelineInput & {
  id: string;
};
