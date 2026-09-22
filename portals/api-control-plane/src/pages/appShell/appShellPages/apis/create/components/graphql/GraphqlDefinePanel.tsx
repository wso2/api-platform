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
  alpha,
  Box,
  Card,
  Divider,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { Compass, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import type { GraphqlCreationWizardDraftState } from '../../types';
import { GraphqlIntrospectionForm } from './GraphqlIntrospectionForm';
import { GraphqlSchemaExplorer } from './GraphqlSchemaExplorer';
import { GraphqlUrlUploadForm } from './GraphqlUrlUploadForm';
import type { GraphqlResolutionFailure, GraphqlResolvedSchema } from './graphqlSourceTypes';

type ApproachKey = 'schema' | 'scratch';

const messages = defineMessages({
  fetchedFromUrl: {
    id: 'api.create.graphql.definePanel.source.fromUrl',
    defaultMessage: 'Imported from {url}',
  },
  fetchedFromFile: {
    id: 'api.create.graphql.definePanel.source.fromFile',
    defaultMessage: 'Imported from {fileName}',
  },
  fetchedFromIntrospection: {
    id: 'api.create.graphql.definePanel.source.fromIntrospection',
    defaultMessage: 'Fetched by introspection from {url}',
  },
  schemaLabel: {
    id: 'api.create.graphql.definePanel.approach.label',
    defaultMessage: 'How do you want to define this GraphQL API?',
  },
  schemaDescription: {
    id: 'api.create.graphql.definePanel.schema.description',
    defaultMessage: 'Import from a URL or upload a schema file.',
  },
  schemaTitle: {
    id: 'api.create.graphql.definePanel.schema.title',
    defaultMessage: 'Start with a schema',
  },
  scratchDescription: {
    id: 'api.create.graphql.definePanel.scratch.description',
    defaultMessage: 'Start blank, or seed the schema from a backend endpoint.',
  },
  scratchTitle: {
    id: 'api.create.graphql.definePanel.scratch.title',
    defaultMessage: 'Design from scratch',
  },
});

type Approach = {
  description: MessageDescriptor;
  icon: ReactNode;
  key: ApproachKey;
  title: MessageDescriptor;
};

const APPROACHES: Approach[] = [
  {
    description: messages.schemaDescription,
    icon: <Compass size={18} />,
    key: 'schema',
    title: messages.schemaTitle,
  },
  {
    description: messages.scratchDescription,
    icon: <Pencil size={18} />,
    key: 'scratch',
    title: messages.scratchTitle,
  },
];

export type GraphqlDefinePanelProps = {
  /** Keeps the wizard footer supplied with the definition currently on screen. */
  onDraftChange: (data: GraphqlCreationWizardDraftState | null) => void;
};

/**
 * A starting guess at the API's name. Unlike REST's `extractApiDetails`,
 * which reads a real `info.title` out of the OpenAPI document, GraphQL SDL
 * and introspection carry no name field at all — so this falls back to the
 * imported file's name, or the endpoint/SDL URL's hostname, humanized. Always
 * just a suggestion: the configure step's own `identifierEdited` guard lets
 * the user override the name (and, in turn, the identifier it derives) by
 * hand, exactly as it already does for a REST import.
 */
const deriveDisplayName = (resolved: GraphqlResolvedSchema): string | undefined => {
  const humanize = (raw: string) =>
    raw
      .replace(/\.(graphql|gql|json)$/i, '')
      .replace(/[-_]+/g, ' ')
      .trim()
      .replace(/\b\w/g, (letter) => letter.toUpperCase());

  if (resolved.sdlFile) return humanize(resolved.sdlFile.name) || undefined;

  const url = resolved.endpointUrl ?? resolved.sdlUrl;
  if (!url) return undefined;
  try {
    const host = new URL(url).hostname.split('.')[0];
    return host ? humanize(host) : undefined;
  } catch {
    return undefined;
  }
};

/**
 * The GraphQL wizard's "how do you want to define this API?" step.
 *
 * Two approaches sit across the top and share one schema explorer: importing
 * a schema (URL/file) or introspecting a backend endpoint both funnel through
 * the same dry-run validation call and land in the same right-hand pane —
 * mirrors `DefineApiPanel`'s contract-vs-scratch split. Back and Next belong
 * to the wizard's shared footer, not this panel.
 */
export const GraphqlDefinePanel = ({ onDraftChange }: GraphqlDefinePanelProps) => {
  const intl = useIntl();
  const [approach, setApproach] = useState<ApproachKey>('schema');
  const [resolved, setResolved] = useState<GraphqlResolvedSchema | null>(null);
  const [failure, setFailure] = useState<GraphqlResolutionFailure | null>(null);

  const handleApproachChange = (next: ApproachKey | null) => {
    if (next === null) return;
    setApproach(next);
    setResolved(null);
    setFailure(null);
  };

  const sourceDescription = (() => {
    if (!resolved) return undefined;
    if (resolved.schemaSource === 'introspection' && resolved.endpointUrl) {
      return intl.formatMessage(messages.fetchedFromIntrospection, { url: resolved.endpointUrl });
    }
    if (resolved.schemaSource === 'url' && resolved.sdlUrl) {
      return intl.formatMessage(messages.fetchedFromUrl, { url: resolved.sdlUrl });
    }
    if (resolved.schemaSource === 'file' && resolved.sdlFile) {
      return intl.formatMessage(messages.fetchedFromFile, { fileName: resolved.sdlFile.name });
    }
    return undefined;
  })();

  useEffect(() => {
    // Fields absent from `resolved` (e.g. `endpointUrl` for a URL/file source)
    // are omitted here rather than sent as an explicit `undefined` — the
    // configure form fills its defaults in via `{...DEFAULT, ...draft}`, and
    // an own property set to `undefined` would clobber that default instead
    // of falling through to it.
    const displayName = resolved === null ? undefined : deriveDisplayName(resolved);

    onDraftChange(
      resolved === null
        ? null
        : {
            schemaSource: resolved.schemaSource,
            sdl: resolved.sdl,
            ...(resolved.sdlUrl === undefined ? {} : { sdlUrl: resolved.sdlUrl }),
            ...(resolved.sdlFile === undefined ? {} : { sdlFile: resolved.sdlFile }),
            ...(resolved.endpointUrl === undefined ? {} : { endpointUrl: resolved.endpointUrl }),
            ...(displayName === undefined ? {} : { displayName }),
          },
    );
    return () => onDraftChange(null);
  }, [onDraftChange, resolved]);

  return (
    <Stack spacing={3}>
      {/* One surface for the whole step: the two approaches sit flush on top of
          the panels they open, like tabs on their own body, rather than
          floating above as separate cards — mirrors `DefineApiPanel`'s own
          selected-approach border treatment. */}
      <Card sx={{ border: 0, overflow: 'visible' }} variant="outlined">
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.schemaLabel)}
          exclusive
          fullWidth
          onChange={(_event, next: ApproachKey | null) => handleApproachChange(next)}
          sx={(theme) => ({
            p: 0,
            '& .MuiToggleButtonGroup-grouped': {
              border: `1px solid ${alpha(theme.palette.text.primary, 0.32)}`,
              borderBottom: 0,
              borderRadius: `${theme.shape.borderRadius}px ${theme.shape.borderRadius}px 0 0`,
              justifyContent: 'flex-start',
              p: 2,
              textTransform: 'none',
              '&:not(:first-of-type)': {
                borderLeft: `1px solid ${alpha(theme.palette.text.primary, 0.32)}`,
                marginLeft: 0,
              },
              '&.Mui-selected, &.Mui-selected:hover': {
                bgcolor: 'action.selected',
                border: `1px solid ${theme.palette.primary.main}`,
                borderBottom: 0,
                borderRadius: `${theme.shape.borderRadius}px ${theme.shape.borderRadius}px 0 0`,
              },
            },
          })}
          value={approach}
        >
          {APPROACHES.map((candidate) => {
            const selected = candidate.key === approach;

            return (
              <ToggleButton key={candidate.key} value={candidate.key}>
                <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', width: '100%' }}>
                  <Box
                    sx={{
                      alignItems: 'center',
                      bgcolor: selected ? 'primary.main' : 'action.hover',
                      borderRadius: 1,
                      color: selected ? 'primary.contrastText' : 'text.secondary',
                      display: 'flex',
                      flexShrink: 0,
                      height: 34,
                      justifyContent: 'center',
                      width: 34,
                    }}
                  >
                    {candidate.icon}
                  </Box>
                  <Stack spacing={0.25} sx={{ minWidth: 0, textAlign: 'left' }}>
                    <Typography color="text.primary" sx={{ fontWeight: 700 }} variant="body1">
                      <FormattedMessage {...candidate.title} />
                    </Typography>
                    <Typography color="text.secondary" variant="body2">
                      <FormattedMessage {...candidate.description} />
                    </Typography>
                  </Stack>
                </Stack>
              </ToggleButton>
            );
          })}
        </ToggleButtonGroup>

        <Stack
          direction={{ lg: 'row', xs: 'column' }}
          divider={
            <Divider
              flexItem
              orientation="vertical"
              sx={{
                borderBottomWidth: { lg: 0, xs: 'thin' },
                borderRightWidth: { lg: 'thin', xs: 0 },
              }}
            />
          }
          sx={(theme) => ({
            border: 1,
            borderColor: 'primary.main',
            borderRadius: `0 0 ${theme.shape.borderRadius}px ${theme.shape.borderRadius}px`,
            borderTop: 0,
            position: 'relative',
            // Covers the shared top border seam with the selected column's own
            // color, so the primary-colored outline reads as wrapping only
            // that column rather than the whole row.
            '&::before': {
              bgcolor: 'primary.main',
              content: '""',
              height: '1px',
              left: approach === 'schema' ? '50%' : 0,
              position: 'absolute',
              top: 0,
              width: '50%',
            },
          })}
        >
          <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
            {approach === 'schema' ? (
              <GraphqlUrlUploadForm onResolved={setResolved} onValidationFailed={setFailure} />
            ) : (
              <GraphqlIntrospectionForm onResolved={setResolved} onValidationFailed={setFailure} />
            )}
          </Box>

          <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
            <GraphqlSchemaExplorer
              error={failure}
              sdl={resolved?.sdl}
              sourceDescription={sourceDescription}
            />
          </Box>
        </Stack>
      </Card>
    </Stack>
  );
};
