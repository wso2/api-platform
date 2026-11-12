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

import { useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Paper,
  Select,
  Stack,
  Tab,
  Tabs,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, Copy, Lock, Plus, Send, Square, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type IntlShape } from 'react-intl';

import { SegmentedControl } from '@/components/common/SegmentedControl';
import { SecondaryText } from '@/components/common/SecondaryText';
import { MethodBadge } from '@/components/SwaggerOperationsView/MethodBadge';
import { methodPalette } from '@/components/SwaggerOperationsView/methodPalette';
import {
  BINARY_FILE_KEY,
  BODY_PROBLEM_KEY,
  PATH_PROBLEM_KEY,
  fieldProblemKey,
  headerProblemKey,
  paramKey,
  paramProblemKey,
  type DraftHeader,
  type Problem,
  type RequestDraft,
} from '../request/buildRequest';
import { prettyPrintJson } from '../format/prettyJson';
import { MASKED_VALUE } from '../request/display';
import type { ConsoleAutoParam, ConsoleOperation, ConsoleParameter } from '../types';
import { SchemaTable } from './SchemaViews';

const messages = defineMessages({
  addHeader: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.addHeader',
    defaultMessage: 'Add header',
    description: 'Button that adds a blank row to the request headers. A command.',
  },
  arrayHint: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.arrayHint',
    defaultMessage: 'Separate values with commas.',
  },
  auto: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.auto',
    defaultMessage: 'Auto',
    description: 'Badge on a header or parameter the console adds itself, such as the test key.',
  },
  autoHint: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.autoHint',
    defaultMessage: 'Added by the console. The value is hidden.',
  },
  bodyTab: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.bodyTab',
    defaultMessage: 'Body',
    description: 'Tab for the request body.',
  },
  chooseFile: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.chooseFile',
    defaultMessage: 'Choose file',
    description: 'Button that opens a file picker. A command.',
  },
  contentType: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.contentType',
    defaultMessage: 'Content type',
    description: 'Accessible label for the request body media-type selector.',
  },
  copy: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.copy',
    defaultMessage: 'Copy',
    description: 'Button that copies the request URL. A command.',
  },
  copied: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.copied',
    defaultMessage: 'Copied',
    description: 'Shown briefly on a copy button after it has copied.',
  },
  deprecated: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.deprecated',
    defaultMessage: 'Deprecated',
    description: 'Badge on an operation or parameter the API marks as deprecated.',
  },
  edit: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.edit',
    defaultMessage: 'Edit',
    description: 'Toggle option showing the request body editor.',
  },
  format: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.format',
    defaultMessage: 'Format',
    description: 'Button that pretty-prints the JSON body. A command.',
  },
  acceptLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.acceptLabel',
    defaultMessage: 'Response format',
    description:
      'Label of the dropdown that picks which media type to ask for in the Accept header.',
  },
  acceptNone: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.acceptNone',
    defaultMessage: 'No preference',
    description: 'Dropdown option that sends no Accept header.',
  },
  headerName: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.headerName',
    defaultMessage: 'Header name',
  },
  headersTab: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.headersTab',
    defaultMessage: 'Headers',
    description: 'Tab for the request headers.',
  },
  headerValue: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.headerValue',
    defaultMessage: 'Header value',
  },
  noFile: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.noFile',
    defaultMessage: 'No file chosen',
  },
  noParams: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.noParams',
    defaultMessage: 'This endpoint takes no parameters.',
  },
  paramColumn: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.paramColumn',
    defaultMessage: 'Parameter',
    description: 'Column heading above the parameter names.',
  },
  paramsTab: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.paramsTab',
    defaultMessage: 'Params',
    description: 'Tab for the request parameters.',
  },
  anyValue: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.anyValue',
    defaultMessage: '— any —',
    description: 'Option that leaves an enumerated parameter unset.',
  },
  problemBoolean: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemBoolean',
    defaultMessage: 'Must be true or false',
  },
  problemEnum: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemEnum',
    defaultMessage: 'Choose one of the listed values',
  },
  problemFile: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemFile',
    defaultMessage: 'Choose a file',
  },
  problemHeaderForbidden: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemHeaderForbidden',
    defaultMessage: 'This header is set by the browser or gateway and cannot be sent from here',
  },
  problemHeaderName: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemHeaderName',
    defaultMessage: 'Not a valid header name',
  },
  problemHeaderValue: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemHeaderValue',
    defaultMessage: 'The value cannot contain line breaks',
  },
  problemInteger: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemInteger',
    defaultMessage: 'Must be a whole number',
  },
  problemJson: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemJson',
    defaultMessage: 'The body is not valid JSON',
  },
  problemMax: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemMax',
    defaultMessage: 'Must be at most {max}',
  },
  problemMin: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemMin',
    defaultMessage: 'Must be at least {min}',
  },
  problemNumber: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemNumber',
    defaultMessage: 'Must be a number',
  },
  problemPath: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemPath',
    defaultMessage: 'This endpoint’s path cannot be called from the console',
  },
  problemRequired: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemRequired',
    defaultMessage: 'Required',
  },
  problemXml: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.problemXml',
    defaultMessage: 'The body is not well-formed XML',
  },
  removeFile: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.removeFile',
    defaultMessage: 'Remove file',
    description: 'Accessible label of the button that clears a chosen file.',
  },
  removeHeader: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.removeHeader',
    defaultMessage: 'Remove header',
    description: 'Accessible label of the button that deletes a header row.',
  },
  copyUrl: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.copyUrl',
    defaultMessage: 'Copy request URL',
    description: 'Accessible label and tooltip of the button copying the request URL.',
  },
  required: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.required',
    defaultMessage: 'required',
    description: 'Marks a parameter that must be provided. Lower-case adjective.',
  },
  requestTabs: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.requestTabs',
    defaultMessage: 'Request sections',
    description: 'Accessible label for the tabs that switch between parameters, headers and body.',
  },
  reset: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.reset',
    defaultMessage: 'Reset to example',
    description: 'Button that restores the body to the documented example. A command.',
  },
  schema: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.schema',
    defaultMessage: 'Schema',
    description: 'Toggle option showing the structure of the request body.',
  },
  send: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.send',
    defaultMessage: 'Send',
    description: 'Button that sends the request. A command.',
  },
  cancel: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.cancel',
    defaultMessage: 'Cancel',
    description: 'Button that stops a request in flight. A command.',
  },
  invalidJson: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.invalidJson',
    defaultMessage: '✕ Not valid JSON',
    description:
      'Live check beside the body editor: the text is not valid JSON. The cross marks failure.',
  },
  busyHint: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.busyHint',
    defaultMessage: 'Another request is still running. Wait for it or cancel it first.',
    description:
      'Tooltip of the disabled Send button while a request for a different endpoint is in flight.',
  },
  sendHint: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.sendHint',
    defaultMessage: 'Send ({shortcut})',
    description:
      'Tooltip of the Send button. The placeholder is the keyboard shortcut: Command (or Ctrl) plus Return.',
  },
  cancelHint: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.cancelHint',
    defaultMessage: 'Cancel ({shortcut})',
    description:
      'Tooltip of the Cancel button. The placeholder is the keyboard shortcut: the Escape key.',
  },
  validJson: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.validJson',
    defaultMessage: '✓ Valid JSON',
  },
  valueColumn: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.valueColumn',
    defaultMessage: 'Value',
    description: 'Column heading above the parameter values.',
  },
  viewLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.viewLabel',
    defaultMessage: 'Body editor or schema',
    description: 'Accessible label for the toggle between the body editor and its schema.',
  },
  bodyEditor: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.bodyEditor',
    defaultMessage: 'Request body',
    description: 'Accessible name of the request body text editor.',
  },
  urlLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.RequestPanel.urlLabel',
    defaultMessage: 'Request URL',
    description: 'Accessible name of the request URL display.',
  },
});

const problemMessage = (intl: IntlShape, problem: Problem): string => {
  switch (problem.code) {
    case 'required':
      return intl.formatMessage(messages.problemRequired);
    case 'integer':
      return intl.formatMessage(messages.problemInteger);
    case 'number':
      return intl.formatMessage(messages.problemNumber);
    case 'boolean':
      return intl.formatMessage(messages.problemBoolean);
    case 'enum':
      return intl.formatMessage(messages.problemEnum);
    case 'min':
      return intl.formatMessage(messages.problemMin, { min: problem.min });
    case 'max':
      return intl.formatMessage(messages.problemMax, { max: problem.max });
    case 'json':
      return intl.formatMessage(messages.problemJson);
    case 'xml':
      return intl.formatMessage(messages.problemXml);
    case 'headerName':
      return intl.formatMessage(messages.problemHeaderName);
    case 'headerForbidden':
      return intl.formatMessage(messages.problemHeaderForbidden);
    case 'headerValue':
      return intl.formatMessage(messages.problemHeaderValue);
    case 'file':
      return intl.formatMessage(messages.problemFile);
    case 'path':
      return intl.formatMessage(messages.problemPath);
  }
};

const visuallyHidden = {
  clip: 'rect(0 0 0 0)',
  height: 1,
  overflow: 'hidden',
  position: 'absolute',
  whiteSpace: 'nowrap',
  width: 1,
} as const;

let headerSequence = 0;
const nextHeaderId = (): string => {
  headerSequence += 1;
  return `header-${headerSequence}`;
};

export type RequestTab = 'params' | 'headers' | 'body';

export type RequestPanelProps = {
  operation: ConsoleOperation;
  draft: RequestDraft;
  onDraft: (update: (draft: RequestDraft) => RequestDraft) => void;
  problems: Record<string, Problem>;
  baseUrl: string;
  /** Path and query as far as they are filled in. */
  preview: { path: string; query: [string, string][] };
  autoHeaders: readonly ConsoleAutoParam[];
  autoQuery: readonly ConsoleAutoParam[];
  tab: RequestTab;
  onTab: (tab: RequestTab) => void;
  sending: boolean;
  /** A request for another operation is in flight, so this one cannot be sent yet. */
  busy?: boolean;
  onSend: () => void;
  onCancel: () => void;
  /** Text to put on the clipboard for "Copy URL" (credentials revealed). */
  copyableUrl: string;
  copied: boolean;
  onCopy: (text: string) => void;
};

const SHORTCUT_SEND = '⌘↵';
const SHORTCUT_CANCEL = 'Esc';

function ProblemText({ problem }: { problem: Problem | undefined }) {
  const intl = useIntl();
  if (!problem) return null;
  return (
    <Typography color="error" role="alert" variant="caption">
      {problemMessage(intl, problem)}
    </Typography>
  );
}

function RequiredChip() {
  return (
    <Chip
      color="warning"
      label={<FormattedMessage {...messages.required} />}
      size="small"
      variant="outlined"
    />
  );
}

function ParameterRow({
  parameter,
  value,
  problem,
  onChange,
}: {
  parameter: ConsoleParameter;
  value: string;
  problem: Problem | undefined;
  onChange: (value: string) => void;
}) {
  const intl = useIntl();
  const labelId = `param-label-${parameter.in}-${parameter.name}`;
  const isEnum = parameter.enum !== undefined && parameter.type !== 'array';
  const isBoolean = parameter.type === 'boolean';
  const options = isBoolean ? ['true', 'false'] : (parameter.enum ?? []);

  return (
    <TableRow>
      <TableCell sx={{ verticalAlign: 'top', width: '34%' }}>
        <Stack spacing={0.5}>
          <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1}>
            <Typography
              fontFamily="monospace"
              id={labelId}
              variant="body2"
              sx={{ wordBreak: 'break-word' }}
            >
              {parameter.name}
            </Typography>
            {parameter.required && <RequiredChip />}
            {parameter.deprecated && (
              <Chip
                label={intl.formatMessage(messages.deprecated)}
                size="small"
                variant="outlined"
              />
            )}
          </Stack>
          <SecondaryText fontFamily="monospace" variant="caption">
            {[parameter.type, parameter.in].join(' · ')}
          </SecondaryText>
        </Stack>
      </TableCell>
      <TableCell>
        <Stack spacing={0.75}>
          {isEnum || isBoolean ? (
            <FormControl error={Boolean(problem)} fullWidth size="small">
              <FormLabel id={`${labelId}-select`} sx={visuallyHidden}>
                {parameter.name}
              </FormLabel>
              <Select
                displayEmpty
                labelId={`${labelId}-select`}
                onChange={(event) => onChange(String(event.target.value))}
                value={value}
              >
                <MenuItem value="">
                  <FormattedMessage {...messages.anyValue} />
                </MenuItem>
                {options.map((option) => (
                  <MenuItem key={option} value={option}>
                    {option}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          ) : (
            <TextField
              error={Boolean(problem)}
              fullWidth
              onChange={(event) => onChange(event.target.value)}
              size="small"
              slotProps={{ htmlInput: { 'aria-label': parameter.name } }}
              value={value}
            />
          )}
          <ProblemText problem={problem} />
          {!problem && parameter.description && (
            <SecondaryText variant="caption">{parameter.description}</SecondaryText>
          )}
          {!problem && parameter.type === 'array' && (
            <SecondaryText variant="caption">
              <FormattedMessage {...messages.arrayHint} />
            </SecondaryText>
          )}
        </Stack>
      </TableCell>
    </TableRow>
  );
}

function ParamsTab({
  operation,
  draft,
  problems,
  onChange,
}: {
  operation: ConsoleOperation;
  draft: RequestDraft;
  problems: Record<string, Problem>;
  onChange: (key: string, value: string) => void;
}) {
  const pathProblem = problems[PATH_PROBLEM_KEY];
  if (operation.parameters.length === 0 && !pathProblem) {
    return (
      <SecondaryText variant="body2">
        <FormattedMessage {...messages.noParams} />
      </SecondaryText>
    );
  }
  return (
    <Stack spacing={1}>
      {pathProblem && <ProblemText problem={pathProblem} />}
      {operation.parameters.length > 0 && (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small" sx={{ '& .MuiTableCell-root': { borderColor: 'divider' } }}>
            <TableHead>
              <TableRow>
                <TableCell>
                  <FormattedMessage {...messages.paramColumn} />
                </TableCell>
                <TableCell>
                  <FormattedMessage {...messages.valueColumn} />
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {operation.parameters.map((parameter) => (
                <ParameterRow
                  key={paramKey(parameter)}
                  onChange={(value) => onChange(paramKey(parameter), value)}
                  parameter={parameter}
                  problem={problems[paramProblemKey(parameter)]}
                  value={draft.params[paramKey(parameter)] ?? ''}
                />
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Stack>
  );
}

/** The media types an operation documents for its responses, in document order, once each. */
const acceptOptionsOf = (operation: ConsoleOperation): string[] =>
  Array.from(new Set(operation.responses.flatMap((response) => response.mediaTypes)));

function HeadersTab({
  operation,
  headers,
  autoHeaders,
  problems,
  onChange,
}: {
  operation: ConsoleOperation;
  headers: DraftHeader[];
  autoHeaders: readonly ConsoleAutoParam[];
  problems: Record<string, Problem>;
  onChange: (next: DraftHeader[]) => void;
}) {
  const intl = useIntl();
  const update = (id: string, patch: Partial<DraftHeader>) =>
    onChange(headers.map((header) => (header.id === id ? { ...header, ...patch } : header)));

  const isAccept = (header: DraftHeader) => header.name.trim().toLowerCase() === 'accept';
  const acceptOptions = acceptOptionsOf(operation);
  const accept = headers.find(isAccept)?.value ?? '';
  // A value the user typed by hand stays selectable rather than vanishing from the list.
  const shownOptions =
    accept !== '' && !acceptOptions.includes(accept) ? [...acceptOptions, accept] : acceptOptions;
  // The console owns Accept when it injects one itself.
  const showAccept =
    shownOptions.length > 0 &&
    !autoHeaders.some((header) => header.name.toLowerCase() === 'accept');
  const chooseAccept = (value: string) => {
    const rest = headers.filter((header) => !isAccept(header));
    onChange(value === '' ? rest : [{ id: 'accept', name: 'Accept', value }, ...rest]);
  };

  return (
    <Stack alignItems="flex-start" spacing={1.5}>
      {showAccept && (
        <FormControl size="small" sx={{ minWidth: 280 }}>
          <FormLabel id="api-test-console-accept" sx={{ mb: 0.5 }}>
            <SecondaryText variant="caption">
              <FormattedMessage {...messages.acceptLabel} />
            </SecondaryText>
          </FormLabel>
          <Select
            displayEmpty
            labelId="api-test-console-accept"
            onChange={(event) => chooseAccept(String(event.target.value))}
            value={accept}
          >
            <MenuItem value="">
              <FormattedMessage {...messages.acceptNone} />
            </MenuItem>
            {shownOptions.map((option) => (
              <MenuItem key={option} value={option}>
                {option}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}
      {(autoHeaders.length > 0 || headers.length > 0) && (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small" sx={{ '& .MuiTableCell-root': { borderColor: 'divider' } }}>
            <TableHead>
              <TableRow>
                <TableCell>
                  <FormattedMessage {...messages.headerName} />
                </TableCell>
                <TableCell>
                  <FormattedMessage {...messages.headerValue} />
                </TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {autoHeaders.map((header) => (
                <TableRow key={`auto-${header.name}`}>
                  <TableCell>
                    <Stack alignItems="center" direction="row" spacing={1}>
                      <Lock size={14} />
                      <Typography fontFamily="monospace" variant="body2">
                        {header.name}
                      </Typography>
                    </Stack>
                  </TableCell>
                  <TableCell>
                    <Tooltip title={intl.formatMessage(messages.autoHint)}>
                      <SecondaryText fontFamily="monospace" variant="body2">
                        {header.secret ? MASKED_VALUE : header.value}
                      </SecondaryText>
                    </Tooltip>
                  </TableCell>
                  <TableCell align="right">
                    <Chip
                      label={intl.formatMessage(messages.auto)}
                      size="small"
                      variant="outlined"
                    />
                  </TableCell>
                </TableRow>
              ))}
              {headers.map((header) => (
                <TableRow key={header.id}>
                  <TableCell sx={{ verticalAlign: 'top' }}>
                    <Stack spacing={0.5}>
                      <TextField
                        error={Boolean(problems[headerProblemKey(header.id)])}
                        fullWidth
                        onChange={(event) => update(header.id, { name: event.target.value })}
                        placeholder={intl.formatMessage(messages.headerName)}
                        size="small"
                        slotProps={{
                          htmlInput: { 'aria-label': intl.formatMessage(messages.headerName) },
                        }}
                        value={header.name}
                      />
                      <ProblemText problem={problems[headerProblemKey(header.id)]} />
                    </Stack>
                  </TableCell>
                  <TableCell sx={{ verticalAlign: 'top' }}>
                    <TextField
                      fullWidth
                      onChange={(event) => update(header.id, { value: event.target.value })}
                      placeholder={intl.formatMessage(messages.headerValue)}
                      size="small"
                      slotProps={{
                        htmlInput: { 'aria-label': intl.formatMessage(messages.headerValue) },
                      }}
                      value={header.value}
                    />
                  </TableCell>
                  <TableCell align="right" sx={{ verticalAlign: 'top' }}>
                    <IconButton
                      aria-label={intl.formatMessage(messages.removeHeader)}
                      onClick={() => onChange(headers.filter((h) => h.id !== header.id))}
                      size="small"
                    >
                      <X size={16} />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      <Button
        onClick={() => onChange([...headers, { id: nextHeaderId(), name: '', value: '' }])}
        size="small"
        startIcon={<Plus size={14} />}
        variant="outlined"
      >
        <FormattedMessage {...messages.addHeader} />
      </Button>
    </Stack>
  );
}

function FilePicker({
  file,
  onChange,
  label,
}: {
  file: File | undefined;
  onChange: (file: File | undefined) => void;
  label: string;
}) {
  const intl = useIntl();
  return (
    <Stack alignItems="center" direction="row" spacing={1}>
      <Button component="label" size="small" variant="outlined">
        <FormattedMessage {...messages.chooseFile} />
        <input
          aria-label={label}
          hidden
          onChange={(event) => {
            onChange(event.target.files?.[0]);
            // Allow choosing the same file again after clearing it.
            event.target.value = '';
          }}
          type="file"
        />
      </Button>
      <SecondaryText noWrap variant="body2">
        {file ? file.name : <FormattedMessage {...messages.noFile} />}
      </SecondaryText>
      {file && (
        <IconButton
          aria-label={intl.formatMessage(messages.removeFile)}
          onClick={() => onChange(undefined)}
          size="small"
        >
          <X size={14} />
        </IconButton>
      )}
    </Stack>
  );
}

function BodyTab({
  operation,
  draft,
  problems,
  onDraft,
}: {
  operation: ConsoleOperation;
  draft: RequestDraft;
  problems: Record<string, Problem>;
  onDraft: RequestPanelProps['onDraft'];
}) {
  const intl = useIntl();
  const [view, setView] = useState<'edit' | 'schema'>('edit');
  const body = operation.body;
  if (!body) return null;
  const option = body.options.find((o) => o.mediaType === draft.bodyMediaType) ?? body.options[0];
  const isText = option.kind === 'json' || option.kind === 'xml' || option.kind === 'text';
  const bodyProblem = problems[BODY_PROBLEM_KEY];
  // With no fields there is no schema to show, so the editor is the only view.
  const activeView = isText && option.fields.length > 0 ? view : 'edit';

  let jsonValid = false;
  if (option.kind === 'json' && draft.bodyText.trim() !== '') {
    try {
      JSON.parse(draft.bodyText);
      jsonValid = true;
    } catch {
      jsonValid = false;
    }
  }

  const selectMediaType = (mediaType: string) => {
    const next = body.options.find((o) => o.mediaType === mediaType);
    onDraft((current) => ({ ...current, bodyMediaType: mediaType, bodyText: next?.example ?? '' }));
  };

  return (
    <Stack spacing={1}>
      <Paper sx={{ overflow: 'hidden' }} variant="outlined">
        <Stack
          alignItems="center"
          direction="row"
          flexWrap="wrap"
          gap={1.5}
          justifyContent="space-between"
          sx={{ px: 1.5, py: 1 }}
        >
          <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1.5}>
            {body.options.length > 1 ? (
              <FormControl size="small">
                <FormLabel id="body-media-type-label" sx={visuallyHidden}>
                  {intl.formatMessage(messages.contentType)}
                </FormLabel>
                <Select
                  labelId="body-media-type-label"
                  onChange={(event) => selectMediaType(String(event.target.value))}
                  value={option.mediaType}
                >
                  {body.options.map((o) => (
                    <MenuItem key={o.mediaType} value={o.mediaType}>
                      {o.mediaType}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            ) : (
              <Chip label={option.mediaType} size="small" variant="outlined" />
            )}
            {isText && option.fields.length > 0 && (
              <SegmentedControl
                aria-label={intl.formatMessage(messages.viewLabel)}
                onChange={setView}
                options={[
                  { label: <FormattedMessage {...messages.edit} />, value: 'edit' },
                  { label: <FormattedMessage {...messages.schema} />, value: 'schema' },
                ]}
                value={activeView}
              />
            )}
            {option.kind === 'json' && draft.bodyText.trim() !== '' && (
              <Typography color={jsonValid ? 'success.main' : 'error'} variant="caption">
                <FormattedMessage {...(jsonValid ? messages.validJson : messages.invalidJson)} />
              </Typography>
            )}
          </Stack>
          {isText && activeView === 'edit' && (
            <Stack direction="row" spacing={1}>
              {option.kind === 'json' && (
                <Button
                  onClick={() => {
                    try {
                      const pretty = prettyPrintJson(draft.bodyText);
                      onDraft((current) => ({ ...current, bodyText: pretty }));
                    } catch {
                      // Not valid JSON: leave the text as typed so it can be corrected.
                    }
                  }}
                  size="small"
                  variant="text"
                >
                  <FormattedMessage {...messages.format} />
                </Button>
              )}
              <Button
                onClick={() => onDraft((current) => ({ ...current, bodyText: option.example }))}
                size="small"
                variant="text"
              >
                <FormattedMessage {...messages.reset} />
              </Button>
            </Stack>
          )}
        </Stack>
        <Divider />

        {isText && activeView === 'edit' && (
          <TextField
            error={Boolean(bodyProblem)}
            fullWidth
            minRows={8}
            multiline
            onChange={(event) =>
              onDraft((current) => ({ ...current, bodyText: event.target.value }))
            }
            slotProps={{
              htmlInput: {
                'aria-label': intl.formatMessage(messages.bodyEditor),
                spellCheck: false,
              },
              input: {
                disableUnderline: true,
                sx: { fontFamily: 'monospace', fontSize: 13, p: 2 },
              },
            }}
            variant="standard"
            value={draft.bodyText}
          />
        )}
        {isText && activeView === 'schema' && <SchemaTable fields={option.fields} />}
        {(option.kind === 'urlencoded' || option.kind === 'multipart') && (
          <TableContainer>
            <Table size="small" sx={{ '& .MuiTableCell-root': { borderColor: 'divider' } }}>
              <TableBody>
                {option.formFields.map((field) => (
                  <TableRow key={field.name}>
                    <TableCell sx={{ verticalAlign: 'top', width: '34%' }}>
                      <Stack spacing={0.5}>
                        <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1}>
                          <Typography
                            fontFamily="monospace"
                            variant="body2"
                            sx={{ wordBreak: 'break-word' }}
                          >
                            {field.name}
                          </Typography>
                          {field.required && <RequiredChip />}
                        </Stack>
                        <SecondaryText fontFamily="monospace" variant="caption">
                          {field.type}
                        </SecondaryText>
                      </Stack>
                    </TableCell>
                    <TableCell>
                      <Stack spacing={0.75}>
                        {field.binary && option.kind === 'multipart' ? (
                          <FilePicker
                            file={draft.files[field.name]}
                            label={field.name}
                            onChange={(file) =>
                              onDraft((current) => ({
                                ...current,
                                files: { ...current.files, [field.name]: file },
                              }))
                            }
                          />
                        ) : (
                          <TextField
                            error={Boolean(problems[fieldProblemKey(field.name)])}
                            fullWidth
                            onChange={(event) =>
                              onDraft((current) => ({
                                ...current,
                                formValues: {
                                  ...current.formValues,
                                  [field.name]: event.target.value,
                                },
                              }))
                            }
                            size="small"
                            slotProps={{ htmlInput: { 'aria-label': field.name } }}
                            value={draft.formValues[field.name] ?? ''}
                          />
                        )}
                        <ProblemText problem={problems[fieldProblemKey(field.name)]} />
                        {field.description !== '' && (
                          <SecondaryText variant="caption">{field.description}</SecondaryText>
                        )}
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
        {option.kind === 'binary' && (
          <FilePicker
            file={draft.files[BINARY_FILE_KEY]}
            label={intl.formatMessage(messages.chooseFile)}
            onChange={(file) =>
              onDraft((current) => ({
                ...current,
                files: { ...current.files, [BINARY_FILE_KEY]: file },
              }))
            }
          />
        )}
      </Paper>
      <ProblemText problem={bodyProblem} />
    </Stack>
  );
}

export function RequestPanel({
  operation,
  draft,
  onDraft,
  problems,
  baseUrl,
  preview,
  autoHeaders,
  autoQuery,
  tab,
  onTab,
  sending,
  busy = false,
  onSend,
  onCancel,
  copyableUrl,
  copied,
  onCopy,
}: RequestPanelProps) {
  const intl = useIntl();
  const hasBody =
    operation.body !== undefined && operation.method !== 'GET' && operation.method !== 'HEAD';

  const problemKeys = Object.keys(problems);
  const paramErrors = problemKeys.filter(
    (key) => key.startsWith('param:') || key === 'path',
  ).length;
  const headerErrors = problemKeys.filter((key) => key.startsWith('header:')).length;
  const bodyErrors = problemKeys.filter((key) => key === 'body' || key.startsWith('field:')).length;

  const activeTab: RequestTab = tab === 'body' && !hasBody ? 'params' : tab;

  // The URL as shown: credentials masked. What is copied reveals them.
  const maskedQuery = preview.query
    .map(([name, value]) => {
      const secret = autoQuery.some((q) => q.name === name && q.secret);
      return `${encodeURIComponent(name)}=${encodeURIComponent(secret ? MASKED_VALUE : value)}`;
    })
    .join('&');
  const shownUrl = `${baseUrl}${preview.path}${maskedQuery === '' ? '' : `?${maskedQuery}`}`;

  const tabLabel = (label: ReactNode, errors: number, count?: number): ReactNode => (
    <Stack alignItems="center" direction="row" spacing={1}>
      <span>{label}</span>
      {(errors > 0 || (count ?? 0) > 0) && (
        <Chip
          color={errors > 0 ? 'error' : 'default'}
          label={errors > 0 ? errors : count}
          size="small"
        />
      )}
    </Stack>
  );

  return (
    <Card component="section" variant="outlined">
      <CardContent>
        <Stack spacing={2.5}>
          <Stack spacing={1}>
            <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1.5}>
              <MethodBadge method={operation.method} />
              <Typography component="h2" variant="h6">
                {operation.summary}
              </Typography>
              {operation.deprecated && (
                <Chip
                  color="warning"
                  label={intl.formatMessage(messages.deprecated)}
                  size="small"
                  variant="outlined"
                />
              )}
            </Stack>
            {operation.description !== '' && (
              <SecondaryText variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
                {operation.description}
              </SecondaryText>
            )}
          </Stack>

          <Stack alignItems="flex-start" direction={{ sm: 'row', xs: 'column' }} spacing={1.5}>
            <TextField
              fullWidth
              size="small"
              sx={{ '& .MuiOutlinedInput-root': { pl: 0.5 } }}
              slotProps={{
                htmlInput: { 'aria-label': intl.formatMessage(messages.urlLabel) },
                input: {
                  startAdornment: (
                    <InputAdornment position="start" sx={{ alignItems: 'center', gap: 1.5 }}>
                      <Typography
                        sx={{
                          color: methodPalette(operation.method).badge,
                          fontSize: 12,
                          fontWeight: 600,
                        }}
                        variant="body2"
                      >
                        {operation.method}
                      </Typography>
                      <Divider flexItem orientation="vertical" />
                    </InputAdornment>
                  ),
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip
                        title={intl.formatMessage(copied ? messages.copied : messages.copyUrl)}
                      >
                        <IconButton
                          aria-label={intl.formatMessage(messages.copyUrl)}
                          edge="end"
                          onClick={() => onCopy(copyableUrl)}
                          size="small"
                        >
                          {copied ? <Check size={16} /> : <Copy size={16} />}
                        </IconButton>
                      </Tooltip>
                    </InputAdornment>
                  ),
                  readOnly: true,
                },
              }}
              value={shownUrl}
            />
            <Tooltip
              title={
                busy
                  ? intl.formatMessage(messages.busyHint)
                  : intl.formatMessage(sending ? messages.cancelHint : messages.sendHint, {
                      shortcut: sending ? SHORTCUT_CANCEL : SHORTCUT_SEND,
                    })
              }
            >
              {/* A disabled button swallows pointer events, so the tooltip hangs off a wrapper. */}
              <span>
                <Button
                  color={sending ? 'inherit' : 'primary'}
                  disabled={busy}
                  onClick={sending ? onCancel : onSend}
                  startIcon={sending ? <Square size={14} /> : <Send size={14} />}
                  sx={{ flexShrink: 0 }}
                  variant={sending ? 'outlined' : 'contained'}
                >
                  {sending ? (
                    <FormattedMessage {...messages.cancel} />
                  ) : (
                    <FormattedMessage {...messages.send} />
                  )}
                </Button>
              </span>
            </Tooltip>
          </Stack>

          <Box>
            <Tabs
              aria-label={intl.formatMessage(messages.requestTabs)}
              onChange={(_event, next: RequestTab) => onTab(next)}
              value={activeTab}
            >
              <Tab
                label={tabLabel(
                  <FormattedMessage {...messages.paramsTab} />,
                  paramErrors,
                  operation.parameters.length,
                )}
                value="params"
              />
              <Tab
                label={tabLabel(
                  <FormattedMessage {...messages.headersTab} />,
                  headerErrors,
                  draft.headers.length + autoHeaders.length,
                )}
                value="headers"
              />
              {hasBody && (
                <Tab
                  label={tabLabel(<FormattedMessage {...messages.bodyTab} />, bodyErrors)}
                  value="body"
                />
              )}
            </Tabs>
            <Divider sx={{ mb: 2 }} />

            {activeTab === 'params' && (
              <ParamsTab
                draft={draft}
                onChange={(key, value) =>
                  onDraft((current) => ({
                    ...current,
                    params: { ...current.params, [key]: value },
                  }))
                }
                operation={operation}
                problems={problems}
              />
            )}
            {activeTab === 'headers' && (
              <HeadersTab
                operation={operation}
                autoHeaders={autoHeaders}
                headers={draft.headers}
                onChange={(next) => onDraft((current) => ({ ...current, headers: next }))}
                problems={problems}
              />
            )}
            {activeTab === 'body' && hasBody && (
              <BodyTab draft={draft} onDraft={onDraft} operation={operation} problems={problems} />
            )}
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}
