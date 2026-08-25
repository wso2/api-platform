/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

export type Gateway = {
  id: string;
  name: string;
};

/** A deployment environment. `gateways` is the set a pipeline stage on this environment may target. */
export type Environment = {
  id: string;
  name: string;
  gateways: Gateway[];
  /** Marks an environment as production-grade for the "Critical" badge on its pipeline stages. */
  critical?: boolean;
};

/**
 * One step of a pipeline: an environment, deploying through every gateway it
 * has — not a single chosen one. `defaultGatewayId` is marked (via toggle) at
 * the moment the environment is added to the pipeline and is shown as the
 * "Default" chip among that environment's gateways on the stage card.
 */
export type PipelineStage = {
  /** Stable per-stage id, distinct from `environmentId` — a pipeline can only use a given environment once today, but stages are still id-keyed rather than environmentId-keyed so that isn't baked into every consumer. */
  id: string;
  environmentId: string;
  defaultGatewayId: string;
};

export type Pipeline = {
  id: string;
  name: string;
  isDefault: boolean;
  stages: PipelineStage[];
};

export type CreatePipelineInput = {
  name: string;
  isDefault: boolean;
  stages: PipelineStage[];
};

export type UpdatePipelineInput = CreatePipelineInput & {
  id: string;
};
