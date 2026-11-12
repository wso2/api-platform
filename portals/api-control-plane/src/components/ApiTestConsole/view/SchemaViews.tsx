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

import { useState } from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Chip,
  Divider,
  Paper,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SegmentedControl } from '@/components/common/SegmentedControl';
import { SecondaryText } from '@/components/common/SecondaryText';
import { methodPalette } from '@/components/SwaggerOperationsView/methodPalette';
import type {
  ConsoleOperation,
  ConsoleResponseDoc,
  ConsoleSchemaDoc,
  ConsoleSchemaField,
} from '../types';
import { CodeView, languageForMediaType } from './CodeView';

const messages = defineMessages({
  example: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.example',
    defaultMessage: 'Example',
    description: 'Toggle option showing an example payload.',
  },
  exampleLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.exampleLabel',
    defaultMessage: 'Example payload',
    description: 'Accessible name for the region showing an example payload.',
  },
  noExample: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.noExample',
    defaultMessage: 'No example is documented for this response.',
  },
  noFields: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.noFields',
    defaultMessage: 'This schema has no fields to list.',
  },
  required: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.required',
    defaultMessage: 'required',
    description: 'Marks a schema field that must be provided. Lower-case adjective.',
  },
  schema: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.schema',
    defaultMessage: 'Schema',
    description:
      'Toggle option showing the structure of a payload, and the heading of the schema page.',
  },
  usedBy: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.usedBy',
    defaultMessage: 'Used by:',
    description: 'Heading above the list of operations that use a schema.',
  },
  viewLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.SchemaViews.viewLabel',
    defaultMessage: 'Example or schema',
    description: 'Accessible label for the toggle between an example and its schema.',
  },
});

/** A schema's fields as a table. Text only: names and descriptions come from the document. */
export function SchemaTable({ fields }: { fields: ConsoleSchemaField[] }) {
  if (fields.length === 0) {
    return (
      <SecondaryText variant="body2">
        <FormattedMessage {...messages.noFields} />
      </SecondaryText>
    );
  }
  return (
    <Paper sx={{ overflow: 'hidden' }} variant="outlined">
      {fields.map((field, index) => (
        <Box key={field.name}>
          {index > 0 && <Divider />}
          <Box
            sx={{
              display: 'grid',
              gap: 1,
              gridTemplateColumns: { sm: 'minmax(0, 1fr) minmax(0, 2fr)', xs: 'minmax(0, 1fr)' },
              px: 2,
              py: 1.5,
            }}
          >
            <Stack alignItems="baseline" direction="row" flexWrap="wrap" gap={1}>
              <Typography
                fontFamily="monospace"
                fontWeight={600}
                variant="body2"
                sx={{ wordBreak: 'break-word' }}
              >
                {field.name}
              </Typography>
              {field.required && (
                <Typography color="warning.main" variant="caption">
                  <FormattedMessage {...messages.required} />
                </Typography>
              )}
            </Stack>
            <Stack spacing={0.25}>
              <Typography
                color="info.main"
                fontFamily="monospace"
                variant="body2"
                sx={{ wordBreak: 'break-word' }}
              >
                {field.type}
              </Typography>
              {field.description !== '' && (
                <SecondaryText variant="body2">{field.description}</SecondaryText>
              )}
            </Stack>
          </Box>
        </Box>
      ))}
    </Paper>
  );
}

export type SchemaPageProps = {
  schema: ConsoleSchemaDoc;
  operations: ConsoleOperation[];
  onOpenOperation: (key: string) => void;
};

/** One schema on its own: its fields, and the operations that use it. */
export function SchemaPage({ schema, operations, onOpenOperation }: SchemaPageProps) {
  const users = operations.filter(
    (operation) =>
      operation.body?.options.some((option) => option.schemaName === schema.name) ||
      operation.responses.some((response) => response.schemaName === schema.name),
  );
  return (
    <Stack spacing={2}>
      <Stack spacing={0.5}>
        <SecondaryText variant="overline">
          <FormattedMessage {...messages.schema} />
        </SecondaryText>
        <Typography component="h2" variant="h5" sx={{ wordBreak: 'break-word' }}>
          {schema.name}
        </Typography>
        {schema.description !== '' && (
          <SecondaryText variant="body2">{schema.description}</SecondaryText>
        )}
      </Stack>
      <SchemaTable fields={schema.fields} />
      {users.length > 0 && (
        <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1}>
          <SecondaryText variant="body2">
            <FormattedMessage {...messages.usedBy} />
          </SecondaryText>
          {users.map((operation) => (
            <Chip
              clickable
              key={operation.key}
              label={
                <Typography
                  component="span"
                  fontFamily="monospace"
                  sx={{ display: 'inline-flex', gap: 0.75 }}
                  variant="caption"
                >
                  <Box
                    component="span"
                    sx={{ color: methodPalette(operation.method).badge, fontWeight: 700 }}
                  >
                    {operation.method}
                  </Box>
                  {operation.path}
                </Typography>
              }
              onClick={() => onOpenOperation(operation.key)}
              size="small"
              variant="outlined"
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
}

const statusColor = (status: string): 'success' | 'warning' | 'error' | 'default' => {
  if (/^2/.test(status)) return 'success';
  if (/^4/.test(status)) return 'warning';
  if (/^5/.test(status)) return 'error';
  return 'default';
};

function ExpectedResponse({ response }: { response: ConsoleResponseDoc }) {
  const intl = useIntl();
  const [view, setView] = useState<'example' | 'schema'>('example');

  return (
    <Accordion disableGutters variant="outlined">
      <AccordionSummary expandIcon={<ChevronDown size={16} />}>
        <Stack
          alignItems="center"
          direction="row"
          spacing={1.5}
          sx={{ minWidth: 0, width: '100%' }}
        >
          <Chip
            color={statusColor(response.status)}
            label={response.status}
            size="small"
            variant="outlined"
          />
          <Typography noWrap variant="body2">
            {response.description}
          </Typography>
          {response.schemaName && (
            <SecondaryText fontFamily="monospace" variant="caption">
              {response.schemaName}
            </SecondaryText>
          )}
        </Stack>
      </AccordionSummary>
      <AccordionDetails>
        <Stack spacing={1.5}>
          <Stack alignItems="center" direction="row" spacing={1.5}>
            <SegmentedControl
              aria-label={intl.formatMessage(messages.viewLabel)}
              onChange={setView}
              options={[
                { label: <FormattedMessage {...messages.example} />, value: 'example' },
                { label: <FormattedMessage {...messages.schema} />, value: 'schema' },
              ]}
              value={view}
            />
            {response.mediaType && (
              <SecondaryText fontFamily="monospace" variant="caption">
                {response.mediaType}
              </SecondaryText>
            )}
          </Stack>
          {view === 'example' ? (
            response.example ? (
              <CodeView
                label={intl.formatMessage(messages.exampleLabel)}
                language={languageForMediaType(response.mediaType)}
                maxHeight={280}
                text={response.example}
              />
            ) : (
              <SecondaryText variant="body2">
                <FormattedMessage {...messages.noExample} />
              </SecondaryText>
            )
          ) : (
            <SchemaTable fields={response.fields} />
          )}
        </Stack>
      </AccordionDetails>
    </Accordion>
  );
}

/** The operation's documented responses, each expandable to an example or its schema. */
export function ExpectedResponses({ responses }: { responses: ConsoleResponseDoc[] }) {
  return (
    <Stack>
      {responses.map((response) => (
        <ExpectedResponse key={response.status} response={response} />
      ))}
    </Stack>
  );
}
