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

import yaml from 'js-yaml';

/**
 * Converts stored OpenAPI content into the shape consumed by the application.
 *
 * This module contains pure, independently testable parsing and normalization
 * logic; transport concerns remain in `restApis.endpoints.ts`.
 */

/** A parsed OpenAPI document. Structure is the spec's, not ours — hence `unknown`. */
export type OpenApiDocument = Record<string, unknown>;

/** An API definition with its version and server URL normalized for consumers. */
export type RestApiDefinition = {
  /** The parsed document, handed to a spec viewer unmodified. */
  spec: OpenApiDocument;
  /** The document's own version string — `3.0.1`, `2.0`, or `'unknown'`. */
  specVersion: string;
  /** Base URL declared by the document, used as the default try-out target. */
  serverUrl?: string;
};

/** A trimmed string, or `undefined` when nothing usable is left. */
const text = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const next = value.trim();
  return next === '' ? undefined : next;
};

/**
 * Extracts the base URL declared by an OpenAPI or Swagger document.
 *
 * For OpenAPI 3 documents, the URL is read from `servers[0].url`. For Swagger 2
 * documents, it is constructed from `schemes`, `host`, and `basePath`, using
 * `https` when no scheme is specified. Relative OpenAPI 3 server URLs are
 * returned unchanged and are resolved by the calling layer.
 */
export const serverUrlOf = (spec: OpenApiDocument): string | undefined => {
  const servers = spec.servers;
  if (Array.isArray(servers) && servers.length > 0) {
    const first = servers[0];
    const url = text((first as Record<string, unknown> | undefined)?.url);
    if (url) return url;
  }

  const host = text(spec.host);
  if (!host) return undefined;

  const schemes = Array.isArray(spec.schemes) ? spec.schemes : [];
  const scheme = text(schemes[0]) ?? 'https';
  const basePath = text(spec.basePath) ?? '';
  // `/` is Swagger 2's "no prefix" value; appending it would produce a trailing
  // slash that doubles up once a path is joined on.
  return `${scheme}://${host}${basePath === '/' ? '' : basePath}`;
};

/** The document's own version string, from whichever field its generation uses. */
export const specVersionOf = (spec: OpenApiDocument): string =>
  text(spec.openapi) ?? text(spec.swagger) ?? 'unknown';

/** A parsed document — an object, not a list or a scalar. */
const isDocument = (value: unknown): value is OpenApiDocument =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/**
 * Parses the content returned by the API definition endpoint.
 *
 * The definition is expected to be serialized as YAML. JSON content is also
 * supported because YAML is a superset of JSON. The parser does not attempt to
 * infer or report the source format.
 *
 * @param content The serialized API definition.
 * @returns The parsed OpenAPI document.
 * @throws If the content is empty, malformed, or does not represent an object.
 */
export const parseSpecContent = (content: string): OpenApiDocument => {
  const source = typeof content === 'string' ? content.trim() : '';
  if (source === '') {
    throw new Error('The API definition is empty');
  }

  // `yaml.load` throws on malformed input, which propagates as the parse
  // failure it is.
  const parsed = yaml.load(source);

  if (!isDocument(parsed)) {
    throw new Error('The API definition is not an OpenAPI document');
  }

  return parsed;
};

/** Prints a document as the YAML the endpoint stores. Used to build samples. */
export const serializeSpecContent = (spec: OpenApiDocument): string =>
  // Keep scalars on one line and expand repeated objects instead of using refs.
  yaml.dump(spec, { indent: 2, lineWidth: -1, noRefs: true });

/**
 * Wraps a parsed document in the fields consumers read.
 *
 * Exported because both the sample and the real endpoint need exactly this —
 * only the `source` argument differs between them.
 */
export const toRestApiDefinition = (
  spec: OpenApiDocument,
): RestApiDefinition => ({
  spec,
  specVersion: specVersionOf(spec),
  serverUrl: serverUrlOf(spec),
});
