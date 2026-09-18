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

/**
 * Associates a definition with the gateway against which the console is
 * performing tests.
 *
 * A definition may specify a server that differs from the gateway selected by
 * the user. The document's server is therefore replaced before it is passed to
 * the viewer.
 *
 * OpenAPI permits `servers` to be overridden at both the path and operation
 * levels, with the most specific definition taking precedence. Consequently,
 * setting only the top-level array would not ensure that all requests use the
 * selected gateway. These overrides are removed so that the gateway is the
 * sole server definition.
 */

const HTTP_METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'] as const;

/**
 * Returns a copy of `spec` whose only server is `baseUrl`.
 *
 * Handles Swagger 2's split representation as well, since a definition uploaded
 * to the platform may be either generation and the viewer renders both.
 * Returns the spec untouched when `baseUrl` is unusable, so a gateway that has
 * not resolved yet renders the document as written rather than a broken one.
 */
export const withServerUrl = (
  spec: Record<string, unknown>,
  baseUrl: string,
): Record<string, unknown> => {
  const normalized = baseUrl.trim().replace(/\/+$/, '');
  if (normalized === '') return spec;

  let parsed: URL;
  try {
    parsed = new URL(normalized);
  } catch {
    return spec;
  }

  const next: Record<string, unknown> = { ...spec, servers: [{ url: normalized }] };

  // Swagger 2 has no `servers`; it composes the address from three fields.
  if (typeof spec.swagger === 'string') {
    const basePath = parsed.pathname.replace(/\/+$/, '');
    next.schemes = [parsed.protocol.replace(':', '')];
    next.host = parsed.host;
    next.basePath = basePath === '' ? '/' : basePath;
  }

  const paths = spec.paths;
  if (typeof paths !== 'object' || paths === null) return next;

  next.paths = Object.fromEntries(
    Object.entries(paths as Record<string, unknown>).map(([path, pathItem]) => {
      if (typeof pathItem !== 'object' || pathItem === null) return [path, pathItem];

      const item = { ...(pathItem as Record<string, unknown>) };
      delete item.servers;

      HTTP_METHODS.forEach((method) => {
        const operation = item[method];
        if (typeof operation !== 'object' || operation === null) return;
        const nextOperation = { ...(operation as Record<string, unknown>) };
        delete nextOperation.servers;
        // Swagger 2 lets an operation pin its own schemes, which would override
        // the document-level one the same way `servers` does.
        delete nextOperation.schemes;
        item[method] = nextOperation;
      });

      return [path, item];
    }),
  );

  return next;
};
