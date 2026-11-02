/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useCallback, useEffect, useRef, useState, type FC } from 'react';
import { Alert, Box, CircularProgress, PageContent, PageTitle } from '@wso2/oxygen-ui';
import PipelineCreatePage from './PipelineCreatePage';
import PipelinesListPage from './PipelinesListPage';
import UnexpectedErrorState from './components/UnexpectedErrorState';
import type { AIWorkspaceHostPort } from './hostPort';
import { PIPELINE_OPERATIONS } from './permissions';
import type { CreatePipelineInput, Environment, Pipeline } from './types';
import {
  assembleEnvironments,
  buildStages,
  DEFAULT_PIPELINE_NAME,
  type EnvironmentDTO,
} from './utils';

export type PipelinesFeatureProps = {
  port: AIWorkspaceHostPort;
};

type EnvironmentListDTO = { count?: number; list?: EnvironmentDTO[] };
// The pipeline list DTO is the API shape verbatim; only the arrays are optional
// on the wire, so reading is a straight pass-through once they are defaulted.
type PipelineDTO = Partial<Pipeline> & { id: string; name: string };
type PipelineListDTO = { count?: number; list?: PipelineDTO[] };

/**
 * The extension's `render(port)` result: an organization-scoped list/create/edit
 * flow over the platform-api deployment pipelines, switching view with local
 * state rather than a nested route. Pipelines are held in the API shape
 * (`promotionPaths`) and sent back verbatim; the host-injected
 * `apiFetch` is the only transport — the component never sees a token or a URL.
 */
const PipelinesFeature: FC<PipelinesFeatureProps> = ({ port }) => {
  const { apiFetch, permissions } = port;
  // What the signed-in user may be offered, from the scopes each operation
  // accepts. The server still decides; this keeps the page from offering a call
  // it would refuse, and from making one it knows will be refused.
  const canList = permissions.canAny(PIPELINE_OPERATIONS.ListPipelines);
  const canReadEnvironments = permissions.canAny(PIPELINE_OPERATIONS.ListEnvironments);
  const canCreate = permissions.canAny(PIPELINE_OPERATIONS.CreatePipeline);
  const canUpdate = permissions.canAny(PIPELINE_OPERATIONS.UpdatePipeline);
  const canDelete = permissions.canAny(PIPELINE_OPERATIONS.DeletePipeline);

  const [view, setView] = useState<'list' | 'create' | 'edit'>('list');
  const [editingPipelineId, setEditingPipelineId] = useState<string | null>(null);
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Guards against a save being issued twice (the create page also disables its
  // button, but this keeps a duplicate POST/PUT from ever leaving here).
  const submittingRef = useRef(false);

  const load = useCallback(async () => {
    if (!canList) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      // Independent of each other, so they go together rather than one after the
      // other: the page waits for the slower of the two instead of their sum.
      // Environments are only asked for when the user may read them: the stage
      // cards fall back to the names a pipeline already carries, so a role that
      // reads pipelines but not environments still gets its list, not a 403.
      const [environmentList, pipelineList] = await Promise.all([
        canReadEnvironments
          ? apiFetch<EnvironmentListDTO>('GET', '/environments')
          : Promise.resolve(undefined),
        apiFetch<PipelineListDTO>('GET', '/pipelines'),
      ]);
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
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Unable to load deployment pipelines.');
    } finally {
      setLoading(false);
    }
  }, [apiFetch, canList, canReadEnvironments]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSubmit = async (input: CreatePipelineInput, pipelineId?: string) => {
    if (submittingRef.current) return;
    submittingRef.current = true;
    try {
      if (pipelineId) {
        await apiFetch('PUT', `/pipelines/${encodeURIComponent(pipelineId)}`, input);
        port.notify(`Pipeline "${input.name}" updated.`, 'success');
      } else {
        await apiFetch('POST', '/pipelines', input);
        port.notify(`Pipeline "${input.name}" created.`, 'success');
      }
      setView('list');
      setEditingPipelineId(null);
      await load();
    } catch (submitError) {
      port.notify(
        submitError instanceof Error ? submitError.message : 'Unable to save the pipeline.',
        'error'
      );
    } finally {
      submittingRef.current = false;
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await apiFetch('DELETE', `/pipelines/${encodeURIComponent(id)}`);
      port.notify('Pipeline deleted.', 'success');
      await load();
    } catch (deleteError) {
      port.notify(
        deleteError instanceof Error ? deleteError.message : 'Unable to delete the pipeline.',
        'error'
      );
    }
  };

  const handleEditClick = (id: string) => {
    setEditingPipelineId(id);
    setView('edit');
  };

  if (!canList) {
    return (
      <PageContent fullWidth>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Continuous Deployment Pipelines</PageTitle.Header>
        </PageTitle>
        <Alert severity="warning">You don&apos;t have permission to view deployment pipelines.</Alert>
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
      <UnexpectedErrorState
        description="We couldn't load your pipelines. This is usually temporary - try again in a moment."
        detail={error}
        onRetry={() => void load()}
      />
    );
  }

  if (view === 'create' || view === 'edit') {
    const editingPipeline =
      view === 'edit' ? pipelines.find((pipeline) => pipeline.id === editingPipelineId) : undefined;
    return (
      <PipelineCreatePage
        environments={environments}
        mode={view}
        initialPipeline={editingPipeline}
        onBack={() => {
          setView('list');
          setEditingPipelineId(null);
        }}
        onSubmit={handleSubmit}
      />
    );
  }

  return (
    <PipelinesListPage
      pipelines={pipelines}
      environments={environments}
      canCreate={canCreate}
      canUpdate={canUpdate}
      canDelete={canDelete}
      onCreateClick={() => setView('create')}
      onEditClick={handleEditClick}
      onDelete={handleDelete}
    />
  );
};

export default PipelinesFeature;
