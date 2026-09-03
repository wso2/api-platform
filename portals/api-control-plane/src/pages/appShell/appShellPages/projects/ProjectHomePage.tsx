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

import { Divider, Grid } from '@wso2/oxygen-ui';
import { useParams } from 'react-router-dom';

import { useProject } from '@/api/resources/projects';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { ApiList } from '@/pages/appShell/appShellPages/apis/listing';
import { ProjectMetadata } from './components/ProjectMetadata';
import { ProjectQuickActions } from './components/ProjectQuickActions';
import { ProjectStatistics } from './components/ProjectStatistics';

// No `ScopeGate`: Overview falls back to the org tier without a project.
export function ProjectHomePage() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const projectQuery = useProject(projectHandler);

  if (projectQuery.isLoading) return <LoadingState label="Loading project" />;
  if (projectQuery.error || !projectQuery.data) {
    return <ErrorState message={projectQuery.error?.message} title="Project not found" />;
  }
  const project = projectQuery.data;

  return (
    <>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        <Grid size={{ xs: 12 }}>
          <ProjectMetadata orgHandle={orgHandle} project={project} />
        </Grid>

        <Divider sx={{ width: '100%' }} />

        <Grid size={{ xs: 12 }} sx={{ minHeight: 400 }}>
          <ApiList />
        </Grid>

        <Grid size={{ xs: 12 }}>
          <ProjectStatistics />
        </Grid>

        <Grid size={{ xs: 12 }}>
          <ProjectQuickActions orgHandle={orgHandle} projectId={project.id} />
        </Grid>
      </Grid>
    </>
  );
}
