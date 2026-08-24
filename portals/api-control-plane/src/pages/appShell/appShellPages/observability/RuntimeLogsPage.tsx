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

import { type FormEvent, useState } from 'react';
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
  useRestApiObservabilityLogs,
  type ObservabilityLogLevel,
  type RestApiObservabilityLog,
  type RestApiObservabilityLogsQuery,
} from '../../../../api/resources/restApis';
import { EmptyState } from '../../../../components/StateViews';
import { runtimeConfig } from '../../../../config/runtime';
import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';

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
  level: ObservabilityLogLevel | ''
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
  };
};

export const parseGatewayTrafficLog = (
  entry: RestApiObservabilityLog
): TrafficLog | undefined => {
  if (typeof entry.log !== 'string') return undefined;
  try {
    const parsed = JSON.parse(entry.log) as unknown;
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
  return (
    <ScopeGate
      prompt="Runtime logs are scoped to one API."
      requires="api"
      to={routes.apiObservabilityLogs}
    >
      <RuntimeLogs />
    </ScopeGate>
  );
}

function RuntimeLogs() {
  const intl = useIntl();
  const { apiHandler } = useParams();
  const [durationMinutes, setDurationMinutes] = useState(60);
  const [search, setSearch] = useState('');
  const [level, setLevel] = useState<ObservabilityLogLevel | ''>('');
  const [request, setRequest] = useState(() => buildQuery(60, '', ''));
  const logsQuery = useRestApiObservabilityLogs(
    apiHandler,
    request,
    {},
    runtimeConfig.observabilityLogsEnabled
  );

  const queryLogs = (event?: FormEvent) => {
    event?.preventDefault();
    setRequest(buildQuery(durationMinutes, search, level));
  };

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
            defaultMessage="Tenant-scoped traffic logs for {apiHandler}. Request and response headers and bodies are not collected."
            values={{ apiHandler }}
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
