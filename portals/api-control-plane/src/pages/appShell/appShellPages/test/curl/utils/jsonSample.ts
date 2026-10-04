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
 * Builds an example request body from an operation's JSON Schema.
 *
 * Backs the body editor's "Insert sample". The point is to get the user to a
 * *sendable* body in one click: the samples this portal renders carry `example`
 * values on most properties, so the result is usually realistic rather than a
 * skeleton of empty strings.
 *
 * Resolution order per property is deliberate — `example`, then `default`, then
 * the first `enum` member, then a type-shaped placeholder. Anything the spec
 * author actually wrote beats anything this module can invent.
 */

type Schema = Record<string, unknown>;

/** Guards runaway recursion on a self-referencing schema. */
const MAX_DEPTH = 6;

const isObject = (value: unknown): value is Schema =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/**
 * Follows a local `$ref` into the document's own component schemas.
 *
 * Only in-document refs are followed. A remote `$ref` would mean fetching a URL
 * out of an untrusted definition, which is exactly the request-forgery shape
 * the portal's SSRF rule forbids — and a sample body is never worth a network
 * call. An unresolvable ref yields `undefined`, and the caller falls back.
 */
export const resolveRef = (spec: Schema, ref: string): Schema | undefined => {
  if (!ref.startsWith('#/')) return undefined;

  const segments = ref
    .slice(2)
    .split('/')
    // JSON Pointer escapes, per RFC 6901: ~1 is "/" and ~0 is "~".
    .map((segment) => segment.replace(/~1/g, '/').replace(/~0/g, '~'));

  let current: unknown = spec;
  for (const segment of segments) {
    if (!isObject(current)) return undefined;
    current = current[segment];
  }

  return isObject(current) ? current : undefined;
};

/** Merges an `allOf` chain into one schema, so composed models produce a body. */
const flatten = (spec: Schema, schema: Schema, depth: number): Schema => {
  const ref = schema.$ref;
  if (typeof ref === 'string') {
    const resolved = depth < MAX_DEPTH ? resolveRef(spec, ref) : undefined;
    return resolved ? flatten(spec, resolved, depth + 1) : {};
  }

  // `oneOf`/`anyOf` are a choice, and the first branch is as good a guess as
  // any — better than emitting nothing for a valid schema.
  const branch = schema.oneOf ?? schema.anyOf;
  if (Array.isArray(branch) && branch.length > 0 && isObject(branch[0])) {
    return flatten(spec, branch[0], depth + 1);
  }

  if (!Array.isArray(schema.allOf)) return schema;

  return schema.allOf.reduce<Schema>(
    (merged, member) => {
      if (!isObject(member)) return merged;
      const resolved = flatten(spec, member, depth + 1);
      const properties = {
        ...(isObject(merged.properties) ? merged.properties : {}),
        ...(isObject(resolved.properties) ? resolved.properties : {}),
      };
      return {
        ...merged,
        ...resolved,
        ...(Object.keys(properties).length > 0 ? { properties } : {}),
      };
    },
    { ...schema, allOf: undefined },
  );
};

/** Whether a property is response-only, including through `$ref`/`allOf`. */
const isResponseOnly = (spec: Schema, property: unknown, depth: number): boolean =>
  isObject(property) && flatten(spec, property, depth).readOnly === true;

/** A placeholder for a schema that says only what type it is. */
const placeholderFor = (type: unknown, format: unknown): unknown => {
  switch (type) {
    case 'integer':
    case 'number':
      return 0;
    case 'boolean':
      return true;
    case 'string':
      return format === 'date-time'
        ? new Date(0).toISOString()
        : format === 'date'
          ? '1970-01-01'
          : 'string';
    default:
      return 'string';
  }
};

/** Builds an example value for one schema. */
export const sampleFromSchema = (spec: Schema, schema: unknown, depth = 0): unknown => {
  if (!isObject(schema) || depth > MAX_DEPTH) return null;

  const resolved = flatten(spec, schema, depth);

  // Whatever the author wrote wins over anything inferred.
  if (resolved.example !== undefined) return resolved.example;
  if (resolved.default !== undefined) return resolved.default;
  if (Array.isArray(resolved.enum) && resolved.enum.length > 0) return resolved.enum[0];

  const type =
    resolved.type ??
    (isObject(resolved.properties) ? 'object' : resolved.items ? 'array' : undefined);

  if (type === 'object' || isObject(resolved.properties)) {
    const properties = isObject(resolved.properties) ? resolved.properties : {};
    return Object.fromEntries(
      Object.entries(properties)
        // Omit response-only properties rather than sampling `undefined`.
        .filter(([, property]) => !isResponseOnly(spec, property, depth + 1))
        .map(([name, property]) => [name, sampleFromSchema(spec, property, depth + 1)]),
    );
  }

  if (type === 'array') {
    // One element, not zero: an empty array tells the user nothing about the
    // shape they are supposed to fill in.
    return [sampleFromSchema(spec, resolved.items, depth + 1)];
  }

  return placeholderFor(type, resolved.format);
};

/** Finds the JSON content entry, allowing case and media-type parameters. */
const jsonContentOf = (content: Schema): unknown => {
  if (content['application/json'] !== undefined) return content['application/json'];

  const key = Object.keys(content).find(
    // Everything from the first `;` is parameters, not part of the type.
    (mediaType) => mediaType.split(';')[0].trim().toLowerCase() === 'application/json',
  );

  return key === undefined ? undefined : content[key];
};

/**
 * The JSON request-body sample for an operation, pretty-printed.
 *
 * Returns `undefined` when the operation takes no JSON body, which is what
 * lets the editor hide "Insert sample" rather than offer a button that inserts
 * `null`.
 */
export const requestBodySample = (
  spec: Schema,
  path: string,
  method: string,
): string | undefined => {
  const paths = spec.paths;
  if (!isObject(paths)) return undefined;

  const pathItem = paths[path];
  if (!isObject(pathItem)) return undefined;

  const operation = pathItem[method.toLowerCase()];
  if (!isObject(operation)) return undefined;

  const requestBody = isObject(operation.requestBody)
    ? operation.requestBody
    : typeof operation.requestBody === 'object'
      ? undefined
      : undefined;
  if (!requestBody) return undefined;

  const resolvedBody =
    typeof requestBody.$ref === 'string' ? resolveRef(spec, requestBody.$ref) : requestBody;
  if (!isObject(resolvedBody) || !isObject(resolvedBody.content)) return undefined;

  const jsonContent = jsonContentOf(resolvedBody.content);
  if (!isObject(jsonContent)) return undefined;

  // A media-type level `example` is the author's own whole-body example and
  // beats anything assembled from the schema.
  const sample =
    jsonContent.example !== undefined
      ? jsonContent.example
      : sampleFromSchema(spec, jsonContent.schema);

  if (sample === null || sample === undefined) return undefined;

  try {
    return JSON.stringify(sample, null, 2);
  } catch {
    return undefined;
  }
};
