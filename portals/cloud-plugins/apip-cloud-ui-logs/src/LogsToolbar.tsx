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

import {
  cloneElement,
  useEffect,
  useId,
  useRef,
  useState,
  type FC,
  type ReactElement,
  type ReactNode,
} from 'react';
import {
  Box,
  Button,
  Checkbox,
  Divider,
  FormControlLabel,
  IconButton,
  InputAdornment,
  Menu,
  MenuItem,
  Paper,
  Popover,
  Radio,
  RadioGroup,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  ChevronDown,
  Clock,
  Funnel,
  RefreshCw,
  Search,
} from '@wso2/oxygen-ui-icons-react';
import { KIND_LABELS, activeFilterCount, rangeLabel, rangeOptionsFor } from './format';
import type { Facet, LogFacets, LogKind, LogLevel, LogQuery, LogViewFilters } from './types';

export type LogsToolbarProps = {
  /** The query in force. The bar is controlled — it holds no draft of its own. */
  query: LogQuery;
  onQueryChange: (query: LogQuery) => void;
  /** Narrowing applied to the loaded lines. Changing it never refetches. */
  view: LogViewFilters;
  onViewChange: (view: LogViewFilters) => void;
  /** Puts every filter back, query and view alike. Leaves the time range alone. */
  onReset: () => void;
  /** Server-reported retention, which decides how far back the range picker may offer. */
  retentionDays: number;
  /** Options and counts, taken from the lines on screen. */
  facets: LogFacets;
  /** The organization's environments. Falls back to `facets` when empty. */
  environments: string[];
  /** Whether the console is polling for new lines. */
  live: boolean;
  onLiveChange: (live: boolean) => void;
  /** A fetch is in flight — disables Refresh so a click cannot queue a second. */
  fetching: boolean;
  onRefresh: () => void;
};

/** Longest search phrase the backend accepts. */
const MAX_SEARCH_LENGTH = 256;

/** Levels offered even when nothing in the buffer carries them. */
const LOG_LEVELS: LogLevel[] = ['DEBUG', 'ERROR', 'INFO', 'WARN'];

/**
 * A facet list that keeps a chosen value visible even when the buffer no longer
 * holds it.
 *
 * Kind, level and environment are query parameters, so selecting one filters the
 * buffer down to it and every other value counts zero and disappears. Without
 * this the reader would be left in a filter whose control had vanished.
 */
const withSelected = (facets: Facet[], selected: readonly string[]): Facet[] => {
  const present = new Set(facets.map((facet) => facet.value));
  return [
    ...facets,
    ...selected.filter((value) => !present.has(value)).map((value) => ({ value, label: value, count: 0 })),
  ];
};

/**
 * One titled column of the filter panel — an announced group, so four columns of
 * checkboxes are not read out as one undifferentiated list.
 */
const FacetGroup: FC<{ title: string; children: ReactNode }> = ({ title, children }) => {
  const titleId = useId();
  return (
    <Stack aria-labelledby={titleId} role="group" spacing={0.5} sx={{ minWidth: 200 }}>
      <Typography
        id={titleId}
        variant="overline"
        color="text.secondary"
        sx={{ fontWeight: 600, letterSpacing: 0.6 }}
      >
        {title}
      </Typography>
      {children}
    </Stack>
  );
};

/**
 * One row of a facet group: a control, its label, and how much of the buffer it
 * is. The count is folded into the control's own name — laid out as a sibling of
 * the label it reaches no accessible name at all, and "Error" without "25" is
 * half of what the row says.
 */
const FacetRow: FC<{
  control: ReactElement<{ inputProps?: Record<string, unknown> }>;
  label: string;
  count: number;
}> = ({ control, count, label }) => (
  <Stack alignItems="center" direction="row" justifyContent="space-between" sx={{ pr: 1 }}>
    <FormControlLabel
      control={cloneElement(control, {
        inputProps: { 'aria-label': `${label}, ${count} lines` },
      })}
      label={<Typography variant="body2">{label}</Typography>}
      sx={{ mr: 1, overflow: 'hidden' }}
    />
    <Typography aria-hidden variant="caption" color="text.secondary">
      {count}
    </Typography>
  </Stack>
);

/**
 * The toolbar: one search box, a time range, everything else behind Filters, and
 * the live-tail switch.
 *
 * Nothing is committed by a button. Each control calls back as it changes and
 * the owner debounces the whole query before it reaches the network, so two
 * boxes ticked in a second cost one request and the search box needs no debounce
 * of its own.
 */
const LogsToolbar: FC<LogsToolbarProps> = ({
  query,
  onQueryChange,
  view,
  onViewChange,
  onReset,
  retentionDays,
  facets,
  environments,
  live,
  onLiveChange,
  fetching,
  onRefresh,
}) => {
  const [rangeAnchor, setRangeAnchor] = useState<HTMLElement | null>(null);
  const [filtersAnchor, setFiltersAnchor] = useState<HTMLElement | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const ranges = rangeOptionsFor(retentionDays);
  const filterCount = activeFilterCount(query, view);

  // `/` jumps to the search box, as the placeholder promises — but not while the
  // reader is already typing somewhere, where it is just a character.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
      // A panel is open and holds the focus trap; stealing focus fights it.
      if (rangeAnchor || filtersAnchor) return;
      const active = document.activeElement;
      const tag = active?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || (active as HTMLElement)?.isContentEditable) {
        return;
      }
      event.preventDefault();
      searchRef.current?.focus();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [filtersAnchor, rangeAnchor]);

  // Retention can shrink under a picked range — it is read from the server and
  // has changed once already. Fall back to the longest range still offered
  // rather than showing a value the list no longer contains.
  const rangeValue = ranges.some((option) => option.minutes === query.rangeMinutes)
    ? query.rangeMinutes
    : ranges[ranges.length - 1].minutes;

  const update = (patch: Partial<LogQuery>) => onQueryChange({ ...query, ...patch });

  // Written back, not just displayed. Showing the shorter range while still
  // asking for the longer one makes the button lie about what is on screen.
  useEffect(() => {
    if (rangeValue !== query.rangeMinutes) update({ rangeMinutes: rangeValue });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rangeValue]);

  const toggle = <T extends string>(list: T[], value: T): T[] =>
    list.includes(value) ? list.filter((entry) => entry !== value) : [...list, value];

  const environmentFacets = withSelected(
    environments.length > 0
      ? environments.map((name) => ({
          value: name,
          label: name,
          count: facets.environments.find((facet) => facet.value === name)?.count ?? 0,
        }))
      : facets.environments,
    query.environment ? [query.environment] : []
  );
  // Both groups are built from their own fixed key set, not from the buffer: a
  // kind or level is a query parameter, so selecting one empties the buffer of
  // every other value and the alternatives would vanish from the panel that has
  // to offer them.
  const kindFacets = (Object.keys(KIND_LABELS) as LogKind[]).map((kind) => ({
    value: kind,
    label: KIND_LABELS[kind],
    count: facets.kinds.find((facet) => facet.value === kind)?.count ?? 0,
  }));
  // No `withSelected` here: `LogLevel` is exactly these four, so a selected
  // level is always already in the list.
  const levelFacets = LOG_LEVELS.map((level) => ({
    value: level,
    // Sentence case, because the chip in the console is the shouted one.
    label: level.charAt(0) + level.slice(1).toLowerCase(),
    count: facets.levels.find((facet) => facet.value === level)?.count ?? 0,
  }));

  return (
    <Paper
      variant="outlined"
      sx={{ alignItems: 'center', display: 'flex', gap: 1, mb: 2, px: 1, py: 0.5 }}
    >
      <TextField
        fullWidth
        inputRef={searchRef}
        inputProps={{ 'aria-label': 'Search logs', maxLength: MAX_SEARCH_LENGTH }}
        InputProps={{
          disableUnderline: true,
          startAdornment: (
            <InputAdornment position="start">
              <Search size={16} />
            </InputAdornment>
          ),
        }}
        placeholder="Search log text — press /"
        size="small"
        value={query.searchPhrase}
        variant="standard"
        onChange={(event) => update({ searchPhrase: event.target.value })}
      />

      <Divider flexItem orientation="vertical" sx={{ my: 0.5 }} />

      <Button
        aria-expanded={Boolean(rangeAnchor)}
        aria-haspopup="menu"
        color={rangeAnchor ? 'primary' : 'inherit'}
        endIcon={<ChevronDown size={16} />}
        onClick={(event) => setRangeAnchor(event.currentTarget)}
        size="small"
        startIcon={<Clock size={16} />}
        sx={{ flexShrink: 0 }}
        variant="outlined"
      >
        {rangeLabel(rangeValue)}
      </Button>
      <Menu anchorEl={rangeAnchor} open={Boolean(rangeAnchor)} onClose={() => setRangeAnchor(null)}>
        {ranges.map((option) => (
          <MenuItem
            key={option.minutes}
            selected={option.minutes === rangeValue}
            onClick={() => {
              update({ rangeMinutes: option.minutes });
              setRangeAnchor(null);
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'center', mr: 1, width: 20 }}>
              {option.minutes === rangeValue ? <Check size={16} /> : null}
            </Box>
            {option.label}
          </MenuItem>
        ))}
      </Menu>

      <Button
        aria-expanded={Boolean(filtersAnchor)}
        aria-haspopup="dialog"
        color={filtersAnchor || filterCount > 0 ? 'primary' : 'inherit'}
        endIcon={<ChevronDown size={16} />}
        onClick={(event) => setFiltersAnchor(event.currentTarget)}
        size="small"
        startIcon={<Funnel size={16} />}
        sx={{ flexShrink: 0 }}
        variant="outlined"
      >
        {filterCount > 0 ? `Filters (${filterCount})` : 'Filters'}
      </Button>
      <Popover
        anchorEl={filtersAnchor}
        anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        open={Boolean(filtersAnchor)}
        transformOrigin={{ horizontal: 'right', vertical: 'top' }}
        onClose={() => setFiltersAnchor(null)}
        slotProps={{
          paper: { 'aria-label': 'Log filters', role: 'dialog', sx: { maxWidth: 760, mt: 1 } },
        }}
      >
        <Box
          sx={{
            columnGap: 3,
            display: 'grid',
            gridTemplateColumns: { sm: 'repeat(3, minmax(0, 1fr))', xs: '1fr' },
            p: 2,
            rowGap: 2.5,
          }}
        >
          <FacetGroup title="Type">
            {kindFacets.map((facet) => (
              <FacetRow
                key={facet.value}
                count={facet.count}
                label={facet.label}
                control={
                  <Checkbox
                    checked={query.kinds.includes(facet.value as LogKind)}
                    size="small"
                    onChange={() => update({ kinds: toggle(query.kinds, facet.value as LogKind) })}
                  />
                }
              />
            ))}
          </FacetGroup>

          <FacetGroup title="Project">
            {facets.projects.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                Nothing loaded yet
              </Typography>
            ) : null}
            {withSelected(facets.projects, view.projects).map((facet) => (
              <FacetRow
                key={facet.value}
                count={facet.count}
                label={facet.label}
                control={
                  <Checkbox
                    checked={view.projects.includes(facet.value)}
                    size="small"
                    onChange={() =>
                      onViewChange({ ...view, projects: toggle(view.projects, facet.value) })
                    }
                  />
                }
              />
            ))}
          </FacetGroup>

          {/* Radios, not checkboxes: the endpoint takes one environment name, so
              offering two would promise a query it cannot make. */}
          <FacetGroup title="Environment">
            {environmentFacets.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                Nothing loaded yet
              </Typography>
            ) : null}
            <RadioGroup aria-label="Environment" value={query.environment}>
              {environmentFacets.map((facet) => (
              <FacetRow
                key={facet.value}
                count={facet.count}
                label={facet.label}
                control={
                  // `onClick`, not `onChange`: a radio fires no change event when
                  // it is already the selected one, and clicking it again is how
                  // the environment filter is cleared.
                  <Radio
                    checked={query.environment === facet.value}
                    size="small"
                    onClick={() =>
                      update({ environment: query.environment === facet.value ? '' : facet.value })
                    }
                  />
                }
              />
              ))}
            </RadioGroup>
          </FacetGroup>

          <FacetGroup title="Level">
            {levelFacets.map((facet) => (
              <FacetRow
                key={facet.value}
                count={facet.count}
                label={facet.label}
                control={
                  <Checkbox
                    checked={query.levels.includes(facet.value as LogLevel)}
                    size="small"
                    onChange={() =>
                      update({ levels: toggle(query.levels, facet.value as LogLevel) })
                    }
                  />
                }
              />
            ))}
            {/* Not a field match upstream, so a line that declares no level —
                every access log — cannot be returned by any of these. */}
            <Typography variant="caption" color="text.secondary" sx={{ pt: 0.5 }}>
              Matched as text in the line.
            </Typography>
          </FacetGroup>
        </Box>

        <Divider />
        <Stack
          alignItems="center"
          direction="row"
          justifyContent="space-between"
          spacing={2}
          sx={{ px: 2, py: 1.5 }}
        >
          <Typography variant="body2" color="text.secondary">
            {filterCount === 0
              ? 'No filters — showing every workload'
              : 'Counts are of the lines currently loaded.'}
          </Typography>
          <Stack alignItems="center" direction="row" spacing={1}>
            <Button disabled={filterCount === 0} onClick={onReset} size="small">
              Reset
            </Button>
            <Button onClick={() => setFiltersAnchor(null)} size="small" variant="contained">
              Done
            </Button>
          </Stack>
        </Stack>
      </Popover>

      <Divider flexItem orientation="vertical" sx={{ my: 0.5 }} />

      <FormControlLabel
        sx={{ flexShrink: 0, mr: 0 }}
        control={
          <Switch
            checked={live}
            size="small"
            onChange={(event) => onLiveChange(event.target.checked)}
          />
        }
        label={<Typography variant="body2">Live tail</Typography>}
      />

      <Tooltip title="Refresh">
        <span>
          <IconButton aria-label="Refresh" disabled={fetching} onClick={onRefresh} size="small">
            <RefreshCw size={16} />
          </IconButton>
        </span>
      </Tooltip>
    </Paper>
  );
};

export default LogsToolbar;
