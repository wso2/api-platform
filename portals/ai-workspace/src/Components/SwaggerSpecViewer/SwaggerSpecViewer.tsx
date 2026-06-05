import React, { useMemo } from 'react';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';
import './SwaggerSpecViewer.css';

const SwaggerUIComponent =
  SwaggerUI as unknown as React.ComponentType<Record<string, unknown>>;

type SwaggerSpecViewerProps = {
  spec: Record<string, unknown>;
  className?: string;
  docExpansion?: 'list' | 'full' | 'none';
  defaultModelsExpandDepth?: number;
  displayRequestDuration?: boolean;
  requestBaseUrl?: string;
  defaultHeaders?: Record<string, string>;
  disableNetworkExecution?: boolean;
  hideInfoSection?: boolean;
  hideServers?: boolean;
  hideAuthorizeButton?: boolean;
  hideTagHeaders?: boolean;
  hideOperationHeader?: boolean;
  disableTryOutBtn?: boolean;
  disableResponseSection?: boolean;
};

type SwaggerSpec = Record<string, unknown>;
type ImmutableRequestLike = {
  get: (key: string) => unknown;
  set: (key: string, value: unknown) => unknown;
};

const HTTP_METHODS = [
  'get',
  'post',
  'put',
  'delete',
  'patch',
  'head',
  'options',
  'trace',
] as const;

function applyRequestBaseUrlToSpec(spec: SwaggerSpec, baseUrl: string): SwaggerSpec {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '');
  let parsedBase: URL;
  try {
    parsedBase = new URL(normalizedBaseUrl);
  } catch {
    return spec;
  }

  const isSwagger2 =
    typeof spec.swagger === 'string' && spec.swagger.startsWith('2.');
  const isOpenApi3 = typeof spec.openapi === 'string';
  const protocol = parsedBase.protocol.replace(':', '');

  const nextSpec: SwaggerSpec = { ...spec };

  if (isOpenApi3) {
    nextSpec.servers = [{ url: normalizedBaseUrl }];
  }

  if (isSwagger2) {
    const basePath = parsedBase.pathname.replace(/\/+$/, '') || '/';
    nextSpec.schemes = [protocol];
    nextSpec.host = parsedBase.host;
    nextSpec.basePath = basePath;
  }

  const rawPaths = spec.paths;
  if (!rawPaths || typeof rawPaths !== 'object') {
    return nextSpec;
  }

  const nextPaths: Record<string, unknown> = {};
  Object.entries(rawPaths as Record<string, unknown>).forEach(
    ([pathKey, pathValue]) => {
      if (!pathValue || typeof pathValue !== 'object') {
        nextPaths[pathKey] = pathValue;
        return;
      }

      const pathItem = pathValue as Record<string, unknown>;
      let nextPathItem: Record<string, unknown> = pathItem;
      let pathChanged = false;

      if (isOpenApi3) {
        nextPathItem = {
          ...nextPathItem,
          servers: [{ url: normalizedBaseUrl }],
        };
        pathChanged = true;
      }

      HTTP_METHODS.forEach((method) => {
        const operationValue = pathItem[method];
        if (!operationValue || typeof operationValue !== 'object') {
          return;
        }

        const operation = operationValue as Record<string, unknown>;
        let nextOperation: Record<string, unknown> = operation;
        let operationChanged = false;

        if (isOpenApi3) {
          nextOperation = {
            ...nextOperation,
            servers: [{ url: normalizedBaseUrl }],
          };
          operationChanged = true;
        }

        if (isSwagger2) {
          nextOperation = {
            ...nextOperation,
            schemes: [protocol],
          };
          operationChanged = true;
        }

        if (operationChanged) {
          if (!pathChanged) {
            nextPathItem = { ...nextPathItem };
            pathChanged = true;
          }
          nextPathItem[method] = nextOperation;
        }
      });

      nextPaths[pathKey] = pathChanged ? nextPathItem : pathItem;
    }
  );

  nextSpec.paths = nextPaths;
  return nextSpec;
}

function rewriteRequestUrlWithBase(originalUrl: string, baseUrl: string): string {
  try {
    const base = new URL(baseUrl);
    const current = /^https?:\/\//i.test(originalUrl)
      ? new URL(originalUrl)
      : new URL(originalUrl, base);

    const basePath = base.pathname.replace(/\/+$/, '');
    const currentPath = current.pathname.startsWith('/')
      ? current.pathname
      : `/${current.pathname}`;
    const shouldPrefixBasePath =
      basePath !== '' &&
      basePath !== '/' &&
      currentPath !== basePath &&
      !currentPath.startsWith(`${basePath}/`);
    const rewrittenPath = shouldPrefixBasePath
      ? `${basePath}${currentPath === '/' ? '' : currentPath}`
      : currentPath;

    return `${base.protocol}//${base.host}${rewrittenPath}${current.search}${current.hash}`;
  } catch {
    return originalUrl;
  }
}

function mergePlainHeaders(
  headers: unknown,
  defaultHeadersObject: Record<string, string>
): Record<string, unknown> {
  const nextHeaders =
    headers && typeof headers === 'object'
      ? { ...(headers as Record<string, unknown>) }
      : {};

  Object.entries(defaultHeadersObject).forEach(([key, value]) => {
    nextHeaders[key] = value;
  });

  return nextHeaders;
}

export default function SwaggerSpecViewer({
  spec,
  className,
  docExpansion = 'list',
  defaultModelsExpandDepth = -1,
  displayRequestDuration = true,
  requestBaseUrl,
  defaultHeaders,
  disableNetworkExecution = false,
  hideInfoSection = false,
  hideServers = false,
  hideAuthorizeButton = false,
  hideTagHeaders = false,
  hideOperationHeader = false,
  disableTryOutBtn = false,
  disableResponseSection = false,
}: SwaggerSpecViewerProps) {
  const normalizedRequestBaseUrl = requestBaseUrl?.trim().replace(/\/+$/, '');
  const normalizedDefaultHeaders = useMemo(
    () =>
      Object.entries(defaultHeaders ?? {}).filter(
        ([key, value]) => Boolean(key?.trim()) && Boolean(value?.trim())
      ),
    [defaultHeaders]
  );
  const defaultHeadersObject = useMemo<Record<string, string>>(
    () => Object.fromEntries(normalizedDefaultHeaders),
    [normalizedDefaultHeaders]
  );

  const specWithRequestBaseUrl = useMemo(() => {
    if (!normalizedRequestBaseUrl) {
      return spec;
    }
    return applyRequestBaseUrlToSpec(spec, normalizedRequestBaseUrl);
  }, [normalizedRequestBaseUrl, spec]);

  const plugin = useMemo(() => {
    const wrapSelectors: Record<string, unknown> = {};
    const wrapComponents: Record<string, unknown> = {};
    const wrapActions: Record<string, unknown> = {};

    if (hideAuthorizeButton) {
      wrapComponents.authorizeBtn = () => () => null;
      wrapSelectors.securityDefinitions = () => () => null;
      wrapSelectors.schemes = () => () => [];
    }

    if (disableNetworkExecution) {
      wrapActions.executeRequest =
        (oriAction: (request: Record<string, unknown>) => unknown) =>
        (request: Record<string, unknown>) => {
          const requestWithNoopFetch = {
            ...request,
            headers: mergePlainHeaders(request.headers, defaultHeadersObject),
            fetch: async () => ({
              ok: true,
              status: 0,
              statusText: 'Execution disabled',
              url: typeof request.url === 'string' ? request.url : '',
              headers: {},
              text: '',
              data: '',
            }),
          };

          return oriAction(requestWithNoopFetch);
        };

      wrapComponents.liveResponse =
        (
          _Original: React.ComponentType<Record<string, unknown>>,
          system: {
            specSelectors?: {
              requestFor?: (path: string, method: string) => unknown;
              mutatedRequestFor?: (path: string, method: string) => unknown;
            };
            Im?: { Map?: (value?: unknown) => unknown };
          }
        ) =>
        (props: Record<string, unknown>) => {
          try {
            const path = props.path as string | undefined;
            const method = props.method as string | undefined;
            const selectors =
              (props.specSelectors as {
                requestFor?: (p: string, m: string) => unknown;
                mutatedRequestFor?: (p: string, m: string) => unknown;
              }) ?? system.specSelectors;

            const rawRequest =
              (path && method && selectors?.mutatedRequestFor
                ? selectors.mutatedRequestFor(path, method)
                : null) ??
              (path && method && selectors?.requestFor
                ? selectors.requestFor(path, method)
                : null);

            if (
              !rawRequest ||
              typeof (rawRequest as { get?: unknown }).get !== 'function' ||
              typeof (rawRequest as { set?: unknown }).set !== 'function'
            ) {
              return null;
            }

            const immutableRequest = rawRequest as ImmutableRequestLike;
            const currentHeaders = immutableRequest.get('headers') as {
              merge?: (value: unknown) => unknown;
            };
            const mergedHeaders =
              currentHeaders && typeof currentHeaders.merge === 'function'
                ? currentHeaders.merge(defaultHeadersObject)
                : system.Im && typeof system.Im.Map === 'function'
                  ? system.Im.Map(defaultHeadersObject)
                  : defaultHeadersObject;
            const request = immutableRequest.set(
              'headers',
              mergedHeaders
            ) as ImmutableRequestLike;

            const getComponent = props.getComponent as
              | ((name: string, noErrorBoundary?: boolean) => React.ComponentType<Record<string, unknown>> | null)
              | undefined;
            const Curl = getComponent?.('curl', true);
            const RequestSnippets = getComponent?.('RequestSnippets', true);
            const requestUrl = String(request.get('url') ?? '');
            const getConfigs = props.getConfigs as
              | (() => Record<string, unknown>)
              | undefined;
            const requestSnippetsEnabled = Boolean(
              getConfigs?.().requestSnippetsEnabled
            );

            return (
              <div>
                {requestSnippetsEnabled && RequestSnippets ? (
                  <RequestSnippets request={request as Record<string, unknown>} />
                ) : Curl ? (
                  <Curl request={request as Record<string, unknown>} />
                ) : null}
                {requestUrl ? (
                  <div className="request-url">
                    <h4>Request URL</h4>
                    <pre className="microlight">{requestUrl}</pre>
                  </div>
                ) : null}
              </div>
            );
          } catch {
            return null;
          }
        };
    }

    const hasWrapSelectors = Object.keys(wrapSelectors).length > 0;
    const hasWrapActions = Object.keys(wrapActions).length > 0;
    const hasWrapComponents =
      hideInfoSection || Object.keys(wrapComponents).length > 0;

    if (!hasWrapSelectors && !hasWrapActions && !hasWrapComponents) {
      return undefined;
    }

    const swaggerPlugin: Record<string, unknown> = {};

    if (hasWrapSelectors || hasWrapActions) {
      swaggerPlugin.statePlugins = {
        spec: {
          ...(hasWrapSelectors ? { wrapSelectors } : {}),
          ...(hasWrapActions ? { wrapActions } : {}),
        },
      };
    }

    if (hasWrapComponents) {
      swaggerPlugin.wrapComponents = {
        ...(hideInfoSection ? { info: () => () => null } : {}),
        ...wrapComponents,
      };
    }

    return swaggerPlugin;
  }, [
    defaultHeadersObject,
    disableNetworkExecution,
    hideAuthorizeButton,
    hideInfoSection,
  ]);

  const plugins = plugin ? [plugin] : undefined;

  const requestInterceptor = useMemo(() => {
    if (!normalizedRequestBaseUrl && normalizedDefaultHeaders.length === 0) {
      return undefined;
    }

    return (request: Record<string, unknown>) => {
      const nextRequest: Record<string, unknown> = { ...request };
      const requestUrl = request.url;

      if (
        normalizedRequestBaseUrl &&
        typeof requestUrl === 'string' &&
        requestUrl.trim()
      ) {
        nextRequest.url = rewriteRequestUrlWithBase(
          requestUrl,
          normalizedRequestBaseUrl
        );
      }

      if (normalizedDefaultHeaders.length > 0) {
        nextRequest.headers = mergePlainHeaders(
          nextRequest.headers,
          defaultHeadersObject
        );
      }

      return nextRequest;
    };
  }, [
    defaultHeadersObject,
    normalizedDefaultHeaders.length,
    normalizedRequestBaseUrl,
  ]);

  const swaggerInstanceKey = useMemo(() => {
    const headersKey = normalizedDefaultHeaders
      .slice()
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]) => `${key}:${value}`)
      .join('|');

    return [
      normalizedRequestBaseUrl ?? '',
      disableNetworkExecution ? 'no-network' : 'network',
      headersKey,
    ].join('::');
  }, [
    disableNetworkExecution,
    normalizedDefaultHeaders,
    normalizedRequestBaseUrl,
  ]);

  const containerClassName = [
    'swagger-spec-viewer',
    hideInfoSection ? 'hide-info-section' : '',
    hideServers ? 'hide-servers' : '',
    hideAuthorizeButton ? 'hide-authorize' : '',
    hideTagHeaders ? 'hide-tag-headers' : '',
    hideOperationHeader ? 'hide-operation-header' : '',
    disableTryOutBtn ? 'disable-try-out-btn' : '',
    disableResponseSection ? 'disable-response-section' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div className={containerClassName}>
      <SwaggerUIComponent
        key={swaggerInstanceKey}
        spec={specWithRequestBaseUrl}
        docExpansion={docExpansion}
        defaultModelsExpandDepth={defaultModelsExpandDepth}
        displayRequestDuration={displayRequestDuration}
        showMutatedRequest={!disableNetworkExecution}
        plugins={plugins}
        requestInterceptor={requestInterceptor}
      />
    </div>
  );
}
