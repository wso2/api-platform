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
import { useState } from 'react';
import { useParams } from 'react-router-dom';

import { AppPage } from '@/components/AppPage';
import { useProject } from '@/api/resources/projects';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiList } from '@/pages/appShell/appShellPages/apis/listing';
import type { ApiTypeFilter } from '@/pages/appShell/appShellPages/apis/listing/apiTypeFilter';
import { ProjectMetadata } from './components/ProjectMetadata';
import { ProjectWelcome } from './components/ProjectWelcome';
import { ProjectStatistics } from './components/ProjectStatistics';

// No `ScopeGate`: Overview falls back to the org tier without a project.
export function ProjectHomePage() {
  const { projectHandler = '' } = useParams();
  return (
    <AppPage>
      {/* Keyed by project: switching projects from the header keeps this page
          mounted, and what was chosen in one project (an API type filter, whether
          the welcome view was showing) must not carry into the next. */}
      <ProjectHomePageContent key={projectHandler} />
    </AppPage>
  );
}

function ProjectHomePageContent() {
  const [apiTypeFilter, setApiTypeFilter] = useState<ApiTypeFilter | null>(null);
  // An empty project's welcome view brings its own greeting and the heading's
  // actions, so the project heading steps aside for it.
  const [showingWelcome, setShowingWelcome] = useState(false);
  const { orgHandle = '', projectHandler = '' } = useParams();
  const { organization } = useConsoleScope();
  const projectQuery = useProject(projectHandler);

  if (projectQuery.isLoading) return <LoadingState label="Loading project" />;
  if (projectQuery.error || !projectQuery.data) {
    return <ErrorState message={projectQuery.error?.message} title="Project not found" />;
  }
  const project = projectQuery.data;

  return (
    <>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        {!showingWelcome && (
          <>
            <Grid size={{ xs: 12 }}>
              <ProjectMetadata project={project} />
            </Grid>

            <Grid size={{ xs: 12 }} sx={{ mt: -1 }}>
              <Divider sx={{ borderBottomWidth: 1 }} />
            </Grid>
          </>
        )}

        <Grid size={{ xs: 12 }} sx={{ minHeight: 400 }}>
          <ApiList
            typeFilter={apiTypeFilter}
            onClearTypeFilter={() => setApiTypeFilter(null)}
            onFirstRunChange={setShowingWelcome}
            renderFirstRun={(createApi) => (
              <ProjectWelcome
                onCreateApi={createApi}
                orgDisplayName={organization?.displayName ?? ''}
                orgHandle={orgHandle}
                projectName={project.displayName}
              />
            )}
            sidebar={
              <ProjectStatistics
                onTypeFilterChange={setApiTypeFilter}
                selectedType={apiTypeFilter}
              />
            }
          />
        </Grid>
      </Grid>
    </>
  );
}
