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

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  alpha,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {
  ArrowDown,
  CircleCheck,
  CircleX,
  Plus,
  RefreshCw,
  Server,
  Trash2,
  X,
} from '@wso2/oxygen-ui-icons-react';

import { useUpdateApi } from '../../../../../api/hooks/useMvpQueries';
import { ConfirmDialog } from '../../../../../components/ConfirmDialog';
import { useNotifications } from '../../../../../components/Notifications';
import type { ApiOperation, ApiDetail } from '../../../../../types/domain';
import {
  type BackendResource,
  discoverBackendResources,
} from './backendDiscovery';
import {
  addOperation,
  getBackendPath,
  HTTP_METHODS,
  isValidUrl,
  methodColor,
  operationsValid,
  removeOperation,
  setBackendPath,
  updateOperation,
  withRoutingEdits,
} from './developEdit';
import { SaveBar } from './SaveBar';

// --- canvas geometry (fixed coords → SVG links need no DOM measurement) ---
const OP_X = 4;
const NODE_W = 290;
const NODE_H = 60;
const STEP = 82; // node height + 22px gap
const HEADER_H = 70; // band above the rows for column captions + backend URL
const COL_GAP = 130; // connector lane between the two columns
const BE_X = OP_X + NODE_W + COL_GAP;
const CANVAS_W = BE_X + NODE_W + 8;

/** Seeds the backend catalog from the resources the API already maps to. */
function seedBackendResources(operations: ApiOperation[]): BackendResource[] {
  const seen = new Set<string>();
  const list: BackendResource[] = [];
  for (const op of operations) {
    const path = getBackendPath(op) ?? op.path;
    const key = `${op.method} ${path}`;
    if (!seen.has(key)) {
      seen.add(key);
      list.push({ method: op.method, path });
    }
  }
  return list;
}

type Selection =
  | { type: 'operation'; index: number }
  | { type: 'upstream' }
  | null;

/** Resolves a method to a solid badge background + readable text from the theme. */
function useBadgeColor() {
  const theme = useTheme();
  return (method: string) => {
    const key = methodColor(method);
    if (key === 'default') {
      return { bg: theme.palette.grey[500], fg: theme.palette.common.white };
    }
    const swatch = theme.palette[key];
    return { bg: swatch.main, fg: swatch.contrastText };
  };
}

/** A rounded "pill" node placed on the routing canvas. */
function ResourcePill({
  method,
  path,
  side,
  selected,
  muted,
  disconnected,
  connectSource,
  connectEligible,
  connectDimmed,
  x,
  y,
  onSelect,
  onRemove,
  onPortClick,
}: {
  method: string;
  path: string;
  side: 'proxy' | 'backend';
  selected: boolean;
  muted?: boolean;
  disconnected?: boolean;
  connectSource?: boolean;
  connectEligible?: boolean;
  connectDimmed?: boolean;
  x: number;
  y: number;
  onSelect: () => void;
  onRemove?: () => void;
  onPortClick?: () => void;
}) {
  const theme = useTheme();
  const badgeColor = useBadgeColor();
  const badge = badgeColor(method);
  const highlighted = selected || connectSource || connectEligible;

  return (
    <Box
      onClick={(event) => {
        event.stopPropagation();
        onSelect();
      }}
      title={
        connectEligible
          ? 'Click to connect this backend resource'
          : disconnected
            ? 'Not connected — use the port handle or the editor to map a backend resource'
            : undefined
      }
      sx={{
        alignItems: 'center',
        bgcolor: connectEligible
          ? alpha(theme.palette.primary.main, 0.06)
          : 'background.paper',
        border: '1px solid',
        borderColor: highlighted ? 'primary.main' : 'divider',
        borderRadius: 999,
        borderStyle: disconnected || connectEligible ? 'dashed' : 'solid',
        boxShadow: highlighted
          ? `0 0 0 4px ${alpha(theme.palette.primary.main, 0.16)}`
          : theme.shadows[1],
        cursor: 'pointer',
        display: 'flex',
        gap: 1.25,
        height: NODE_H,
        left: x,
        opacity: connectDimmed ? 0.3 : muted || disconnected ? 0.6 : 1,
        pl: side === 'proxy' ? 2 : 2.25,
        position: 'absolute',
        pr: 1.5,
        top: y,
        transition: 'border-color .15s, box-shadow .15s, opacity .15s',
        width: NODE_W,
        '&:hover': {
          borderColor: highlighted ? 'primary.main' : 'primary.light',
        },
        '&:hover .rt-remove': { opacity: 1, pointerEvents: 'auto' },
        // left accent strip (proxy column only)
        '&::before':
          side === 'proxy'
            ? {
                bgcolor: badge.bg,
                borderRadius: '4px',
                bottom: 11,
                content: '""',
                left: 6,
                position: 'absolute',
                top: 11,
                width: 4,
              }
            : undefined,
      }}
    >
      {/* method badge */}
      <Box
        sx={{
          alignItems: 'center',
          bgcolor: badge.bg,
          borderRadius: 999,
          color: badge.fg,
          display: 'inline-flex',
          flex: 'none',
          fontSize: 12,
          fontWeight: 700,
          height: 28,
          justifyContent: 'center',
          letterSpacing: '.3px',
          minWidth: 64,
          px: 1.25,
        }}
      >
        {method}
      </Box>
      <Typography
        noWrap
        sx={{ fontFamily: 'monospace', fontWeight: 500, minWidth: 0 }}
      >
        {path || '/'}
      </Typography>

      {onRemove && (
        <IconButton
          aria-label="Remove resource"
          className="rt-remove"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
          size="small"
          sx={{
            color: 'error.main',
            flex: 'none',
            ml: 'auto',
            opacity: 0,
            pointerEvents: 'none',
            transition: 'opacity .12s',
            '&:hover': { bgcolor: alpha(theme.palette.error.main, 0.12) },
          }}
        >
          <X size={15} />
        </IconButton>
      )}

      {/* connection port — on the proxy side it starts a connection */}
      <Box
        aria-label={onPortClick ? 'Connect to a backend resource' : undefined}
        onClick={
          onPortClick
            ? (event) => {
                event.stopPropagation();
                onPortClick();
              }
            : undefined
        }
        title={onPortClick ? 'Connect to a backend resource' : undefined}
        sx={{
          alignItems: 'center',
          bgcolor: connectSource ? 'primary.dark' : 'primary.main',
          border: `2px solid ${theme.palette.background.paper}`,
          borderRadius: '50%',
          cursor: onPortClick ? 'crosshair' : 'default',
          display: 'flex',
          height: connectSource ? 15 : 11,
          justifyContent: 'center',
          position: 'absolute',
          right: side === 'proxy' ? -7 : undefined,
          left: side === 'backend' ? -6 : undefined,
          top: '50%',
          transform: 'translateY(-50%)',
          transition: 'height .12s, width .12s',
          width: connectSource ? 15 : 11,
          zIndex: 4,
          '&:hover': onPortClick
            ? {
                boxShadow: `0 0 0 4px ${alpha(theme.palette.primary.main, 0.25)}`,
              }
            : undefined,
        }}
      />
    </Box>
  );
}

export function RoutingTab({ detail }: { detail: ApiDetail }) {
  const theme = useTheme();
  const { notify } = useNotifications();
  const update = useUpdateApi();

  const [operations, setOperations] = useState<ApiOperation[]>(
    detail.operations
  );
  // The backend's resources are a catalog that comes from the backend service.
  // Seed it from the resources the API already maps to; changing the backend
  // URL re-discovers the catalog from the backend's OpenAPI/Swagger contract.
  const [backendResources, setBackendResources] = useState<BackendResource[]>(
    () => seedBackendResources(detail.operations)
  );
  const [prodUrl, setProdUrl] = useState(detail.endpoints.prodUrl || '');
  const [sandboxUrl, setSandboxUrl] = useState(
    detail.endpoints.sandboxUrl || ''
  );
  const [selection, setSelection] = useState<Selection>(null);
  // Indices of resources whose proxy↔backend link has been removed. The two
  // resources stay; only the connection is severed (a UI/routing concept —
  // a severed resource simply passes through with no explicit mapping).
  const [disconnected, setDisconnected] = useState<Set<number>>(new Set());
  const isConnected = (index: number) => !disconnected.has(index);

  // --- backend-contract discovery -------------------------------------------
  // When the backend URL changes we fetch its OpenAPI/Swagger contract and
  // rebuild the backend catalog from it. Proxy resources whose method+path
  // exist in the contract auto-map (passthrough); the rest hang dry.
  const [discovery, setDiscovery] = useState<{
    status: 'idle' | 'loading' | 'done' | 'error';
    count?: number;
    message?: string;
  }>({ status: 'idle' });
  const discoverAbort = useRef<AbortController | null>(null);
  // The backend URL discovery last ran for — so the debounced effect fires
  // only on an actual change, never for the initially-seeded URL.
  const lastDiscoveredUrl = useRef(detail.endpoints.prodUrl || '');

  const runDiscovery = useCallback(async (url: string) => {
    lastDiscoveredUrl.current = url;
    discoverAbort.current?.abort();
    const controller = new AbortController();
    discoverAbort.current = controller;
    setDiscovery({ status: 'loading' });
    try {
      const resources = await discoverBackendResources(url, controller.signal);
      if (controller.signal.aborted) return;
      if (resources.length === 0) {
        setDiscovery({
          status: 'error',
          message:
            'No OpenAPI/Swagger contract found at the backend URL — map resources manually.',
        });
        return;
      }
      // Rebuild the catalog and re-evaluate every mapping: matches auto-connect,
      // non-matches fall back to "disconnected" (hanging dry) automatically.
      setBackendResources(resources);
      setDisconnected(new Set());
      setDiscovery({ status: 'done', count: resources.length });
    } catch {
      if (controller.signal.aborted) return;
      setDiscovery({
        status: 'error',
        message:
          'Could not fetch the backend contract (CORS or unreachable) — map resources manually.',
      });
    }
  }, []);

  // Auto-discover (debounced) when the backend URL is changed to a new valid
  // value.
  useEffect(() => {
    const url = prodUrl.trim();
    if (!url || !isValidUrl(url) || url === lastDiscoveredUrl.current) return;
    const handle = setTimeout(() => runDiscovery(url), 700);
    return () => clearTimeout(handle);
  }, [prodUrl, runDiscovery]);

  // Cancel any in-flight discovery on unmount.
  useEffect(() => () => discoverAbort.current?.abort(), []);

  const urlsValid = isValidUrl(prodUrl) && isValidUrl(sandboxUrl);
  const canSave = operationsValid(operations) && urlsValid && !update.isPending;

  const addRow = () => {
    setOperations(addOperation(operations));
    setSelection({ type: 'operation', index: operations.length });
  };
  const patchOp = (index: number, patch: Partial<ApiOperation>) =>
    setOperations(updateOperation(operations, index, patch));
  const removeRow = (index: number) => {
    setOperations(removeOperation(operations, index));
    // Drop this row from the disconnected set and shift the rest down.
    setDisconnected((prev) => {
      const next = new Set<number>();
      prev.forEach((value) => {
        if (value < index) next.add(value);
        else if (value > index) next.add(value - 1);
      });
      return next;
    });
    setSelection(null);
  };
  // Set (or clear → passthrough) the backend path an operation rewrites to.
  const mapBackend = (index: number, backendPath: string | undefined) =>
    setOperations(
      updateOperation(
        operations,
        index,
        setBackendPath(operations[index], backendPath)
      )
    );

  // Sever a connection: keep both resources, drop the explicit mapping.
  const disconnect = (index: number) => {
    setDisconnected((prev) => new Set(prev).add(index));
    mapBackend(index, undefined);
  };
  const reconnect = (index: number) =>
    setDisconnected((prev) => {
      const next = new Set(prev);
      next.delete(index);
      return next;
    });

  // --- delete confirmations --------------------------------------------------
  const [confirm, setConfirm] = useState<{
    title: string;
    message: string;
    confirmLabel: string;
    destructive: boolean;
    onConfirm: () => void;
  } | null>(null);

  const confirmRemoveRow = (index: number) => {
    const op = operations[index];
    setConfirm({
      title: 'Remove resource',
      message:
        `Remove the ${op.method} ${op.path || '/'} proxy resource? ` +
        `This can't be undone.`,
      confirmLabel: 'Remove',
      destructive: true,
      onConfirm: () => removeRow(index),
    });
  };

  const confirmDisconnect = (index: number) => {
    const op = operations[index];
    setConfirm({
      title: 'Remove connection',
      message:
        `Remove the connection for ${op.method} ${op.path || '/'}? ` +
        'Both resources stay — only the link is removed.',
      confirmLabel: 'Remove connection',
      destructive: false,
      onConfirm: () => disconnect(index),
    });
  };

  // --- cross-row connections -------------------------------------------------
  // A proxy resource may connect to any backend resource of the *same method*,
  // on any row. The connection is stored as the proxy op's backend path.
  const [connectingFrom, setConnectingFrom] = useState<number | null>(null);

  // Index in the backend catalog a proxy op currently connects to (its rewrite
  // target, matched by method + path), or -1 when there is no matching resource.
  const targetRow = (index: number) => {
    const op = operations[index];
    const wanted = getBackendPath(op) ?? op.path;
    return backendResources.findIndex(
      (resource) => resource.method === op.method && resource.path === wanted
    );
  };

  // A proxy row is linked when it isn't severed and points at a real resource.
  const linked = (index: number) => isConnected(index) && targetRow(index) >= 0;

  const canConnect = (from: number, resourceIndex: number) =>
    operations[from]?.method === backendResources[resourceIndex]?.method;

  // Finish a connection started from `connectingFrom` onto backend resource row.
  const connectTo = (resourceIndex: number) => {
    const from = connectingFrom;
    if (from === null) return;
    setConnectingFrom(null);
    if (!canConnect(from, resourceIndex)) return;
    reconnect(from);
    const resource = backendResources[resourceIndex];
    // Mapping to a resource with the proxy's own path is just passthrough.
    mapBackend(
      from,
      resource.path === operations[from].path ? undefined : resource.path
    );
    setSelection({ type: 'operation', index: from });
  };

  const save = () => {
    update.mutate(
      withRoutingEdits(detail, { operations, prodUrl, sandboxUrl }),
      {
        onSuccess: () => notify('Routing saved.', 'success'),
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Save failed',
            'error'
          ),
      }
    );
  };

  // Per-row backend path: explicit rewrite target, else the proxy path (passthrough).
  const backendFor = (op: ApiOperation) => ({
    path: getBackendPath(op) ?? op.path,
    passthrough: getBackendPath(op) === undefined,
  });

  const rowY = (i: number) => HEADER_H + i * STEP;
  const canvasH =
    HEADER_H + Math.max(operations.length, backendResources.length, 1) * STEP;
  const selectedOp =
    selection?.type === 'operation' ? operations[selection.index] : undefined;

  return (
    <Stack spacing={3}>
      {/* heading + add */}
      <Box sx={{ alignItems: 'flex-start', display: 'flex', gap: 3 }}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="h6">Routing</Typography>
          <Typography color="text.secondary" variant="body2">
            Map API proxy resources to backend resources. An unmapped resource
            passes through unchanged.
          </Typography>
        </Box>
        <Button onClick={addRow} size="small" startIcon={<Plus size={16} />}>
          Add resource
        </Button>
      </Box>

      {connectingFrom !== null && operations[connectingFrom] && (
        <Alert
          icon={<Plus size={18} />}
          onClose={() => setConnectingFrom(null)}
          severity="info"
          sx={{ alignItems: 'center' }}
        >
          Connecting{' '}
          <Box component="span" sx={{ fontWeight: 700 }}>
            {operations[connectingFrom].method}{' '}
            {operations[connectingFrom].path}
          </Box>{' '}
          — pick a matching backend resource (same method), or close to cancel.
        </Alert>
      )}

      {discovery.status === 'error' && discovery.message && (
        <Alert
          onClose={() => setDiscovery({ status: 'idle' })}
          severity="warning"
          sx={{ alignItems: 'center' }}
        >
          {discovery.message}
        </Alert>
      )}

      <Stack
        alignItems="flex-start"
        direction={{ xs: 'column', md: 'row' }}
        spacing={2}
      >
        {/* mapping canvas */}
        <Card
          sx={{ flex: 1, overflowX: 'auto', width: '100%' }}
          variant="outlined"
        >
          <CardContent>
            <Box
              onClick={() => setConnectingFrom(null)}
              sx={{ height: canvasH, position: 'relative', width: CANVAS_W }}
            >
              {/* column titles */}
              <Typography
                sx={{
                  color: 'text.primary',
                  fontWeight: 700,
                  left: OP_X,
                  position: 'absolute',
                  textAlign: 'center',
                  top: 0,
                  width: NODE_W,
                }}
                variant="subtitle2"
              >
                API proxy resources
              </Typography>
              <Box
                sx={{
                  left: BE_X,
                  position: 'absolute',
                  top: 0,
                  width: NODE_W,
                }}
              >
                <Typography
                  sx={{
                    color: 'text.primary',
                    fontWeight: 700,
                    textAlign: 'center',
                  }}
                  variant="subtitle2"
                >
                  Backend resources
                </Typography>
                <Box
                  onClick={() => setSelection({ type: 'upstream' })}
                  sx={{
                    alignItems: 'center',
                    bgcolor: 'action.hover',
                    border: '1px solid',
                    borderColor:
                      selection?.type === 'upstream'
                        ? 'primary.main'
                        : 'divider',
                    borderRadius: 1.5,
                    cursor: 'pointer',
                    display: 'flex',
                    gap: 1,
                    mt: 0.5,
                    px: 1.25,
                    py: 0.75,
                  }}
                >
                  <Server
                    color={theme.palette.primary.main}
                    size={15}
                    style={{ flex: 'none' }}
                  />
                  {/* Inline-editable backend URL — type directly here; the
                        underline + placeholder signal it is an input. */}
                  <TextField
                    error={!isValidUrl(prodUrl)}
                    fullWidth
                    onChange={(event) => setProdUrl(event.target.value)}
                    placeholder="Set backend URL"
                    slotProps={{
                      htmlInput: { 'aria-label': 'Backend URL' },
                      input: {
                        sx: {
                          fontFamily: 'monospace',
                          fontSize: 12.5,
                          '&::before': { borderColor: 'divider' },
                        },
                      },
                    }}
                    sx={{ flex: 1, minWidth: 0 }}
                    value={prodUrl}
                    variant="standard"
                  />
                  {discovery.status === 'loading' ? (
                    <CircularProgress size={14} sx={{ flex: 'none' }} />
                  ) : (
                    <Tooltip title="Discover backend resources from its OpenAPI/Swagger contract">
                      <span>
                        <IconButton
                          aria-label="Discover backend resources"
                          disabled={!prodUrl.trim() || !isValidUrl(prodUrl)}
                          onClick={(event) => {
                            event.stopPropagation();
                            runDiscovery(prodUrl.trim());
                          }}
                          size="small"
                          sx={{ flex: 'none' }}
                        >
                          <RefreshCw size={13} />
                        </IconButton>
                      </span>
                    </Tooltip>
                  )}
                </Box>
              </Box>

              {/* connectors: proxy row → its backend row (curved bezier).
                    Purely decorative — must NOT capture pointer events, or this
                    full-canvas overlay would swallow clicks/typing on elements
                    painted before it (e.g. the backend-URL field in the header).
                    The interactive sever controls are separate hit-area Boxes. */}
              <Box
                component="svg"
                sx={{
                  height: '100%',
                  left: 0,
                  pointerEvents: 'none',
                  position: 'absolute',
                  top: 0,
                  width: '100%',
                }}
              >
                {operations.map((op, i) => {
                  if (!linked(i)) return null;
                  const j = targetRow(i);
                  const y1 = rowY(i) + NODE_H / 2;
                  const y2 = rowY(j) + NODE_H / 2;
                  const x1 = OP_X + NODE_W;
                  const x2 = BE_X;
                  const mid = (x1 + x2) / 2;
                  const active =
                    selection?.type === 'operation' && selection.index === i;
                  // All connections share one style; direct (same-row) links
                  // simply render as a straight segment of the same curve.
                  return (
                    <path
                      d={`M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`}
                      fill="none"
                      key={i}
                      stroke={
                        active
                          ? theme.palette.primary.main
                          : alpha(theme.palette.primary.main, 0.5)
                      }
                      strokeWidth={active ? 2.6 : 2}
                    />
                  );
                })}
              </Box>

              {/* connector hit areas: hover over a link reveals a control
                    that severs the connection. Both the proxy and backend
                    resources stay — only the link is removed. */}
              {operations.map((op, i) => {
                if (!linked(i)) return null;
                const j = targetRow(i);
                // Centre the control on the (possibly diagonal) link midpoint.
                const midTop = (rowY(i) + rowY(j)) / 2;
                return (
                  <Box
                    key={`link-${i}`}
                    sx={{
                      alignItems: 'center',
                      display: 'flex',
                      height: NODE_H,
                      justifyContent: 'center',
                      left: OP_X + NODE_W,
                      position: 'absolute',
                      top: midTop,
                      width: COL_GAP,
                      zIndex: 3,
                      '&:hover .rt-link-del': {
                        opacity: 1,
                        pointerEvents: 'auto',
                      },
                    }}
                  >
                    <IconButton
                      aria-label="Remove connection"
                      className="rt-link-del"
                      onClick={() => confirmDisconnect(i)}
                      size="small"
                      sx={{
                        bgcolor: 'background.paper',
                        border: '1px solid',
                        borderColor: 'error.main',
                        boxShadow: 1,
                        color: 'error.main',
                        opacity: 0,
                        pointerEvents: 'none',
                        transition: 'opacity .12s',
                        '&:hover': {
                          bgcolor: alpha(theme.palette.error.main, 0.12),
                        },
                      }}
                    >
                      <X size={14} />
                    </IconButton>
                  </Box>
                );
              })}

              {/* proxy resources (left column) */}
              {operations.map((op, i) => {
                const connecting = connectingFrom !== null;
                const selected =
                  selection?.type === 'operation' && selection.index === i;
                return (
                  <ResourcePill
                    connectSource={connecting && connectingFrom === i}
                    disconnected={!linked(i)}
                    key={`op-${i}`}
                    method={op.method}
                    onPortClick={() => setConnectingFrom(i)}
                    onRemove={() => confirmRemoveRow(i)}
                    onSelect={() =>
                      setSelection({ type: 'operation', index: i })
                    }
                    path={op.path}
                    selected={selected}
                    side="proxy"
                    x={OP_X}
                    y={rowY(i)}
                  />
                );
              })}

              {/* backend resources (right column — fixed catalog) */}
              {backendResources.map((resource, r) => {
                const connecting = connectingFrom !== null;
                // A resource is "live" when some linked proxy routes to it.
                const targeted = operations.some(
                  (_op, k) => linked(k) && targetRow(k) === r
                );
                const eligible = connecting && canConnect(connectingFrom, r);
                return (
                  <ResourcePill
                    connectDimmed={connecting && !eligible}
                    connectEligible={eligible}
                    key={`be-${resource.method}-${resource.path}`}
                    method={resource.method}
                    muted={!connecting && !targeted}
                    onSelect={() => {
                      if (connecting && eligible) connectTo(r);
                    }}
                    path={resource.path}
                    selected={false}
                    side="backend"
                    x={BE_X}
                    y={rowY(r)}
                  />
                );
              })}

              {operations.length === 0 && (
                <Typography
                  color="text.secondary"
                  sx={{ left: OP_X, position: 'absolute', top: HEADER_H + 6 }}
                  variant="body2"
                >
                  No resources yet. Add one to route to the backend.
                </Typography>
              )}
            </Box>
          </CardContent>
        </Card>

        {/* detail / editor panel */}
        <Card
          sx={{ flexShrink: 0, width: { xs: '100%', md: 332 } }}
          variant="outlined"
        >
          <CardContent>
            {selectedOp ? (
              <ResourceMappingEditor
                backendPassthrough={getBackendPath(selectedOp) === undefined}
                backendPath={backendFor(selectedOp).path}
                // Same-method backend resources this proxy may target.
                connectOptions={backendResources
                  .filter((resource) => resource.method === selectedOp.method)
                  .map((resource) => resource.path)}
                onChange={(patch) =>
                  patchOp((selection as { index: number }).index, patch)
                }
                onClose={() => setSelection(null)}
                onMapBackend={(value) =>
                  mapBackend((selection as { index: number }).index, value)
                }
                onRemove={() =>
                  confirmRemoveRow((selection as { index: number }).index)
                }
                operation={selectedOp}
              />
            ) : selection?.type === 'upstream' ? (
              <Stack spacing={2}>
                <Box
                  sx={{
                    alignItems: 'center',
                    display: 'flex',
                    justifyContent: 'space-between',
                  }}
                >
                  <Typography variant="subtitle1">Backend endpoint</Typography>
                  <IconButton onClick={() => setSelection(null)} size="small">
                    <X size={16} />
                  </IconButton>
                </Box>
                <TextField
                  error={!isValidUrl(prodUrl)}
                  fullWidth
                  label="Production URL"
                  onChange={(event) => setProdUrl(event.target.value)}
                  placeholder="https://backend.example.com/api"
                  size="small"
                  slotProps={{
                    input: {
                      endAdornment:
                        prodUrl.trim() === '' ? null : (
                          <InputAdornment position="end">
                            {isValidUrl(prodUrl) ? (
                              <CircleCheck
                                color={theme.palette.success.main}
                                size={16}
                              />
                            ) : (
                              <CircleX
                                color={theme.palette.error.main}
                                size={16}
                              />
                            )}
                          </InputAdornment>
                        ),
                    },
                  }}
                  value={prodUrl}
                />
                <TextField
                  error={!isValidUrl(sandboxUrl)}
                  fullWidth
                  label="Sandbox URL (optional)"
                  onChange={(event) => setSandboxUrl(event.target.value)}
                  placeholder="https://sandbox.example.com/api"
                  size="small"
                  value={sandboxUrl}
                />
                <Divider />
                <Box
                  sx={{
                    alignItems: 'center',
                    display: 'flex',
                    justifyContent: 'space-between',
                  }}
                >
                  <Typography variant="subtitle2">Backend resources</Typography>
                  <Button
                    disabled={
                      !prodUrl.trim() ||
                      !isValidUrl(prodUrl) ||
                      discovery.status === 'loading'
                    }
                    onClick={() => runDiscovery(prodUrl.trim())}
                    size="small"
                    startIcon={
                      discovery.status === 'loading' ? (
                        <CircularProgress size={14} />
                      ) : (
                        <RefreshCw size={14} />
                      )
                    }
                  >
                    Discover
                  </Button>
                </Box>
                <Typography color="text.secondary" variant="caption">
                  {discovery.status === 'loading'
                    ? 'Fetching the backend contract…'
                    : discovery.status === 'done'
                      ? `Found ${discovery.count} backend resource${
                          discovery.count === 1 ? '' : 's'
                        }. Matching proxy resources were auto-mapped; the rest are left unmapped.`
                      : discovery.status === 'error'
                        ? discovery.message
                        : 'Resources are discovered from the backend’s OpenAPI/Swagger contract when the URL changes.'}
                </Typography>
              </Stack>
            ) : (
              <Box
                sx={{
                  alignItems: 'center',
                  color: 'text.secondary',
                  display: 'flex',
                  justifyContent: 'center',
                  minHeight: 76,
                  textAlign: 'center',
                }}
              >
                <Typography variant="body2">
                  Select a resource or the backend to edit it.
                </Typography>
              </Box>
            )}
          </CardContent>
        </Card>
      </Stack>

      <SaveBar disabled={!canSave} onSave={save} saving={update.isPending} />

      <ConfirmDialog
        confirmLabel={confirm?.confirmLabel ?? 'Confirm'}
        destructive={confirm?.destructive ?? true}
        message={confirm?.message ?? ''}
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          confirm?.onConfirm();
          setConfirm(null);
        }}
        open={confirm !== null}
        title={confirm?.title ?? ''}
      />
    </Stack>
  );
}

const PASSTHROUGH = '__passthrough__';

function ResourceMappingEditor({
  operation,
  backendPath,
  backendPassthrough,
  connectOptions,
  onChange,
  onMapBackend,
  onRemove,
  onClose,
}: {
  operation: ApiOperation;
  backendPath: string;
  backendPassthrough: boolean;
  connectOptions: string[];
  onChange: (patch: Partial<ApiOperation>) => void;
  onMapBackend: (value: string | undefined) => void;
  onRemove: () => void;
  onClose: () => void;
}) {
  const theme = useTheme();
  const badgeColor = useBadgeColor();
  const badge = badgeColor(operation.method);

  return (
    <Stack spacing={2}>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'space-between',
        }}
      >
        <Typography variant="subtitle1">Resource mapping</Typography>
        <IconButton onClick={onClose} size="small">
          <X size={16} />
        </IconButton>
      </Box>

      {/* mapping summary: proxy resource → backend resource */}
      <Box>
        <Typography
          color="text.secondary"
          sx={{
            display: 'block',
            fontWeight: 600,
            letterSpacing: '.4px',
            mb: 0.75,
            textAlign: 'center',
          }}
          variant="caption"
        >
          API PROXY RESOURCE
        </Typography>
        <Box
          sx={{
            alignItems: 'center',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1.5,
            display: 'flex',
            gap: 1,
            justifyContent: 'center',
            px: 1.5,
            py: 1,
          }}
        >
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: badge.bg,
              borderRadius: 999,
              color: badge.fg,
              display: 'inline-flex',
              flex: 'none',
              fontSize: 11,
              fontWeight: 700,
              height: 24,
              justifyContent: 'center',
              minWidth: 56,
              px: 1,
            }}
          >
            {operation.method}
          </Box>
          <Typography noWrap sx={{ fontFamily: 'monospace', fontSize: 14 }}>
            {operation.path || '/'}
          </Typography>
        </Box>
      </Box>

      <Box
        sx={{
          color: 'primary.main',
          display: 'flex',
          justifyContent: 'center',
        }}
      >
        <ArrowDown size={18} />
      </Box>

      <Box>
        <Typography
          color="text.secondary"
          sx={{
            display: 'block',
            fontWeight: 600,
            letterSpacing: '.4px',
            mb: 0.75,
            textAlign: 'center',
          }}
          variant="caption"
        >
          BACKEND RESOURCE
        </Typography>
        <Box
          sx={{
            alignItems: 'center',
            bgcolor: alpha(theme.palette.primary.main, 0.05),
            border: '1px solid',
            borderColor: alpha(theme.palette.primary.main, 0.4),
            borderRadius: 1.5,
            display: 'flex',
            gap: 1,
            justifyContent: 'center',
            px: 1.5,
            py: 1,
          }}
        >
          <Typography noWrap sx={{ fontFamily: 'monospace', fontSize: 14 }}>
            {backendPath || '/'}
          </Typography>
        </Box>
        <Typography
          color="text.secondary"
          sx={{ display: 'block', mt: 0.75, textAlign: 'center' }}
          variant="caption"
        >
          {backendPassthrough
            ? 'Passthrough — same path as the proxy resource.'
            : 'Requests are rewritten to this backend path.'}
        </Typography>
      </Box>

      <Divider />

      {/* editable fields */}
      <TextField
        fullWidth
        label="Method"
        onChange={(event) =>
          onChange({ method: event.target.value as ApiOperation['method'] })
        }
        select
        size="small"
        value={operation.method}
      >
        {HTTP_METHODS.map((m) => (
          <MenuItem key={m} value={m}>
            {m}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        error={!operation.path.trim().startsWith('/')}
        fullWidth
        label="Proxy path"
        onChange={(event) => onChange({ path: event.target.value })}
        placeholder="/sample/{id}"
        size="small"
        value={operation.path}
      />
      <TextField
        fullWidth
        helperText="Route this proxy resource to any backend resource of the same method."
        label="Connect to backend resource"
        onChange={(event) => {
          const value = event.target.value;
          onMapBackend(
            value === PASSTHROUGH || value === operation.path
              ? undefined
              : value
          );
        }}
        select
        size="small"
        value={backendPassthrough ? PASSTHROUGH : backendPath}
      >
        <MenuItem value={PASSTHROUGH}>
          <em>Passthrough (same path)</em>
        </MenuItem>
        {(backendPassthrough || connectOptions.includes(backendPath)
          ? connectOptions
          : [backendPath, ...connectOptions]
        ).map((path) => (
          <MenuItem key={path} value={path}>
            {operation.method} {path}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        fullWidth
        label="Name (optional)"
        onChange={(event) =>
          onChange({ name: event.target.value || undefined })
        }
        size="small"
        value={operation.name || ''}
      />

      <Divider />
      <Button
        color="error"
        onClick={onRemove}
        size="small"
        startIcon={<Trash2 size={16} />}
      >
        Remove resource
      </Button>
    </Stack>
  );
}
