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
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type FC,
  type ReactNode,
  type UIEvent,
} from 'react';
import { Box, Button, Chip, IconButton, Paper, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import {
  ArrowDownToLine,
  Braces,
  Check,
  Copy,
  Eraser,
  WrapText,
} from '@wso2/oxygen-ui-icons-react';

/** One rendered console row. `raw` is offered behind the console's raw toggle. */
export type ConsoleLine = {
  /** Stable identity, used to de-duplicate across polls and as the React key. */
  id: string;
  /** Upper-case level name; drives the row colour. */
  level: string;
  /** RFC 3339, when the record carries one. */
  timestamp?: string;
  /** Which workload wrote the line. The column disappears when no row has one;
   * this view is org-wide, so without it gateways are indistinguishable. */
  source?: string;
  /** Human-readable rendering of the record. */
  message: string;
  /** Unprocessed payload, when it differs from `message`. */
  raw?: string;
};

/**
 * Fixed rather than theme tokens: a log console reads as terminal output in both
 * themes. Every foreground clears 4.5:1 against `surface`.
 */
const consoleColors = {
  surface: '#0f1419',
  border: '#2a323d',
  text: '#d5dae1',
  dim: '#7d8794',
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

/** How much of the source column a long pod name may take before it is cut. */
const SOURCE_COLUMN_MAX = 24;

/** Treat the viewport as "at the bottom" within this many pixels. */
const FOLLOW_THRESHOLD_PX = 24;

const pad = (value: number, size = 2) => String(value).padStart(size, '0');

/**
 * A fixed-width local timestamp, not locale-formatted: console output is copied
 * into tickets and scripts, so rows must share a shape and sort lexically.
 */
export function formatConsoleTimestamp(value?: string): string {
  const parsed = value ? new Date(value) : undefined;
  if (!parsed || Number.isNaN(parsed.getTime())) return '-'.padEnd(TIMESTAMP_COLUMN);
  const date = `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}`;
  const time = `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`;
  return `${date} ${time}.${pad(parsed.getMilliseconds(), 3)}`;
}

/**
 * The source column, padded to the settled width and cut from the right — a pod
 * name's prefix names the workload, its tail is a scheduler hash.
 */
export function formatConsoleSource(source: string | undefined, width: number): string {
  if (width === 0) return '';
  const text = source ?? '';
  return (text.length > width ? `${text.slice(0, width - 1)}…` : text).padEnd(width) + ' ';
}

/** The exact text one row contributes to a selection or to "copy all". */
export function consoleLineText(line: ConsoleLine, raw: boolean, sourceWidth = 0): string {
  const body = raw && line.raw !== undefined ? line.raw : line.message;
  return `${formatConsoleTimestamp(line.timestamp)} ${formatConsoleSource(
    line.source,
    sourceWidth
  )}${line.level.padEnd(LEVEL_COLUMN)}${body}`;
}

/**
 * One console row. Column padding lives inside the text nodes rather than in
 * flex or grid columns, so a selection dragged across rows copies out with real
 * spaces and the alignment that is on screen.
 */
const ConsoleRow: FC<{
  line: ConsoleLine;
  raw: boolean;
  wrap: boolean;
  sourceWidth: number;
}> = ({ line, raw, wrap, sourceWidth }) => {
  const body = raw && line.raw !== undefined ? line.raw : line.message;

  return (
    <Box
      sx={{
        overflowWrap: wrap ? 'anywhere' : 'normal',
        whiteSpace: wrap ? 'pre-wrap' : 'pre',
      }}
    >
      <Box component="span" sx={{ color: consoleColors.dim }}>
        {`${formatConsoleTimestamp(line.timestamp)} `}
      </Box>
      {sourceWidth > 0 ? (
        <Box component="span" sx={{ color: consoleColors.dim }}>
          {formatConsoleSource(line.source, sourceWidth)}
        </Box>
      ) : null}
      <Box
        component="span"
        sx={{ color: consoleColors.level[line.level] ?? consoleColors.level.DEFAULT }}
      >
        {line.level.padEnd(LEVEL_COLUMN)}
      </Box>
      {body}
    </Box>
  );
};

export type LogConsoleProps = {
  lines: ConsoleLine[];
  /** Shown in place of rows when `lines` is empty. */
  emptyMessage: string;
  /** Accessible name for the scrollable output region. */
  label: string;
  /** Streams new rows in: enables auto-follow and the live announcement. */
  live?: boolean;
  /** Live, but the last poll failed — the rows on screen are no longer current. */
  stale?: boolean;
  /** Toolbar slot for owner-specific controls, e.g. a live-tail switch. */
  actions?: ReactNode;
  /** Enables the Clear control when provided. */
  onClear?: () => void;
  /** Told when the clipboard refused, so the host can raise its own notice. */
  onCopyError?: (message: string) => void;
  /** Height of the scrollable output region. */
  height?: number | string;
};

/**
 * Terminal-style log output with native text selection. Rows are plain text
 * nodes in a `white-space: pre` block — no flex columns, no virtualisation —
 * because both break dragging a selection across rows and copying it as usable
 * text. So the owner must cap what it hands over; a few thousand rows is fine.
 */
const LogConsole: FC<LogConsoleProps> = ({
  lines,
  emptyMessage,
  label,
  live = false,
  stale = false,
  actions,
  onClear,
  onCopyError,
  height = 460,
}) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [wrap, setWrap] = useState(false);
  const [raw, setRaw] = useState(false);
  const [follow, setFollow] = useState(true);
  const [copied, setCopied] = useState(false);

  // Wide enough for the longest source on screen, so the message column stays
  // straight. Zero drops the column rather than indenting past an empty gap.
  const sourceWidth = useMemo(() => {
    let widest = 0;
    for (const line of lines) {
      if (line.source && line.source.length > widest) widest = line.source.length;
    }
    return Math.min(widest, SOURCE_COLUMN_MAX);
  }, [lines]);

  /*
   * Scrolling away from the bottom pins the view so a live tail cannot yank the
   * page out from under a selection; scrolling back re-arms the follow.
   */
  const handleScroll = (event: UIEvent<HTMLDivElement>) => {
    const node = event.currentTarget;
    setFollow(node.scrollHeight - node.scrollTop - node.clientHeight <= FOLLOW_THRESHOLD_PX);
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
    if (!copied) return undefined;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copyAll = async () => {
    const text = lines.map((line) => consoleLineText(line, raw, sourceWidth)).join('\n');
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard access is unavailable');
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      // Blocked clipboard (insecure origin, denied permission) is expected.
      onCopyError?.('Could not copy to the clipboard. Select the lines and copy them manually.');
    }
  };

  const hasRaw = lines.some((line) => line.raw !== undefined);

  return (
    <Paper
      variant="outlined"
      sx={{ bgcolor: consoleColors.surface, borderColor: consoleColors.border }}
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
        <Typography variant="caption" sx={{ color: consoleColors.dim }}>
          {lines.length === 1 ? '1 line' : `${lines.length} lines`}
        </Typography>
        {/* A failed poll leaves the rows on screen; saying "Live" over them
            would claim they are current. */}
        {live ? (
          <Chip
            label={stale ? 'Not updating' : 'Live'}
            color={stale ? 'warning' : 'success'}
            size="small"
            variant="outlined"
          />
        ) : null}
        <Box sx={{ flex: 1 }} />
        {actions}
        {hasRaw ? (
          <Tooltip title="Show raw log payload">
            <IconButton
              aria-label="Show raw log payload"
              aria-pressed={raw}
              onClick={() => setRaw((value) => !value)}
              size="small"
              sx={{ color: raw ? consoleColors.text : consoleColors.dim }}
            >
              <Braces size={16} />
            </IconButton>
          </Tooltip>
        ) : null}
        <Tooltip title="Wrap long lines">
          <IconButton
            aria-label="Wrap long lines"
            aria-pressed={wrap}
            onClick={() => setWrap((value) => !value)}
            size="small"
            sx={{ color: wrap ? consoleColors.text : consoleColors.dim }}
          >
            <WrapText size={16} />
          </IconButton>
        </Tooltip>
        {onClear ? (
          <Tooltip title="Clear console">
            <IconButton
              aria-label="Clear console"
              disabled={lines.length === 0}
              onClick={onClear}
              size="small"
              sx={{ color: consoleColors.dim }}
            >
              <Eraser size={16} />
            </IconButton>
          </Tooltip>
        ) : null}
        <Button
          disabled={lines.length === 0}
          onClick={copyAll}
          size="small"
          startIcon={copied ? <Check size={16} /> : <Copy size={16} />}
          sx={{ color: consoleColors.text, flexShrink: 0 }}
        >
          {copied ? 'Copied' : 'Copy all'}
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
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 12.5,
            height,
            lineHeight: 1.6,
            overflow: 'auto',
            px: 1.5,
            py: 1,
            resize: 'vertical',
            userSelect: 'text',
            '&::selection, & *::selection': { bgcolor: consoleColors.selection },
          }}
        >
          {lines.length === 0 ? (
            <Box sx={{ color: consoleColors.dim }}>{emptyMessage}</Box>
          ) : (
            lines.map((line) => (
              <ConsoleRow
                key={line.id}
                line={line}
                raw={raw}
                sourceWidth={sourceWidth}
                wrap={wrap}
              />
            ))
          )}
        </Box>
        {!follow && lines.length > 0 ? (
          <Button
            onClick={scrollToLatest}
            size="small"
            startIcon={<ArrowDownToLine size={16} />}
            sx={{ bottom: 12, position: 'absolute', right: 20 }}
            variant="contained"
          >
            Jump to latest
          </Button>
        ) : null}
      </Box>
    </Paper>
  );
};

export default LogConsole;
