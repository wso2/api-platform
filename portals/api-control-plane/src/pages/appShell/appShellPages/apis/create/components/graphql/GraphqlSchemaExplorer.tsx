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
  Box,
  Chip,
  IconButton,
  InputAdornment,
  MenuItem,
  OutlinedInput,
  Select,
  Skeleton,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Download, Search } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { hairline } from '@/theme/receipes';
import {
  formatSdl,
  parseGraphQLSdl,
  summarizeSchema,
  type GraphQLFieldSummary,
  type GraphQLTypeKind,
  type GraphQLTypeSummary,
} from '../../utils/graphqlSchema';

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
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
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

/**
 * One color per named-type kind, same reasoning as `OPERATION_COLOR`: each
 * kind of schema construct gets its own tone instead of every badge reading as
 * the same neutral gray chip. Only 5 distinct tones are available (see
 * `SchemaChipColor`) for 6 kinds, so the two "terminal value" kinds —
 * `ENUM` (a closed set of literal values) and `SCALAR` (a primitive) — share
 * the neutral `default` gray, while the four structural kinds each get their
 * own color.
 */
const KIND_COLOR: Record<GraphQLTypeKind, SchemaChipColor> = {
  ENUM: 'default',
  INPUT_OBJECT: 'info',
  INTERFACE: 'error',
  OBJECT: 'primary',
  SCALAR: 'default',
  UNION: 'success',
};

/**
 * Rows for the empty state's skeleton preview — a hint at the shape a
 * resolved schema takes (labelled kind + a field-length placeholder bar),
 * not a fake schema of its own. Colors reuse `OPERATION_COLOR`/`KIND_COLOR`
 * directly so the preview never drifts out of sync with the real badges.
 */
const SKELETON_ROWS: { color: SchemaChipColor; label: string; width: string }[] = [
  { color: OPERATION_COLOR.query, label: 'QUERY', width: '70%' },
  { color: OPERATION_COLOR.mutation, label: 'MUTATION', width: '55%' },
  { color: KIND_COLOR.OBJECT, label: 'OBJECT', width: '40%' },
];

const TypeRow = ({ type }: { type: GraphQLTypeSummary }) => {
  const intl = useIntl();
  const fieldCount = type.fields?.length ?? type.enumValues?.length ?? type.unionMembers?.length ?? 0;

  return (
    <Accordion disableGutters sx={(theme) => ({ border: hairline(theme), borderColor: 'divider' })}>
      <AccordionSummary expandIcon={<ChevronDown size={18} />}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          <Chip
            color={KIND_COLOR[type.kind]}
            label={KIND_LABELS[type.kind]}
            size="small"
            sx={{ fontFamily: 'monospace', fontWeight: 700, minWidth: 74 }}
          />
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
export const GraphqlSchemaExplorer = ({ sdl, sourceDescription }: GraphqlSchemaExplorerProps) => {
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
        height: '100%',
        minHeight: 0,
        minWidth: 0,
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
        {sdl === undefined ? (
          <Stack
            sx={(theme) => ({
              alignItems: 'center',
              border: hairline(theme),
              borderColor: 'divider',
              borderRadius: 2,
              height: '100%',
              justifyContent: 'center',
              px: 3,
              textAlign: 'center',
            })}
          >
            <Stack spacing={1.5} sx={{ mb: 3, width: '100%', maxWidth: 280 }}>
              {SKELETON_ROWS.map((row, index) => (
                <Stack
                  direction="row"
                  key={row.label}
                  spacing={1.5}
                  sx={{ alignItems: 'center', opacity: 1 - index * 0.3 }}
                >
                  <Chip
                    color={row.color}
                    label={row.label}
                    size="small"
                    sx={{ flexShrink: 0, fontFamily: 'monospace', fontWeight: 700 }}
                  />
                  <Skeleton
                    height={20}
                    sx={{ borderRadius: 1 }}
                    variant="rectangular"
                    width={row.width}
                  />
                </Stack>
              ))}
            </Stack>
            <Typography sx={{ fontWeight: 700 }} variant="body1">
              <FormattedMessage {...messages.emptyTitle} />
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
              <FormattedMessage {...messages.emptyBody} />
            </Typography>
          </Stack>
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
              overflow: 'auto',
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
              title="Query"
            />
            <OperationSection
              color={OPERATION_COLOR.mutation}
              fields={summary?.mutationFields.filter((field) => matches(field.name)) ?? []}
              hint={<FormattedMessage {...messages.mutationHint} />}
              title="Mutation"
            />
            <OperationSection
              color={OPERATION_COLOR.subscription}
              fields={summary?.subscriptionFields.filter((field) => matches(field.name)) ?? []}
              hint={<FormattedMessage {...messages.subscriptionHint} />}
              title="Subscription"
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
