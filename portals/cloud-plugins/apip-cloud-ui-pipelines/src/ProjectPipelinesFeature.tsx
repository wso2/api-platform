/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useCallback, useEffect, useMemo, useState, type FC } from 'react';
import { Alert, Box, Button, CircularProgress, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import ProjectPipelinesPage from './ProjectPipelinesPage';
import type { AIWorkspaceHostPort } from './hostPort';
import { PIPELINE_OPERATIONS } from './permissions';
import type { Environment, Pipeline } from './types';
import {
  assembleEnvironments,
  buildStages,
  DEFAULT_PIPELINE_NAME,
  type EnvironmentDTO,
} from './utils';

export type ProjectPipelinesFeatureProps = {
  port: AIWorkspaceHostPort;
};

type EnvironmentListDTO = { count?: number; list?: EnvironmentDTO[] };
type PipelineDTO = Partial<Pipeline> & { id: string; name: string };
type PipelineListDTO = { count?: number; list?: PipelineDTO[] };
type ProjectPipelineDTO = { pipeline?: string };

/**
 * The project-scoped extension entry: binds this project to a single organization
 * deployment pipeline via `GET`/`PUT /projects/{projectHandle}/deployment-pipeline`.
 * The page shows the organization pipelines to choose from and previews the bound
 * one's promotion stages; the binding references the pipeline by its resource id.
 * Data flows through the host-injected `apiFetch`.
 */
const ProjectPipelinesFeature: FC<ProjectPipelinesFeatureProps> = ({ port }) => {
  const { apiFetch, projectHandle, permissions } = port;
  // The page reads the binding and the pipelines to choose from, so both must be
  // allowed for it to render at all. Environments only decorate the stages and
  // are asked for only when the user may read them. The server still decides;
  // this keeps the page from making a call it knows will be refused.
  const canView =
    permissions.canAny(PIPELINE_OPERATIONS.GetProjectDeploymentPipeline) &&
    permissions.canAny(PIPELINE_OPERATIONS.ListPipelines);
  const canReadEnvironments = permissions.canAny(PIPELINE_OPERATIONS.ListEnvironments);
  const canBind = permissions.canAny(PIPELINE_OPERATIONS.SetProjectDeploymentPipeline);

  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [boundPipeline, setBoundPipeline] = useState('');
  const [loading, setLoading] = useState(true);
  const [associating, setAssociating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!projectHandle) {
      setError('Open a project to configure its deployment pipeline.');
      setLoading(false);
      return;
    }
    if (!canView) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const environmentList = canReadEnvironments
        ? await apiFetch<EnvironmentListDTO>('GET', '/environments')
        : undefined;
      const pipelineList = await apiFetch<PipelineListDTO>('GET', '/pipelines');
      // The binding endpoint returns 200 with an empty `pipeline` for a project
      // that has no binding yet, so absence is a successful empty read — not an
      // error. Any thrown error here is therefore a real failure (not-found,
      // authorization, transport) and is allowed to propagate to the outer catch;
      // swallowing it as "unbound" would let Save overwrite an existing binding.
      const binding = await apiFetch<ProjectPipelineDTO>(
        'GET',
        `/projects/${encodeURIComponent(projectHandle)}/deployment-pipeline`
      );
      // The binding stores the pipeline's OpenChoreo resource name — `Pipeline.id`
      // here — not its display name.
      const bound = binding?.pipeline ?? '';
      const assembledEnvironments = assembleEnvironments(environmentList?.list ?? []);
      setEnvironments(assembledEnvironments);
      setPipelines(
        (pipelineList?.list ?? []).map((dto) => {
          const base = {
            id: dto.id,
            name: dto.name,
            promotionPaths: dto.promotionPaths ?? [],
          };
          return {
            ...base,
            isDefault: dto.name === DEFAULT_PIPELINE_NAME,
            stages: buildStages(base, assembledEnvironments),
          };
        })
      );
      setBoundPipeline(bound);
    } catch (loadError) {
      setError(
        loadError instanceof Error ? loadError.message : 'Unable to load the deployment pipeline.'
      );
    } finally {
      setLoading(false);
    }
  }, [apiFetch, canReadEnvironments, canView, projectHandle]);

  useEffect(() => {
    void load();
  }, [load]);

  // The pipeline(s) the project is currently bound to (zero or one), matched by the
  // bound resource id. Passed as the page's current selection.
  const currentPipelines = useMemo(
    () => pipelines.filter((pipeline) => pipeline.id === boundPipeline),
    [pipelines, boundPipeline]
  );

  const handleAssociate = (pipelineIds: string[]) => {
    const pipeline = pipelines.find((candidate) => candidate.id === pipelineIds[0]);
    // Ignore re-entry while a write is in flight so concurrent PUTs cannot race.
    if (!projectHandle || !pipeline || associating || !canBind) return;
    setAssociating(true);
    void (async () => {
      try {
        // The binding references the pipeline by its resource id; the notification
        // shows the friendly display name.
        await apiFetch('PUT', `/projects/${encodeURIComponent(projectHandle)}/deployment-pipeline`, {
          pipeline: pipeline.id,
        });
        setBoundPipeline(pipeline.id);
        port.notify(`Project bound to pipeline "${pipeline.name}".`, 'success');
      } catch (saveError) {
        port.notify(
          saveError instanceof Error ? saveError.message : 'Unable to save the deployment pipeline.',
          'error'
        );
      } finally {
        setAssociating(false);
      }
    })();
  };

  if (!canView) {
    return (
      <PageContent fullWidth>
        <PageTitle sx={{ mb: 3 }}>
          <PageTitle.Header>Deployment Pipeline</PageTitle.Header>
        </PageTitle>
        <Alert severity="warning">
          You don&apos;t have permission to view this project&apos;s deployment pipeline.
        </Alert>
      </PageContent>
    );
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          border: '1px dashed',
          borderColor: 'divider',
          borderRadius: 1.5,
          py: 6,
          px: 3,
          textAlign: 'center',
        }}
      >
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {error}
        </Typography>
        <Button variant="outlined" size="small" onClick={() => void load()}>
          Retry
        </Button>
      </Box>
    );
  }

  return (
    <ProjectPipelinesPage
      pipelines={currentPipelines}
      organizationPipelines={pipelines}
      environments={environments}
      onAssociate={handleAssociate}
      saving={associating}
      canSave={canBind}
    />
  );
};

export default ProjectPipelinesFeature;
