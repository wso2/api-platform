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

import { flatten, resolveRef, sampleFromSchema } from './jsonSample';
import {
  CONSOLE_METHODS,
  type ConsoleBodyKind,
  type ConsoleBodyOption,
  type ConsoleFormField,
  type ConsoleMethod,
  type ConsoleOperation,
  type ConsoleParameter,
  type ConsoleParamType,
  type ConsoleResponseDoc,
  type ConsoleSchemaDoc,
  type ConsoleSchemaField,
  type ConsoleSpecModel,
} from '../types';

/**
 * Reads an OpenAPI document into the shape the console renders.
 *
 * Everything here is data extraction over a parsed document: nothing is
 * evaluated, fetched or interpreted as markup. Only in-document `$ref`s are
 * followed (see `resolveRef`), so a definition cannot make the console reach
 * for a URL. Text that came from the document — summaries, descriptions,
 * examples — is returned as plain strings and is only ever rendered as text.
 *
 * OpenAPI 3.x is the primary target. Swagger 2.0 operations are read as far as
 * parameters, a JSON/form body and responses go, so an older definition still
 * produces a usable console rather than an empty one.
 */

type Json = Record<string, unknown>;

const isObject = (value: unknown): value is Json =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const str = (value: unknown): string => (typeof value === 'string' ? value : '');

const num = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

/** Hops a chain of local `$ref`s may take. Guards a ref that points at itself. */
const MAX_REF_HOPS = 8;

/** Follows `$ref`s to the node they name; `undefined` when a ref cannot be resolved. */
const deref = (spec: Json, node: unknown): Json | undefined => {
  let current = node;
  for (let hop = 0; hop < MAX_REF_HOPS; hop += 1) {
    if (!isObject(current)) return undefined;
    if (typeof current.$ref !== 'string') return current;
    current = resolveRef(spec, current.$ref);
  }
  return undefined;
};

/** The last segment of a `$ref`, which is the schema's name. */
const refName = (ref: unknown): string | undefined =>
  typeof ref === 'string' && ref.startsWith('#/') ? ref.split('/').pop() : undefined;

/** A short human label for a schema: `string (uuid)`, `array<Book>`, `Book`. */
export const typeLabel = (spec: Json, schema: unknown, depth = 0): string => {
  if (!isObject(schema) || depth > 4) return 'object';
  const named = refName(schema.$ref);
  if (named) return named;

  const resolved = flatten(spec, schema, 0);
  if (resolved.type === 'array') return `array<${typeLabel(spec, resolved.items, depth + 1)}>`;
  if (Array.isArray(resolved.enum)) return `${str(resolved.type) || 'string'} · enum`;

  const base = str(resolved.type) || (isObject(resolved.properties) ? 'object' : 'any');
  return str(resolved.format) ? `${base} (${str(resolved.format)})` : base;
};

/** How many levels of nested objects are listed under a field. */
const MAX_NESTING = 3;

/** The fields of an object schema, composed models included. */
export const fieldsOf = (
  spec: Json,
  schema: unknown,
  nesting = 0,
  visiting: readonly string[] = [],
): ConsoleSchemaField[] => {
  if (!isObject(schema)) return [];
  // A schema that contains itself is listed once, not unfolded forever.
  const own = refName(schema.$ref);
  if (own && visiting.includes(own)) return [];
  const path = own ? [...visiting, own] : visiting;
  const resolved = flatten(spec, schema, 0);

  // A response that is a list of something reads best as that something's fields.
  const target =
    resolved.type === 'array' && isObject(resolved.items)
      ? flatten(spec, resolved.items, 0)
      : resolved;
  if (!isObject(target.properties)) return [];

  const required = new Set(
    Array.isArray(target.required)
      ? target.required.filter((n): n is string => typeof n === 'string')
      : [],
  );
  return Object.entries(target.properties).map(([name, property]) => {
    const detail = isObject(property) ? flatten(spec, property, 0) : {};
    const enumValues = Array.isArray(detail.enum)
      ? ` (${detail.enum.map(String).join(' | ')})`
      : '';
    // An object, or an array of objects, can be opened to show its own fields.
    const nested =
      nesting < MAX_NESTING && isObject(property)
        ? fieldsOf(spec, detail.type === 'array' ? detail.items : property, nesting + 1, path)
        : [];
    return {
      name,
      type: typeLabel(spec, property),
      required: required.has(name),
      description: `${str(detail.description)}${enumValues}`.trim(),
      ...(nested.length > 0 ? { children: nested } : {}),
    };
  });
};

const stringify = (value: unknown): string => {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return '';
  }
};

/** The media type without parameters, lower-cased. */
const baseMediaType = (mediaType: string): string => mediaType.split(';')[0].trim().toLowerCase();

export const bodyKindOf = (mediaType: string): ConsoleBodyKind => {
  const base = baseMediaType(mediaType);
  if (base === 'application/x-www-form-urlencoded') return 'urlencoded';
  if (base.startsWith('multipart/')) return 'multipart';
  if (base === 'application/xml' || base === 'text/xml' || base.endsWith('+xml')) return 'xml';
  if (base === 'application/json' || base.endsWith('+json')) return 'json';
  if (base.startsWith('text/')) return 'text';
  return 'binary';
};

/** An example for a media-type entry: the author's own, else one built from the schema. */
const exampleOf = (spec: Json, media: Json): string => {
  if (media.example !== undefined) return stringify(media.example);
  if (isObject(media.examples)) {
    const first = Object.values(media.examples)
      .map((e) => deref(spec, e))
      .find(isObject);
    if (first && first.value !== undefined) return stringify(first.value);
  }
  if (!isObject(media.schema)) return '';
  const sample = sampleFromSchema(spec, media.schema);
  return sample === null || sample === undefined ? '' : stringify(sample);
};

const formFieldsOf = (spec: Json, schema: unknown): ConsoleFormField[] => {
  if (!isObject(schema)) return [];
  const resolved = flatten(spec, schema, 0);
  if (!isObject(resolved.properties)) return [];
  const required = new Set(
    Array.isArray(resolved.required)
      ? resolved.required.filter((n): n is string => typeof n === 'string')
      : [],
  );
  return Object.entries(resolved.properties).map(([name, property]) => {
    const detail = isObject(property) ? flatten(spec, property, 0) : {};
    const sample = sampleFromSchema(spec, property);
    return {
      name,
      type: typeLabel(spec, property),
      required: required.has(name),
      description: str(detail.description),
      binary: detail.format === 'binary',
      example:
        typeof sample === 'string' || typeof sample === 'number' || typeof sample === 'boolean'
          ? String(sample)
          : '',
    };
  });
};

const bodyOptionOf = (spec: Json, mediaType: string, mediaValue: unknown): ConsoleBodyOption => {
  const media = isObject(mediaValue) ? mediaValue : {};
  const kind = bodyKindOf(mediaType);
  const schema = media.schema;
  const isForm = kind === 'urlencoded' || kind === 'multipart';
  return {
    mediaType,
    kind,
    example: isForm || kind === 'binary' ? '' : exampleOf(spec, media),
    schemaName: isObject(schema) ? refName(schema.$ref) : undefined,
    fields: fieldsOf(spec, schema),
    formFields: isForm ? formFieldsOf(spec, schema) : [],
  };
};

const PARAM_TYPES: readonly ConsoleParamType[] = [
  'string',
  'integer',
  'number',
  'boolean',
  'array',
];

const parameterOf = (spec: Json, raw: unknown): ConsoleParameter | undefined => {
  const parameter = deref(spec, raw);
  if (!parameter) return undefined;
  const { name } = parameter;
  const location = parameter.in;
  if (typeof name !== 'string' || name === '') return undefined;
  if (location !== 'path' && location !== 'query' && location !== 'header') return undefined;

  // Swagger 2.0 carries the type on the parameter itself.
  const schema = isObject(parameter.schema) ? flatten(spec, parameter.schema, 0) : parameter;
  const declared = str(schema.type);
  const type = (PARAM_TYPES as readonly string[]).includes(declared)
    ? (declared as ConsoleParamType)
    : 'string';
  const example = parameter.example ?? schema.example ?? schema.default;

  return {
    name,
    in: location,
    // A path parameter is always required, whatever the document says.
    required: location === 'path' ? true : parameter.required === true,
    type,
    description: str(parameter.description) || undefined,
    enum: Array.isArray(schema.enum) ? schema.enum.map(String) : undefined,
    example:
      typeof example === 'string' || typeof example === 'number' || typeof example === 'boolean'
        ? String(example)
        : undefined,
    minimum: num(schema.minimum),
    maximum: num(schema.maximum),
    deprecated: parameter.deprecated === true,
    style: typeof parameter.style === 'string' ? parameter.style : undefined,
    explode: typeof parameter.explode === 'boolean' ? parameter.explode : undefined,
  };
};

const SWAGGER2_FORM = ['application/x-www-form-urlencoded', 'multipart/form-data'];

const bodyOf = (
  spec: Json,
  operation: Json,
  pathParameters: unknown[] = [],
): { required: boolean; options: ConsoleBodyOption[] } | undefined => {
  const body = deref(spec, operation.requestBody);
  if (body && isObject(body.content)) {
    const options = Object.entries(body.content).map(([mediaType, media]) =>
      bodyOptionOf(spec, mediaType, media),
    );
    if (options.length > 0) return { required: body.required === true, options };
  }

  // Swagger 2.0: a single `in: body` parameter, or `in: formData` parameters.
  // Operation-level parameters override path-level ones of the same name and place.
  const own = (Array.isArray(operation.parameters) ? operation.parameters : [])
    .map((p) => deref(spec, p))
    .filter(isObject);
  const ownKeys = new Set(own.map((p) => `${str(p.in)}:${str(p.name)}`));
  const params = [
    ...pathParameters
      .map((p) => deref(spec, p))
      .filter(isObject)
      .filter((p) => !ownKeys.has(`${str(p.in)}:${str(p.name)}`)),
    ...own,
  ];
  const bodyParam = params.find((p) => p.in === 'body');
  const consumes = (
    Array.isArray(operation.consumes)
      ? operation.consumes
      : Array.isArray(spec.consumes)
        ? spec.consumes
        : []
  ).filter((c): c is string => typeof c === 'string');
  if (bodyParam) {
    const mediaType = consumes[0] ?? 'application/json';
    return {
      required: bodyParam.required === true,
      options: [bodyOptionOf(spec, mediaType, { schema: bodyParam.schema })],
    };
  }
  const formParams = params.filter((p) => p.in === 'formData');
  if (formParams.length > 0) {
    const mediaType =
      consumes.find((c) => SWAGGER2_FORM.includes(c)) ?? 'application/x-www-form-urlencoded';
    const properties = Object.fromEntries(
      formParams.map((p) => [
        str(p.name),
        {
          type: p.type === 'file' ? 'string' : p.type,
          format: p.type === 'file' ? 'binary' : p.format,
          description: p.description,
          enum: p.enum,
        },
      ]),
    );
    const requiredNames = formParams.filter((p) => p.required === true).map((p) => str(p.name));
    return {
      required: requiredNames.length > 0,
      options: [
        bodyOptionOf(spec, mediaType, {
          schema: { type: 'object', properties, required: requiredNames },
        }),
      ],
    };
  }
  return undefined;
};

const responsesOf = (spec: Json, operation: Json): ConsoleResponseDoc[] => {
  if (!isObject(operation.responses)) return [];
  return Object.entries(operation.responses).map(([status, raw]) => {
    const response = deref(spec, raw) ?? {};
    const content = isObject(response.content) ? response.content : {};
    // Swagger 2.0 lists what an operation produces separately from its responses.
    const produced = (
      Array.isArray(operation.produces)
        ? operation.produces
        : Array.isArray(spec.produces)
          ? spec.produces
          : []
    ).filter((type): type is string => typeof type === 'string');
    const mediaTypes = Object.keys(content).length > 0 ? Object.keys(content) : produced;
    const mediaType = Object.keys(content)[0];
    const media = mediaType ? (content[mediaType] as unknown) : undefined;
    const mediaObject = isObject(media) ? media : {};
    // Swagger 2.0 keeps the schema directly on the response.
    const schema = mediaObject.schema ?? response.schema;
    return {
      status,
      description: str(response.description),
      mediaType,
      mediaTypes,
      schemaName: isObject(schema) ? refName(schema.$ref) : undefined,
      example: mediaType
        ? exampleOf(spec, mediaObject) || undefined
        : isObject(schema)
          ? stringify(sampleFromSchema(spec, schema))
          : undefined,
      fields: fieldsOf(spec, schema),
    };
  });
};

const groupOf = (operation: Json, path: string): string => {
  const tag = Array.isArray(operation.tags)
    ? operation.tags.find((t) => typeof t === 'string' && t !== '')
    : undefined;
  if (typeof tag === 'string') return tag;
  const segment = path.split('/').find((part) => part !== '' && !part.startsWith('{'));
  return segment ? `/${segment}` : '/';
};

const schemasOf = (spec: Json): ConsoleSchemaDoc[] => {
  const components = isObject(spec.components) ? spec.components : {};
  const source = isObject(components.schemas)
    ? components.schemas
    : isObject(spec.definitions)
      ? spec.definitions
      : {};
  return Object.entries(source).map(([name, schema]) => ({
    name,
    description: isObject(schema) ? str(schema.description) : '',
    fields: fieldsOf(spec, schema),
  }));
};

/** Reads the whole document. Never throws: an unreadable part is skipped, not fatal. */
export const parseConsoleSpec = (spec: Json): ConsoleSpecModel => {
  const operations: ConsoleOperation[] = [];
  const paths = isObject(spec.paths) ? spec.paths : {};

  Object.entries(paths).forEach(([path, rawItem]) => {
    const item = deref(spec, rawItem);
    if (!item) return;

    CONSOLE_METHODS.forEach((method: ConsoleMethod) => {
      const operation = item[method.toLowerCase()];
      if (!isObject(operation)) return;

      // Operation-level parameters override path-level ones of the same name and place.
      const own = (Array.isArray(operation.parameters) ? operation.parameters : [])
        .map((p) => parameterOf(spec, p))
        .filter((p): p is ConsoleParameter => p !== undefined);
      const ownKeys = new Set(own.map((p) => `${p.in}:${p.name}`));
      const shared = (Array.isArray(item.parameters) ? item.parameters : [])
        .map((p) => parameterOf(spec, p))
        .filter((p): p is ConsoleParameter => p !== undefined && !ownKeys.has(`${p.in}:${p.name}`));

      operations.push({
        key: `${method} ${path}`,
        method,
        path,
        summary: str(operation.summary) || `${method} ${path}`,
        description: str(operation.description),
        group: groupOf(operation, path),
        deprecated: operation.deprecated === true,
        parameters: [...shared, ...own],
        body: bodyOf(spec, operation, Array.isArray(item.parameters) ? item.parameters : []),
        responses: responsesOf(spec, operation),
      });
    });
  });

  return { operations, schemas: schemasOf(spec) };
};
