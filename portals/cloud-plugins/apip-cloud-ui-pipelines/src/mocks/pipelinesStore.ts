/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import mockData from './pipelines.mock.json';
import type { CreatePipelineInput, Environment, Pipeline, PipelineStage, UpdatePipelineInput } from '../types';

/** The checked-in mock JSON predates per-stage ids; backfill one at seed time. */
type SeedPipeline = Omit<Pipeline, 'stages'> & { stages: Omit<PipelineStage, 'id'>[] };

/**
 * In-memory stand-in for the pipelines backend. Seeded once from the checked-in
 * mock JSON and mutated in place for the lifetime of the tab, so create/delete
 * are reflected immediately without a real API — swap for real client calls
 * once the pipelines service exists.
 */
let pipelines: Pipeline[] = (mockData.pipelines as SeedPipeline[]).map((pipeline) => ({
  ...pipeline,
  stages: pipeline.stages.map((stage) => ({ ...stage, id: crypto.randomUUID() })),
}));

const environments: Environment[] = mockData.environments as Environment[];

/** Project associations contain only explicitly-added pipeline ids. The
 * organization default is inherited dynamically and is never persisted here. */
const projectPipelineIds = new Map<string, Set<string>>();

const projectKey = (orgHandle: string, projectHandle: string) =>
  `${orgHandle}/${projectHandle}`;

export function listEnvironments(): Environment[] {
  return environments;
}

export function listPipelines(): Pipeline[] {
  return pipelines;
}

export function createPipeline(input: CreatePipelineInput): Pipeline {
  const created: Pipeline = {
    id: `pipeline-${Date.now()}`,
    name: input.name,
    isDefault: input.isDefault,
    stages: input.stages,
  };
  pipelines = input.isDefault
    ? [...pipelines.map((pipeline) => ({ ...pipeline, isDefault: false })), created]
    : [...pipelines, created];
  return created;
}

export function updatePipeline(input: UpdatePipelineInput): Pipeline {
  const updated: Pipeline = {
    id: input.id,
    name: input.name,
    isDefault: input.isDefault,
    stages: input.stages,
  };
  pipelines = pipelines.map((pipeline) => {
    if (pipeline.id === input.id) return updated;
    return input.isDefault ? { ...pipeline, isDefault: false } : pipeline;
  });
  return updated;
}

export function deletePipeline(id: string): void {
  pipelines = pipelines.filter((pipeline) => pipeline.id !== id);
  projectPipelineIds.forEach((ids) => ids.delete(id));
}

export function listProjectPipelines(orgHandle: string, projectHandle: string): Pipeline[] {
  const associatedIds = projectPipelineIds.get(projectKey(orgHandle, projectHandle)) ?? new Set<string>();
  return pipelines.filter((pipeline) => pipeline.isDefault || associatedIds.has(pipeline.id));
}

export function associateProjectPipelines(
  orgHandle: string,
  projectHandle: string,
  pipelineIds: readonly string[]
): void {
  const key = projectKey(orgHandle, projectHandle);
  const ids = new Set(projectPipelineIds.get(key));
  pipelineIds.forEach((id) => {
    const pipeline = pipelines.find((candidate) => candidate.id === id);
    if (pipeline && !pipeline.isDefault) ids.add(id);
  });
  projectPipelineIds.set(key, ids);
}

export function removeProjectPipeline(
  orgHandle: string,
  projectHandle: string,
  pipelineId: string
): void {
  projectPipelineIds.get(projectKey(orgHandle, projectHandle))?.delete(pipelineId);
}
