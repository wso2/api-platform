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

import { type FormEvent, useMemo, useState } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  FormControl,
  FormLabel,
  MenuItem,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';

import {
  useObservabilityLogs,
  type ObservabilityLogLevel,
  type ObservabilityLogsScope,
  type RestApiObservabilityLog,
  type RestApiObservabilityLogsQuery,
  useRestApis,
} from '../../../../api/resources/restApis';
import { useProjects } from '../../../../api/resources/projects';
import { EmptyState } from '../../../../components/StateViews';
import { runtimeConfig } from '../../../../config/runtime';

type TrafficLog = {
  timestamp?: string;
  correlationId?: string;
  status?: number;
  operation?: { method?: string; path?: string };
  target?: { statusCode?: number };
};

const buildQuery = (
  durationMinutes: number,
  search: string,
  level: ObservabilityLogLevel | '',
  filters: Pick<
    RestApiObservabilityLogsQuery,
    'component' | 'environment' | 'project'
  > = {}
): RestApiObservabilityLogsQuery => {
  const end = new Date();
  return {
    startTime: new Date(
      end.getTime() - durationMinutes * 60 * 1000
    ).toISOString(),
    endTime: end.toISOString(),
    limit: 100,
    query: search.trim() || undefined,
    logLevels: level ? [level] : undefined,
    ...filters,
  };
};

export const parseGatewayTrafficLog = (
  entry: RestApiObservabilityLog
): TrafficLog | undefined => {
  if (typeof entry.log !== 'string') return undefined;
  try {
    const structuredLog = entry.log.trim().replace(/^\[pol\]\s*/, '');
    const parsed = JSON.parse(structuredLog) as unknown;
    return parsed && typeof parsed === 'object'
      ? (parsed as TrafficLog)
      : undefined;
  } catch {
    return undefined;
  }
};

const levelColor = (level: string) => {
  if (level === 'ERROR') return 'error' as const;
  if (level === 'WARN') return 'warning' as const;
  if (level === 'INFO') return 'info' as const;
  return 'default' as const;
};

function LogRecord({ entry }: { entry: RestApiObservabilityLog }) {
  const intl = useIntl();
  const traffic = parseGatewayTrafficLog(entry);
  const level = String(entry.level || entry.logLevel || 'INFO').toUpperCase();
  const timestamp = traffic?.timestamp || entry.timestamp;
  const status = traffic?.status ?? traffic?.target?.statusCode;
  const request = [traffic?.operation?.method, traffic?.operation?.path]
    .filter(Boolean)
    .join(' ');
  const summary = request
    ? `${request}${status !== undefined ? ` → ${status}` : ''}`
    : entry.log ||
      intl.formatMessage({
        id: 'appShell.runtimeLogsPage.gatewayTrafficEvent',
        defaultMessage: 'Gateway traffic event',
      });

  return (
    <Card variant="outlined">
      <CardContent>
        <Stack alignItems="center" direction="row" spacing={1}>
          <Chip color={levelColor(level)} label={level} size="small" />
          <Typography
            component="time"
            color="text.secondary"
            dateTime={timestamp}
            variant="body2"
          >
            {timestamp
              ? new Date(timestamp).toLocaleString()
              : intl.formatMessage({
                  id: 'appShell.runtimeLogsPage.unknownTime',
                  defaultMessage: 'Unknown time',
                })}
          </Typography>
          {traffic?.correlationId && (
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage
                id="appShell.runtimeLogsPage.correlation"
                defaultMessage="Correlation: {id}"
                values={{ id: traffic.correlationId }}
              />
            </Typography>
          )}
        </Stack>
        <Typography
          component="pre"
          sx={{
            fontFamily: 'monospace',
            fontSize: 13,
            mb: 0,
            mt: 1.5,
            overflowWrap: 'anywhere',
            whiteSpace: 'pre-wrap',
          }}
        >
          {summary}
        </Typography>
      </CardContent>
    </Card>
  );
}

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
  const [request, setRequest] = useState(() =>
    buildQuery(60, '', '', {
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
  const logsQuery = useObservabilityLogs(
    scope,
    request,
    {},
    runtimeConfig.observabilityLogsEnabled
  );

  const queryLogs = (event?: FormEvent) => {
    event?.preventDefault();
    setRequest(
      buildQuery(durationMinutes, search, level, {
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
          <FormattedMessage
            id="appShell.runtimeLogsPage.subHeader"
            defaultMessage="Tenant-scoped traffic logs for {scopeLabel}. Request and response headers and bodies are not collected."
            values={{ scopeLabel }}
          />
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
            onSubmit={queryLogs}
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
                placeholder={intl.formatMessage({
                  id: 'appShell.runtimeLogsPage.searchPlaceholder',
                  defaultMessage: 'Method, path, status, or correlation ID',
                })}
                size="small"
                value={search}
              />
            </FormControl>
            <Button
              disabled={logsQuery.isFetching}
              type="submit"
              variant="contained"
            >
              {logsQuery.isFetching ? (
                <FormattedMessage
                  id="appShell.runtimeLogsPage.querying"
                  defaultMessage="Querying…"
                />
              ) : (
                <FormattedMessage
                  id="appShell.runtimeLogsPage.queryLogs"
                  defaultMessage="Query logs"
                />
              )}
            </Button>
          </Stack>

          {logsQuery.isLoading && (
            <Stack alignItems="center" direction="row" spacing={1}>
              <CircularProgress size={18} />
              <Typography>
                <FormattedMessage
                  id="appShell.runtimeLogsPage.loading"
                  defaultMessage="Loading gateway logs…"
                />
              </Typography>
            </Stack>
          )}
          {logsQuery.error && (
            <Alert severity="error">
              <FormattedMessage
                id="appShell.runtimeLogsPage.loadError"
                defaultMessage="Could not load gateway logs. {message}"
                values={{ message: logsQuery.error.message }}
              />
            </Alert>
          )}
          {!logsQuery.isLoading &&
            !logsQuery.error &&
            logsQuery.data?.items.length === 0 && (
              <EmptyState
                description={intl.formatMessage({
                  id: 'appShell.runtimeLogsPage.emptyDescription',
                  defaultMessage: 'Try a wider time range or remove a filter.',
                })}
                title={intl.formatMessage({
                  id: 'appShell.runtimeLogsPage.emptyTitle',
                  defaultMessage: 'No gateway logs found',
                })}
              />
            )}
          {logsQuery.data?.items.map((entry, index) => (
            <LogRecord
              entry={entry}
              key={`${entry.timestamp || 'log'}-${index}`}
            />
          ))}
        </>
      )}
    </Stack>
  );
}
