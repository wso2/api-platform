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
 * Types shared by the API test console.
 *
 * The console is deliberately independent of any one page: it takes an OpenAPI
 * document, a base URL and a {@link ConsoleTransport}, and nothing else. How a
 * request actually leaves the browser — relayed through the control plane or
 * sent directly — is the transport's business, never the console's.
 */

export const CONSOLE_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const;
export type ConsoleMethod = (typeof CONSOLE_METHODS)[number];

/** Parameter slots the console can fill. `cookie` is not offered: browsers forbid setting it. */
export type ConsoleParamLocation = 'path' | 'query' | 'header';

export type ConsoleParamType = 'string' | 'integer' | 'number' | 'boolean' | 'array';

export type ConsoleParameter = {
  name: string;
  in: ConsoleParamLocation;
  required: boolean;
  type: ConsoleParamType;
  description?: string;
  enum?: string[];
  /** A value suggested by the spec (`example`, then `default`). */
  example?: string;
  minimum?: number;
  maximum?: number;
  deprecated: boolean;
};

export type ConsoleSchemaField = {
  name: string;
  type: string;
  required: boolean;
  description: string;
};

/** How a request body is encoded. Decides the editor offered and the wire format. */
export type ConsoleBodyKind = 'json' | 'xml' | 'text' | 'urlencoded' | 'multipart' | 'binary';

/** One field of a `urlencoded` or `multipart` body. */
export type ConsoleFormField = ConsoleSchemaField & {
  /** `format: binary` — a file input rather than a text input. */
  binary: boolean;
  example: string;
};

export type ConsoleBodyOption = {
  mediaType: string;
  kind: ConsoleBodyKind;
  /** Text offered as the starting point. Empty for form and binary bodies. */
  example: string;
  schemaName?: string;
  fields: ConsoleSchemaField[];
  formFields: ConsoleFormField[];
};

export type ConsoleResponseDoc = {
  status: string;
  description: string;
  mediaType?: string;
  schemaName?: string;
  example?: string;
  fields: ConsoleSchemaField[];
};

export type ConsoleOperation = {
  /** `METHOD path`, unique within a document. */
  key: string;
  method: ConsoleMethod;
  /** Path template as declared, e.g. `/books/{id}`. */
  path: string;
  summary: string;
  description: string;
  /** Grouping label: the first tag, or the first path segment. */
  group: string;
  deprecated: boolean;
  parameters: ConsoleParameter[];
  body?: { required: boolean; options: ConsoleBodyOption[] };
  responses: ConsoleResponseDoc[];
};

export type ConsoleSchemaDoc = {
  name: string;
  description: string;
  fields: ConsoleSchemaField[];
};

export type ConsoleSpecModel = {
  operations: ConsoleOperation[];
  schemas: ConsoleSchemaDoc[];
};

/** One part of a multipart body: a text field or a chosen file. */
export type ConsoleBodyPart =
  | { name: string; value: string }
  | { name: string; file: File };

export type ConsoleBody =
  | { kind: 'text'; mediaType: string; text: string }
  | { kind: 'form'; fields: [string, string][] }
  | { kind: 'multipart'; parts: ConsoleBodyPart[] }
  | { kind: 'binary'; mediaType: string; file: File };

/**
 * What a transport is asked to send.
 *
 * `baseUrl` and `path` stay separate so a transport can prove the request goes
 * where the console says it does: the base is the gateway's invoke URL and is
 * not editable, and the path is the operation's own, with parameter values
 * already percent-encoded as single segments.
 */
export type ConsoleSendRequest = {
  method: ConsoleMethod;
  baseUrl: string;
  path: string;
  query: [string, string][];
  headers: [string, string][];
  body?: ConsoleBody;
};

export type ConsoleSendResult = {
  status: number;
  statusText: string;
  headers: [string, string][];
  /** Decoded body text. Empty when the body is binary or absent. */
  bodyText: string;
  /** The body was cut at the console's display limit. */
  truncated: boolean;
  /** The body is not text, so it is not shown. */
  binary: boolean;
  sizeBytes: number;
  /** Total time as the user experienced it. */
  durationMs: number;
  /** The URL that was called. */
  url: string;
};

export type ConsoleTransport = (
  request: ConsoleSendRequest,
  signal: AbortSignal,
) => Promise<ConsoleSendResult>;

/**
 * A failure of the transport itself, as opposed to the API answering with an
 * error status. `message` is already user-facing and localized; the console
 * shows it as plain text and never interprets it.
 */
export class ConsoleTransportError extends Error {
  readonly aborted: boolean;

  constructor(message: string, options: { aborted?: boolean } = {}) {
    super(message);
    this.name = 'ConsoleTransportError';
    this.aborted = options.aborted ?? false;
  }
}

/** A console-supplied parameter, e.g. the test key. Shown locked, masked when secret. */
export type ConsoleAutoParam = {
  name: string;
  value: string;
  secret?: boolean;
};
