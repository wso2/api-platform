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

import { useMemo, useState } from 'react';
import {
  Badge,
  Box,
  Card,
  CardContent,
  Input,
  MenuItem,
  Select,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { BodyEditor } from './components/BodyEditor';
import { CurlCommandPanel } from './components/CurlCommandPanel';
import { requestBodySample } from './utils/jsonSample';
import { KeyValueEditor } from './components/KeyValueEditor';
import { STANDARD_REQUEST_HEADERS } from './components/standardHeaders';
import { activeRows, HTTP_METHODS, type ConsoleRequest, type HttpMethod } from '../utils/types';

const messages = defineMessages({
  bodyTab: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.bodyTab',
    defaultMessage: 'Body',
    description: 'Tab holding the request body editor. A noun.',
  },
  headerHelp: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.headerHelp',
    defaultMessage:
      'Type in the last row to add a header, or pick a standard name from the list. Uncheck a row to exclude it.',
    description: 'Instruction under the headers table.',
  },
  headerName: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.headerName',
    defaultMessage: 'New header',
    description: 'Placeholder in the blank row where a new header name is typed.',
  },
  headersTab: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.headersTab',
    defaultMessage: 'Headers',
    description: 'Tab holding the request headers table. A noun.',
  },
  methodLabel: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.methodLabel',
    defaultMessage: 'HTTP method',
    description: 'Accessible label for the method picker.',
  },
  pathLabel: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.pathLabel',
    defaultMessage: 'Request path',
    description: 'Accessible label for the field holding the path after the gateway URL.',
  },
  queryHelp: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.queryHelp',
    defaultMessage: 'Type in the last row to add a query parameter. Uncheck a row to exclude it.',
    description: 'Instruction under the query parameters table.',
  },
  queryName: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.queryName',
    defaultMessage: 'New parameter',
    description: 'Placeholder in the blank row where a new query parameter name is typed.',
  },
  queryTab: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.queryTab',
    defaultMessage: 'Query parameters',
    description: 'Tab holding the query parameters table. A noun.',
  },
  title: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.title',
    defaultMessage: 'cURL builder',
    description: 'Heading of the panel where a request is composed by hand. A noun.',
  },
  value: {
    id: 'apiControlPlane.pages.test.console.CurlBuilder.value',
    defaultMessage: 'Value',
    description: 'Placeholder in the blank row where a new value is typed.',
  },
});

type BuilderTab = 'query' | 'headers' | 'body';

type CurlBuilderProps = {
  onChange: (request: ConsoleRequest) => void;
  onRegenerateSecret?: () => void;
  regenerating?: boolean;
  request: ConsoleRequest;
  /** The definition, for building a body sample from the operation's schema. */
  spec?: Record<string, unknown>;
};

/**
 * Composes a request by hand and shows the resulting command.
 *
 * State lives with the page, not here, which is what makes the one-way sync
 * work: filling in a try-out form in the Console view replaces this request, so
 * switching views shows the command for what was just being tested. Edits made
 * here stay here; they do not write back into swagger's form, because doing
 * that means dispatching undocumented swagger actions that break on upgrade.
 */
export function CurlBuilder({
  onChange,
  onRegenerateSecret,
  regenerating,
  request,
  spec,
}: CurlBuilderProps) {
  const intl = useIntl();
  const [tab, setTab] = useState<BuilderTab>('query');

  const sample = useMemo(
    () => (spec ? requestBodySample(spec, request.path, request.method) : undefined),
    [request.method, request.path, spec],
  );

  const queryCount = activeRows(request.queryParams).length;
  const headerCount = activeRows(request.headers).length;
  // Counts what a body actually contributes: one for a raw payload, or the
  // number of fields that would be sent for an encoded one.
  const bodyCount =
    request.bodyMode === 'none'
      ? 0
      : request.bodyMode === 'raw'
        ? request.body.trim() === ''
          ? 0
          : 1
        : activeRows(request.formFields).length;

  /** A count badge, or nothing — a "0" badge is noise. */
  const withBadge = (label: React.ReactNode, count: number) =>
    count === 0 ? (
      label
    ) : (
      <Badge badgeContent={count} color="primary" sx={{ pr: 1.5 }}>
        <Box sx={{ pr: 0.5 }}>{label}</Box>
      </Badge>
    );

  return (
    <Card variant="outlined">
      <Box sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2, py: 1.5 }}>
        <Stack alignItems="baseline" direction="row" flexWrap="wrap" spacing={1} useFlexGap>
          <Typography variant="subtitle2">
            <FormattedMessage {...messages.title} />
          </Typography>
        </Stack>
      </Box>

      <CardContent sx={{ p: 0 }}>
        <Stack direction="row" spacing={1} sx={{ p: 2 }}>
          <Select
            aria-label={intl.formatMessage(messages.methodLabel)}
            onChange={(event) => onChange({ ...request, method: event.target.value as HttpMethod })}
            size="small"
            sx={{ flexShrink: 0, width: 130 }}
            value={request.method}
          >
            {HTTP_METHODS.map((method) => (
              <MenuItem key={method} value={method}>
                {method}
              </MenuItem>
            ))}
          </Select>

          <Stack
            alignItems="center"
            direction="row"
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              flexGrow: 1,
              minWidth: 0,
              px: 1.5,
            }}
          >
            {/* The gateway base is fixed to the selected gateway. */}
            <Typography
              color="text.disabled"
              noWrap
              sx={{ flexShrink: 1, minWidth: 0 }}
              variant="body2"
            >
              {request.baseUrl}
            </Typography>
            <Input
              disableUnderline
              fullWidth
              inputProps={{ 'aria-label': intl.formatMessage(messages.pathLabel) }}
              onChange={(event) => onChange({ ...request, path: event.target.value })}
              value={request.path}
            />
          </Stack>
        </Stack>

        <Tabs
          onChange={(_event, next) => setTab(next as BuilderTab)}
          sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2 }}
          value={tab}
        >
          <Tab
            label={withBadge(<FormattedMessage {...messages.queryTab} />, queryCount)}
            value="query"
          />
          <Tab
            label={withBadge(<FormattedMessage {...messages.headersTab} />, headerCount)}
            value="headers"
          />
          <Tab
            label={withBadge(<FormattedMessage {...messages.bodyTab} />, bodyCount)}
            value="body"
          />
        </Tabs>

        {tab === 'query' && (
          <KeyValueEditor
            helperText={<FormattedMessage {...messages.queryHelp} />}
            namePlaceholder={intl.formatMessage(messages.queryName)}
            onChange={(queryParams) => onChange({ ...request, queryParams })}
            rows={request.queryParams}
            valuePlaceholder={intl.formatMessage(messages.value)}
          />
        )}

        {tab === 'headers' && (
          <KeyValueEditor
            helperText={<FormattedMessage {...messages.headerHelp} />}
            nameOptions={STANDARD_REQUEST_HEADERS}
            namePlaceholder={intl.formatMessage(messages.headerName)}
            onChange={(headers) => onChange({ ...request, headers })}
            onRegenerateSecret={onRegenerateSecret}
            regenerating={regenerating}
            rows={request.headers}
            valuePlaceholder={intl.formatMessage(messages.value)}
          />
        )}

        {tab === 'body' && (
          <BodyEditor
            fields={request.formFields}
            mode={request.bodyMode}
            onChange={(body) => onChange({ ...request, body })}
            onFieldsChange={(formFields) => onChange({ ...request, formFields })}
            onModeChange={(bodyMode) => onChange({ ...request, bodyMode })}
            onRawFormatChange={(rawFormat) => onChange({ ...request, rawFormat })}
            rawFormat={request.rawFormat}
            sample={sample}
            value={request.body}
          />
        )}

        <CurlCommandPanel request={request} />
      </CardContent>
    </Card>
  );
}
