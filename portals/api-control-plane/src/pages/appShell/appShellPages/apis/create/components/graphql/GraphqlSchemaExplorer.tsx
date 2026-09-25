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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  alpha,
  Box,
  Chip,
  IconButton,
  InputAdornment,
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
  type Theme,
} from '@wso2/oxygen-ui';
import { ChevronDown, Download, Search } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  ResourcePreviewPlaceholder,
  type PlaceholderRow,
  type PlaceholderRowTone,
} from '@/pages/appShell/appShellPages/apis/components/ResourcePreviewPlaceholder';
import { hairline } from '@/theme/receipes';
import { PANE_HEIGHT } from '../ApiResourcesPreview';
import {
  formatSdl,
  parseGraphQLSdl,
  summarizeSchema,
  type GraphQLFieldSummary,
  type GraphQLTypeKind,
  type GraphQLTypeSummary,
} from '../../utils/graphqlSchema';
import type { GraphqlResolutionFailure } from './graphqlSourceTypes';

const messages = defineMessages({
  allKinds: {
    id: 'api.create.graphql.schemaExplorer.filter.allKinds',
    defaultMessage: 'All kinds',
  },
  deprecated: {
    id: 'api.create.graphql.schemaExplorer.field.deprecated',
    defaultMessage: 'deprecated',
  },
  download: {
    id: 'api.create.graphql.schemaExplorer.action.download',
    defaultMessage: 'Download SDL',
  },
  emptyBody: {
    id: 'api.create.graphql.schemaExplorer.empty.body',
    defaultMessage: 'Types appear once a schema is fetched or created.',
  },
  emptyTitle: {
    id: 'api.create.graphql.schemaExplorer.empty.title',
    defaultMessage: 'Schema will show here',
  },
  explorerView: {
    id: 'api.create.graphql.schemaExplorer.view.explorer',
    defaultMessage: 'Explorer',
  },
  fieldCount: {
    id: 'api.create.graphql.schemaExplorer.type.fieldCount',
    defaultMessage: '{count, plural, one {# field} other {# fields}}',
  },
  invalidSdl: {
    id: 'api.create.graphql.schemaExplorer.invalidSdl',
    defaultMessage: 'This schema could not be parsed: {reason}',
  },
  resolutionFailedGeneric: {
    id: 'api.create.graphql.schemaExplorer.resolutionFailed.generic',
    defaultMessage: 'Schema could not be resolved.',
  },
  sdlErrorWithLocation: {
    id: 'api.create.graphql.schemaExplorer.resolutionFailed.sdlErrorWithLocation',
    defaultMessage: 'Line {line}, column {column}: {message}',
    description:
      '{line}/{column} are 1-based positions in the SDL the user submitted; {message} is the parser\'s own error text.',
  },
  mutationHint: {
    id: 'api.create.graphql.schemaExplorer.section.mutationHint',
    defaultMessage: 'Entry points that change data',
  },
  queryHint: {
    id: 'api.create.graphql.schemaExplorer.section.queryHint',
    defaultMessage: 'Entry points into the schema',
  },
  sdlView: {
    id: 'api.create.graphql.schemaExplorer.view.sdl',
    defaultMessage: 'SDL',
  },
  search: {
    id: 'api.create.graphql.schemaExplorer.search.placeholder',
    defaultMessage: 'Search types and fields',
  },
  subscriptionHint: {
    id: 'api.create.graphql.schemaExplorer.section.subscriptionHint',
    defaultMessage: 'Entry points that stream updates',
  },
  title: {
    id: 'api.create.graphql.schemaExplorer.title',
    defaultMessage: 'Schema',
  },
  typesInSchema: {
    id: 'api.create.graphql.schemaExplorer.section.typesInSchema',
    defaultMessage: '{count, plural, one {# in this schema} other {# in this schema}}',
  },
});

/** Field row shared by the Query/Mutation/Subscription sections and a type's own fields. */
const FieldRow = ({ field }: { field: GraphQLFieldSummary }) => (
  <Stack
    direction="row"
    spacing={1}
    sx={(theme) => ({
      alignItems: 'center',
      border: hairline(theme),
      borderColor: 'divider',
      borderRadius: 1,
      fontFamily: 'monospace',
      px: 1.5,
      py: 1,
    })}
  >
    <Typography component="span" sx={{ fontWeight: 600 }} variant="body2">
      {field.name}
    </Typography>
    {field.args ? (
      <Typography color="text.secondary" component="span" variant="body2">
        {field.args}
      </Typography>
    ) : null}
    <Typography color="text.disabled" component="span" variant="body2">
      :
    </Typography>
    <Typography color="info.main" component="span" variant="body2">
      {field.type}
    </Typography>
    <Box sx={{ flex: 1 }} />
    {field.deprecated ? (
      <Chip label={<FormattedMessage {...messages.deprecated} />} size="small" variant="outlined" />
    ) : null}
  </Stack>
);

/**
 * Chip colors this file assigns by hand — Oxygen's own `Chip` `color` prop
 * union, minus two tokens that don't actually read as distinct in this theme:
 * `secondary.main` is a near-white gray with no readable contrast text defined
 * for it (a filled `color="secondary"` chip renders as white-on-white), and
 * `warning` (`#ed6c02`, MUI's default) is close enough to this theme's own
 * `primary` orange (`#ff7300`) to look like the same color at a glance. The
 * remaining five — orange, green, red, blue, neutral gray — are the tones
 * that actually look different from each other here.
 */
type SchemaChipColor = 'default' | 'error' | 'info' | 'primary' | 'success';

/**
 * One color per root operation, mirroring how `SwaggerOperationsView` tints
 * REST's GET/POST/PUT/DELETE pills so a GraphQL schema reads with the same
 * at-a-glance distinction: Query (read) lands on `info` like GET, Mutation
 * (write) on `success` like POST, and Subscription — no REST equivalent —
 * gets `error` as its own clearly distinct third tone.
 */
const OPERATION_COLOR: Record<'mutation' | 'query' | 'subscription', SchemaChipColor> = {
  mutation: 'success',
  query: 'info',
  subscription: 'error',
};

const OperationSection = ({
  color,
  fields,
  hint,
  title,
}: {
  color: SchemaChipColor;
  fields: GraphQLFieldSummary[];
  hint: ReactNode;
  title: string;
}) => {
  if (fields.length === 0) return null;

  return (
    <Stack spacing={0.75}>
      <Stack
        direction="row"
        spacing={1.5}
        sx={(theme) => {
          const tone = paletteTone(theme, color);

          return {
            alignItems: 'center',
            bgcolor: tone.bg,
            border: hairline(theme),
            borderColor: tone.border,
            borderRadius: 0.75,
            px: 1.35,
            py: 0.9,
          };
        }}
      >
        <Chip
          color={color}
          label={title}
          size="small"
          sx={{ fontFamily: 'monospace', fontWeight: 700 }}
        />
        <Typography color="text.secondary" variant="caption">
          {hint}
        </Typography>
      </Stack>
      {fields.map((field) => (
        <FieldRow field={field} key={field.name} />
      ))}
    </Stack>
  );
};

const KIND_LABELS: Record<GraphQLTypeKind, string> = {
  ENUM: 'ENUM',
  INPUT_OBJECT: 'INPUT',
  INTERFACE: 'INTERFACE',
  OBJECT: 'OBJECT',
  SCALAR: 'SCALAR',
  UNION: 'UNION',
};

/** A `SchemaChipColor` resolved against the theme, for `ResourcePreviewPlaceholder`'s row tones. */
const paletteTone = (theme: Theme, color: SchemaChipColor): PlaceholderRowTone => {
  const hex = color === 'default' ? theme.palette.text.secondary : theme.palette[color].main;
  return { badge: hex, bg: alpha(hex, 0.14), border: hex };
};

/**
 * Rows for the empty state's mock listing — a hint at the shape a resolved
 * schema takes (an operation badge + two placeholder bars), styled like
 * REST's own `ResourcePreviewPlaceholder` empty state so both creation
 * wizards' source steps read as the same control. Colors reuse
 * `OPERATION_COLOR` so the preview never drifts out of sync with the real
 * badges — only the three root operations get a badge here, matching the
 * explorer now that individual types no longer carry their own kind chip.
 */
const SCHEMA_PLACEHOLDER_ROWS: PlaceholderRow[] = [
  { label: 'QUERY', tone: (theme) => paletteTone(theme, OPERATION_COLOR.query) },
  { label: 'MUTATION', tone: (theme) => paletteTone(theme, OPERATION_COLOR.mutation) },
  { ghost: true, label: 'SUBSCRIPTION', tone: (theme) => paletteTone(theme, OPERATION_COLOR.subscription) },
];

const TypeRow = ({ type }: { type: GraphQLTypeSummary }) => {
  const intl = useIntl();
  const fieldCount = type.fields?.length ?? type.enumValues?.length ?? type.unionMembers?.length ?? 0;

  return (
    <Accordion disableGutters sx={(theme) => ({ border: hairline(theme), borderColor: 'divider' })}>
      <AccordionSummary expandIcon={<ChevronDown size={18} />}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          {/* No chip here — only the root Query/Mutation/Subscription
              operations carry a colored chip; a type's kind is plain,
              muted text (still available to filter on via the Kind
              select above). */}
          <Typography
            color="text.secondary"
            sx={{ flexShrink: 0, fontFamily: 'monospace', fontWeight: 700, minWidth: 74 }}
            variant="caption"
          >
            {KIND_LABELS[type.kind]}
          </Typography>
          <Typography sx={{ fontFamily: 'monospace', fontWeight: 700 }} variant="body2">
            {type.name}
          </Typography>
          <Typography color="text.secondary" variant="caption">
            {intl.formatMessage(messages.fieldCount, { count: fieldCount })}
          </Typography>
        </Stack>
      </AccordionSummary>
      <AccordionDetails>
        <Stack spacing={0.75}>
          {type.fields?.map((field) => <FieldRow field={field} key={field.name} />)}
          {type.enumValues?.map((value) => (
            <Typography key={value} sx={{ fontFamily: 'monospace' }} variant="body2">
              {value}
            </Typography>
          ))}
          {type.unionMembers?.map((member) => (
            <Typography key={member} sx={{ fontFamily: 'monospace' }} variant="body2">
              {member}
            </Typography>
          ))}
        </Stack>
      </AccordionDetails>
    </Accordion>
  );
};

export type GraphqlSchemaExplorerProps = {
  /** Resolved SDL text; `undefined` before anything has loaded. */
  sdl?: string;
  /** e.g. "Imported from https://…" / "Fetched by introspection from https://…". */
  sourceDescription?: string;
  /**
   * The source step's last validation failure, if any — shown here instead
   * of the generic "Schema will show here" empty state once a check has
   * actually been attempted and failed.
   */
  error?: GraphqlResolutionFailure | null;
};

/**
 * Right-hand pane of the GraphQL wizard's source step: an Explorer view
 * (Query/Mutation/Subscription entry points, then every other named type as an
 * expandable row) and an SDL view of the same schema, or an empty state before
 * anything has loaded.
 *
 * Presentational only — no data fetching of its own — so it is reusable
 * wherever else a resolved SDL needs to be shown.
 */
export const GraphqlSchemaExplorer = ({ error, sdl, sourceDescription }: GraphqlSchemaExplorerProps) => {
  const intl = useIntl();
  const [view, setView] = useState<'explorer' | 'sdl'>('explorer');
  const [search, setSearch] = useState('');
  const [kindFilter, setKindFilter] = useState<GraphQLTypeKind | 'all'>('all');

  const parsed = useMemo(() => (sdl === undefined ? undefined : parseGraphQLSdl(sdl)), [sdl]);
  const schema = parsed && 'schema' in parsed ? parsed.schema : undefined;
  const summary = useMemo(() => (schema ? summarizeSchema(schema) : undefined), [schema]);

  const query = search.trim().toLowerCase();
  const matches = (name: string) => query === '' || name.toLowerCase().includes(query);

  const filteredTypes = useMemo(() => {
    if (!summary) return [];
    return summary.types.filter(
      (type) => (kindFilter === 'all' || type.kind === kindFilter) && matches(type.name),
    );
  }, [kindFilter, query, summary]);

  const downloadSdl = () => {
    if (sdl === undefined) return;
    const blob = new Blob([sdl], { type: 'application/graphql' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'schema.graphql';
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: PANE_HEIGHT,
        minHeight: 0,
        minWidth: 0,
        overflow: 'hidden',
        width: '100%',
      }}
    >
      <Stack
        direction="row"
        spacing={1.5}
        sx={{ alignItems: 'center', flexShrink: 0, flexWrap: 'wrap', rowGap: 1 }}
      >
        <Typography noWrap sx={{ fontWeight: 700 }} variant="subtitle1">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Box sx={{ flex: 1, minWidth: 0 }} />
        {sdl !== undefined ? (
          <Stack
            direction="row"
            spacing={1.5}
            sx={{ alignItems: 'center', flexShrink: 0 }}
          >
            <ToggleButtonGroup
              exclusive
              onChange={(_event, next: 'explorer' | 'sdl' | null) => {
                if (next !== null) setView(next);
              }}
              size="small"
              value={view}
            >
              <ToggleButton sx={{ textTransform: 'none' }} value="explorer">
                <FormattedMessage {...messages.explorerView} />
              </ToggleButton>
              <ToggleButton sx={{ textTransform: 'none' }} value="sdl">
                <FormattedMessage {...messages.sdlView} />
              </ToggleButton>
            </ToggleButtonGroup>
            {view === 'sdl' ? (
              <Tooltip title={intl.formatMessage(messages.download)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.download)}
                  onClick={downloadSdl}
                  size="small"
                >
                  <Download size={18} />
                </IconButton>
              </Tooltip>
            ) : null}
          </Stack>
        ) : null}
      </Stack>

      {sourceDescription ? (
        <Typography
          color="text.secondary"
          noWrap
          sx={{ flexShrink: 0, mt: 0.25 }}
          title={sourceDescription}
          variant="caption"
        >
          {sourceDescription}
        </Typography>
      ) : null}

      <Box sx={{ flex: 1, minHeight: 0, minWidth: 0, mt: 1.5, overflow: 'auto' }}>
        {sdl === undefined && error ? (
          <Stack spacing={1.5}>
            {error.sdlErrors && error.sdlErrors.length > 0 ? (
              error.sdlErrors.map((issue, index) => (
                <Alert key={index} severity="error">
                  {issue.line !== undefined && issue.column !== undefined ? (
                    <FormattedMessage
                      {...messages.sdlErrorWithLocation}
                      values={{ column: issue.column, line: issue.line, message: issue.message }}
                    />
                  ) : (
                    issue.message
                  )}
                </Alert>
              ))
            ) : (
              <Alert severity="error">
                {error.message ?? intl.formatMessage(messages.resolutionFailedGeneric)}
              </Alert>
            )}
          </Stack>
        ) : sdl === undefined ? (
          <ResourcePreviewPlaceholder
            description={intl.formatMessage(messages.emptyBody)}
            rows={SCHEMA_PLACEHOLDER_ROWS}
            title={intl.formatMessage(messages.emptyTitle)}
          />
        ) : parsed && 'error' in parsed ? (
          <Alert severity="error">
            <FormattedMessage {...messages.invalidSdl} values={{ reason: parsed.error }} />
          </Alert>
        ) : view === 'sdl' ? (
          <Box
            component="pre"
            sx={(theme) => ({
              border: hairline(theme),
              borderColor: 'divider',
              borderRadius: 2,
              fontFamily: 'monospace',
              fontSize: theme.typography.body2.fontSize,
              m: 0,
              maxWidth: '100%',
              // Horizontal only: the surrounding content box (below) is the
              // single vertical scroll owner, the same one-scrollbar contract
              // `ApiResourcesPreview`'s own raw-text view keeps.
              overflowX: 'auto',
              p: 2,
              whiteSpace: 'pre',
            })}
          >
            {schema ? formatSdl(schema) : sdl}
          </Box>
        ) : (
          <Stack spacing={2}>
            <Stack direction="row" spacing={1.5}>
              <OutlinedInput
                fullWidth
                onChange={(event) => setSearch(event.target.value)}
                placeholder={intl.formatMessage(messages.search)}
                size="small"
                startAdornment={
                  <InputAdornment position="start">
                    <Search size={18} />
                  </InputAdornment>
                }
                value={search}
              />
              <Select<GraphQLTypeKind | 'all'>
                onChange={(event) => setKindFilter(event.target.value as GraphQLTypeKind | 'all')}
                size="small"
                sx={{ minWidth: 160 }}
                value={kindFilter}
              >
                <MenuItem value="all">{intl.formatMessage(messages.allKinds)}</MenuItem>
                {(Object.keys(KIND_LABELS) as GraphQLTypeKind[]).map((kind) => (
                  <MenuItem key={kind} value={kind}>
                    {KIND_LABELS[kind]}
                  </MenuItem>
                ))}
              </Select>
            </Stack>

            <OperationSection
              color={OPERATION_COLOR.query}
              fields={summary?.queryFields.filter((field) => matches(field.name)) ?? []}
              hint={<FormattedMessage {...messages.queryHint} />}
              title="QUERY"
            />
            <OperationSection
              color={OPERATION_COLOR.mutation}
              fields={summary?.mutationFields.filter((field) => matches(field.name)) ?? []}
              hint={<FormattedMessage {...messages.mutationHint} />}
              title="MUTATION"
            />
            <OperationSection
              color={OPERATION_COLOR.subscription}
              fields={summary?.subscriptionFields.filter((field) => matches(field.name)) ?? []}
              hint={<FormattedMessage {...messages.subscriptionHint} />}
              title="SUBSCRIPTION"
            />

            {filteredTypes.length > 0 ? (
              <Stack spacing={0.75}>
                <Stack direction="row" spacing={1.5} sx={{ alignItems: 'baseline' }}>
                  <Typography sx={{ fontFamily: 'monospace', fontWeight: 700 }} variant="subtitle2">
                    <FormattedMessage {...messages.typesInSchema} values={{ count: filteredTypes.length }} />
                  </Typography>
                </Stack>
                {filteredTypes.map((type) => (
                  <TypeRow key={type.name} type={type} />
                ))}
              </Stack>
            ) : null}
          </Stack>
        )}
      </Box>
    </Box>
  );
};
