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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box,
  FormControl,
  FormLabel,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import { Search } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import SwaggerUI from 'swagger-ui-react';
// Shared spec-editor styles; the console's overrides follow.
import '@/components/SwaggerSpecViewer/specViewerStyles';
import './TestConsoleSpecViewer.css';

import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { buildConsoleRequest } from '../utils/operationRequest';
import {
  filterSpecResources,
  hasResourceOperations,
  SPEC_HTTP_METHODS,
  type ResourceMethod,
} from './utils/filterSpec';
import {
  formValuesOf,
  resolveShownOperation,
  specJsonOf,
  type SwaggerSystemLike,
} from './utils/shownOperation';
import { testConsoleRelayPlugin, type RelayContext } from './utils/proxyTransport';
import { withServerUrl } from './utils/specServers';
import { fromSwaggerRequest } from './utils/swaggerRequest';
import type { ConsoleRequest, KeyValueRow } from '../utils/types';

/**
 * The test console's spec viewer.
 *
 * A sibling of `components/SwaggerSpecViewer` rather than a change to it: that
 * component is mounted by the API overview and the creation wizard, where a
 * regression is a regression in two shipped pages.
 *
 * ## Nothing here wraps a swagger *component*
 *
 * No `wrapComponents`. An earlier version injected a reporter beside swagger's
 * own `OperationContainer` to get a live view of the form — wrapping the very
 * component that owns expand/collapse, on the code path that was reported
 * broken, and in a way this repo cannot test (jsdom resolves swagger's `node`
 * build, which carries its own React and will not reconcile against React 19;
 * the browser resolves `swagger-ui-es-bundle-core`, which uses ours). Reading
 * the store on a short interval gets the same result without touching anything
 * that renders — see `shownOperation.ts`.
 *
 * There *is* one plugin: the relay transport in `proxyTransport.ts`. It wraps a
 * state **action** (`spec.executeRequest`) rather than a component, so it never
 * participates in rendering and the reconciliation problem above does not reach
 * it. Its own module explains why the relay exists and why the request the user
 * sees is still the real gateway call.
 *
 * ## Styling comes from the shared spec viewer
 *
 * This component carries the shared `swagger-spec-viewer` class and imports
 * that component's stylesheet, so both spec editors in the portal are painted
 * by one set of rules instead of two that happen to agree. What the console
 * wants hidden is expressed through the shared sheet's own modifier classes,
 * and `TestConsoleSpecViewer.css` holds only the single rule the console has to
 * differ on (the Execute button's label).
 *
 * ## swagger-ui-react reads most props exactly once
 *
 * Its wrapper builds the system inside an effect with `[]` deps, so
 * `requestInterceptor`, `onComplete` and every config flag are captured at
 * **mount**; only `spec` and `url` are watched afterwards. Passing a fresh
 * closure each render therefore does not update anything — it silently keeps
 * the first one, which here meant a `baseUrl` of `''` (gateways had not loaded)
 * and no test key. So the interceptor is built once and reads live values
 * through a ref.
 */

const messages = defineMessages({
  allMethods: {
    id: 'apiControlPlane.pages.test.console.TestConsoleSpecViewer.allMethods',
    defaultMessage: 'All methods',
    description: 'Option that leaves the resource list unfiltered by HTTP method.',
  },
  methodLabel: {
    id: 'apiControlPlane.pages.test.console.TestConsoleSpecViewer.methodLabel',
    defaultMessage: 'Filter by method',
    description: 'Accessible label for the HTTP method filter above the resource list.',
  },
  noResources: {
    id: 'apiControlPlane.pages.test.console.TestConsoleSpecViewer.noResources',
    defaultMessage: 'No resources match your search.',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.test.console.TestConsoleSpecViewer.searchPlaceholder',
    defaultMessage: 'Search resources by path or description',
  },
});

const SwaggerUIComponent = SwaggerUI as unknown as React.ComponentType<Record<string, unknown>>;

/** Delay resource filtering to avoid resetting open or partially filled forms. */
const SEARCH_DEBOUNCE_MS = 250;

/** Poll the open operation's form; Swagger emits no parameter-edit event. */
const SYNC_INTERVAL_MS = 250;

/**
 * Appends the console's query parameters to an outgoing request URL.
 *
 * Used for credentials that the `api-key-auth` policy places in the query
 * string. `URL`/`searchParams` handles encoding, and `set` prevents duplicate
 * parameters when a request is repeated.
 *
 * Unparseable URLs are returned unchanged so Swagger can send the request
 * without modification.
 */
export const withExtraQueryParams = (rawUrl: unknown, params: readonly KeyValueRow[]): unknown => {
  if (typeof rawUrl !== 'string' || params.length === 0) return rawUrl;

  try {
    const url = new URL(rawUrl);
    params
      .filter((param) => param.enabled && param.name.trim() !== '')
      .forEach((param) => url.searchParams.set(param.name.trim(), param.value));
    return url.toString();
  } catch {
    return rawUrl;
  }
};

export type TestConsoleSpecViewerProps = {
  spec: Record<string, unknown>;
  /** Invoke URL of the selected gateway. Every request is retargeted here. */
  baseUrl: string;
  /** The API under test. Sent to the relay, which resolves the target itself. */
  restApiId: string;
  /** Which of the API's deployed gateways to send to. */
  gatewayId: string;
  /** Organization handle, so the relay's lookup is scoped the same way the page is. */
  orgHandle: string;
  /** Headers the console adds to every request, e.g. the test key. */
  extraHeaders?: KeyValueRow[];
  /** Query parameters added to every request, including query-based credentials. */
  extraQueryParams?: KeyValueRow[];
  /** Credential header or query parameter to mask in the cURL view. */
  secretHeaderName?: string;
  /** Called with the complete request represented by the form. */
  onRequestChange?: (request: ConsoleRequest) => void;
};

/** Everything the mount-time closures need to read at call time. */
type LiveProps = {
  baseUrl: string;
  extraHeaders: KeyValueRow[];
  extraQueryParams: KeyValueRow[];
  onRequestChange?: (request: ConsoleRequest) => void;
  secretHeaderName?: string;
};

export default function TestConsoleSpecViewer({
  baseUrl,
  extraHeaders,
  extraQueryParams,
  gatewayId,
  onRequestChange,
  orgHandle,
  restApiId,
  secretHeaderName,
  spec,
}: TestConsoleSpecViewerProps) {
  const intl = useIntl();
  const theme = useTheme();

  /** Live values, updated on each render for request interception. */
  const live = useRef<LiveProps>({
    baseUrl,
    extraHeaders: extraHeaders ?? [],
    extraQueryParams: extraQueryParams ?? [],
    onRequestChange,
    secretHeaderName,
  });
  live.current = {
    baseUrl,
    extraHeaders: extraHeaders ?? [],
    extraQueryParams: extraQueryParams ?? [],
    onRequestChange,
    secretHeaderName,
  };

  /**
   * Live relay context. Separate from `live` above because it is read by the
   * transport rather than by the form sync, and because it is the one thing a
   * captured closure would get wrong in a way the user would notice: swagger
   * keeps only the first `plugins` value, so a captured gateway id would stay
   * pinned to whichever gateway was selected at mount.
   */
  const relay = useRef<RelayContext>({ baseUrl, gatewayId, intl, orgHandle, restApiId });
  relay.current = { baseUrl, gatewayId, intl, orgHandle, restApiId };

  /**
   * Sends try-out requests through the BFF instead of straight at the gateway.
   *
   * Built once, like every other mount-time value swagger keeps. It replaces
   * only the transport; the request swagger built, displayed and put in the
   * curl snippet is the real gateway call, untouched. See proxyTransport.ts.
   */
  const plugins = useMemo(() => [testConsoleRelayPlugin(relay)], []);

  /** Swagger's system, handed over once it has finished initialising. */
  const system = useRef<SwaggerSystemLike | undefined>(undefined);

  /** Last request published, so an unchanged form publishes nothing. */
  const lastSerialized = useRef<string | undefined>(undefined);

  /**
   * Document configured for the selected gateway.
   *
   * Preserve its reference across renders to prevent swagger-ui-react from
   * reparsing the document and resetting the console state.
   */
  const targetedSpec = useMemo(() => withServerUrl(spec, baseUrl), [baseUrl, spec]);

  const [search, setSearch] = useState('');
  const [method, setMethod] = useState<ResourceMethod>('all');
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);

  /**
   * The document as rendered.
   *
   * `filterSpecResources` returns `targetedSpec` unchanged while nothing is
   * being filtered, so the common case keeps one stable reference and swagger
   * never re-parses.
   */
  const displayedSpec = useMemo(
    () => filterSpecResources(targetedSpec, debouncedSearch, method),
    [debouncedSearch, method, targetedSpec],
  );

  const hasMatches = useMemo(() => hasResourceOperations(displayedSpec), [displayedSpec]);

  /** Theme values consumed by `TestConsoleSpecViewer.css`. */
  const themeVariables = useMemo(
    () =>
      ({
        '--tc-primary-contrast': theme.palette.primary.contrastText,
      }) as React.CSSProperties,
    [theme],
  );

  /** Publishes the open operation's request, if it has changed. */
  const publish = (request: ConsoleRequest) => {
    // Exclude row IDs from comparison because they are regenerated on each
    // build; including them would make every polling interval appear changed
    // and cause the cURL view to remount its inputs during editing.
    const serialized = JSON.stringify(request, (key, value) => (key === 'id' ? undefined : value));
    if (serialized === lastSerialized.current) return;
    lastSerialized.current = serialized;
    live.current.onRequestChange?.(request);
  };

  /** Polls the open operation's form. Started once, torn down on unmount. */
  useEffect(() => {
    const tick = () => {
      const current = live.current;
      if (!current.onRequestChange || !system.current) return;

      try {
        const shown = resolveShownOperation(system.current);
        if (!shown) return;

        const specJson = specJsonOf(system.current);
        if (!specJson) return;

        const { bodyValue, contentType, parameterValues } = formValuesOf(
          system.current,
          shown.path,
          shown.method,
        );

        const built = buildConsoleRequest({
          baseUrl: current.baseUrl,
          bodyValue,
          contentType,
          extraHeaders: current.extraHeaders,
          extraQueryParams: current.extraQueryParams,
          method: shown.method,
          parameterValues,
          path: shown.path,
          spec: specJson,
        });
        // An operation the console cannot represent keeps the previous command
        // rather than replacing it with one built from a substituted verb.
        if (built) publish(built);
      } catch {
        // Keep the previous command if Swagger's unpublished store shape changes.
      }
    };

    const timer = window.setInterval(tick, SYNC_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, []);

  /**
   * Adds the console's headers to every outgoing request, and reports the
   * request actually being sent.
   *
   * The authoritative capture: by the time an interceptor runs the URL and
   * headers are final, so it supersedes whatever the poll inferred. Built once
   * and reading through the ref, since swagger only ever sees this first value.
   */
  const requestInterceptor = useMemo(
    () => (request: Record<string, unknown>) => {
      const current = live.current;
      const headers = { ...((request.headers as Record<string, unknown>) ?? {}) };

      current.extraHeaders
        .filter((header) => header.enabled && header.name.trim() !== '')
        .forEach((header) => {
          const name = header.name.trim();
          Object.keys(headers).forEach((existingName) => {
            if (existingName.toLowerCase() === name.toLowerCase()) {
              delete headers[existingName];
            }
          });
          headers[name] = header.value;
        });

      const next: Record<string, unknown> = {
        ...request,
        headers,
        url: withExtraQueryParams(request.url, current.extraQueryParams),
      };

      const captured = fromSwaggerRequest(next, current.baseUrl, current.secretHeaderName);
      if (captured) publish(captured);

      return next;
    },
    [],
  );

  const onComplete = useMemo(
    () => (loaded: SwaggerSystemLike) => {
      system.current = loaded;
    },
    [],
  );

  return (
    <Box
      // Hide info header and server picker; console manages gateway selection
      className="swagger-spec-viewer hide-info-section hide-servers test-console-spec-viewer"
      style={themeVariables}
    >
      <Stack spacing={2}>
        <Stack direction="row" spacing={2}>
          <TextField
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            size="small"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={18} />
                  </InputAdornment>
                ),
              },
            }}
            value={search}
          />
          {/* Hidden label for accessibility; Select's aria-labelledby points here */}
          <FormControl sx={{ flexShrink: 0, width: 180 }}>
            <FormLabel id="test-console-method-label" sx={{ display: 'none' }}>
              <FormattedMessage {...messages.methodLabel} />
            </FormLabel>
            <Select
              labelId="test-console-method-label"
              onChange={(event) => setMethod(event.target.value as ResourceMethod)}
              size="small"
              value={method}
            >
              <MenuItem value="all">
                <FormattedMessage {...messages.allMethods} />
              </MenuItem>
              {SPEC_HTTP_METHODS.map((verb) => (
                <MenuItem key={verb} value={verb}>
                  {verb.toUpperCase()}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Stack>

        {/* Keep viewer mounted to preserve swagger state when hidden */}
        <Box hidden={!hasMatches}>
          <SwaggerUIComponent
            defaultModelsExpandDepth={1}
            displayRequestDuration
            docExpansion="list"
            onComplete={onComplete}
            plugins={plugins}
            requestInterceptor={requestInterceptor}
            spec={displayedSpec}
            tryItOutEnabled
          />
        </Box>

        {!hasMatches && (
          <Typography color="text.secondary" sx={{ py: 2 }} variant="body2">
            <FormattedMessage {...messages.noResources} />
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
