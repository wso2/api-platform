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

import Editor from '@monaco-editor/react';
import {
  Alert,
  Box,
  FormControlLabel,
  Stack,
  Switch,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { useEffect, useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import SwaggerSpecViewer from '@/components/SwaggerSpecViewer';
import { ResourcePreviewPlaceholder } from '../../components/ResourcePreviewPlaceholder';
import { serializeSpec, type SpecDocument, type SpecFormat } from '../utils/specText';
import type { SpecIssue } from '../utils/specValidation';
import { SpecIssueList } from './SpecIssueList';
import { SpecSourceEditor } from './SpecSourceEditor';

const messages = defineMessages({
  editorLoading: {
    id: 'api.create.apiResourcesPreview.editorLoading',
    defaultMessage: 'Loading editor…',
    description: 'Placeholder shown while Monaco editor initialises.',
  },
  formatLabel: {
    id: 'api.create.apiResourcesPreview.formatLabel',
    defaultMessage: 'Source format',
    description: 'Accessible name for the YAML / JSON toggle buttons.',
  },
  source: {
    id: 'api.create.apiResourcesPreview.source',
    defaultMessage: 'Source',
    description: "Toggle that swaps the rendered resources for the definition's own text.",
  },
  title: {
    id: 'api.create.apiResourcesPreview.title',
    defaultMessage: 'API resources',
  },
});

/**
 * How tall the pane is allowed to get. Bounded rather than content-sized: a
 * definition with fifty operations would otherwise run far past the form beside
 * it and take the whole page's scrollbar with it. The clamp keeps it usable on
 * a laptop screen without leaving a stubby box on a tall one.
 */
const PANE_HEIGHT = 'clamp(420px, calc(100vh - 260px), 560px)';

/** Returns 'json' when rawText starts with `{`, otherwise 'yaml'. */
const detectFormat = (rawText: string | undefined): SpecFormat =>
  rawText !== undefined && rawText.trimStart().startsWith('{') ? 'json' : 'yaml';

export type ApiResourcesPreviewProps = {
  /** Optional fixed pane height for layouts that must align with an adjacent state. */
  height?: number | string;
  /**
   * Called with the serialized spec text before the editor save is committed.
   * Return a non-empty array to block the save and display the messages inline;
   * return null or an empty array to proceed. When absent the editor skips
   * backend validation and relies on the frontend check alone.
   */
  onBeforeSave?: (specText: string) => Promise<string[] | null>;
  /** Forwarded from SpecSourceEditor — fired when edit mode opens or closes. */
  onEditingChange?: (isEditing: boolean) => void;
  /**
   * Adopts a definition edited in the Source view. Supplying it is what makes
   * the Source view editable at all; without it the pane stays read-only.
   * `rawText` is the exact text the user approved (preserving YAML/JSON format).
   */
  onSpecChange?: (spec: SpecDocument, rawText: string) => void;
  /**
   * The original uploaded or downloaded spec text. When present the Source
   * view shows exactly what the user gave us — preserving YAML format,
   * comments, and anchors — rather than a re-serialized copy.
   */
  rawText?: string;
  /**
   * The fetched definition, as a parsed object rather than a URL, so the viewer
   * never re-downloads the document and the Source view prints the same object
   * the resources are drawn from. It is not a guarantee of no network activity:
   * swagger-client resolves `$ref`s while rendering, and `specValidation`
   * reports an external `$ref` as a warning rather than rejecting it, so a
   * document naming remote refs can have the preview fetch from whatever host
   * they point at. Nothing else here issues a request. try-it-out is off.
   */
  spec?: SpecDocument;
  /**
   * What the current definition's own check says about it. Only passed once the
   * document has been edited here; an unedited contract's warnings belong to
   * the import, and are reported beside the source it was imported from.
   */
  warnings?: SpecIssue[];
};

/**
 * Right-hand pane of the contract step: the resources of the fetched
 * definition, or an empty state saying that is what will land here.
 */
export const ApiResourcesPreview = ({
  height = PANE_HEIGHT,
  onBeforeSave,
  onEditingChange,
  onSpecChange,
  rawText,
  spec,
  warnings,
}: ApiResourcesPreviewProps) => {
  const intl = useIntl();
  const [showSource, setShowSource] = useState(false);
  const hasContract = spec !== undefined;
  const editable = hasContract && onSpecChange !== undefined;

  // Default format is derived from the spec's own format. Reset it whenever a
  // new contract lands so the toggle tracks the new file rather than the old one.
  const [format, setFormat] = useState<SpecFormat>(() => detectFormat(rawText));
  useEffect(() => {
    setFormat(detectFormat(rawText));
  }, [rawText]);

  // Text for the read-only Monaco editor. Prefer rawText when it already
  // matches the chosen format (preserves YAML comments, anchors, and layout).
  const displayText = useMemo((): string => {
    if (spec === undefined) return '';
    const rawIsYaml = rawText !== undefined && !rawText.trimStart().startsWith('{');
    // Preserve rawText only for YAML — comments, anchors, and key order survive.
    // JSON is always re-serialized so it comes out pretty-printed regardless of
    // whether the uploaded file was minified.
    if (format === 'yaml' && rawIsYaml) return rawText!;
    return serializeSpec(spec, format);
  }, [format, rawText, spec]);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height,
        // Keep the title fixed; `minHeight: 0` lets the content area shrink.
        minHeight: 0,
        overflow: 'hidden',
      }}
    >
      {hasContract ? (
        <Stack
          direction="row"
          spacing={2}
          sx={{
            alignItems: 'center',
            flexShrink: 0,
            justifyContent: 'space-between',
          }}
        >
          {/* Left: title + YAML/JSON toggle (only visible in read-only source mode) */}
          <Stack alignItems="center" direction="row" spacing={1}>
            <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
              <FormattedMessage {...messages.title} />
            </Typography>
            {showSource && !editable && (
              <ToggleButtonGroup
                aria-label={intl.formatMessage(messages.formatLabel)}
                color="primary"
                exclusive
                onChange={(_event, next: SpecFormat | null) => {
                  if (next !== null) setFormat(next);
                }}
                size="small"
                value={format}
              >
                <ToggleButton value="yaml">YAML</ToggleButton>
                <ToggleButton value="json">JSON</ToggleButton>
              </ToggleButtonGroup>
            )}
          </Stack>

          {/* Right: Source switch */}
          <FormControlLabel
            control={
              <Switch
                checked={showSource}
                onChange={(event) => setShowSource(event.target.checked)}
                size="small"
                // MUI v9 routes input attributes through slotProps; the older
                // `inputProps` never reaches the element, leaving the control
                // without an accessible name.
                slotProps={{
                  input: { 'aria-label': intl.formatMessage(messages.source) },
                }}
              />
            }
            label={<FormattedMessage {...messages.source} />}
            labelPlacement="start"
            sx={{ m: 0 }}
          />
        </Stack>
      ) : null}

      {/* The edited definition's own verdict, above both views because it
          describes the document rather than either way of looking at it. */}
      {hasContract && (warnings?.length ?? 0) > 0 ? (
        <Alert severity="warning" sx={{ flexShrink: 0, mt: 1 }}>
          <SpecIssueList issues={warnings ?? []} />
        </Alert>
      ) : null}

      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          mt: hasContract ? 1 : 0,
          // Monaco manages its own scrolling; the SwaggerSpecViewer is a block
          // this box has to scroll for.
          overflow: showSource ? 'hidden' : 'auto',
        }}
      >
        {/* Editable source view: SpecSourceEditor owns format toggle + save bar. */}
        {hasContract && showSource && editable ? (
          <SpecSourceEditor
            onBeforeSave={onBeforeSave}
            onEditingChange={onEditingChange}
            onSave={onSpecChange}
            rawText={rawText}
            spec={spec}
          />
        ) : null}

        {/* Read-only source view: Monaco editor, format toggled in the header above. */}
        {hasContract && showSource && !editable ? (
          <Editor
            height="100%"
            language={format}
            loading={
              <Box sx={{ bgcolor: '#1e1e1e', height: '100%', p: 2 }}>
                <Typography color="text.disabled" variant="body2">
                  {intl.formatMessage(messages.editorLoading)}
                </Typography>
              </Box>
            }
            options={{
              automaticLayout: true,
              fontSize: 12,
              lineHeight: 20,
              minimap: { enabled: false },
              readOnly: true,
              scrollBeyondLastLine: false,
              wordWrap: 'on',
            }}
            theme="vs-dark"
            value={displayText}
          />
        ) : null}

        {hasContract && !showSource ? (
          <Box
            sx={{
              // Swagger UI ships its own canvas; keep it from fighting the
              // pane's background.
              '& .swagger-ui': { bgcolor: 'transparent' },
            }}
          >
            {/* The shared viewer, in its read-only shape: the info block and
                the servers/authorize strip belong to a try-it-out console,
                which this preview is not, and the wizard has no backend to
                call yet. Its own resource search replaces Swagger's tag
                filter — a path/description match reads better in a pane this
                narrow than a list of tags. */}
            <SwaggerSpecViewer
              disableTryOutBtn
              displayRequestDuration={false}
              enableResourceSearch
              hideAuthorizeButton
              hideInfoSection
              hideServers
              spec={spec}
            />
          </Box>
        ) : null}

        {hasContract ? null : <ResourcePreviewPlaceholder />}
      </Box>
    </Box>
  );
};
