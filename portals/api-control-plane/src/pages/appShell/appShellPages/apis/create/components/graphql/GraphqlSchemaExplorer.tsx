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
  Alert,
  Box,
  ButtonBase,
  Chip,
  Collapse,
  Divider,
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
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Download, Search } from '@wso2/oxygen-ui-icons-react';
import { useId, useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { methodPalette, SwaggerResourceRow } from '@/components/SwaggerOperationsView';
import {
  ResourcePreviewPlaceholder,
  type PlaceholderRow,
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
  valueCount: {
    id: 'api.create.graphql.schemaExplorer.type.valueCount',
    defaultMessage: '{count, plural, one {# value} other {# values}}',
    description: 'Number of values an ENUM type declares.',
  },
  memberCount: {
    id: 'api.create.graphql.schemaExplorer.type.memberCount',
    defaultMessage: '{count, plural, one {# member} other {# members}}',
    description: 'Number of member types a UNION type declares.',
  },
  noMatches: {
    id: 'api.create.graphql.schemaExplorer.search.noMatches',
    defaultMessage: 'Nothing in this schema matches "{query}".',
    description: 'Shown when the schema search box matches no operation, type, field, enum value or union member.',
  },
  invalidSdl: {
    id: 'api.create.graphql.schemaExplorer.invalidSdl',
    defaultMessage: 'This schema could not be parsed: {reason}',
  },
  sdlErrorWithLocation: {
    id: 'api.create.graphql.schemaExplorer.resolutionFailed.sdlErrorWithLocation',
    defaultMessage: 'Line {line}, column {column}: {message}',
    description:
      '{line}/{column} are 1-based positions in the SDL the user submitted; {message} is the parser\'s own error text.',
  },
  arguments: {
    id: 'api.create.graphql.schemaExplorer.operation.arguments',
    defaultMessage: 'Arguments',
    description: 'Heading over the list of arguments a GraphQL query, mutation or subscription accepts.',
  },
  noArguments: {
    id: 'api.create.graphql.schemaExplorer.operation.noArguments',
    defaultMessage: 'No arguments',
    description: 'Shown under the Arguments heading when a GraphQL operation takes no arguments.',
  },
  returns: {
    id: 'api.create.graphql.schemaExplorer.operation.returns',
    defaultMessage: 'Returns',
    description: 'Heading over the type a GraphQL query, mutation or subscription returns.',
  },
  sdlView: {
    id: 'api.create.graphql.schemaExplorer.view.sdl',
    defaultMessage: 'SDL',
  },
  search: {
    id: 'api.create.graphql.schemaExplorer.search.placeholder',
    defaultMessage: 'Search types and fields',
  },
  title: {
    id: 'api.create.graphql.schemaExplorer.title',
    defaultMessage: 'Schema',
  },
  mutationCount: {
    id: 'api.create.graphql.schemaExplorer.mutations.count',
    defaultMessage: '{count, plural, one {# mutation} other {# mutations}}',
    description: 'Subtitle of the collapsible Mutations group: how many mutation fields it lists.',
  },
  mutationsTitle: {
    id: 'api.create.graphql.schemaExplorer.mutations.title',
    defaultMessage: 'Mutations',
    description: 'Heading of the collapsible group listing a GraphQL schema\'s mutation fields.',
  },
  queryCount: {
    id: 'api.create.graphql.schemaExplorer.queries.count',
    defaultMessage: '{count, plural, one {# query} other {# queries}}',
    description: 'Subtitle of the collapsible Queries group: how many query fields it lists.',
  },
  queriesTitle: {
    id: 'api.create.graphql.schemaExplorer.queries.title',
    defaultMessage: 'Queries',
    description: 'Heading of the collapsible group listing a GraphQL schema\'s query fields.',
  },
  subscriptionCount: {
    id: 'api.create.graphql.schemaExplorer.subscriptions.count',
    defaultMessage: '{count, plural, one {# subscription} other {# subscriptions}}',
    description: 'Subtitle of the collapsible Subscriptions group: how many subscription fields it lists.',
  },
  subscriptionsTitle: {
    id: 'api.create.graphql.schemaExplorer.subscriptions.title',
    defaultMessage: 'Subscriptions',
    description: 'Heading of the collapsible group listing a GraphQL schema\'s subscription fields.',
  },
  typeCount: {
    id: 'api.create.graphql.schemaExplorer.types.count',
    defaultMessage: '{count, plural, one {# type} other {# types}}',
    description: 'Subtitle of the collapsible Types group: how many named types it lists.',
  },
  typesTitle: {
    id: 'api.create.graphql.schemaExplorer.types.title',
    defaultMessage: 'Types',
    description: 'Heading of the collapsible group listing every non-root type in a GraphQL schema.',
  },
});

/** One field of a non-root type, shown inside that type's expanded row. */
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

/** The three GraphQL root operation kinds, labelled as their row badge reads. */
type OperationKind = 'MUTATION' | 'QUERY' | 'SUBSCRIPTION';

/** A details heading inside an expanded operation row. */
const DetailHeading = ({ children }: { children: ReactNode }) => (
  <Typography color="text.secondary" sx={{ fontWeight: 600 }} variant="caption">
    {children}
  </Typography>
);

/** An expanded operation row's body: its arguments, then the type it returns. */
const OperationDetails = ({ field }: { field: GraphQLFieldSummary }) => (
  <Stack spacing={1.5}>
    <Stack spacing={0.5}>
      <DetailHeading>
        <FormattedMessage {...messages.arguments} />
      </DetailHeading>
      {field.arguments.length === 0 ? (
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.noArguments} />
        </Typography>
      ) : (
        field.arguments.map((arg) => (
          <Typography component="div" key={arg.name} sx={{ fontFamily: 'monospace' }} variant="body2">
            <Box component="span" sx={{ fontWeight: 600 }}>
              {arg.name}
            </Box>
            <Box component="span" sx={{ color: 'text.disabled' }}>
              {': '}
            </Box>
            <Box component="span" sx={{ color: 'info.main' }}>
              {arg.type}
            </Box>
            {arg.defaultValue !== undefined ? (
              <Box component="span" sx={{ color: 'text.secondary' }}>
                {` = ${arg.defaultValue}`}
              </Box>
            ) : null}
          </Typography>
        ))
      )}
    </Stack>
    <Stack spacing={0.5}>
      <DetailHeading>
        <FormattedMessage {...messages.returns} />
      </DetailHeading>
      <Typography color="info.main" sx={{ fontFamily: 'monospace' }} variant="body2">
        {field.type}
      </Typography>
    </Stack>
  </Stack>
);

/**
 * One row per root operation field, drawn exactly like a REST resource row
 * (`SwaggerResourceRow`): a solid QUERY/MUTATION/SUBSCRIPTION badge and the
 * field name on a row tinted in the badge's colour, expanding to show the
 * field's arguments and return type — the layout the Bijira console uses.
 */
const OperationRows = ({ fields, kind }: { fields: GraphQLFieldSummary[]; kind: OperationKind }) => (
  <>
    {fields.map((field) => (
      <SwaggerResourceRow
        badge={
          field.deprecated ? (
            <Chip label={<FormattedMessage {...messages.deprecated} />} size="small" variant="outlined" />
          ) : null
        }
        description={field.description}
        key={field.name}
        method={kind}
        path={field.name}
        pathVariant="text"
      >
        <OperationDetails field={field} />
      </SwaggerResourceRow>
    ))}
  </>
);

/**
 * Rows for the empty state's mock listing — a hint at the shape a resolved
 * schema takes, styled like REST's own `ResourcePreviewPlaceholder` empty
 * state. Tones come from the same `methodPalette` the real operation rows
 * use, so the preview never drifts out of sync with them.
 */
const SCHEMA_PLACEHOLDER_ROWS: PlaceholderRow[] = [
  { label: 'QUERY', tone: () => methodPalette('QUERY') },
  { label: 'MUTATION', tone: () => methodPalette('MUTATION') },
  { ghost: true, label: 'SUBSCRIPTION', tone: () => methodPalette('SUBSCRIPTION') },
];

/**
 * The type's member rows (fields, enum values or union members) and the
 * matching count label — only one of the three is ever present on a type.
 */
const typeCountLabel = (type: GraphQLTypeSummary) => {
  if (type.enumValues) return { count: type.enumValues.length, message: messages.valueCount };
  if (type.unionMembers) return { count: type.unionMembers.length, message: messages.memberCount };
  if (type.fields) return { count: type.fields.length, message: messages.fieldCount };
  return undefined;
};

const typeMemberNames = (type: GraphQLTypeSummary): string[] => [
  ...(type.fields?.map((field) => field.name) ?? []),
  ...(type.enumValues ?? []),
  ...(type.unionMembers ?? []),
];

/**
 * A type's badge label — the SDL keyword it is declared with (`type Country`,
 * `enum Status`, …), uppercased like the QUERY/MUTATION badges. Not
 * translated: these are SDL keywords. Also the kind filter's option labels.
 */
const TYPE_KEYWORDS: Record<GraphQLTypeKind, string> = {
  ENUM: 'ENUM',
  INPUT_OBJECT: 'INPUT',
  INTERFACE: 'INTERFACE',
  OBJECT: 'TYPE',
  SCALAR: 'SCALAR',
  UNION: 'UNION',
};

/**
 * One named type, drawn exactly like an operation row (`SwaggerResourceRow`):
 * a solid kind badge and the type name on a row tinted in the badge's colour,
 * its member count as the summary line, expanding to list its members.
 */
const TypeRow = ({ defaultExpanded, type }: { defaultExpanded: boolean; type: GraphQLTypeSummary }) => {
  const intl = useIntl();
  const countLabel = typeCountLabel(type);

  // A scalar declares no members: no count to show and nothing to expand, so
  // it renders as a flat row (SwaggerResourceRow is expandable only with children).
  if (!countLabel) {
    return <SwaggerResourceRow method={TYPE_KEYWORDS[type.kind]} path={type.name} pathVariant="text" />;
  }

  return (
    <SwaggerResourceRow
      defaultExpanded={defaultExpanded}
      description={intl.formatMessage(countLabel.message, { count: countLabel.count })}
      method={TYPE_KEYWORDS[type.kind]}
      path={type.name}
      pathVariant="text"
    >
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
    </SwaggerResourceRow>
  );
};

/**
 * One collapsible section of the explorer — Queries, Mutations,
 * Subscriptions or Types — as the Bijira console groups a schema: a header
 * with the section's name and how many rows it holds, over its rows. Forced
 * open while a search is active, so a collapsed group never hides a match.
 * Renders nothing when it has no rows (an absent root type, or a search that
 * filtered every row out).
 */
const SchemaGroup = ({
  children,
  count,
  countMessage,
  forceOpen,
  title,
}: {
  children: ReactNode;
  count: number;
  countMessage: MessageDescriptor;
  forceOpen: boolean;
  title: MessageDescriptor;
}) => {
  const bodyId = useId();
  const [expanded, setExpanded] = useState(true);
  const open = expanded || forceOpen;

  if (count === 0) return null;

  return (
    <Box
      sx={(theme) => ({
        border: hairline(theme),
        borderColor: 'divider',
        borderRadius: 1,
        minWidth: 0,
        overflow: 'hidden',
      })}
    >
      <ButtonBase
        aria-controls={bodyId}
        aria-expanded={open}
        disabled={forceOpen}
        onClick={() => setExpanded((current) => !current)}
        sx={{
          alignItems: 'center',
          bgcolor: 'action.hover',
          display: 'flex',
          gap: 1.5,
          justifyContent: 'flex-start',
          px: 1.5,
          py: 1.25,
          textAlign: 'left',
          width: '100%',
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
            <FormattedMessage {...title} />
          </Typography>
          <Typography color="text.secondary" variant="caption">
            <FormattedMessage {...countMessage} values={{ count }} />
          </Typography>
        </Box>
        {open ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
      </ButtonBase>
      <Collapse in={open} timeout="auto">
        <Stack
          id={bodyId}
          spacing={1}
          sx={(theme) => ({ borderTop: hairline(theme), borderColor: 'divider', p: 1.5 })}
        >
          {children}
        </Stack>
      </Collapse>
    </Box>
  );
};

export type GraphqlSchemaExplorerProps = {
  /** Resolved SDL text; `undefined` before anything has loaded. */
  sdl?: string;
  /** e.g. "Imported from https://…" / "Fetched by introspection from https://…". */
  sourceDescription?: string;
  /**
   * The source step's last validation failure, if any. Its SDL errors (with
   * line/column) are shown here instead of the "Schema will show here" empty
   * state; a failure without them leaves the empty state in place, since the
   * source form already reports it.
   */
  error?: GraphqlResolutionFailure | null;
  /**
   * `pane` (default): the creation wizard's fixed-height right-hand pane.
   * `card`: the body of the API overview's Schema card, laid out like REST's
   * Resources card — a "Schema" header strip carrying the Explorer/SDL toggle,
   * a divider, then the content, growing to a capped height.
   */
  variant?: 'card' | 'pane';
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
export const GraphqlSchemaExplorer = ({
  error,
  sdl,
  sourceDescription,
  variant = 'pane',
}: GraphqlSchemaExplorerProps) => {
  const intl = useIntl();
  const [view, setView] = useState<'explorer' | 'sdl'>('explorer');
  const [search, setSearch] = useState('');
  const [kindFilter, setKindFilter] = useState<GraphQLTypeKind | 'all'>('all');

  // Nothing loaded and nothing attempted yet — only the placeholder shows.
  // Only the parser's own line/column errors are worth this pane: a failure
  // without them carries just the backend's fixed, generic message, which the
  // source form already states in its own words right next to the field.
  const sdlErrors = error?.sdlErrors && error.sdlErrors.length > 0 ? error.sdlErrors : undefined;
  const isEmpty = sdl === undefined && !sdlErrors;
  const isCard = variant === 'card';
  // The card's header strip is the section's own title, so it always shows;
  // the pane drops its heading over the empty state, where the placeholder
  // already says what will show here.
  const showHeader = isCard || !isEmpty;

  const parsed = useMemo(() => (sdl === undefined ? undefined : parseGraphQLSdl(sdl)), [sdl]);
  const schema = parsed && 'schema' in parsed ? parsed.schema : undefined;
  const summary = useMemo(() => (schema ? summarizeSchema(schema) : undefined), [schema]);

  const query = search.trim().toLowerCase();
  const matches = (name: string) => query === '' || name.toLowerCase().includes(query);

  // A type surfaces if its own name matches, or any of its fields, enum values
  // or union members does — the search box's placeholder promises "types and
  // fields", and a field of a non-root type (Book.isbn), an enum's values
  // (ACTIVE, INACTIVE, ...) or a union's members are what a reader actually
  // searches for, not just the containing type's own name. A type surfaced
  // only by one of its members is rendered expanded, so the match is visible.
  const filteredTypes = useMemo(() => {
    if (!summary) return [];
    return summary.types
      .filter((type) => kindFilter === 'all' || type.kind === kindFilter)
      .map((type) => ({
        type,
        nameMatch: matches(type.name),
        memberMatch: query !== '' && typeMemberNames(type).some(matches),
      }))
      .filter(({ memberMatch, nameMatch }) => nameMatch || memberMatch);
  }, [kindFilter, query, summary]);

  const queryFields = summary?.queryFields.filter((field) => matches(field.name)) ?? [];
  const mutationFields = summary?.mutationFields.filter((field) => matches(field.name)) ?? [];
  const subscriptionFields = summary?.subscriptionFields.filter((field) => matches(field.name)) ?? [];
  const nothingMatches =
    query !== '' &&
    filteredTypes.length === 0 &&
    queryFields.length === 0 &&
    mutationFields.length === 0 &&
    subscriptionFields.length === 0;

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
        height: isCard ? undefined : PANE_HEIGHT,
        minHeight: 0,
        minWidth: 0,
        overflow: 'hidden',
        width: '100%',
      }}
    >
      {showHeader ? (
        <Stack
          direction="row"
          spacing={1.5}
          sx={{
            alignItems: 'center',
            flexShrink: 0,
            flexWrap: 'wrap',
            rowGap: 1,
            ...(isCard && { minHeight: 32, px: 2, py: 1.5 }),
          }}
        >
          {isCard ? (
            // Same heading as REST's Resources card (`ResourcesPanel`).
            <Typography noWrap sx={{ fontWeight: 600 }} variant="h6">
              <FormattedMessage {...messages.title} />
            </Typography>
          ) : (
            <Typography noWrap sx={{ fontWeight: 700 }} variant="subtitle1">
              <FormattedMessage {...messages.title} />
            </Typography>
          )}
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
      ) : null}
      {isCard ? <Divider /> : null}

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

      <Box
        sx={
          isCard
            ? // REST's Resources card body: padded, scrolling past a capped height.
              { maxHeight: { md: 720, xs: 420 }, minWidth: 0, overflowY: 'auto', px: 2, py: 1.5 }
            : { flex: 1, minHeight: 0, minWidth: 0, mt: showHeader ? 1.5 : 0, overflow: 'auto' }
        }
      >
        {sdl === undefined && sdlErrors ? (
          <Stack spacing={1.5}>
            {sdlErrors.map((issue, index) => (
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
            ))}
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
              // Matches the read-only Monaco viewer `ApiResourcesPreview` uses
              // for REST's own raw-text view exactly (12px / 20px line-height)
              // — this box is a plain <pre>, not Monaco, but should still read
              // as the same code viewer rather than at body-text scale.
              fontSize: 12,
              lineHeight: '20px',
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
                {(Object.keys(TYPE_KEYWORDS) as GraphQLTypeKind[]).map((kind) => (
                  <MenuItem key={kind} value={kind}>
                    {TYPE_KEYWORDS[kind]}
                  </MenuItem>
                ))}
              </Select>
            </Stack>

            <SchemaGroup
              count={queryFields.length}
              countMessage={messages.queryCount}
              forceOpen={query !== ''}
              title={messages.queriesTitle}
            >
              <OperationRows fields={queryFields} kind="QUERY" />
            </SchemaGroup>
            <SchemaGroup
              count={mutationFields.length}
              countMessage={messages.mutationCount}
              forceOpen={query !== ''}
              title={messages.mutationsTitle}
            >
              <OperationRows fields={mutationFields} kind="MUTATION" />
            </SchemaGroup>
            <SchemaGroup
              count={subscriptionFields.length}
              countMessage={messages.subscriptionCount}
              forceOpen={query !== ''}
              title={messages.subscriptionsTitle}
            >
              <OperationRows fields={subscriptionFields} kind="SUBSCRIPTION" />
            </SchemaGroup>
            <SchemaGroup
              count={filteredTypes.length}
              countMessage={messages.typeCount}
              forceOpen={query !== ''}
              title={messages.typesTitle}
            >
              {filteredTypes.map(({ memberMatch, nameMatch, type }) => {
                const autoExpand = memberMatch && !nameMatch;
                // Keyed on autoExpand so a row remounts (and picks up the new
                // defaultExpanded) when a search starts or stops matching
                // only inside it; the reader can still toggle it freely.
                return <TypeRow defaultExpanded={autoExpand} key={`${type.name}:${autoExpand}`} type={type} />;
              })}
            </SchemaGroup>

            {nothingMatches ? (
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.noMatches} values={{ query: search.trim() }} />
              </Typography>
            ) : null}
          </Stack>
        )}
      </Box>
    </Box>
  );
};
