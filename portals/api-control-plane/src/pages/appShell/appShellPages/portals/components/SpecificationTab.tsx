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

import { lazy, Suspense, useDeferredValue, useMemo } from 'react';
import {
  Alert,
  Box,
  Grid,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { LoadingState } from '@/components/StateViews';
import SwaggerSpecViewer from '@/components/SwaggerSpecViewer';
import { parseSpecText, serializeSpec, type SpecFormat } from '../../apis/create/utils/specText';
import type { DefinitionFormat } from '../utils/storedDefinition';

/**
 * Same lazy split as `SpecSourceEditor`, and deliberately the same module:
 * Monaco is the heaviest thing this app loads, and one shared chunk beats two
 * copies of its wiring. Keep this a `lazy()`/`import()` — a static import
 * would put the whole editor back into this page's chunk.
 */
const CodeEditor = lazy(() =>
  import('@/components/CodeEditor/CodeEditor').then((module) => ({ default: module.CodeEditor })),
);

/** Each pane's height once the two stack on a narrow screen; side by side they fill the available height. */
const PANE_HEIGHT_XS = 480;

const messages = defineMessages({
  resourcesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.resourcesTitle',
    defaultMessage: 'Resources',
    description: "Heading of the pane listing the definition's operations, beside the editor.",
  },
  resourcesEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.resourcesEmpty',
    defaultMessage: 'No resources to show. Fix the definition on the left to see them here.',
    description: "Shown in the resources pane when the definition can't be read or has no paths.",
  },
  editorLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.editorLabel',
    defaultMessage: 'API definition ({format})',
    description:
      'Accessible name for the editor holding the definition\u2019s own text. {format} is JSON, YAML or GraphQL.',
  },
  editorLoading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.editorLoading',
    defaultMessage: 'Loading editor',
  },
  malformed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.malformed',
    defaultMessage: 'This is not valid {format}: {detail}',
    description:
      '{format} is JSON, YAML or GraphQL; {detail} is the parser’s own message naming the line.',
  },
  formatLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SpecificationTab.formatLabel',
    defaultMessage: 'Source format',
    description: 'Accessible name for the YAML / JSON toggle buttons.',
  },
});

/** Format names are the same in every language. */
const FORMAT_LABELS: Record<DefinitionFormat, string> = {
  graphql: 'GraphQL',
  json: 'JSON',
  yaml: 'YAML',
};

export type SpecificationTabProps = {
  disabled?: boolean;
  /**
   * The serialization the text is in, which is also the one it is parsed as on
   * save. A GraphQL SDL has only one, so it shows no JSON/YAML switch.
   */
  format: DefinitionFormat;
  /**
   * Every keystroke, or a format switch that re-prints the text, with the buffer's full text.
   * Never called when `readOnly`: the caller re-prints the text itself on `onFormatChange`.
   */
  onChange?: (text: string) => void;
  /** A JSON/YAML switch; never called for a GraphQL SDL, which has no other format. */
  onFormatChange?: (format: SpecFormat) => void;
  /** The parser's own complaint, when the current buffer doesn't read in its format. */
  parseError?: string;
  /** Shows the definition without letting it be edited, for a version that can't be changed. */
  readOnly?: boolean;
  /** The definition's raw text, in `format`. */
  text: string;
};

/**
 * "Specification" — the draft's OpenAPI definition (`.../draft/definition`).
 * The source panel of the API Definition page, reduced to what publishing needs:
 * a JSON/YAML switch and the editor. It is editable unless `readOnly`, since
 * opening the draft is already the choice to edit; import, download and the
 * resources view stay with the API Definition page, and the definition is saved
 * with the rest of the draft, so there is no Save here.
 */
export function SpecificationTab({
  disabled,
  format,
  onChange,
  onFormatChange,
  parseError,
  readOnly,
  text,
}: SpecificationTabProps) {
  const intl = useIntl();

  // The resources pane follows the buffer, a beat behind typing so the viewer isn't rebuilt per keystroke.
  const deferredText = useDeferredValue(text);
  const spec = useMemo(() => {
    if (format === 'graphql') return undefined;
    const parsed = parseSpecText(deferredText, format);
    if (parsed.status !== 'parsed') return undefined;
    const paths = parsed.spec.paths;
    return paths && typeof paths === 'object' && Object.keys(paths).length > 0
      ? parsed.spec
      : undefined;
  }, [deferredText, format]);
  const showResources = format !== 'graphql';

  const switchFormat = (next: SpecFormat) => {
    if (next === format || disabled || format === 'graphql') return;
    const parsed = parseSpecText(text, format);
    // A buffer that doesn't parse can't be re-printed; only the language
    // switches, and the error is reported when it is saved.
    if (parsed.status === 'parsed' && !readOnly) onChange?.(serializeSpec(parsed.spec, next));
    onFormatChange?.(next);
  };

  return (
    <Grid container sx={{ height: { md: '100%', xs: 'auto' }, overflow: { md: 'hidden' } }}>
      <Grid
        size={{ md: showResources ? 6 : 12, xs: 12 }}
        sx={{ height: { md: '100%', xs: PANE_HEIGHT_XS }, minHeight: 0, minWidth: 0 }}
      >
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            height: '100%',
            minHeight: 0,
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          {/* A GraphQL SDL has a single serialization, so there is nothing to switch between. */}
          {format !== 'graphql' && (
            <Stack
              alignItems="center"
              direction="row"
              spacing={1}
              sx={{
                borderBottom: 1,
                borderColor: 'divider',
                flexShrink: 0,
                justifyContent: 'flex-end',
                px: 2,
                py: 1,
              }}
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
            </Stack>
          )}
          {parseError && (
            <Alert severity="error" sx={{ borderRadius: 0, flexShrink: 0 }}>
              <FormattedMessage
                {...messages.malformed}
                values={{ detail: parseError, format: FORMAT_LABELS[format] }}
              />
            </Alert>
          )}
          <Box sx={{ flex: 1, minHeight: 0 }}>
            <Suspense
              fallback={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}
            >
              <CodeEditor
                ariaLabel={intl.formatMessage(messages.editorLabel, {
                  format: FORMAT_LABELS[format],
                })}
                language={format}
                onChange={onChange}
                readOnly={disabled || readOnly}
                value={text}
              />
            </Suspense>
          </Box>
        </Box>
      </Grid>
      {showResources && (
        <Grid
          size={{ md: 6, xs: 12 }}
          sx={{
            borderColor: 'divider',
            borderLeft: { md: 1 },
            borderTop: { md: 0, xs: 1 },
            display: 'flex',
            flexDirection: 'column',
            height: { md: '100%', xs: PANE_HEIGHT_XS },
            minHeight: 0,
            minWidth: 0,
            '& .swagger-ui': { bgcolor: 'transparent' },
          }}
        >
          <Box sx={{ borderBottom: 1, borderColor: 'divider', flexShrink: 0, px: 2, py: 1.5 }}>
            <Typography sx={{ fontWeight: 600 }} variant="subtitle1">
              <FormattedMessage {...messages.resourcesTitle} />
            </Typography>
          </Box>
          <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 2, py: 1 }}>
            {spec ? (
              <SwaggerSpecViewer
                disableTryOutBtn
                displayRequestDuration={false}
                enableResourceSearch
                hideAuthorizeButton
                hideInfoSection
                hideServers
                hideTagHeaders
                spec={spec}
              />
            ) : (
              <Typography color="text.secondary" sx={{ py: 2 }} variant="body2">
                <FormattedMessage {...messages.resourcesEmpty} />
              </Typography>
            )}
          </Box>
        </Grid>
      )}
    </Grid>
  );
}
