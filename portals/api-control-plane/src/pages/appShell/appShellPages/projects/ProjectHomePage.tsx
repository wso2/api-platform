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
import { Layers } from '@wso2/oxygen-ui-icons-react';
import { useParams } from 'react-router-dom';

import { useProject } from '@/api/resources/projects';
import { QuickStartBanner } from '@/components/common/QuickStartBanner';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { ApiList } from '@/pages/appShell/appShellPages/apis/listing';

// No `ScopeGate`: Overview falls back to the org tier without a project.
export function ProjectHomePage() {
  const { projectHandler = '' } = useParams();
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
          <QuickStartBanner
            description={project.description || 'Add an API to this project.'}
            icon={<Layers size={32} />}
            title={project.displayName}
          />
        </Grid>

        <Divider sx={{ width: '100%' }} />

        <Grid size={{ xs: 12 }}>
          <ApiList />
        </Grid>
      </Grid>
    </>
  );
}
