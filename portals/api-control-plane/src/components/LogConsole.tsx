/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
  type UIEvent,
} from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  Box,
  Button,
  Chip,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowDownToLine,
  Braces,
  Check,
  Copy,
  Eraser,
  WrapText,
} from '@wso2/oxygen-ui-icons-react';

import { useNotifications } from './Notifications';

/** One rendered console row. `raw` is offered behind the console's raw toggle. */
export type ConsoleLine = {
  /** Stable identity, used for de-duplication across polls and as the React key. */
  id: string;
  /** Upper-case level name; drives the row colour. */
  level: string;
  /** ISO timestamp, when the record carries one. */
  timestamp?: string;
  /** Human-readable rendering of the record. */
  message: string;
  /** Unprocessed payload, when it differs from `message`. */
  raw?: string;
};

/**
 * Console palette. A log console reads as terminal output in both themes, so
 * these are fixed rather than theme tokens. Every foreground clears 4.5:1
 * against `surface`.
 */
const consoleColors = {
  surface: '#0f1419',
  border: '#2a323d',
  text: '#d5dae1',
  timestamp: '#7d8794',
  selection: '#2d4f6e',
  level: {
    ERROR: '#ff7b72',
    WARN: '#e3b341',
    INFO: '#79c0ff',
    DEBUG: '#8b949e',
    DEFAULT: '#d5dae1',
  } as Record<string, string>,
};

/** Widest level name plus a trailing space, so messages line up in one column. */
const LEVEL_COLUMN = 6;

/** Timestamp column width: `YYYY-MM-DD HH:mm:ss.SSS`. */
const TIMESTAMP_COLUMN = 23;

/** Treat the viewport as "at the bottom" within this many pixels. */
const FOLLOW_THRESHOLD_PX = 24;

const pad = (value: number, size = 2) => String(value).padStart(size, '0');

/**
 * Renders a fixed-width local timestamp. Deliberately not locale-formatted:
 * console output is copied into tickets and scripts, so every row must have the
 * same shape and sort lexicographically.
 */
export const formatConsoleTimestamp = (value?: string): string => {
  const parsed = value ? new Date(value) : undefined;
  if (!parsed || Number.isNaN(parsed.getTime())) {
    return '-'.padEnd(TIMESTAMP_COLUMN);
  }
  const date = `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}`;
  const time = `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`;
  return `${date} ${time}.${pad(parsed.getMilliseconds(), 3)}`;
};

/** The exact text one row contributes to a selection or to "copy all". */
export const consoleLineText = (line: ConsoleLine, raw: boolean): string =>
  `${formatConsoleTimestamp(line.timestamp)} ${line.level.padEnd(LEVEL_COLUMN)}${
    raw && line.raw !== undefined ? line.raw : line.message
  }`;

/**
 * One console row.
 *
 * Column padding lives inside the text nodes rather than in flex or grid
 * columns, so a selection dragged across rows copies out with real spaces and
 * the same alignment that is on screen.
 */
function ConsoleRow({
  line,
  raw,
  wrap,
}: {
  line: ConsoleLine;
  raw: boolean;
  wrap: boolean;
}) {
  const timestamp = `${formatConsoleTimestamp(line.timestamp)} `;
  const level = line.level.padEnd(LEVEL_COLUMN);
  const body = raw && line.raw !== undefined ? line.raw : line.message;

  return (
    <Box
      sx={{
        overflowWrap: wrap ? 'anywhere' : 'normal',
        whiteSpace: wrap ? 'pre-wrap' : 'pre',
      }}
    >
      <Box component="span" sx={{ color: consoleColors.timestamp }}>
        {timestamp}
      </Box>
      <Box
        component="span"
        sx={{
          color:
            consoleColors.level[line.level] ?? consoleColors.level.DEFAULT,
        }}
      >
        {level}
      </Box>
      {body}
    </Box>
  );
}

export type LogConsoleProps = {
  lines: ConsoleLine[];
  /** Shown in place of rows when `lines` is empty. */
  emptyMessage: string;
  /** Accessible name for the scrollable output region. */
  label: string;
  /** Streams new rows in: enables auto-follow and the live announcement. */
  live?: boolean;
  /** Toolbar slot for owner-specific controls, e.g. a live-tail switch. */
  actions?: ReactNode;
  /** Enables the Clear control when provided. */
  onClear?: () => void;
  /** Height of the scrollable output region. */
  height?: number | string;
};

/**
 * Terminal-style log output: one row per record, native text selection, and
 * copy of exactly what is on screen.
 *
 * Rows are plain text nodes inside a `white-space: pre` block — no flex columns
 * and no virtualisation — because both would break selecting a range with the
 * mouse and copying it as usable text. The trade-off is that the owner must cap
 * how many lines it hands over; a few thousand rows is comfortable.
 */
export function LogConsole({
  lines,
  emptyMessage,
  label,
  live = false,
  actions,
  onClear,
  height = 460,
}: LogConsoleProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [wrap, setWrap] = useState(false);
  const [raw, setRaw] = useState(false);
  const [follow, setFollow] = useState(true);
  const [copied, setCopied] = useState(false);

  /*
   * Scrolling away from the bottom pins the view so a live tail cannot yank the
   * page out from under a selection; scrolling back re-arms the follow.
   */
  const handleScroll = (event: UIEvent<HTMLDivElement>) => {
    const node = event.currentTarget;
    setFollow(
      node.scrollHeight - node.scrollTop - node.clientHeight <=
        FOLLOW_THRESHOLD_PX
    );
  };

  const scrollToLatest = useCallback(() => {
    const node = scrollRef.current;
    if (!node) return;
    node.scrollTop = node.scrollHeight;
    setFollow(true);
  }, []);

  // Runs before paint so appended rows never flash at the old scroll offset.
  useLayoutEffect(() => {
    const node = scrollRef.current;
    if (!node || !follow) return;
    node.scrollTop = node.scrollHeight;
  }, [follow, lines, wrap, raw]);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copyAll = async () => {
    const text = lines.map((line) => consoleLineText(line, raw)).join('\n');
    try {
      if (!navigator.clipboard?.writeText) {
        throw new Error('Clipboard access is unavailable');
      }
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      // Blocked clipboard (insecure origin, denied permission) is expected.
      notify(
        intl.formatMessage({
          id: 'logConsole.copyFailed',
          defaultMessage:
            'Could not copy to the clipboard. Select the lines and copy them manually.',
        }),
        'error'
      );
    }
  };

  const hasRaw = lines.some((line) => line.raw !== undefined);

  return (
    <Paper
      sx={{ bgcolor: consoleColors.surface, borderColor: consoleColors.border }}
      variant="outlined"
    >
      <Stack
        alignItems="center"
        direction="row"
        spacing={1}
        sx={{
          borderBottom: '1px solid',
          borderColor: consoleColors.border,
          // Slotted `actions` inherit the console foreground rather than the
          // page theme, which would be dark-on-dark here.
          color: consoleColors.text,
          flexWrap: 'wrap',
          px: 1.5,
          py: 1,
        }}
      >
        <Typography sx={{ color: consoleColors.timestamp }} variant="caption">
          <FormattedMessage
            id="logConsole.lineCount"
            defaultMessage="{count, plural, one {# line} other {# lines}}"
            values={{ count: lines.length }}
          />
        </Typography>
        {live && (
          <Chip
            label={intl.formatMessage({
              id: 'logConsole.liveBadge',
              defaultMessage: 'Live',
            })}
            color="success"
            size="small"
            variant="outlined"
          />
        )}
        <Box sx={{ flex: 1 }} />
        {actions}
        {hasRaw && (
          <Tooltip
            title={intl.formatMessage({
              id: 'logConsole.toggleRaw',
              defaultMessage: 'Show raw log payload',
            })}
          >
            <IconButton
              aria-label={intl.formatMessage({
                id: 'logConsole.toggleRaw',
                defaultMessage: 'Show raw log payload',
              })}
              aria-pressed={raw}
              onClick={() => setRaw((value) => !value)}
              size="small"
              sx={{ color: raw ? consoleColors.text : consoleColors.timestamp }}
            >
              <Braces size={16} />
            </IconButton>
          </Tooltip>
        )}
        <Tooltip
          title={intl.formatMessage({
            id: 'logConsole.toggleWrap',
            defaultMessage: 'Wrap long lines',
          })}
        >
          <IconButton
            aria-label={intl.formatMessage({
              id: 'logConsole.toggleWrap',
              defaultMessage: 'Wrap long lines',
            })}
            aria-pressed={wrap}
            onClick={() => setWrap((value) => !value)}
            size="small"
            sx={{ color: wrap ? consoleColors.text : consoleColors.timestamp }}
          >
            <WrapText size={16} />
          </IconButton>
        </Tooltip>
        {onClear && (
          <Tooltip
            title={intl.formatMessage({
              id: 'logConsole.clear',
              defaultMessage: 'Clear console',
            })}
          >
            <IconButton
              aria-label={intl.formatMessage({
                id: 'logConsole.clear',
                defaultMessage: 'Clear console',
              })}
              disabled={lines.length === 0}
              onClick={onClear}
              size="small"
              sx={{ color: consoleColors.timestamp }}
            >
              <Eraser size={16} />
            </IconButton>
          </Tooltip>
        )}
        <Button
          disabled={lines.length === 0}
          onClick={copyAll}
          size="small"
          startIcon={copied ? <Check size={16} /> : <Copy size={16} />}
          sx={{ color: consoleColors.text, flexShrink: 0 }}
        >
          {copied ? (
            <FormattedMessage id="logConsole.copied" defaultMessage="Copied" />
          ) : (
            <FormattedMessage
              id="logConsole.copyAll"
              defaultMessage="Copy all"
            />
          )}
        </Button>
      </Stack>

      <Box sx={{ position: 'relative' }}>
        <Box
          aria-label={label}
          aria-live={live ? 'polite' : 'off'}
          onScroll={handleScroll}
          ref={scrollRef}
          role="log"
          tabIndex={0}
          sx={{
            color: consoleColors.text,
            fontFamily:
              'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 12.5,
            height,
            lineHeight: 1.6,
            overflow: 'auto',
            px: 1.5,
            py: 1,
            resize: 'vertical',
            userSelect: 'text',
            '&::selection, & *::selection': {
              bgcolor: consoleColors.selection,
            },
          }}
        >
          {lines.length === 0 ? (
            <Box sx={{ color: consoleColors.timestamp }}>{emptyMessage}</Box>
          ) : (
            lines.map((line) => (
              <ConsoleRow key={line.id} line={line} raw={raw} wrap={wrap} />
            ))
          )}
        </Box>
        {!follow && lines.length > 0 && (
          <Button
            onClick={scrollToLatest}
            size="small"
            startIcon={<ArrowDownToLine size={16} />}
            sx={{ bottom: 12, position: 'absolute', right: 20 }}
            variant="contained"
          >
            <FormattedMessage
              id="logConsole.jumpToLatest"
              defaultMessage="Jump to latest"
            />
          </Button>
        )}
      </Box>
    </Paper>
  );
}
