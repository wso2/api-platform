/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { type FormEvent, useEffect, useMemo, useState } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  CircularProgress,
  FormControl,
  FormControlLabel,
  FormLabel,
  MenuItem,
  PageTitle,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';

import {
  useObservabilityLogTail,
  type ObservabilityLogLevel,
  type ObservabilityLogsScope,
  type ObservabilityLogTailFilters,
  useRestApis,
} from '../../../../api/resources/restApis';
import { useProjects } from '../../../../api/resources/projects';
import { LogConsole, type ConsoleLine } from '../../../../components/LogConsole';
import { runtimeConfig } from '../../../../config/runtime';
import { mergeLogLines } from './logLines';

/**
 * Rows the console keeps. Selection and copy work on real DOM nodes rather than
 * a virtualised window, so the buffer is capped to keep the page responsive
 * during a long tail; the oldest rows fall off first.
 */
const MAX_CONSOLE_LINES = 2000;

/** Records requested per poll. */
const PAGE_LIMIT = 100;

const buildFilters = (
  durationMinutes: number,
  search: string,
  level: ObservabilityLogLevel | '',
  scopeFilters: Pick<
    ObservabilityLogTailFilters,
    'component' | 'environment' | 'project'
  > = {}
): ObservabilityLogTailFilters => ({
  durationMinutes,
  limit: PAGE_LIMIT,
  query: search.trim() || undefined,
  logLevels: level ? [level] : undefined,
  ...scopeFilters,
});

export function RuntimeLogsPage() {
  return <RuntimeLogs />;
}

function RuntimeLogs() {
  const intl = useIntl();
  const { apiHandler, projectHandler } = useParams();
  const aggregateScope = !apiHandler;
  const projectScope = Boolean(projectHandler && !apiHandler);
  const [durationMinutes, setDurationMinutes] = useState(60);
  const [search, setSearch] = useState('');
  const [level, setLevel] = useState<ObservabilityLogLevel | ''>('');
  const [project, setProject] = useState(projectHandler || '');
  const [component, setComponent] = useState('');
  const [environment, setEnvironment] = useState('development');
  const [live, setLive] = useState(true);
  const [lines, setLines] = useState<ConsoleLine[]>([]);
  /** Watermark set by Clear, so the next poll cannot refill the wiped window. */
  const [clearedAt, setClearedAt] = useState(0);
  const [filters, setFilters] = useState(() =>
    buildFilters(60, '', '', {
      environment: aggregateScope ? 'development' : undefined,
      project: aggregateScope ? projectHandler : undefined,
    })
  );
  const scope = useMemo<ObservabilityLogsScope>(
    () =>
      apiHandler
        ? { restApiId: apiHandler }
        : projectHandler
          ? { projectId: projectHandler }
          : {},
    [apiHandler, projectHandler]
  );
  const projectsQuery = useProjects({ limit: 100 });
  const apisQuery = useRestApis({}, { projectId: project || undefined });
  const projects = projectsQuery.data?.list ?? [];
  const components = apisQuery.data?.list ?? [];
  const logsQuery = useObservabilityLogTail(scope, filters, {
    enabled: runtimeConfig.observabilityLogsEnabled,
    live,
  });

  /*
   * Each poll returns the whole rolling window, so records already on screen are
   * dropped and only new ones are appended — the console grows instead of being
   * rebuilt, which is what keeps an in-progress selection intact.
   */
  const page = logsQuery.data;
  useEffect(() => {
    if (!page) return;
    setLines((previous) =>
      mergeLogLines(previous, page.items, MAX_CONSOLE_LINES, clearedAt)
    );
  }, [clearedAt, page]);

  const clearConsole = () => {
    setLines([]);
    setClearedAt(Date.now());
  };

  const applyFilters = (event?: FormEvent) => {
    event?.preventDefault();
    setLines([]);
    setClearedAt(0);
    setFilters(
      buildFilters(durationMinutes, search, level, {
        component: aggregateScope ? component || undefined : undefined,
        environment: aggregateScope ? environment : undefined,
        project: aggregateScope ? project || undefined : undefined,
      })
    );
  };

  const scopeLabel = apiHandler || projectHandler || 'this organization';

  return (
    <Stack spacing={2.5}>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage
            id="appShell.runtimeLogsPage.header"
            defaultMessage="Gateway logs"
          />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          {apiHandler ? (
            <FormattedMessage
              id="appShell.runtimeLogsPage.apiSubHeader"
              defaultMessage="API traffic logs for {scopeLabel}. Request and response headers and bodies are not collected."
              values={{ scopeLabel }}
            />
          ) : (
            <FormattedMessage
              id="appShell.runtimeLogsPage.aggregateSubHeader"
              defaultMessage="Gateway runtime logs for {scopeLabel}, including operational and traffic records."
              values={{ scopeLabel }}
            />
          )}
        </PageTitle.SubHeader>
      </PageTitle>

      {!runtimeConfig.observabilityLogsEnabled ? (
        <Alert severity="info">
          <FormattedMessage
            id="appShell.runtimeLogsPage.notConfigured"
            defaultMessage="Gateway log retrieval is not configured for this deployment."
          />
        </Alert>
      ) : (
        <>
          <Stack
            alignItems={{ md: 'flex-end', xs: 'stretch' }}
            component="form"
            direction={{ md: 'row', xs: 'column' }}
            onSubmit={applyFilters}
            spacing={1.5}
            sx={{ flexWrap: { md: 'wrap' } }}
          >
            <FormControl sx={{ minWidth: 150 }}>
              <FormLabel>
                <FormattedMessage
                  id="appShell.runtimeLogsPage.timeRange"
                  defaultMessage="Time range"
                />
              </FormLabel>
              <Select
                onChange={(event) =>
                  setDurationMinutes(Number(event.target.value))
                }
                size="small"
                value={durationMinutes}
              >
                <MenuItem value={15}>
                  <FormattedMessage
                    id="appShell.runtimeLogsPage.last15Minutes"
                    defaultMessage="Last 15 minutes"
                  />
                </MenuItem>
                <MenuItem value={60}>
                  <FormattedMessage
                    id="appShell.runtimeLogsPage.lastHour"
                    defaultMessage="Last hour"
                  />
                </MenuItem>
                <MenuItem value={360}>
                  <FormattedMessage
                    id="appShell.runtimeLogsPage.last6Hours"
                    defaultMessage="Last 6 hours"
                  />
                </MenuItem>
                <MenuItem value={1440}>
                  <FormattedMessage
                    id="appShell.runtimeLogsPage.last24Hours"
                    defaultMessage="Last 24 hours"
                  />
                </MenuItem>
              </Select>
            </FormControl>
            {aggregateScope && (
              <>
                <FormControl sx={{ minWidth: 180 }}>
                  <FormLabel>
                    <FormattedMessage
                      id="appShell.runtimeLogsPage.project"
                      defaultMessage="Project"
                    />
                  </FormLabel>
                  <Select
                    disabled={projectScope || projectsQuery.isLoading}
                    onChange={(event) => {
                      setProject(String(event.target.value));
                      setComponent('');
                    }}
                    size="small"
                    value={project}
                  >
                    {!projectScope && (
                      <MenuItem value="">
                        <FormattedMessage
                          id="appShell.runtimeLogsPage.allProjects"
                          defaultMessage="All projects"
                        />
                      </MenuItem>
                    )}
                    {projectScope &&
                      !projects.some((item) => item.id === project) && (
                        <MenuItem value={project}>{project}</MenuItem>
                      )}
                    {projects.map((item) => (
                      <MenuItem key={item.id} value={item.id}>
                        {item.displayName}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
                <FormControl sx={{ minWidth: 190 }}>
                  <FormLabel>
                    <FormattedMessage
                      id="appShell.runtimeLogsPage.component"
                      defaultMessage="Component"
                    />
                  </FormLabel>
                  <Select
                    disabled={!project || apisQuery.isLoading}
                    onChange={(event) =>
                      setComponent(String(event.target.value))
                    }
                    size="small"
                    value={component}
                  >
                    <MenuItem value="">
                      <FormattedMessage
                        id="appShell.runtimeLogsPage.allComponents"
                        defaultMessage="All components"
                      />
                    </MenuItem>
                    {components.map((item) => (
                      <MenuItem key={item.id} value={item.id || ''}>
                        {item.displayName}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
                <FormControl sx={{ minWidth: 160 }}>
                  <FormLabel>
                    <FormattedMessage
                      id="appShell.runtimeLogsPage.environment"
                      defaultMessage="Environment"
                    />
                  </FormLabel>
                  <Select
                    onChange={(event) =>
                      setEnvironment(String(event.target.value))
                    }
                    size="small"
                    value={environment}
                  >
                    <MenuItem value="development">
                      <FormattedMessage
                        id="appShell.runtimeLogsPage.environmentDevelopment"
                        defaultMessage="Development"
                      />
                    </MenuItem>
                    <MenuItem value="stage">
                      <FormattedMessage
                        id="appShell.runtimeLogsPage.environmentStage"
                        defaultMessage="Stage"
                      />
                    </MenuItem>
                    <MenuItem value="production">
                      <FormattedMessage
                        id="appShell.runtimeLogsPage.environmentProduction"
                        defaultMessage="Production"
                      />
                    </MenuItem>
                  </Select>
                </FormControl>
              </>
            )}
            <FormControl sx={{ minWidth: 130 }}>
              <FormLabel>
                <FormattedMessage
                  id="appShell.runtimeLogsPage.level"
                  defaultMessage="Level"
                />
              </FormLabel>
              <Select
                onChange={(event) =>
                  setLevel(event.target.value as ObservabilityLogLevel | '')
                }
                size="small"
                value={level}
              >
                <MenuItem value="">
                  <FormattedMessage
                    id="appShell.runtimeLogsPage.allLevels"
                    defaultMessage="All levels"
                  />
                </MenuItem>
                {(['ERROR', 'WARN', 'INFO', 'DEBUG'] as const).map((value) => (
                  <MenuItem key={value} value={value}>
                    {value}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl sx={{ flex: 1 }}>
              <FormLabel>
                <FormattedMessage
                  id="appShell.runtimeLogsPage.search"
                  defaultMessage="Search"
                />
              </FormLabel>
              <TextField
                inputProps={{ maxLength: 256 }}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={
                  apiHandler
                    ? intl.formatMessage({
                        id: 'appShell.runtimeLogsPage.apiSearchPlaceholder',
                        defaultMessage:
                          'Method, path, status, or correlation ID',
                      })
                    : intl.formatMessage({
                        id: 'appShell.runtimeLogsPage.aggregateSearchPlaceholder',
                        defaultMessage:
                          'Message, route, project, or correlation ID',
                      })
                }
                size="small"
                value={search}
              />
            </FormControl>
            {/* Never disabled on fetch: a live tail is almost always fetching,
                and the console toolbar already shows that progress. */}
            <Button type="submit" variant="contained">
              <FormattedMessage
                id="appShell.runtimeLogsPage.applyFilters"
                defaultMessage="Apply filters"
              />
            </Button>
          </Stack>

          {logsQuery.error && (
            <Alert severity="error">
              <FormattedMessage
                id="appShell.runtimeLogsPage.loadError"
                defaultMessage="Could not load gateway logs. {message}"
                values={{ message: logsQuery.error.message }}
              />
            </Alert>
          )}

          <LogConsole
            actions={
              <Stack alignItems="center" direction="row" spacing={1}>
                {logsQuery.isFetching && <CircularProgress size={14} />}
                {!live && (
                  <Button
                    disabled={logsQuery.isFetching}
                    onClick={() => logsQuery.refetch()}
                    size="small"
                    sx={{ color: 'inherit' }}
                  >
                    <FormattedMessage
                      id="appShell.runtimeLogsPage.refresh"
                      defaultMessage="Refresh"
                    />
                  </Button>
                )}
                <FormControlLabel
                  control={
                    <Switch
                      checked={live}
                      color="success"
                      onChange={(event) => setLive(event.target.checked)}
                      size="small"
                    />
                  }
                  label={
                    <Typography variant="caption">
                      <FormattedMessage
                        id="appShell.runtimeLogsPage.liveTail"
                        defaultMessage="Live tail"
                      />
                    </Typography>
                  }
                  sx={{ mr: 0 }}
                />
              </Stack>
            }
            emptyMessage={
              logsQuery.isLoading
                ? intl.formatMessage({
                    id: 'appShell.runtimeLogsPage.loading',
                    defaultMessage: 'Loading gateway logs…',
                  })
                : intl.formatMessage({
                    id: 'appShell.runtimeLogsPage.emptyConsole',
                    defaultMessage:
                      'No gateway logs in this window. Try a wider time range or remove a filter.',
                  })
            }
            label={intl.formatMessage({
              id: 'appShell.runtimeLogsPage.consoleLabel',
              defaultMessage: 'Gateway log output',
            })}
            lines={lines}
            live={live}
            onClear={clearConsole}
          />
        </>
      )}
    </Stack>
  );
}
