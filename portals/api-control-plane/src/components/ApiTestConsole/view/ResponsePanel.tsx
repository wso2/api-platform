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

import { useMemo } from 'react';
import {
  Alert,
  Box,
  Divider,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Stack,
  Tab,
  Tabs,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, Copy, Download, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SecondaryText } from '@/components/common/SecondaryText';

import type { ConsoleOperation, ConsoleSendRequest, ConsoleSendResult } from '../types';
import { prettyPrintJson } from '../format/prettyJson';
import { displayUrl, toCurl } from '../request/display';
import { ExpectedResponses } from './SchemaViews';
import { CodeView } from './CodeView';

const messages = defineMessages({
  bodyTab: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.bodyTab',
    defaultMessage: 'Body',
    description: 'Tab showing the response body.',
  },
  binary: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.binary',
    defaultMessage: 'This response is not text, so it is not shown.',
  },
  bodyLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.bodyLabel',
    defaultMessage: 'Response body',
    description: 'Accessible name of the region showing the response body.',
  },
  clear: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.clear',
    defaultMessage: 'Clear',
    description: 'Button that discards the shown response. A command.',
  },
  copy: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.copy',
    defaultMessage: 'Copy',
    description: 'Button that copies the response body. A command.',
  },
  copied: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.copied',
    defaultMessage: 'Copied',
    description: 'Shown briefly on a copy button after it has copied.',
  },
  curl: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.curl',
    defaultMessage: 'cURL',
  },
  download: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.download',
    defaultMessage: 'Download',
    description: 'Button that saves the response body as a file. A command.',
  },
  emptyBody: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.emptyBody',
    defaultMessage: 'The response has no body.',
  },
  emptyHint: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.emptyHint',
    defaultMessage: 'Press Send to run this request. Possible responses:',
  },
  headersLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.headersLabel',
    defaultMessage: 'Response headers',
    description: 'Accessible name of the region listing response headers.',
  },
  headersTab: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.headersTab',
    defaultMessage: 'Headers',
    description: 'Tab listing the response headers.',
  },
  requestTab: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.requestTab',
    defaultMessage: 'Request',
    description: 'Tab showing the request that produced the response.',
  },
  requestUrl: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.requestUrl',
    defaultMessage: 'Request URL',
  },
  response: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.response',
    defaultMessage: 'Response',
    description: 'Heading of the response section.',
  },
  responseTabs: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.responseTabs',
    defaultMessage: 'Response sections',
    description: 'Accessible label for the tabs that switch between body, headers and request.',
  },
  time: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.time',
    defaultMessage: '{ms, number} ms',
  },
  truncated: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.truncated',
    defaultMessage: 'The response is larger than the console shows, so it was cut off.',
  },
  waiting: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.waiting',
    defaultMessage: 'Waiting for the gateway…',
  },
  failed: {
    id: 'apiControlPlane.components.ApiTestConsole.ResponsePanel.failed',
    defaultMessage: 'The request did not complete',
    description:
      'Heading above the reason a request could not be completed (not an error status from the API).',
  },
});

export type Outcome = {
  result?: ConsoleSendResult;
  /** User-facing reason the request did not complete. Plain text. */
  error?: string;
  sent?: {
    request: ConsoleSendRequest;
    secretHeaders: ReadonlySet<string>;
    secretQuery: ReadonlySet<string>;
    /** How it was sent, as of sending; switching modes afterwards must not relabel it. */
    routeLabel?: string;
  };
};

export type ResponseTab = 'body' | 'headers' | 'request';

export type ResponsePanelProps = {
  operation: ConsoleOperation;
  outcome: Outcome | undefined;
  sending: boolean;
  /** How the request was sent, e.g. "via gateway". Optional. */
  routeLabel?: string;
  tab: ResponseTab;
  onTab: (tab: ResponseTab) => void;
  onClear: () => void;
  /** Which copy button was just used, for the brief "Copied". */
  copiedKey: string;
  onCopy: (text: string, key: string) => void;
};

const PRETTY_LIMIT_CHARS = 1024 * 1024;

/** Pretty-prints JSON bodies, and says whether the text is JSON. Anything else is returned as received. */
export const prettyBody = (
  text: string,
  contentType: string | undefined,
): { text: string; json: boolean } => {
  const looksJson = /json/i.test(contentType ?? '') || /^\s*[[{]/.test(text);
  if (!looksJson || text.length > PRETTY_LIMIT_CHARS) return { text, json: false };
  try {
    return { text: prettyPrintJson(text), json: true };
  } catch {
    return { text, json: false };
  }
};

const headerValueOf = (headers: [string, string][], name: string): string | undefined =>
  headers.find(([key]) => key.toLowerCase() === name)?.[1];

const extensionFor = (contentType: string | undefined, json: boolean): string => {
  if (json) return 'json';
  if (/xml/i.test(contentType ?? '')) return 'xml';
  return 'txt';
};

/** Saves text as a file. A Blob URL to local text, revoked immediately: nothing is fetched or executed. */
const downloadText = (text: string, filename: string, mime: string) => {
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

const statusChipColor = (status: number): 'success' | 'warning' | 'error' =>
  status < 300 ? 'success' : status < 500 ? 'warning' : 'error';

export function ResponsePanel({
  operation,
  outcome,
  sending,
  routeLabel,
  tab,
  onTab,
  onClear,
  copiedKey,
  onCopy,
}: ResponsePanelProps) {
  const intl = useIntl();
  const result = outcome?.result;

  const body = useMemo(
    () =>
      result
        ? prettyBody(result.bodyText, headerValueOf(result.headers, 'content-type'))
        : undefined,
    [result],
  );

  const chip = (label: string, color: 'success' | 'warning' | 'error' | 'default' = 'default') => (
    <Chip
      color={color}
      label={label}
      size="small"
      sx={{ height: 20, '& .MuiChip-label': { fontSize: 11, px: 1 } }}
      variant={color === 'default' ? 'filled' : 'outlined'}
    />
  );

  return (
    <Card component="section" variant="outlined">
      <CardContent>
        <Stack spacing={2}>
          <Stack
            alignItems="center"
            direction="row"
            flexWrap="wrap"
            gap={1.5}
            justifyContent="space-between"
          >
            <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1.5}>
              <Typography component="h3" variant="h6">
                <FormattedMessage {...messages.response} />
              </Typography>
              {result && !sending && (
                <>
                  {chip(
                    `${result.status} ${result.statusText}`.trim(),
                    statusChipColor(result.status),
                  )}
                  {chip(intl.formatMessage(messages.time, { ms: result.durationMs }))}
                  {chip(intl.formatNumber(result.sizeBytes, { style: 'unit', unit: 'byte' }))}
                  {(outcome?.sent?.routeLabel ?? routeLabel) && (
                    <SecondaryText variant="caption">
                      {outcome?.sent?.routeLabel ?? routeLabel}
                    </SecondaryText>
                  )}
                </>
              )}
            </Stack>
            {outcome && !sending && (
              <Stack direction="row" spacing={0.5}>
                {result && (
                  <>
                    <Tooltip
                      title={intl.formatMessage(
                        copiedKey === 'body' ? messages.copied : messages.copy,
                      )}
                    >
                      <Button
                        aria-label={intl.formatMessage(messages.copy)}
                        color="inherit"
                        sx={{ borderColor: 'divider', minWidth: 0, px: 1 }}
                        variant="outlined"
                        onClick={() => onCopy(body?.text ?? '', 'body')}
                        size="small"
                      >
                        {copiedKey === 'body' ? <Check size={16} /> : <Copy size={16} />}
                      </Button>
                    </Tooltip>
                    <Tooltip title={intl.formatMessage(messages.download)}>
                      <span>
                        <Button
                          aria-label={intl.formatMessage(messages.download)}
                          color="inherit"
                          sx={{ borderColor: 'divider', minWidth: 0, px: 1 }}
                          variant="outlined"
                          disabled={result.binary}
                          onClick={() =>
                            downloadText(
                              body?.text ?? '',
                              `response.${extensionFor(headerValueOf(result.headers, 'content-type'), body?.json ?? false)}`,
                              body?.json ? 'application/json' : 'text/plain',
                            )
                          }
                          size="small"
                        >
                          <Download size={16} />
                        </Button>
                      </span>
                    </Tooltip>
                  </>
                )}
                <Tooltip title={intl.formatMessage(messages.clear)}>
                  <Button
                    aria-label={intl.formatMessage(messages.clear)}
                    color="inherit"
                    sx={{ borderColor: 'divider', minWidth: 0, px: 1 }}
                    variant="outlined"
                    onClick={onClear}
                    size="small"
                  >
                    <X size={16} />
                  </Button>
                </Tooltip>
              </Stack>
            )}
          </Stack>

          {sending && (
            <Stack
              alignItems="center"
              aria-live="polite"
              direction="row"
              justifyContent="center"
              spacing={1.5}
              sx={{ py: 5 }}
            >
              <CircularProgress size={18} />
              <SecondaryText variant="body2">
                <FormattedMessage {...messages.waiting} />
              </SecondaryText>
            </Stack>
          )}

          {!sending && !outcome && (
            <Stack spacing={1.5}>
              <SecondaryText variant="body2">
                <FormattedMessage {...messages.emptyHint} />
              </SecondaryText>
              <ExpectedResponses responses={operation.responses} />
            </Stack>
          )}

          {!sending && outcome?.error && (
            <Alert severity="error">
              <Typography fontWeight="fontWeightMedium" variant="body2">
                <FormattedMessage {...messages.failed} />
              </Typography>
              {/* Plain text: the message is wording we chose, never interpreted as markup. */}
              <Typography variant="body2">{outcome.error}</Typography>
            </Alert>
          )}

          {!sending && result && body && (
            <Stack spacing={2}>
              <Box>
                <Tabs
                  aria-label={intl.formatMessage(messages.responseTabs)}
                  onChange={(_event, next: ResponseTab) => onTab(next)}
                  value={tab}
                >
                  <Tab label={intl.formatMessage(messages.bodyTab)} value="body" />
                  <Tab
                    label={`${intl.formatMessage(messages.headersTab)} (${result.headers.length})`}
                    value="headers"
                  />
                  {outcome?.sent && (
                    <Tab label={intl.formatMessage(messages.requestTab)} value="request" />
                  )}
                </Tabs>
                <Divider />
              </Box>

              {tab === 'body' && (
                <Stack spacing={1.5}>
                  {result.truncated && (
                    <Alert severity="info">
                      <FormattedMessage {...messages.truncated} />
                    </Alert>
                  )}
                  {result.binary ? (
                    <SecondaryText variant="body2">
                      <FormattedMessage {...messages.binary} />
                    </SecondaryText>
                  ) : result.bodyText === '' ? (
                    <SecondaryText variant="body2">
                      <FormattedMessage {...messages.emptyBody} />
                    </SecondaryText>
                  ) : (
                    <CodeView
                      label={intl.formatMessage(messages.bodyLabel)}
                      language={body.json ? 'json' : 'markup'}
                      text={body.text}
                    />
                  )}
                </Stack>
              )}

              {tab === 'headers' && (
                <CodeView
                  label={intl.formatMessage(messages.headersLabel)}
                  language="bash"
                  text={result.headers.map(([name, value]) => `${name}: ${value}`).join('\n')}
                />
              )}

              {tab === 'request' && outcome?.sent && (
                <Stack spacing={2}>
                  <Stack spacing={1}>
                    <Typography variant="subtitle2">
                      <FormattedMessage {...messages.requestUrl} />
                    </Typography>
                    <CodeView
                      label={intl.formatMessage(messages.requestUrl)}
                      language="bash"
                      maxHeight={120}
                      text={displayUrl(outcome.sent.request, {
                        reveal: false,
                        secretHeaders: outcome.sent.secretHeaders,
                        secretQuery: outcome.sent.secretQuery,
                      })}
                    />
                  </Stack>
                  <Stack spacing={1}>
                    <Stack alignItems="center" direction="row" justifyContent="space-between">
                      <Typography variant="subtitle2">
                        <FormattedMessage {...messages.curl} />
                      </Typography>
                      <Button
                        onClick={() =>
                          outcome.sent &&
                          // Copying reveals credentials, because a placeholder would not run.
                          onCopy(toCurl(outcome.sent.request, { reveal: true }), 'curl')
                        }
                        size="small"
                      >
                        {copiedKey === 'curl' ? (
                          <FormattedMessage {...messages.copied} />
                        ) : (
                          <FormattedMessage {...messages.copy} />
                        )}
                      </Button>
                    </Stack>
                    <CodeView
                      label={intl.formatMessage(messages.curl)}
                      language="bash"
                      maxHeight={240}
                      text={toCurl(outcome.sent.request, {
                        reveal: false,
                        secretHeaders: outcome.sent.secretHeaders,
                        secretQuery: outcome.sent.secretQuery,
                      })}
                    />
                  </Stack>
                </Stack>
              )}
            </Stack>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}
