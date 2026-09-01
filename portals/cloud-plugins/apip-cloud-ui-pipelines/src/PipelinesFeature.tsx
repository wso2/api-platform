/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useState, type FC } from 'react';
import PipelineCreatePage from './PipelineCreatePage';
import PipelinesListPage from './PipelinesListPage';
import ProjectPipelinesPage from './ProjectPipelinesPage';
import {
  associateProjectPipelines,
  createPipeline,
  deletePipeline,
  listEnvironments,
  listPipelines,
  listProjectPipelines,
  removeProjectPipeline,
  updatePipeline,
} from './mocks/pipelinesStore';
import type { AIWorkspaceHostPort } from './hostPort';
import type { CreatePipelineInput } from './types';

export type PipelinesFeatureProps = {
  port: AIWorkspaceHostPort;
};

/**
 * The extension's `render(port)` result: a self-contained list/create/edit
 * flow backed by the mock store, switching view with local state rather than
 * a nested route — `AIWorkspaceExtension` only carries one `path` per sidebar
 * entry today, so a sub-router isn't available to this extension yet.
 */
const PipelinesFeature: FC<PipelinesFeatureProps> = ({ port }) => {
  const [view, setView] = useState<'list' | 'create' | 'edit'>('list');
  const [editingPipelineId, setEditingPipelineId] = useState<string | null>(null);
  const [pipelines, setPipelines] = useState(() => listPipelines());
  const [environments] = useState(() => listEnvironments());
  const [projectPipelines, setProjectPipelines] = useState(() =>
    port.projectHandle ? listProjectPipelines(port.orgHandle, port.projectHandle) : []
  );

  useEffect(() => {
    if (port.projectHandle) {
      setProjectPipelines(listProjectPipelines(port.orgHandle, port.projectHandle));
    }
  }, [port.orgHandle, port.projectHandle]);

  const handleSubmit = (input: CreatePipelineInput, pipelineId?: string) => {
    if (pipelineId) {
      const updated = updatePipeline({ ...input, id: pipelineId });
      setPipelines(listPipelines());
      port.notify(`Pipeline "${updated.name}" updated.`, 'success');
    } else {
      const created = createPipeline(input);
      setPipelines(listPipelines());
      port.notify(`Pipeline "${created.name}" created.`, 'success');
    }
    setView('list');
    setEditingPipelineId(null);
  };

  const handleDelete = (id: string) => {
    deletePipeline(id);
    setPipelines(listPipelines());
    port.notify('Pipeline deleted.', 'success');
  };

  const handleEditClick = (id: string) => {
    setEditingPipelineId(id);
    setView('edit');
  };

  if (port.projectHandle) {
    const projectHandle = port.projectHandle;
    return (
      <ProjectPipelinesPage
        pipelines={projectPipelines}
        organizationPipelines={listPipelines()}
        environments={environments}
        onAssociate={(pipelineIds) => {
          associateProjectPipelines(port.orgHandle, projectHandle, pipelineIds);
          setProjectPipelines(listProjectPipelines(port.orgHandle, projectHandle));
          port.notify(
            `${pipelineIds.length} pipeline${pipelineIds.length === 1 ? '' : 's'} added to the project.`,
            'success'
          );
        }}
        onRemove={(pipelineId) => {
          removeProjectPipeline(port.orgHandle, projectHandle, pipelineId);
          setProjectPipelines(listProjectPipelines(port.orgHandle, projectHandle));
          port.notify('Pipeline removed from the project.', 'success');
        }}
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
      onCreateClick={() => setView('create')}
      onEditClick={handleEditClick}
      onDelete={handleDelete}
    />
  );
};

export default PipelinesFeature;
