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
  Button,
  Drawer,
  IconButton,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Maximize2, Minimize2, Pencil } from '@wso2/oxygen-ui-icons-react';
import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { LoadingState } from '@/components/StateViews';
import { hairline } from '@/theme/receipes';
import {
  parseSpecText,
  serializeSpec,
  type SpecDocument,
  type SpecFormat,
} from '../utils/specText';
import { validateApiSpec, type SpecIssue } from '../utils/specValidation';
import { SpecIssueList } from './SpecIssueList';

/**
 * Monaco is the single heaviest thing this app can load, and nothing needs it
 * until someone actually opens the Source view; so it is split out into its
 * own chunk and fetched then, rather than riding along with the wizard.
 */
const SpecCodeEditor = lazy(() =>
  import('./SpecCodeEditor').then((module) => ({ default: module.SpecCodeEditor })),
);

const messages = defineMessages({
  cancel: {
    id: 'api.create.specSourceEditor.action.cancel',
    defaultMessage: 'Cancel',
    description: 'Leaves the editor and discards the edits made in it.',
  },
  collapse: {
    id: 'api.create.specSourceEditor.action.collapse',
    defaultMessage: 'Collapse editor',
    description: 'Closes the expanded side panel and returns the editor to the preview pane.',
  },
  edit: {
    id: 'api.create.specSourceEditor.action.edit',
    defaultMessage: 'Edit',
    description: 'Opens the definition’s own text for editing.',
  },
  editorLoading: {
    id: 'api.create.specSourceEditor.editorLoading',
    defaultMessage: 'Loading editor',
    description: 'Shown for the moment the code editor is being fetched.',
  },
  expand: {
    id: 'api.create.specSourceEditor.action.expand',
    defaultMessage: 'Expand editor',
    description: 'Opens the definition in a full-height side panel with more room.',
  },
  expandedTitle: {
    id: 'api.create.specSourceEditor.expandedTitle',
    defaultMessage: 'API definition',
    description: 'Heading of the expanded side panel holding the definition.',
  },
  formatLabel: {
    id: 'api.create.specSourceEditor.formatLabel',
    defaultMessage: 'Source format',
    description: 'Accessible name for the pair of buttons choosing JSON or YAML.',
  },
  inExpandedView: {
    id: 'api.create.specSourceEditor.inExpandedView',
    defaultMessage: 'This definition is open in the expanded editor.',
    description: 'Stands in for the editor in the pane while the side panel has it.',
  },
  malformed: {
    id: 'api.create.specSourceEditor.malformed',
    defaultMessage: 'This could not be read as {format}: {detail}',
    description:
      'Shown when edited text is not valid JSON/YAML. {format} is the format name, {detail} the parser’s own message naming the line.',
  },
  notAnObject: {
    id: 'api.create.specSourceEditor.notAnObject',
    defaultMessage: 'A definition has to be an object at the top level, not a list or a value.',
  },
  save: {
    id: 'api.create.specSourceEditor.action.save',
    defaultMessage: 'Save',
    description: 'Checks the edited definition and, if it holds up, adopts it.',
  },
});

/** The two formats the same document can be read and typed in. */
const FORMATS: { key: SpecFormat; label: string }[] = [
  // Format names, not prose — they read the same in every locale, so they are
  // deliberately outside `defineMessages`.
  { key: 'json', label: 'JSON' },
  { key: 'yaml', label: 'YAML' },
];

/**
 * How wide the expanded panel gets. Wide enough for a definition's longest
 * lines without wrapping, capped so it never covers the whole window on a
 * large screen; the wizard behind it stays visible as context.
 */
const EXPANDED_WIDTH = { md: 'min(1100px, 92vw)', xs: '100%' };

/** What the editor is currently complaining about, if anything. */
type EditorProblem =
  /** Syntax: the parser couldn't read the text at all. */
  | { format: SpecFormat; reason: string; kind: 'malformed' }
  /** Read, but the top level isn't an object. */
  | { kind: 'notAnObject' }
  /** Read and shaped right, but not a definition this step can use. */
  | { issues: SpecIssue[]; kind: 'invalid' };

export type SpecSourceEditorProps = {
  /**
   * Adopts an edited definition. Only ever called with one that has already
   * passed `validateApiSpec`, alongside the warnings that pass raised, so the
   * caller never has to re-run the check to find out what it now says.
   */
  onSave: (spec: SpecDocument, warnings: SpecIssue[]) => void;
  /** The definition as it currently stands. */
  spec: SpecDocument;
};

/**
 * The definition's own text, readable and editable.
 *
 * Reading and editing are the same editor in two modes rather than two
 * different widgets, so line numbers, folding and the format switch behave
 * identically either way; only typing is gated. The explicit Save is what gives
 * the re-check something to happen on, and it refuses anything
 * `validateApiSpec` calls an error; so the document behind the preview is only
 * ever one the rest of the wizard can work with. Warnings still pass, exactly
 * as they do for a freshly imported contract.
 */
export const SpecSourceEditor = ({ onSave, spec }: SpecSourceEditorProps) => {
  const intl = useIntl();
  const [editing, setEditing] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [format, setFormat] = useState<SpecFormat>('json');
  const [draft, setDraft] = useState('');
  const [problem, setProblem] = useState<EditorProblem | null>(null);

  // Serialize once per document and format; both are expensive and unchanged
  // between renders that touch neither.
  const readText = useMemo(() => serializeSpec(spec, format), [format, spec]);

  // A document replaced from outside; another contract fetched, the approach
  // switched; leaves any half-finished edit describing something that is no
  // longer on screen, so the editor closes rather than saving over it.
  useEffect(() => {
    setEditing(false);
    setProblem(null);
  }, [spec]);

  const startEditing = () => {
    setDraft(serializeSpec(spec, format));
    setProblem(null);
    setEditing(true);
  };

  const cancelEditing = () => {
    setEditing(false);
    setProblem(null);
  };

  /**
   * Switching format while editing re-prints what is in the buffer rather than
   * the last saved document, so edits survive the switch; text that doesn't
   * parse can't be re-printed at all, so the switch is refused and the syntax
   * error stands. In read mode there is no buffer to preserve — the document
   * is simply printed the other way.
   */
  const changeFormat = (next: SpecFormat) => {
    if (!editing) {
      setFormat(next);
      return;
    }

    const parsed = parseSpecText(draft, format);
    if (parsed.status === 'malformed') {
      setProblem({ format, kind: 'malformed', reason: parsed.reason });
      return;
    }
    if (parsed.status === 'notAnObject') {
      setProblem({ kind: 'notAnObject' });
      return;
    }
    setDraft(serializeSpec(parsed.spec, next));
    setProblem(null);
    setFormat(next);
  };

  const save = () => {
    const parsed = parseSpecText(draft, format);
    if (parsed.status === 'malformed') {
      setProblem({ format, kind: 'malformed', reason: parsed.reason });
      return;
    }
    if (parsed.status === 'notAnObject') {
      setProblem({ kind: 'notAnObject' });
      return;
    }

    // The same gate every imported contract passes, so a hand-edited
    // definition is held to exactly the standard a fetched one was.
    const validation = validateApiSpec(parsed.spec);
    if (validation.status === 'invalid') {
      setProblem({ issues: validation.issues, kind: 'invalid' });
      return;
    }

    setProblem(null);
    setEditing(false);
    onSave(parsed.spec, validation.warnings);
  };

  /**
   * The controls, built once and rendered in whichever of the two places
   * currently holds the editor. Two copies would put two "Save" buttons on the
   * page, which is a problem for anyone reaching them by name.
   */
  const toolbar = (
    <Stack
      direction="row"
      spacing={1}
      sx={{ alignItems: 'center', flexShrink: 0, justifyContent: 'space-between' }}
    >
      <ToggleButtonGroup
        aria-label={intl.formatMessage(messages.formatLabel)}
        exclusive
        onChange={(_event, next: SpecFormat | null) => {
          // `exclusive` reports null when the active button is clicked again;
          // keep the current format rather than clearing it.
          if (next !== null) {
            changeFormat(next);
          }
        }}
        size="small"
        value={format}
      >
        {FORMATS.map((candidate) => (
          <ToggleButton key={candidate.key} value={candidate.key}>
            {candidate.label}
          </ToggleButton>
        ))}
      </ToggleButtonGroup>

      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        {editing ? (
          <>
            <Button onClick={cancelEditing} size="small" type="button" variant="text">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button onClick={save} size="small" type="button" variant="contained">
              <FormattedMessage {...messages.save} />
            </Button>
          </>
        ) : (
          <Button
            onClick={startEditing}
            size="small"
            startIcon={<Pencil size={16} />}
            type="button"
            variant="outlined"
          >
            <FormattedMessage {...messages.edit} />
          </Button>
        )}

        <Tooltip title={intl.formatMessage(expanded ? messages.collapse : messages.expand)}>
          <IconButton
            aria-label={intl.formatMessage(expanded ? messages.collapse : messages.expand)}
            onClick={() => setExpanded(!expanded)}
            size="small"
          >
            {expanded ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
          </IconButton>
        </Tooltip>
      </Stack>
    </Stack>
  );

  const problemAlert =
    problem === null ? null : (
      <Alert severity="error" sx={{ flexShrink: 0 }}>
        {problem.kind === 'invalid' ? <SpecIssueList issues={problem.issues} /> : null}
        {problem.kind === 'notAnObject' ? <FormattedMessage {...messages.notAnObject} /> : null}
        {problem.kind === 'malformed' ? (
          <FormattedMessage
            {...messages.malformed}
            // Both ride in as data: the format is a name, and the reason is the
            // parser's message about the user's own text.
            values={{
              detail: problem.reason,
              format: problem.format === 'yaml' ? 'YAML' : 'JSON',
            }}
          />
        ) : null}
      </Alert>
    );

  /**
   * The editor itself. Expanding re-parents it, which Monaco can only survive
   * as a remount; the text is React state and comes through untouched, but the
   * undo stack and cursor start fresh on each side.
   */
  const body = (
    <Box
      sx={(theme) => ({
        border: hairline(theme),
        borderColor: 'divider',
        borderRadius: 1,
        flex: 1,
        minHeight: 0,
        // Monaco draws its own scrollbars right up to the edge; the radius
        // above only reads as one if the corners actually clip.
        overflow: 'hidden',
      })}
    >
      <Suspense fallback={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}>
        <SpecCodeEditor
          format={format}
          minimap={expanded}
          onChange={setDraft}
          readOnly={!editing}
          value={editing ? draft : readText}
        />
      </Suspense>
    </Box>
  );

  return (
    <>
      <Stack spacing={1} sx={{ height: '100%', minHeight: 0 }}>
        {expanded ? (
          // The panel has the editor and the only copy of the controls; the
          // pane says where they went rather than showing a dead duplicate.
          <Stack sx={{ alignItems: 'center', flex: 1, justifyContent: 'center', p: 2 }}>
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.inExpandedView} />
            </Typography>
          </Stack>
        ) : (
          <>
            {toolbar}
            {problemAlert}
            {body}
          </>
        )}
      </Stack>

      <Drawer
        anchor="right"
        onClose={() => setExpanded(false)}
        open={expanded}
        slotProps={{ paper: { sx: { width: EXPANDED_WIDTH } } }}
      >
        {/* Mounted only while open, so exactly one editor exists at a time. */}
        {expanded ? (
          <Stack spacing={2} sx={{ height: '100%', minHeight: 0, p: 3 }}>
            <Typography sx={{ flexShrink: 0, fontWeight: 700 }} variant="h6">
              <FormattedMessage {...messages.expandedTitle} />
            </Typography>
            {toolbar}
            {problemAlert}
            {body}
          </Stack>
        ) : null}
      </Drawer>
    </>
  );
};
