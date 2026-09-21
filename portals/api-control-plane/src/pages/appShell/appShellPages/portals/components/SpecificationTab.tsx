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

import { lazy, Suspense, useState } from 'react';
import { Alert, Box, Button, Stack, ToggleButton, ToggleButtonGroup } from '@wso2/oxygen-ui';
import { Pencil } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { LoadingState } from '@/components/StateViews';
import { hairline } from '@/theme/receipes';
import { parseSpecText, serializeSpec, type SpecFormat } from '../../apis/create/utils/specText';

/** Same lazy split as `SpecSourceEditor` — Monaco is the heaviest thing this app loads. */
const SpecCodeEditor = lazy(() =>
  import('../../apis/create/components/SpecCodeEditor').then((module) => ({
    default: module.SpecCodeEditor,
  })),
);

const messages = defineMessages({
  editorLoading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.editorLoading',
    defaultMessage: 'Loading editor',
  },
  malformed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.malformed',
    defaultMessage: 'This is not valid {format}: {detail}',
    description: '{format} is JSON or YAML; {detail} is the parser’s own message naming the line.',
  },
  formatLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.formatLabel',
    defaultMessage: 'Source format',
    description: 'Accessible name for the YAML / JSON toggle buttons.',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.edit',
    defaultMessage: 'Edit',
  },
});

/** Format names are the same in every language. */
const FORMAT_LABELS: Record<SpecFormat, string> = { json: 'JSON', yaml: 'YAML' };

export type SpecificationTabProps = {
  disabled?: boolean;
  /** The serialization the text is in, which is also the one it is parsed as on save. */
  format: SpecFormat;
  /** Every keystroke, or a format switch that re-prints the text, with the buffer's full text. */
  onChange: (text: string) => void;
  onFormatChange: (format: SpecFormat) => void;
  /** The parser's own complaint, when the current buffer doesn't read in its format. */
  parseError?: string;
  /** The definition's raw text, in `format`. */
  text: string;
};

/**
 * "Specification" — the draft's OpenAPI definition (`.../draft/definition`).
 * The source panel of the API Definition page, reduced to what publishing needs:
 * a JSON/YAML switch, an Edit button and the editor. Opens read-only once there
 * is a definition to protect; import, download and the resources view stay with
 * the API Definition page, and the definition is saved with the rest of the
 * draft, so there is no Save here.
 */
export function SpecificationTab({ disabled, format, onChange, onFormatChange, parseError, text }: SpecificationTabProps) {
  const intl = useIntl();
  const [isEditing, setIsEditing] = useState(false);

  const hasText = text.trim() !== '';
  // A definition that failed to parse sends the user back here to fix it, so
  // it stays editable without another click.
  const editable = isEditing || !hasText || Boolean(parseError);

  const switchFormat = (next: SpecFormat) => {
    if (next === format || disabled) return;
    const parsed = parseSpecText(text, format);
    // A buffer that doesn't parse can't be re-printed; only the language
    // switches, and the error is reported when it is saved.
    if (parsed.status === 'parsed') onChange(serializeSpec(parsed.spec, next));
    onFormatChange(next);
  };

  return (
    <Box
      sx={(theme) => ({
        border: hairline(theme),
        borderColor: 'divider',
        borderRadius: 1,
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        overflow: 'hidden',
      })}
    >
      <Stack
        alignItems="center"
        direction="row"
        spacing={1}
        sx={{ borderBottom: 1, borderColor: 'divider', flexShrink: 0, justifyContent: 'flex-end', px: 2, py: 1 }}
      >
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.formatLabel)}
          color="primary"
          disabled={disabled}
          exclusive
          onChange={(_event, next: SpecFormat | null) => {
            if (next !== null) switchFormat(next);
          }}
          size="small"
          value={format}
        >
          <ToggleButton value="yaml">{FORMAT_LABELS.yaml}</ToggleButton>
          <ToggleButton value="json">{FORMAT_LABELS.json}</ToggleButton>
        </ToggleButtonGroup>
        {!editable && (
          <Button
            disabled={disabled}
            onClick={() => setIsEditing(true)}
            size="small"
            startIcon={<Pencil size={16} />}
            variant="outlined"
          >
            <FormattedMessage {...messages.edit} />
          </Button>
        )}
      </Stack>
      {parseError && (
        <Alert severity="error" sx={{ borderRadius: 0, flexShrink: 0 }}>
          <FormattedMessage
            {...messages.malformed}
            values={{ detail: parseError, format: FORMAT_LABELS[format] }}
          />
        </Alert>
      )}
      <Box sx={{ flex: 1, minHeight: 0 }}>
        <Suspense fallback={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}>
          <SpecCodeEditor format={format} onChange={onChange} readOnly={disabled || !editable} value={text} />
        </Suspense>
      </Box>
    </Box>
  );
}
