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

import { lazy, Suspense, useMemo } from 'react';
import {
  Box,
  Button,
  FormControl,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Sparkles, WandSparkles } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { LoadingState } from '@/components/StateViews';
import type { CodeEditorLanguage } from '@/components/CodeEditor';
import { formatBody, validateBody } from '../utils/bodyValidation';
import { KeyValueEditor } from './KeyValueEditor';
import type { BodyMode, KeyValueRow, RawFormat } from '../../utils/types';

/** Lazy-load Monaco so the test page avoids its ~3.9 MB editor bundle. */
const CodeEditor = lazy(() =>
  import('@/components/CodeEditor/CodeEditor').then((module) => ({ default: module.CodeEditor })),
);

const messages = defineMessages({
  bodyLabel: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.bodyLabel',
    defaultMessage: 'Request body',
    description: 'Accessible label for the request body text area.',
  },
  editorLoading: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.editorLoading',
    defaultMessage: 'Loading editor',
    description: 'Shown in place of the request body editor while it is being fetched.',
  },
  fieldHelp: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.fieldHelp',
    defaultMessage: 'Type in the last row to add a field. Uncheck a row to exclude it.',
    description: 'Instruction under the form-field table of a form or url-encoded body.',
  },
  fieldName: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.fieldName',
    defaultMessage: 'New field',
    description: 'Placeholder in the blank row where a new body field name is typed.',
  },
  fieldValue: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.fieldValue',
    defaultMessage: 'Value',
    description: 'Placeholder in the blank row where a new body field value is typed.',
  },
  format: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.format',
    defaultMessage: 'Format',
    description: 'Button that re-indents the JSON body. A command.',
  },
  formatLabel: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.formatLabel',
    defaultMessage: 'Raw body format',
    description: 'Accessible label for the raw body format dropdown (JSON, XML or plain text).',
  },
  formData: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.formData',
    defaultMessage: 'Form data',
    description: 'Body type option for multipart/form-data, edited as key/value fields. A noun.',
  },
  insertSample: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.insertSample',
    defaultMessage: 'Insert sample',
    description: "Button that fills the body with an example built from the API's schema.",
  },
  invalid: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.invalid',
    defaultMessage: 'Invalid {format}: {reason}',
    description:
      "Validation message under the body editor. {format} is JSON or XML; {reason} is the parser's own message and is not translated.",
  },
  modeLabel: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.modeLabel',
    defaultMessage: 'Body type',
    description:
      'Accessible label for the body type dropdown (none, raw, form data, URL encoded). Not shown on screen.',
  },
  none: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.none',
    defaultMessage: 'None',
    description: 'Body type option meaning the request carries no body.',
  },
  placeholderJson: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.placeholderJson',
    defaultMessage: 'Enter a JSON request body',
    description: 'Placeholder inside the empty request body editor, in JSON mode.',
  },
  placeholderText: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.placeholderText',
    defaultMessage: 'Enter the request body',
    description: 'Placeholder inside the empty request body editor, in plain-text mode.',
  },
  placeholderXml: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.placeholderXml',
    defaultMessage: 'Enter an XML request body',
    description: 'Placeholder inside the empty request body editor, in XML mode.',
  },
  raw: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.raw',
    defaultMessage: 'Raw',
    description: 'Body type option for a body typed as text, in JSON, XML or plain text.',
  },
  sentAs: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.sentAs',
    defaultMessage: 'Sent as {contentType}',
    description:
      'Note under the body editor naming the Content-Type the body implies. {contentType} is a media type and is not translated.',
  },
  sentAsMultipart: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.sentAsMultipart',
    defaultMessage: 'Sent as multipart/form-data, with a boundary curl generates.',
    description:
      'Note under a form-data body, explaining that the Content-Type is not set by the console.',
  },
  text: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.text',
    defaultMessage: 'Text',
    description: 'Raw body format option for plain text.',
  },
  urlEncoded: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.urlEncoded',
    defaultMessage: 'URL encoded',
    description:
      'Body type option for application/x-www-form-urlencoded, edited as key/value fields.',
  },
  valid: {
    id: 'apiControlPlane.pages.test.console.BodyEditor.valid',
    defaultMessage: 'Valid {format}',
    description:
      'Validation message confirming the body parses. {format} is JSON or XML, not translated.',
  },
});

/**
 * Height of the editor well.
 *
 * Roughly the eight lines the old textarea started at. Monaco scrolls rather
 * than growing, so this is the whole height and not a minimum.
 */
const EDITOR_HEIGHT = 220;

/**
 * Monaco's language for each raw format.
 *
 * Highlighting, folding and bracket matching come free for all three. Only
 * JSON gets Monaco's own as-you-type diagnostics, which is why `validateBody`
 * still runs below: it is what produces the verdict line, and the only check
 * XML has at all.
 */
const EDITOR_LANGUAGE_FOR: Record<RawFormat, CodeEditorLanguage> = {
  json: 'json',
  text: 'plaintext',
  xml: 'xml',
};

/** Format names as shown in the format dropdown and the validation line. Not translated. */
const FORMAT_LABELS: Record<RawFormat, string> = {
  json: 'JSON',
  text: 'Text',
  xml: 'XML',
};

type BodyEditorProps = {
  fields: KeyValueRow[];
  mode: BodyMode;
  onChange: (body: string) => void;
  onFieldsChange: (fields: KeyValueRow[]) => void;
  onModeChange: (mode: BodyMode) => void;
  onRawFormatChange: (format: RawFormat) => void;
  rawFormat: RawFormat;
  /** Absent when the operation declares no JSON body to build one from. */
  sample?: string;
  value: string;
};

/**
 * Editor for request bodies in the cURL builder.
 *
 * Supports empty, raw, multipart, and URL-encoded bodies. Raw bodies support
 * JSON, XML, and plain text; the selected format determines content type and
 * validation. Encoded bodies use key/value tables.
 *
 * Raw bodies use the shared, lazy-loaded CodeEditor for syntax highlighting,
 * folding, and bracket matching. JSON receives Monaco diagnostics; XML uses
 * `validateBody`, since Monaco provides no XML language service.
 */
export function BodyEditor({
  fields,
  mode,
  onChange,
  onFieldsChange,
  onModeChange,
  onRawFormatChange,
  rawFormat,
  sample,
  value,
}: BodyEditorProps) {
  const intl = useIntl();

  const validation = useMemo(
    () => (mode === 'raw' ? validateBody(value, rawFormat) : { state: 'empty' as const }),
    [mode, rawFormat, value],
  );

  const invalid = validation.state === 'invalid';

  const placeholder =
    rawFormat === 'json'
      ? messages.placeholderJson
      : rawFormat === 'xml'
        ? messages.placeholderXml
        : messages.placeholderText;

  const reindent = () => {
    const next = formatBody(value, rawFormat);
    // Skip invalid input; XML and text are intentionally unchanged.
    if (next !== undefined) onChange(next);
  };

  return (
    <Box sx={{ px: 2, py: 2 }}>
      <Stack
        alignItems="center"
        direction="row"
        flexWrap="wrap"
        justifyContent="space-between"
        spacing={1}
        sx={{ mb: 1.5 }}
        useFlexGap
      >
        <Stack alignItems="center" direction="row" flexWrap="wrap" spacing={1} useFlexGap>
          <FormControl size="small" sx={{ minWidth: 148 }}>
            <FormLabel id="curl-body-mode-label" sx={{ display: 'none' }}>
              <FormattedMessage {...messages.modeLabel} />
            </FormLabel>
            <Select
              labelId="curl-body-mode-label"
              onChange={(event) => onModeChange(event.target.value as BodyMode)}
              value={mode}
            >
              <MenuItem value="none">
                <FormattedMessage {...messages.none} />
              </MenuItem>
              <MenuItem value="raw">
                <FormattedMessage {...messages.raw} />
              </MenuItem>
              <MenuItem value="form-data">
                <FormattedMessage {...messages.formData} />
              </MenuItem>
              <MenuItem value="url-encoded">
                <FormattedMessage {...messages.urlEncoded} />
              </MenuItem>
            </Select>
          </FormControl>

          {mode === 'raw' && (
            <FormControl size="small" sx={{ minWidth: 148 }}>
              <FormLabel id="curl-body-format-label" sx={{ display: 'none' }}>
                <FormattedMessage {...messages.formatLabel} />
              </FormLabel>
              <Select
                labelId="curl-body-format-label"
                onChange={(event) => onRawFormatChange(event.target.value as RawFormat)}
                value={rawFormat}
              >
                <MenuItem value="json">{FORMAT_LABELS.json}</MenuItem>
                <MenuItem value="xml">{FORMAT_LABELS.xml}</MenuItem>
                <MenuItem value="text">
                  <FormattedMessage {...messages.text} />
                </MenuItem>
              </Select>
            </FormControl>
          )}
        </Stack>

        {mode === 'raw' && (
          <Stack direction="row" spacing={1}>
            {/* Offered only for JSON: it is the one format that round-trips
                through a parser without changing what gets sent. */}
            {rawFormat === 'json' && (
              <Button
                onClick={reindent}
                size="small"
                startIcon={<WandSparkles size={16} />}
                variant="text"
              >
                <FormattedMessage {...messages.format} />
              </Button>
            )}
            {sample && rawFormat === 'json' && (
              <Button
                onClick={() => onChange(sample)}
                size="small"
                startIcon={<Sparkles size={16} />}
                variant="text"
              >
                <FormattedMessage {...messages.insertSample} />
              </Button>
            )}
          </Stack>
        )}
      </Stack>

      {mode === 'raw' && (
        <>
          {/* Fixed height lets Monaco render; vertical resize supports long bodies. */}
          <Box
            sx={{
              border: '1px solid',
              borderColor: invalid ? 'error.main' : 'divider',
              borderRadius: 1,
              height: EDITOR_HEIGHT,
              overflow: 'hidden',
              resize: 'vertical',
            }}
          >
            <Suspense
              fallback={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}
            >
              <CodeEditor
                ariaLabel={intl.formatMessage(messages.bodyLabel)}
                language={EDITOR_LANGUAGE_FOR[rawFormat]}
                onChange={onChange}
                placeholder={intl.formatMessage(placeholder)}
                value={value}
              />
            </Suspense>
          </Box>

          {/* Plain text has nothing to validate, so it gets a Content-Type note
              instead of a verdict it could never fail. */}
          <Typography
            color={
              invalid ? 'error' : validation.state === 'valid' ? 'success.main' : 'text.secondary'
            }
            sx={{ display: 'block', mt: 1 }}
            variant="caption"
          >
            {validation.state === 'empty' ? null : validation.state === 'invalid' ? (
              <FormattedMessage
                {...messages.invalid}
                values={{ format: FORMAT_LABELS[rawFormat], reason: validation.reason }}
              />
            ) : validation.state === 'valid' ? (
              <FormattedMessage {...messages.valid} values={{ format: FORMAT_LABELS[rawFormat] }} />
            ) : (
              <FormattedMessage {...messages.sentAs} values={{ contentType: 'text/plain' }} />
            )}
          </Typography>
        </>
      )}

      {(mode === 'form-data' || mode === 'url-encoded') && (
        <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
          <KeyValueEditor
            helperText={<FormattedMessage {...messages.fieldHelp} />}
            namePlaceholder={intl.formatMessage(messages.fieldName)}
            onChange={onFieldsChange}
            rows={fields}
            valuePlaceholder={intl.formatMessage(messages.fieldValue)}
          />
        </Box>
      )}

      {mode === 'url-encoded' && (
        <Typography color="text.secondary" sx={{ display: 'block', mt: 1 }} variant="caption">
          <FormattedMessage
            {...messages.sentAs}
            values={{ contentType: 'application/x-www-form-urlencoded' }}
          />
        </Typography>
      )}

      {mode === 'form-data' && (
        <Typography color="text.secondary" sx={{ display: 'block', mt: 1 }} variant="caption">
          <FormattedMessage {...messages.sentAsMultipart} />
        </Typography>
      )}
    </Box>
  );
}
