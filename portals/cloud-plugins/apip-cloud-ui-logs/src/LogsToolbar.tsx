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

import { useEffect, useState, type FC, type FormEvent } from 'react';
import {
  Box,
  Button,
  Checkbox,
  FormControl,
  FormLabel,
  ListItemText,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { rangeOptionsFor } from './format';
import type { LogFacets, LogKindFilter, LogLevel, LogQuery, LogViewFilters } from './types';

export type LogsToolbarProps = {
  /** The query currently in force, which seeds the draft. */
  query: LogQuery;
  /** The view narrowing currently in force. */
  view: LogViewFilters;
  /** Server-reported retention, which decides how far back the range picker may offer. */
  retentionDays: number;
  /** Options for the view filters, taken from the lines on screen. */
  facets: LogFacets;
  /** The organization's environments. Falls back to the loaded lines when empty. */
  environments: string[];
  onApply: (query: LogQuery, view: LogViewFilters) => void;
};

const KIND_OPTIONS: { value: LogKindFilter; label: string }[] = [
  { value: 'all', label: 'All logs' },
  { value: 'access', label: 'Request traffic' },
  { value: 'operational', label: 'Gateway activity' },
];

/** Levels offered by the filter, in severity order. */
const LOG_LEVELS: LogLevel[] = ['ERROR', 'WARN', 'INFO', 'DEBUG'];

const ALL_LEVELS_LABEL = 'All levels';

/** Longest search phrase the backend accepts. */
const MAX_SEARCH_LENGTH = 256;

/**
 * The filter bar.
 *
 * Everything is committed together by Apply rather than on each change: the
 * first four fields are a fresh query against the observability API — a search
 * over a time range, and not cheap — and applying the whole bar at once also
 * means one console reset instead of one per keystroke.
 */
const LogsToolbar: FC<LogsToolbarProps> = ({
  query,
  view,
  retentionDays,
  facets,
  environments,
  onApply,
}) => {
  const [draftQuery, setDraftQuery] = useState(query);
  const [draftView, setDraftView] = useState(view);
  const ranges = rangeOptionsFor(retentionDays);

  // Keep the draft in step when the applied values are reset from outside.
  useEffect(() => setDraftQuery(query), [query]);
  useEffect(() => setDraftView(view), [view]);

  // Retention can shrink under a picked range — it is read from the server and
  // has changed once already. Fall back to the longest range still offered
  // rather than showing a value the list no longer contains, which renders the
  // select blank.
  const rangeValue = ranges.some((option) => option.minutes === draftQuery.rangeMinutes)
    ? draftQuery.rangeMinutes
    : ranges[ranges.length - 1].minutes;

  // An active filter outlives the lines it was derived from: Apply empties the
  // buffer, and a quiet pod never comes back into the facets at all. Without its
  // own value the select reads as "All" while still filtering, and disabling an
  // empty list would strand the user in a filter they cannot clear.
  const withActive = (options: string[], active: string): string[] =>
    active && !options.includes(active) ? [active, ...options] : options;

  const projectOptions = withActive(facets.projects, draftView.project);
  const podOptions = withActive(
    draftView.project ? (facets.podsByProject[draftView.project] ?? []) : facets.pods,
    draftView.pod
  );
  const environmentOptions = withActive(
    environments.length > 0 ? environments : facets.environments,
    draftQuery.environment
  );

  const submit = (event: FormEvent) => {
    event.preventDefault();
    onApply({ ...draftQuery, rangeMinutes: rangeValue }, draftView);
  };

  return (
    <Box sx={{ mb: 2 }}>
      <Stack
        alignItems={{ md: 'flex-end', xs: 'stretch' }}
        component="form"
        direction={{ md: 'row', xs: 'column' }}
        onSubmit={submit}
        spacing={1.5}
        sx={{ flexWrap: { md: 'wrap' } }}
      >
        <FormControl sx={{ minWidth: 150 }}>
          <FormLabel>Time range</FormLabel>
          <Select
            size="small"
            value={rangeValue}
            onChange={(event) =>
              setDraftQuery({ ...draftQuery, rangeMinutes: Number(event.target.value) })
            }
          >
            {ranges.map((option) => (
              <MenuItem key={option.minutes} value={option.minutes}>
                {option.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 170 }}>
          <FormLabel>Project</FormLabel>
          <Select
            disabled={projectOptions.length === 0}
            displayEmpty
            size="small"
            value={draftView.project}
            onChange={(event) =>
              // Clearing the pod with the project keeps the pair valid: the pod
              // list is the chosen project's.
              setDraftView({ ...draftView, project: String(event.target.value), pod: '' })
            }
          >
            <MenuItem value="">All projects</MenuItem>
            {projectOptions.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ maxWidth: 240, minWidth: 200 }}>
          <FormLabel>Pod</FormLabel>
          <Select
            disabled={podOptions.length === 0}
            displayEmpty
            size="small"
            value={draftView.pod}
            renderValue={(selected) => (
              <Typography noWrap variant="inherit">
                {(selected as string) || 'All pods'}
              </Typography>
            )}
            onChange={(event) => setDraftView({ ...draftView, pod: String(event.target.value) })}
          >
            <MenuItem value="">All pods</MenuItem>
            {podOptions.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 150 }}>
          <FormLabel>Environment</FormLabel>
          <Select
            disabled={environmentOptions.length === 0}
            displayEmpty
            size="small"
            value={draftQuery.environment}
            onChange={(event) =>
              setDraftQuery({ ...draftQuery, environment: String(event.target.value) })
            }
          >
            <MenuItem value="">All environments</MenuItem>
            {environmentOptions.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 170 }}>
          <FormLabel>Type</FormLabel>
          <Select
            size="small"
            value={draftQuery.kind}
            onChange={(event) =>
              setDraftQuery({ ...draftQuery, kind: event.target.value as LogKindFilter })
            }
          >
            {KIND_OPTIONS.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 170 }}>
          <FormLabel>Level</FormLabel>
          {/* `renderValue` owns the closed-state text for a multi-select, so it
              also supplies the "everything" label for an empty selection —
              `displayEmpty` alone would render a blank box. */}
          <Select
            displayEmpty
            multiple
            size="small"
            value={draftQuery.levels}
            renderValue={(selected) =>
              (selected as LogLevel[]).length === 0
                ? ALL_LEVELS_LABEL
                : (selected as LogLevel[]).join(', ')
            }
            onChange={(event) =>
              setDraftQuery({
                ...draftQuery,
                levels: (typeof event.target.value === 'string'
                  ? event.target.value.split(',')
                  : event.target.value) as LogLevel[],
              })
            }
          >
            {LOG_LEVELS.map((level) => (
              <MenuItem key={level} value={level}>
                <Checkbox checked={draftQuery.levels.includes(level)} size="small" />
                <ListItemText primary={level} />
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ flex: 1, minWidth: 220 }}>
          <FormLabel>Search</FormLabel>
          {/*
            One case-sensitive substring matched anywhere in the line. The
            backend indexes a line as a single opaque value, so there is no field
            to search within and no second phrase to AND.
          */}
          <TextField
            inputProps={{ maxLength: MAX_SEARCH_LENGTH }}
            placeholder="Method, path, status, or any text in the line"
            size="small"
            value={draftQuery.searchPhrase}
            onChange={(event) =>
              setDraftQuery({ ...draftQuery, searchPhrase: event.target.value })
            }
          />
        </FormControl>

        {/* Never disabled on fetch: a live tail is almost always fetching, and
            the console toolbar already shows that progress. */}
        <Button type="submit" variant="contained">
          Apply
        </Button>
      </Stack>

      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
        Project and pod narrow the lines already on screen. Time range, environment, type, level
        and search are sent to the query.
      </Typography>
    </Box>
  );
};

export default LogsToolbar;
