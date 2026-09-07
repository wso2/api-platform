/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type {
  DefaultGateway,
  Environment,
  Gateway,
  Pipeline,
  PipelineStage,
  PromotionPath,
} from './types';

/** The organization's default pipeline is the one named `default`. */
export const DEFAULT_PIPELINE_NAME = 'default';

/**
 * The display order for the linear chain view: walk the promotion graph from its
 * head (a source that is never a target). This is view-only ordering, not a data
 * translation — the stored shape is still the API's `promotionPaths`. A pipeline
 * built here is always linear, so it round-trips exactly; a branching graph
 * authored elsewhere degrades gracefully (the first target of each source is
 * followed and any unreached environments are appended).
 */
export const orderEnvironments = (paths: PromotionPath[]): string[] => {
  const next = new Map<string, string>();
  const sources = new Set<string>();
  const targets = new Set<string>();
  const firstSeen: string[] = [];
  const see = (name: string) => {
    if (!firstSeen.includes(name)) firstSeen.push(name);
  };
  for (const path of paths) {
    see(path.sourceEnvironment);
    sources.add(path.sourceEnvironment);
    if (!next.has(path.sourceEnvironment) && path.targetEnvironments.length > 0) {
      next.set(path.sourceEnvironment, path.targetEnvironments[0]);
    }
    for (const target of path.targetEnvironments) {
      see(target);
      targets.add(target);
    }
  }
  const head = firstSeen.find((name) => sources.has(name) && !targets.has(name)) ?? firstSeen[0];
  const chain: string[] = [];
  const walked = new Set<string>();
  let current: string | undefined = head;
  while (current && !walked.has(current)) {
    walked.add(current);
    chain.push(current);
    current = next.get(current);
  }
  for (const name of firstSeen) {
    if (!walked.has(name)) chain.push(name);
  }
  return chain;
};

/**
 * Whether a pipeline's promotion graph is a simple linear chain the builder can
 * edit without losing structure. The builder (`PipelineCreatePage`) only models a
 * chain: it derives an order via `orderEnvironments` and, on save, re-emits the
 * consecutive pairs. That round-trips exactly for a linear graph, but silently
 * drops edges for a branching (fan-out/fan-in) or cyclic one — e.g. `A -> [B, C]`
 * would be saved back as `A -> B -> C`. So editing is offered only when the chain
 * the builder would produce reproduces the original edge set exactly; otherwise
 * the pipeline is read-only in this UI until a graph-preserving editor exists.
 */
export const isLinearPipeline = (paths: PromotionPath[]): boolean => {
  if (paths.length === 0) return true;
  const edgeKey = (source: string, target: string) => `${source} ${target}`;
  const original = paths
    .flatMap((path) => path.targetEnvironments.map((target) => edgeKey(path.sourceEnvironment, target)))
    .sort();
  const chain = orderEnvironments(paths);
  const rebuilt = chain
    .slice(0, -1)
    .map((environment, index) => edgeKey(environment, chain[index + 1]))
    .sort();
  return (
    original.length === rebuilt.length && original.every((edge, index) => edge === rebuilt[index])
  );
};

/**
 * The gateway name shown for an environment in a pipeline: the one marked
 * default, or the environment's single gateway when it has exactly one
 * (defaulted implicitly). Falls back to the raw id if the reference data has no
 * matching gateway.
 */
export const resolveGatewayName = (
  pipeline: Pipeline,
  environments: Environment[],
  environmentName: string
): string => {
  const environment = environments.find((candidate) => candidate.name === environmentName);
  const markedId = pipeline.defaultGateways.find(
    (entry) => entry.environment === environmentName
  )?.gatewayId;
  const gatewayId =
    markedId ?? (environment?.gateways.length === 1 ? environment.gateways[0].id : '');
  return environment?.gateways.find((gateway) => gateway.id === gatewayId)?.name ?? gatewayId;
};

/**
 * The default gateway id for one environment of a pipeline: the one marked
 * default, or the environment's single gateway when it has exactly one
 * (defaulted implicitly), or '' otherwise.
 */
export const resolveDefaultGatewayId = (
  pipeline: { defaultGateways: DefaultGateway[] },
  environments: Environment[],
  environmentName: string
): string => {
  const environment = environments.find((candidate) => candidate.name === environmentName);
  const markedId = pipeline.defaultGateways.find(
    (entry) => entry.environment === environmentName
  )?.gatewayId;
  return markedId ?? (environment?.gateways.length === 1 ? environment.gateways[0].id : '');
};

/**
 * Projects a pipeline's `promotionPaths` + `defaultGateways` into the
 * promotion-ordered `PipelineStage[]` the stage-card chain renders. Environments
 * are resolved to their assembled ids so the cards can look up the environment and
 * its default gateway.
 */
export const buildStages = (
  pipeline: { promotionPaths: PromotionPath[]; defaultGateways: DefaultGateway[] },
  environments: Environment[]
): PipelineStage[] =>
  orderEnvironments(pipeline.promotionPaths).map((environmentName) => {
    const environmentId =
      environments.find((environment) => environment.name === environmentName)?.id ?? environmentName;
    return {
      id: environmentId,
      environmentId,
      defaultGatewayId: resolveDefaultGatewayId(pipeline, environments, environmentName),
    };
  });

/** Reference-data shapes returned by the platform-api list endpoints. */
export type EnvironmentDTO = { id?: string; name: string; isProduction?: boolean };
export type ManagedGatewayDTO = {
  id: string;
  environment: string;
  host?: string;
  /** The gateway's human-friendly display name; preferred over the host for labels. */
  displayName?: string;
};

/**
 * Joins `/environments` with `/managed-gateways` (grouped by environment name)
 * into the reference `Environment[]` the picker and cards render. Reference-data
 * assembly for lookups — it does not touch the pipeline shape.
 */
export const assembleEnvironments = (
  environments: EnvironmentDTO[],
  gateways: ManagedGatewayDTO[]
): Environment[] =>
  environments.map((environment) => ({
    id: environment.id ?? environment.name,
    name: environment.name,
    critical: environment.isProduction ?? false,
    gateways: gateways
      .filter((gateway) => gateway.environment === environment.name)
      .map((gateway): Gateway => ({
        id: gateway.id,
        name: gateway.displayName || gateway.host || gateway.id,
      })),
  }));
