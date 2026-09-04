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
import { useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  ListingTable,
  MenuItem,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';

import {
  useRestApiObservabilityTrace,
  useRestApiObservabilityTraces,
  type RestApiObservabilityTracesQuery,
} from '../../../../api/resources/restApis';
import { runtimeConfig } from '../../../../config/runtime';
import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';

const buildQuery = (
  durationMinutes: number,
  search: string,
  environment: string
): RestApiObservabilityTracesQuery => {
  const end = new Date();
  return {
    startTime: new Date(end.getTime() - durationMinutes * 60 * 1000).toISOString(),
    endTime: end.toISOString(),
    limit: 100,
    query: search.trim() || undefined,
    environment,
  };
};

const formatDuration = (durationNs?: number) => {
  if (durationNs === undefined) return '-';
  if (durationNs < 1_000_000) return `${(durationNs / 1_000).toFixed(2)} μs`;
  if (durationNs < 1_000_000_000) return `${(durationNs / 1_000_000).toFixed(2)} ms`;
  return `${(durationNs / 1_000_000_000).toFixed(2)} s`;
};

const formatTime = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
};

export function TracesPage() {
  return (
    <ScopeGate prompt="Traces are reported per API." requires="api" to={routes.apiObservabilityTraces}>
      <Traces />
    </ScopeGate>
  );
}

function Traces() {
  const { apiHandler } = useParams();
  const [durationMinutes, setDurationMinutes] = useState(60);
  const [search, setSearch] = useState('');
  const [environment, setEnvironment] = useState('development');
  const [selectedTraceId, setSelectedTraceId] = useState<string>();
  const [query, setQuery] = useState(() => buildQuery(60, '', 'development'));
  const enabled = runtimeConfig.observabilityTracesEnabled;
  const tracesQuery = useRestApiObservabilityTraces(apiHandler, query, enabled);
  const traceQuery = useRestApiObservabilityTrace(
    apiHandler,
    selectedTraceId,
    query,
    enabled
  );

  const applyFilters = (event: FormEvent) => {
    event.preventDefault();
    setSelectedTraceId(undefined);
    setQuery(buildQuery(durationMinutes, search, environment));
  };

  const traces = tracesQuery.data?.items ?? [];
  const spans = traceQuery.data?.spans ?? [];

  return (
    <Stack spacing={2.5}>
      <PageTitle>
        <PageTitle.Header>Distributed traces</PageTitle.Header>
        <PageTitle.SubHeader>
          Gateway request traces for {apiHandler}.
        </PageTitle.SubHeader>
      </PageTitle>

      {!enabled ? (
        <Alert severity="info">Trace retrieval is not configured for this deployment.</Alert>
      ) : (
        <>
          <Stack
            alignItems={{ md: 'flex-end', xs: 'stretch' }}
            component="form"
            direction={{ md: 'row', xs: 'column' }}
            onSubmit={applyFilters}
            spacing={1.5}
          >
            <FormControl sx={{ minWidth: 160 }}>
              <FormLabel>Time range</FormLabel>
              <Select
                onChange={(event) => setDurationMinutes(Number(event.target.value))}
                size="small"
                value={durationMinutes}
              >
                <MenuItem value={15}>Last 15 minutes</MenuItem>
                <MenuItem value={60}>Last hour</MenuItem>
                <MenuItem value={360}>Last 6 hours</MenuItem>
                <MenuItem value={1440}>Last 24 hours</MenuItem>
              </Select>
            </FormControl>
            <FormControl sx={{ minWidth: 160 }}>
              <FormLabel>Environment</FormLabel>
              <Select
                onChange={(event) => setEnvironment(String(event.target.value))}
                size="small"
                value={environment}
              >
                <MenuItem value="development">Development</MenuItem>
                <MenuItem value="stage">Stage</MenuItem>
                <MenuItem value="production">Production</MenuItem>
              </Select>
            </FormControl>
            <TextField
              label="Search"
              onChange={(event) => setSearch(event.target.value)}
              size="small"
              value={search}
            />
            <Button type="submit" variant="contained">Apply filters</Button>
            <Button onClick={() => tracesQuery.refetch()} variant="outlined">Refresh</Button>
          </Stack>

          {tracesQuery.isLoading ? (
            <Stack alignItems="center" py={4}><CircularProgress /></Stack>
          ) : tracesQuery.isError ? (
            <Alert severity="error">Unable to load traces.</Alert>
          ) : traces.length === 0 ? (
            <Alert severity="info">No traces were found for this API and time range.</Alert>
          ) : (
            <ListingTable.Container>
              <ListingTable>
                <ListingTable.Head>
                  <ListingTable.Row>
                    <ListingTable.Cell>Trace</ListingTable.Cell>
                    <ListingTable.Cell>Started</ListingTable.Cell>
                    <ListingTable.Cell>Duration</ListingTable.Cell>
                    <ListingTable.Cell>Spans</ListingTable.Cell>
                    <ListingTable.Cell>Status</ListingTable.Cell>
                    <ListingTable.Cell align="right">Actions</ListingTable.Cell>
                  </ListingTable.Row>
                </ListingTable.Head>
                <ListingTable.Body>
                  {traces.map((trace) => (
                    <ListingTable.Row key={trace.traceId}>
                      <ListingTable.Cell>{trace.traceName || trace.rootSpanName || trace.traceId}</ListingTable.Cell>
                      <ListingTable.Cell>{formatTime(trace.startTime)}</ListingTable.Cell>
                      <ListingTable.Cell>{formatDuration(trace.durationNs)}</ListingTable.Cell>
                      <ListingTable.Cell>{trace.spanCount ?? '-'}</ListingTable.Cell>
                      <ListingTable.Cell>{trace.hasErrors ? 'Error' : 'Success'}</ListingTable.Cell>
                      <ListingTable.Cell align="right">
                        <Button onClick={() => setSelectedTraceId(trace.traceId)} size="small">
                          View spans
                        </Button>
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  ))}
                </ListingTable.Body>
              </ListingTable>
            </ListingTable.Container>
          )}

          {selectedTraceId && (
            <Stack spacing={1.5}>
              <Typography variant="h5">Trace spans</Typography>
              {traceQuery.isLoading ? (
                <Stack alignItems="center" py={3}><CircularProgress size={28} /></Stack>
              ) : traceQuery.isError ? (
                <Alert severity="error">Unable to load trace spans.</Alert>
              ) : (
                <ListingTable.Container>
                  <ListingTable>
                    <ListingTable.Head>
                      <ListingTable.Row>
                        <ListingTable.Cell>Span</ListingTable.Cell>
                        <ListingTable.Cell>Kind</ListingTable.Cell>
                        <ListingTable.Cell>Started</ListingTable.Cell>
                        <ListingTable.Cell>Duration</ListingTable.Cell>
                        <ListingTable.Cell>Status</ListingTable.Cell>
                      </ListingTable.Row>
                    </ListingTable.Head>
                    <ListingTable.Body>
                      {spans.map((span, index) => (
                        <ListingTable.Row key={span.spanId || index}>
                          <ListingTable.Cell>{span.spanName || span.spanId || '-'}</ListingTable.Cell>
                          <ListingTable.Cell>{span.spanKind || '-'}</ListingTable.Cell>
                          <ListingTable.Cell>{formatTime(span.startTime)}</ListingTable.Cell>
                          <ListingTable.Cell>{formatDuration(span.durationNs)}</ListingTable.Cell>
                          <ListingTable.Cell>{span.status?.code || 'unset'}</ListingTable.Cell>
                        </ListingTable.Row>
                      ))}
                    </ListingTable.Body>
                  </ListingTable>
                </ListingTable.Container>
              )}
            </Stack>
          )}
        </>
      )}
    </Stack>
  );
}
