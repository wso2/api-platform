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

import { useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  ComplexSelect,
  FormControl,
  FormLabel,
  MenuItem,
  PageContent,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Layers } from '@wso2/oxygen-ui-icons-react';
import { useNavigate } from 'react-router-dom';

import { useRestApis } from '../api/resources/restApis';
import { ErrorState, LoadingState } from '../components/StateViews';
import { routes, type ApiPathBuilder } from '../routes/paths';
import { useConsoleScope } from './ConsoleScopeContext';
import { FormattedMessage } from 'react-intl';

/** The scope a page needs before it can render anything meaningful. */
export type RequiredScope = 'project' | 'api';

export type ScopeGateProps = {
  children: ReactNode;
  /**
   * Sentence naming what this page manages and at which level, e.g.
   * `"APIs are created and managed at the project level."` Supplied per page so
   * the copy reads naturally (a generated one can't get singular/plural right).
   */
  prompt?: string;
  /** Scope the page needs. */
  requires: RequiredScope;
  /**
   * This page's own path builder —> pass the `routes.*` function the page is
   * registered under (e.g. `routes.apiDeploy`). Called with the handles the
   * user picks, so submitting lands on this same page, now fully scoped.
   */
  to: ApiPathBuilder;
};

/**
 * Renders `children` only once the route carries the scope the page needs,
 * and a scope picker until then.
 *
 * Every sidebar item is always visible, including ones for pages that live
 * deeper than the current scope — clicking Deploy from an org-level page is a
 * normal thing to do. Those items link to the page's *scope-less alias* (see
 * `ScopeHandle` in `routes/paths.ts`), so the page mounts, this gate takes
 * over, and picking a project (and API) navigates to the fully-scoped URL
 * where `children` render. The alternative — hiding the item until scope
 * happens to be right — leaves the user with no way to reach the page at all.
 */
export function ScopeGate({ children, prompt, requires, to }: ScopeGateProps) {
  const { isApiScope, isProjectScope } = useConsoleScope();
  const satisfied = requires === 'api' ? isApiScope : isProjectScope;

  // Picker in separate component so API-list query only mounts when gate is closed.
  if (satisfied) return <>{children}</>;
  return <ScopeSelection prompt={prompt} requires={requires} to={to} />;
}

const DEFAULT_PROMPT: Record<RequiredScope, string> = {
  api: 'This page is available at the API level.',
  project: 'This page is available at the project level.',
};

function ScopeSelection({
  prompt,
  requires,
  to,
}: Omit<ScopeGateProps, 'children'>) {
  const navigate = useNavigate();
  const { isLoading, isProjectScope, params, projects, projectsError, organization, organizations } =
    useConsoleScope();
  // When only the API is missing, the project stays fixed at the route's own,
  // switching project is the header switcher's job, not this card's.
  const [chosenProject, setChosenProject] = useState(
    params.projectHandler ?? ''
  );
  const [chosenApi, setChosenApi] = useState('');

  const needsApi = requires === 'api';
  const apisQuery = useRestApis(
    {},
    { projectId: needsApi ? chosenProject || undefined : undefined }
  );
  const apis = apisQuery.data?.list ?? [];

  if (projectsError) {
    return (
      <PageContent>
        <ErrorState message="Unable to load projects" />
      </PageContent>
    );
  }
  if (isLoading && projects.length === 0) {
    return <LoadingState label="Loading projects" />;
  }

  const orgHandle = params.orgHandle ?? '';
  const canContinue = Boolean(chosenProject && (!needsApi || chosenApi));
  const submit = () => {
    if (!canContinue) return;
    // `replace` keeps Back on the previous page.
    navigate(to(orgHandle, chosenProject, chosenApi || null), {
      replace: true,
    });
  };

  return (
    <>
      <Card variant="outlined" sx={{ maxWidth: 920 }}>
        <CardContent sx={{ p: 3 }}>
          <Stack spacing={2}>
            <Box>
              <Typography variant="h5" sx={{ fontWeight: 600 }}>
                {prompt ?? DEFAULT_PROMPT[requires]}
              </Typography>
              <Typography color="text.secondary">
                {needsApi
                  ? <FormattedMessage
                      id="scopeGate.selectProjectAndApi"
                      defaultMessage='Select a project and an API to switch to API level and continue.'
                      />
                  : <FormattedMessage
                      id="scopeGate.selectProject"
                      defaultMessage='Select a project to switch to project level and continue.'
                    />}
              </Typography>
            </Box>
          

          {projects.length === 0 ? (
            <Box sx={{ mt: 3 }}>
              <Typography color="text.secondary">
                <FormattedMessage
                  id="scopeGate.noProjects"
                  defaultMessage="You have no projects yet. Create a project to continue."
                />
              </Typography>
              <Button
                variant="contained"
                sx={{ mt: 2 }}
                onClick={() => navigate(routes.projects(orgHandle))}
              >
                <FormattedMessage
                  id="scopeGate.goToProjects"
                  defaultMessage="Go to Projects"
                />
              </Button>
            </Box>
          ) : (
            <Stack
              spacing={1.5}
              sx={{ ml: 'auto', flexShrink: 0 }}
              direction={{ xs: 'column', sm: 'row' }}
              alignItems={{ xs: 'stretch', sm: 'flex-end' }}
            >
              {!isProjectScope && (
                <FormControl fullWidth sx={{ maxWidth: 400 }}>
                    {/*
                      `id` here and `labelId` below: a bare FormLabel is not
                      associated with the Select, which leaves the combobox with
                      no accessible name for screen readers (and nothing for a
                      test to query it by).
                    */}
                    <FormLabel id="scope-gate-project-label">
                      <FormattedMessage
                        id="scopeGate.project"
                        defaultMessage="Project"
                        />
                    </FormLabel>
                    <Select
                      labelId="scope-gate-project-label"
                      value={
                        chosenProject || '__loading__'
                      }
                      onChange={(event) => {
                        setChosenProject(String(event.target.value));
                        // The previous pick belongs to the previous project.
                        setChosenApi('');
                      }}
                      displayEmpty
                      disabled={
                        isLoading ||
                        !organization ||
                        organizations.length === 0
                      }
                      MenuProps={{ PaperProps: { sx: { maxHeight: 300 } } }}
                    >
                      {isLoading ? (
                        <MenuItem value="__loading__" disabled>
                          <FormattedMessage
                            id="scopeGate.loadingProjects"
                            defaultMessage="Loading projects..."
                          />
                        </MenuItem>
                      ) : projects.length === 0 ? (
                        <MenuItem value="" disabled>
                          <FormattedMessage
                            id="scopeGate.noProjects"
                            defaultMessage="No projects available"
                          />
                        </MenuItem>
                      ) : (projects.map((project) => (
                          <ComplexSelect.MenuItem
                            key={project.id}
                            value={project.id}
                          >
                            <Stack direction="row" alignItems="center" spacing={1}>
                              <ComplexSelect.MenuItem.Icon>
                                <Layers size={18} />
                              </ComplexSelect.MenuItem.Icon>
                              <ComplexSelect.MenuItem.Text
                                primary={project.displayName}
                                secondary={project.id}
                              />
                            </Stack>
                          </ComplexSelect.MenuItem>
                      )))
                    }
                    </Select>
                </FormControl>
                
              )}

              {needsApi && (
                <FormControl fullWidth sx={{ maxWidth: 300 }}>
                    <FormLabel id="scope-gate-api-label">
                      <FormattedMessage
                        id="scopeGate.api"
                        defaultMessage="API"
                      />
                    </FormLabel>
                    <Select
                      labelId="scope-gate-api-label"
                      value={chosenApi || '__loading__'}
                      onChange={(event) => setChosenApi(String(event.target.value))}
                      displayEmpty
                      disabled={!chosenProject || apisQuery.isPending}
                      MenuProps={{ PaperProps: { sx: { maxHeight: 300 } } }}
                    >
                      {apisQuery.isPending ? (
                        <MenuItem value="__loading__" disabled>
                          <FormattedMessage
                            id="scopeGate.loadingApis"
                            defaultMessage="Loading APIs..."
                          />
                        </MenuItem>
                      ) : apis.length === 0 ? (
                        <MenuItem value="" disabled>
                          <FormattedMessage
                            id="scopeGate.noApis"
                            defaultMessage="No APIs available"
                          />
                        </MenuItem>
                      ) : (
                        apis.map((api) => (
                          <ComplexSelect.MenuItem key={api.id} value={api.id ?? ''}>
                            <Stack direction="row" alignItems="center" spacing={1}>
                              <ComplexSelect.MenuItem.Icon>
                                <Boxes size={18} />
                              </ComplexSelect.MenuItem.Icon>
                            <ComplexSelect.MenuItem.Text
                              primary={api.displayName}
                              secondary={api.version} /> 
                            </Stack>
                          </ComplexSelect.MenuItem>
                        ))
                      )}
                    </Select>
                </FormControl>
              )}

              <Button
                sx={{ height: '100%'}}
                variant="contained"
                disabled={!canContinue}
                onClick={submit}
              >
                {needsApi ? (
                  <FormattedMessage
                    id="scopeGate.goToApiLevel"
                    defaultMessage="Go to API Level"
                  />
                ) : (
                  <FormattedMessage
                    id="scopeGate.goToProjectLevel"
                    defaultMessage="Go to Project Level"
                  />
                )}
              </Button>
            </Stack>
          )}
          </Stack>
            

          {needsApi &&
            chosenProject &&
            !apisQuery.isPending &&
            apis.length === 0 && (
              <Typography color="text.secondary" sx={{ mt: 2 }}>
                <FormattedMessage
                  id="scopeGate.noApis"
                  defaultMessage="This project has no APIs yet. Create an API to continue."
                />
              </Typography>
            )}
        </CardContent>
      </Card>
    </>
  );
}
