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
  memo,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type FC,
  type ReactNode,
  type UIEvent,
} from 'react';
import {
  Box,
  CircularProgress,
  Collapse,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronRight, Copy, Download, WrapText } from '@wso2/oxygen-ui-icons-react';
import { orderLines } from './consoleLines';

/** One label/value pair in a row's expanded detail. */
export type ConsoleDetail = { label: string; value: string };

/** One rendered console row. `raw` is the unprocessed payload, shown when expanded. */
export type ConsoleLine = {
  /** Stable identity, used to de-duplicate across polls and as the React key. */
  id: string;
  /** Upper-case level name; drives the chip and the row colour. */
  level: string;
  /** RFC 3339, when the record carries one. */
  timestamp?: string;
  /** Which workload wrote the line. This view is org-wide, so without it
   * gateways are indistinguishable. */
  source?: string;
  /** Human-readable rendering of the record. */
  message: string;
  /** Unprocessed payload, when it differs from `message`. */
  raw?: string;
  /** Shown as a grid when the row is expanded. */
  details?: ConsoleDetail[];
};

/**
 * Fixed rather than theme tokens: a log console reads as terminal output in both
 * themes. Every foreground clears 4.5:1 against `surface`.
 */
const consoleColors = {
  surface: '#0f1419',
  raised: '#161c24',
  border: '#2a323d',
  text: '#d5dae1',
  dim: '#7d8794',
  selection: '#2d4f6e',
  level: {
    ERROR: '#ff7b72',
    WARN: '#e3b341',
    INFO: '#79c0ff',
    DEBUG: '#8b949e',
    LOG: '#8b949e',
    DEFAULT: '#d5dae1',
  } as Record<string, string>,
  /** Chip fill and row wash, keyed the same way as the foregrounds. */
  levelFill: {
    ERROR: 'rgba(248, 81, 73, 0.18)',
    WARN: 'rgba(227, 179, 65, 0.16)',
    INFO: 'rgba(56, 139, 253, 0.18)',
    DEBUG: 'rgba(139, 148, 158, 0.16)',
    DEFAULT: 'rgba(139, 148, 158, 0.16)',
  } as Record<string, string>,
  levelWash: {
    ERROR: 'rgba(248, 81, 73, 0.06)',
    WARN: 'rgba(227, 179, 65, 0.05)',
  } as Record<string, string>,
  /** Wash behind the row whose detail is open. */
  open: 'rgba(255,255,255,0.03)',
};

/** Widest level name plus a trailing space, so messages line up in one column. */
const LEVEL_COLUMN = 6;

/** Timestamp column width: `YYYY-MM-DD HH:mm:ss.SSS`. */
const TIMESTAMP_COLUMN = 23;

/** How much of the source column a long pod name may take before it is cut. */
const SOURCE_COLUMN_MAX = 24;

/** Treat the viewport as "at the latest edge" within this many pixels. */
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

/** Just the clock part, which is what a row shows — the date is in the detail. */
export function formatRowTime(value?: string): string {
  const full = formatConsoleTimestamp(value);
  return full.trim().includes(' ') ? full.split(' ')[1] : full.trim();
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

/** The exact text one row contributes to a copy or a download. */
export function consoleLineText(line: ConsoleLine, raw: boolean, sourceWidth = 0): string {
  const body = raw && line.raw !== undefined ? line.raw : line.message;
  return `${formatConsoleTimestamp(line.timestamp)} ${formatConsoleSource(
    line.source,
    sourceWidth
  )}${line.level.padEnd(LEVEL_COLUMN)}${body}`;
}

/**
 * Row grid: disclosure, time, level, source, message.
 *
 * The message track is what decides whether a long line can be read. `1fr`
 * makes every row exactly as wide as the console and clips the overflow, so a
 * line longer than the pane simply ends in an ellipsis and the rest is
 * unreachable. `max-content` lets the row grow past the pane instead, which is
 * what gives the scroll container something to scroll sideways.
 */
const rowColumns = (wrap: boolean, fixedColumns: string) =>
  `${fixedColumns} ${wrap ? 'minmax(0, 1fr)' : 'max-content'}`;

const ConsoleRow: FC<{
  line: ConsoleLine;
  wrap: boolean;
  expanded: boolean;
  onToggle: (id: string) => void;
  actions?: ReactNode;
  /**
   * The pane's own visible width. The detail panel is pinned to the left edge
   * and held to this, so it stays put and stays readable while the rows behind
   * it scroll sideways — and so its own content cannot widen the scroll area.
   */
  detailWidth?: number;
}> = memo(({ actions, detailWidth, expanded, line, onToggle, wrap }) => {
  const foreground = consoleColors.level[line.level] ?? consoleColors.level.DEFAULT;
  // Not derived from `line.id`: that is the raw log line, spaces and all, and
  // `aria-controls` is a space-separated list of id references — so one row
  // named half a dozen elements that do not exist.
  const panelId = useId();

  /*
   * The row is not a button. Wrapping it in one would make the text unselectable
   * in WebKit, which applies `-webkit-user-select: none` to form controls, and
   * would turn the mouse-up of a drag-select into a toggle. Only the chevron is
   * focusable; clicking elsewhere on the row still expands it, unless the click
   * ended a selection.
   */
  const toggleUnlessSelecting = () => {
    if (window.getSelection()?.toString()) return;
    onToggle(line.id);
  };

  return (
    <Box
      data-line-id={line.id}
      sx={{
        bgcolor: expanded ? consoleColors.open : consoleColors.levelWash[line.level],
        borderBottom: '1px solid',
        borderColor: 'rgba(255,255,255,0.04)',
        // The open row is the one being read, and a live tail keeps pushing it
        // down the list. The accent is how it stays findable.
        boxShadow: expanded ? `inset 2px 0 0 ${foreground}` : 'none',
      }}
    >
      <Box
        onClick={toggleUnlessSelecting}
        sx={{
          alignItems: 'baseline',
          columnGap: 1.5,
          display: 'grid',
          gridTemplateColumns: {
            md: rowColumns(wrap, '24px 96px 64px 168px'),
            xs: rowColumns(wrap, '24px 96px 64px'),
          },
          px: 1,
          py: 0.5,
          '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
        }}
      >
        <Box
          // Only while open: `unmountOnExit` means there is nothing to point at
          // otherwise, and a dangling reference is worse than none.
          aria-controls={expanded ? panelId : undefined}
          aria-expanded={expanded}
          aria-label={expanded ? 'Hide details' : 'Show details'}
          component="button"
          onClick={(event) => {
            event.stopPropagation();
            onToggle(line.id);
          }}
          sx={{
            background: 'none',
            border: 0,
            color: consoleColors.dim,
            cursor: 'pointer',
            lineHeight: 1,
            p: 0,
          }}
        >
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </Box>
        <Box component="span" sx={{ color: consoleColors.dim }}>
          {formatRowTime(line.timestamp)}
        </Box>
        <Box
          component="span"
          sx={{
            bgcolor: consoleColors.levelFill[line.level] ?? consoleColors.levelFill.DEFAULT,
            borderRadius: 0.5,
            color: foreground,
            fontSize: 10.5,
            fontWeight: 700,
            letterSpacing: 0.4,
            px: 0.75,
            py: 0.25,
            textAlign: 'center',
          }}
        >
          {line.level}
        </Box>
        <Box
          component="span"
          // A fixed track, unlike the message: letting it size to content would
          // put every row's message at a different x. Still clipped, therefore —
          // the full name is in the detail grid, and on hover.
          title={line.source}
          sx={{
            color: consoleColors.dim,
            display: { md: 'block', xs: 'none' },
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {line.source}
        </Box>
        <Box
          component="span"
          sx={{
            // Only the levels that mean something went wrong tint their message;
            // colouring the rest too would leave nothing standing out.
            color: line.level === 'ERROR' || line.level === 'WARN' ? foreground : consoleColors.text,
            // Never clipped: unwrapped it runs on and the pane scrolls to it,
            // wrapped it breaks onto more lines. An ellipsis here was the only
            // state in which part of a log line could not be read at all.
            overflow: 'visible',
            whiteSpace: wrap ? 'pre-wrap' : 'nowrap',
            overflowWrap: wrap ? 'anywhere' : 'normal',
          }}
        >
          {line.message}
        </Box>
      </Box>

      {/* No animation. A height that grows over 300ms inside a scrolling
          container means every measurement taken during it is of a layout that
          no longer exists a frame later — which is what made expanding a row
          jump the view. Instant is also what a log console should feel like. */}
      <Collapse id={panelId} in={expanded} timeout={0} unmountOnExit>
        <Box
          sx={{
            bgcolor: consoleColors.raised,
            // Sticky, so scrolling right to read a long line does not drag the
            // detail off the screen with it.
            left: 0,
            position: 'sticky',
            px: 3,
            py: 2,
            width: detailWidth ? `${detailWidth}px` : '100%',
          }}
        >
          <Box
            sx={{
              columnGap: 3,
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
              rowGap: 1.5,
            }}
          >
            {(line.details ?? []).map((detail) => (
              <Box key={detail.label}>
                <Typography
                  sx={{ color: consoleColors.dim, fontSize: 10.5, letterSpacing: 0.6 }}
                  variant="overline"
                >
                  {detail.label}
                </Typography>
                <Box sx={{ color: consoleColors.text, wordBreak: 'break-word' }}>
                  {detail.value}
                </Box>
              </Box>
            ))}
          </Box>
          {line.raw !== undefined ? (
            <Box
              sx={{
                bgcolor: consoleColors.surface,
                borderRadius: 1,
                color: consoleColors.dim,
                mt: 2,
                overflowWrap: 'anywhere',
                p: 1.5,
                whiteSpace: 'pre-wrap',
              }}
            >
              {line.raw}
            </Box>
          ) : null}
          {actions ? <Box sx={{ mt: 2 }}>{actions}</Box> : null}
        </Box>
      </Collapse>
    </Box>
  );
});
ConsoleRow.displayName = 'ConsoleRow';

export type LogConsoleProps = {
  /** Always oldest-first; `newestFirst` decides which end is shown at the top. */
  lines: ConsoleLine[];
  /** Shown in place of rows when `lines` is empty. */
  emptyMessage: ReactNode;
  /** Accessible name for the scrollable output region. */
  label: string;
  /** Streams new rows in: enables auto-follow and the live announcement. */
  live?: boolean;
  /** Live, but the last poll failed — the rows on screen are no longer current. */
  stale?: boolean;
  /**
   * Renders the newest row at the top, and follows that edge instead of the
   * bottom. Display only: the caller's buffer stays oldest-first, so the ordering
   * and trimming it depends on are untouched.
   */
  newestFirst?: boolean;
  /** A fetch is in flight. Shown in the header: with no Apply button, this is
   * the only thing that says a filter change was heard. */
  busy?: boolean;
  /** Buttons offered inside an expanded row, e.g. "show only this project". */
  rowActions?: (line: ConsoleLine) => ReactNode;
  /** Enables Download when provided. Called at click time, so the name it
   * stamps is the moment of the save rather than of the last render. */
  downloadFileName?: () => string;
  /** Told when the clipboard refused, so the host can raise its own notice. */
  onCopyError?: (message: string) => void;
  /** Height of the scrollable output region. */
  height?: number | string;
};

/**
 * Terminal-style log output: one row per record, expandable to the metadata the
 * line itself does not show.
 *
 * Rows are a grid rather than one `white-space: pre` block, which is what lets a
 * row disclose. The cost is that dragging a selection across rows no longer
 * copies as clean text — Copy and Download rebuild it from `consoleLineText`
 * instead, so what leaves the page is still aligned.
 *
 * No virtualisation, so the owner must cap what it hands over; a few thousand
 * rows is fine.
 */
const LogConsole: FC<LogConsoleProps> = ({
  lines,
  emptyMessage,
  label,
  live = false,
  stale = false,
  newestFirst = false,
  busy = false,
  rowActions,
  downloadFileName,
  onCopyError,
  height = 520,
}) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  /** The rows' own container, which is wider than the pane when lines are long. */
  const rowsRef = useRef<HTMLDivElement>(null);
  /** Where the reader is: the topmost visible row, and its offset from the top edge. */
  const anchorRef = useRef<{ id: string; offset: number } | null>(null);
  /**
   * `follow` as the effects below read it. They must not re-run when it changes:
   * `handleScroll` writes it, and an effect that both depends on it and writes
   * `scrollTop` turns one flick of the wheel into a fight with the reader —
   * scroll, get pulled back, get pulled back again.
   */
  const followRef = useRef(true);
  /** Last vertical position seen, to tell a sideways scroll from a real one. */
  const lastTopRef = useRef(0);
  const [wrap, setWrap] = useState(false);
  const [follow, setFollowState] = useState(true);
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  /** The pane's visible width, for the pinned detail panel. */
  const [paneWidth, setPaneWidth] = useState(0);
  const toggleRow = useCallback(
    (id: string) => setExpanded((current) => (current === id ? null : id)),
    []
  );

  /** Ref first, so an effect running in this same commit reads the new value. */
  const setFollow = useCallback((value: boolean) => {
    followRef.current = value;
    setFollowState(value);
  }, []);

  // Wide enough for the longest source on screen, so copied text stays straight.
  const sourceWidth = useMemo(() => {
    let widest = 0;
    for (const line of lines) {
      if (line.source && line.source.length > widest) widest = line.source.length;
    }
    return Math.min(widest, SOURCE_COLUMN_MAX);
  }, [lines]);

  // What is on screen, and therefore what Copy and Download produce.
  const rendered = useMemo(() => orderLines(lines, newestFirst), [lines, newestFirst]);

  /**
   * Records the topmost row still in view. This runs on every scroll event, so
   * it is a binary search over the row elements rather than a walk over all of
   * them — `offsetTop` is monotonic, and the buffer runs to a couple of thousand
   * rows.
   */
  const captureAnchor = useCallback(() => {
    const node = scrollRef.current;
    if (!node || !rowsRef.current) return;
    const rows = rowsRef.current.children;
    let low = 0;
    let high = rows.length - 1;
    anchorRef.current = null;
    while (low <= high) {
      const middle = (low + high) >> 1;
      const row = rows[middle] as HTMLElement;
      if (row.offsetTop + row.offsetHeight > node.scrollTop) {
        anchorRef.current = row.dataset.lineId
          ? { id: row.dataset.lineId, offset: row.offsetTop - node.scrollTop }
          : null;
        high = middle - 1;
      } else {
        low = middle + 1;
      }
    }
  }, []);

  /** Puts the recorded row back where it sat. */
  const restoreAnchor = useCallback(() => {
    const node = scrollRef.current;
    const anchor = anchorRef.current;
    if (!node || !anchor) return;
    const row = node.querySelector<HTMLElement>(`[data-line-id="${CSS.escape(anchor.id)}"]`);
    if (row) node.scrollTop = row.offsetTop - anchor.offset;
  }, []);

  /*
   * Scrolling away from the newest row pins the view so a live tail cannot yank
   * the page out from under a selection; scrolling back re-arms the follow. The
   * newest row is at the top when `newestFirst`, so the edge to watch flips with
   * it — otherwise every new line would scroll the reader away from it.
   */
  const distanceFromLatest = (node: HTMLDivElement) =>
    newestFirst ? node.scrollTop : node.scrollHeight - node.scrollTop - node.clientHeight;

  // The anchor is recorded here, as the reader moves — not at the end of the
  // last content change. Recorded there, a poll landing after a scroll would
  // restore the position the reader had already left.
  const handleScroll = (event: UIEvent<HTMLDivElement>) => {
    const node = event.currentTarget;
    captureAnchor();
    // Scrolling sideways to read a long line says nothing about whether the
    // reader wants the newest one. Sitting at the top edge it would say the
    // opposite of what they meant: the distance is still zero, so follow would
    // re-arm and the tail would start pulling the view again.
    if (node.scrollTop === lastTopRef.current) return;
    lastTopRef.current = node.scrollTop;
    setFollow(distanceFromLatest(node) <= FOLLOW_THRESHOLD_PX);
  };

  const scrollToLatest = useCallback(() => {
    const node = scrollRef.current;
    if (!node) return;
    node.scrollTop = newestFirst ? 0 : node.scrollHeight;
    setFollow(true);
  }, [newestFirst, setFollow]);

  /**
   * Holds the reader's place across a content change.
   *
   * Anchored on a row, not on the scroll height. Newest-first PREPENDS, so
   * without this every poll would push what is being read further down — but a
   * height delta is the wrong correction for it: wrapping lines, expanding a row
   * and trimming the oldest rows off the far end all change the height without
   * inserting anything above the viewport, and charging that delta to `scrollTop`
   * throws the reader across the buffer. Chrome and Firefox anchor for us
   * natively; Safari does not, which is why this exists at all.
   */
  useLayoutEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    // `!expanded` as well as the ref: an open row is being read, and pinning to
    // the newest line is what used to slide it off the screen.
    if (followRef.current && !expanded) node.scrollTop = newestFirst ? 0 : node.scrollHeight;
    else restoreAnchor();
    captureAnchor();
  }, [captureAnchor, expanded, lines, newestFirst, restoreAnchor, wrap]);

  /**
   * Opening a row is a request to read it, so the tail stops pulling the view to
   * the newest line. Without this the opened row drifts down the list on every
   * poll and leaves the viewport, taking its detail with it — which reads as the
   * detail scrolling away on its own. "Jump to latest" is how the reader comes
   * back.
   *
   * The detail is also brought into view when it opened below the fold, but
   * never so far that its own row goes off the top.
   */
  useLayoutEffect(() => {
    const node = scrollRef.current;
    if (!node || !expanded) return;
    setFollow(false);
    const row = node.querySelector<HTMLElement>(`[data-line-id="${CSS.escape(expanded)}"]`);
    if (!row) return;
    const below = row.offsetTop + row.offsetHeight - (node.scrollTop + node.clientHeight);
    if (below > 0) node.scrollTop += Math.min(below, row.offsetTop - node.scrollTop);
    captureAnchor();
  }, [captureAnchor, expanded, setFollow]);

  // Flipping the sort moves the reader to the other end of the output, so whether
  // they are at the latest edge has to be asked again — otherwise the console can
  // sit pinned to the newest row with auto-follow switched off.
  useLayoutEffect(() => {
    const node = scrollRef.current;
    if (node) setFollow(distanceFromLatest(node) <= FOLLOW_THRESHOLD_PX);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [newestFirst]);

  /*
   * The detail panel is pinned to the left edge and sized to the pane, so it
   * needs the pane's width as a number. `clientWidth` excludes the vertical
   * scrollbar, which is what "visible" means here.
   */
  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return undefined;
    setPaneWidth(node.clientWidth);
    // Absent in jsdom, and a missing observer only costs the panel its exact
    // width — it falls back to 100% of the row.
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(() => setPaneWidth(node.clientWidth));
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!copied) return undefined;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const asText = () =>
    rendered.map((line) => consoleLineText(line, true, sourceWidth)).join('\n');

  const copyAll = async () => {
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard access is unavailable');
      await navigator.clipboard.writeText(asText());
      setCopied(true);
    } catch {
      // Blocked clipboard (insecure origin, denied permission) is expected.
      onCopyError?.('Could not copy to the clipboard. Select the lines and copy them manually.');
    }
  };

  /*
   * Saves what is on screen, which is not everything that matched: the owner caps
   * the buffer. The tooltip says so, because a file named "logs" reads as the
   * whole answer otherwise.
   */

  const download = () => {
    const url = URL.createObjectURL(new Blob([asText()], { type: 'text/plain' }));
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = downloadFileName?.() ?? 'logs.log';
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    // Revoked, or every save leaks a blob for the life of the tab — but on a
    // later task: Safari cancels a download whose blob URL is revoked in the
    // same one.
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  return (
    <Paper
      variant="outlined"
      sx={{
        bgcolor: consoleColors.surface,
        borderColor: consoleColors.border,
        overflow: 'hidden',
      }}
    >
      <Stack
        alignItems="center"
        direction="row"
        spacing={1}
        sx={{
          borderBottom: '1px solid',
          borderColor: consoleColors.border,
          color: consoleColors.text,
          px: 1.5,
          py: 1,
        }}
      >
        <Box
          sx={{
            bgcolor: stale ? consoleColors.level.WARN : '#3fb950',
            borderRadius: '50%',
            height: 8,
            opacity: live ? 1 : 0.35,
            width: 8,
          }}
        />
        <Typography sx={{ fontFamily: 'inherit' }} variant="body2">
          {lines.length === 1 ? '1 line' : `${lines.length} lines`}
        </Typography>
        {/* A failed poll leaves the rows on screen; saying "streaming" over them
            would claim they are current. */}
        <Typography variant="caption" sx={{ color: consoleColors.dim }}>
          {!live ? 'paused' : stale ? 'not updating' : 'streaming'}
        </Typography>
        {busy ? <CircularProgress size={12} sx={{ color: consoleColors.dim }} /> : null}
        <Box sx={{ flex: 1 }} />
        <Tooltip title={wrap ? 'Stop wrapping — scroll sideways instead' : 'Wrap long lines'}>
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
        <Tooltip title={copied ? 'Copied' : 'Copy the lines on screen'}>
          <span>
            <IconButton
              aria-label="Copy the lines on screen"
              disabled={lines.length === 0}
              onClick={copyAll}
              size="small"
              sx={{ color: copied ? consoleColors.text : consoleColors.dim }}
            >
              <Copy size={16} />
            </IconButton>
          </span>
        </Tooltip>
        {downloadFileName ? (
          <Tooltip title="Download the lines on screen — not everything that matched">
            <span>
              <IconButton
                aria-label="Download the lines on screen"
                disabled={lines.length === 0}
                onClick={download}
                size="small"
                sx={{ color: consoleColors.dim }}
              >
                <Download size={16} />
              </IconButton>
            </span>
          </Tooltip>
        ) : null}
      </Stack>

      <Box sx={{ position: 'relative' }}>
        <Box
          aria-label={label}
          // Not announced. A live tail can add a hundred rows every five seconds,
          // and `role="log"` would read them all out, newest-first — the line
          // count in the header is the summary worth hearing instead.
          aria-live="off"
          onKeyDown={(event) => {
            if (event.key === 'Escape' && expanded) setExpanded(null);
          }}
          onScroll={handleScroll}
          ref={scrollRef}
          role="log"
          tabIndex={0}
          sx={{
            color: consoleColors.text,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 12.5,
            height,
            lineHeight: 1.7,
            overflow: 'auto',
            // So a row's `offsetTop` is measured from this box and not from the
            // positioned wrapper outside it. The anchor arithmetic depends on it.
            position: 'relative',
            resize: 'vertical',
            userSelect: 'text',
            '&::selection, & *::selection': { bgcolor: consoleColors.selection },
          }}
        >
          <Box
            ref={rowsRef}
            sx={{
              // Every row as wide as the widest one, so a row's wash, border and
              // hover still span the pane once it has been scrolled sideways —
              // and so the pinned detail panel has a containing block that
              // covers the whole scrollable width.
              minWidth: '100%',
              width: wrap ? 'auto' : 'max-content',
            }}
          >
            {rendered.length === 0 ? (
              <Box sx={{ color: consoleColors.dim, px: 2, py: 2 }}>{emptyMessage}</Box>
            ) : (
              rendered.map((line) => (
                <ConsoleRow
                  key={line.id}
                  // Only the open row builds its actions; doing it for all of them
                  // would run a per-row search on every keystroke and every poll.
                  actions={expanded === line.id ? rowActions?.(line) : undefined}
                  // Likewise: handing every row the width would invalidate two
                  // thousand memos on a window resize, and only one row has a
                  // panel to size.
                  detailWidth={expanded === line.id ? paneWidth : undefined}
                  expanded={expanded === line.id}
                  line={line}
                  onToggle={toggleRow}
                  wrap={wrap}
                />
              ))
            )}
          </Box>
        </Box>
        {!follow && rendered.length > 0 ? (
          <Box
            component="button"
            onClick={scrollToLatest}
            sx={{
              bgcolor: consoleColors.raised,
              border: '1px solid',
              borderColor: consoleColors.border,
              borderRadius: 4,
              bottom: 12,
              color: consoleColors.text,
              cursor: 'pointer',
              font: 'inherit',
              fontSize: 12,
              left: '50%',
              position: 'absolute',
              px: 1.5,
              py: 0.5,
              transform: 'translateX(-50%)',
            }}
          >
            Jump to latest
          </Box>
        ) : null}
      </Box>
    </Paper>
  );
};

export default LogConsole;
