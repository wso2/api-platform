/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

/**
 * Turning a credential typed into a policy parameter into a stored secret.
 *
 * A policy definition declares which of its parameters are credentials with the
 * standard JSON Schema `writeOnly: true`. Where it does, a value typed into the
 * form is created as a secret and the parameter is saved as a
 * `{{ secret "handle" }}` reference instead of the credential itself — the same
 * thing this application already does for an upstream auth value.
 *
 * Where it does not, the value is saved as typed. An older gateway, or a policy
 * whose definition predates the annotation, declares nothing, and nothing here
 * treats that as an error or blocks the save: the parameter is stored as
 * plain text and resolving that is the operator's call, not this form's.
 * Everything below is therefore written to find nothing rather than to guess.
 */

import {
  buildSecretPlaceholder,
  createSecret,
  generateSecretHandle,
} from '../apis/secretApis';
import { logger } from './logger';

/**
 * Locates one parameter inside a policy's parameter map. Each element is a
 * property name, except {@link ARRAY_MARKER}, which stands for every element of
 * the array at that position.
 */
export type WriteOnlyPath = string[];

/** Path element standing for every element of an array. */
export const ARRAY_MARKER = '[]';

/** True when a value is already a reference rather than a credential. */
const isSecretPlaceholder = (value: string): boolean =>
  /^\{\{\s*secret\s+\\?"[^"\\]+\\?"\s*\}\}$/.test(value.trim());

const isRecord = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === 'object' && !Array.isArray(value);

/**
 * Collects the parameter paths a policy definition declares write-only.
 *
 * Three shapes are in use across the sources this application reads from, so
 * all three are accepted: a JSON Schema object node (`{type, properties}`),
 * which is what a policy definition publishes; a bare map of name to
 * definition; and a list of named entries. An input in none of these yields no
 * paths, which saves everything as typed.
 */
export const collectWriteOnlyPaths = (
  source: Record<string, unknown> | undefined,
): WriteOnlyPath[] => {
  if (!source) return [];
  const parameters = source.parameters ?? source.params;
  if (!parameters) return [];

  const found: WriteOnlyPath[] = [];

  if (Array.isArray(parameters)) {
    parameters.forEach((entry) => {
      if (isRecord(entry) && entry.writeOnly === true && typeof entry.name === 'string') {
        found.push([entry.name]);
      }
    });
    return found;
  }

  if (!isRecord(parameters)) return [];

  // A schema node carries its members under `properties`; a bare map is the
  // members themselves.
  const node = isRecord(parameters.properties) ? parameters : { properties: parameters };
  walkSchema(node, [], found);
  return found;
};

/** Appends to `found` the write-only paths at and below `node`. */
const walkSchema = (
  node: Record<string, unknown>,
  prefix: WriteOnlyPath,
  found: WriteOnlyPath[],
): void => {
  const writeOnly = node.writeOnly === true;
  if (writeOnly && prefix.length > 0) {
    found.push(prefix);
  }

  if (isRecord(node.properties)) {
    Object.entries(node.properties).forEach(([name, child]) => {
      const childPath = [...prefix, name];
      // A declaration on the root says every parameter is credential material.
      // There is no path that addresses the map itself, so it is expanded into
      // one path per parameter.
      if (writeOnly && prefix.length === 0) {
        found.push(childPath);
      }
      if (isRecord(child)) {
        walkSchema(child, childPath, found);
      }
    });
  }

  if (isRecord(node.items)) {
    walkSchema(node.items, [...prefix, ARRAY_MARKER], found);
  }
};

/** Reads the value at `path`, or undefined where the path does not resolve. */
const valueAt = (container: unknown, path: WriteOnlyPath): unknown => {
  return path.reduce<unknown>((current, element) => {
    if (element === ARRAY_MARKER) return undefined; // handled by the caller
    return isRecord(current) ? current[element] : undefined;
  }, container);
};

/** Returns a copy of `container` with `path` set to `value`. */
const withValueAt = (
  container: Record<string, unknown>,
  path: WriteOnlyPath,
  value: unknown,
): Record<string, unknown> => {
  const [head, ...rest] = path;
  if (rest.length === 0) {
    return { ...container, [head]: value };
  }
  const child = container[head];
  return {
    ...container,
    [head]: withValueAt(isRecord(child) ? child : {}, rest, value),
  };
};

/** How a created secret is labelled, so it is identifiable in the secret list. */
export type SecretDescription = {
  /** The artifact the policy is attached to, e.g. an LLM provider handle. */
  artifactName: string;
  /** The policy the parameter belongs to. */
  policyName: string;
};

/**
 * Replaces every declared write-only parameter holding a typed-in credential
 * with a `{{ secret "handle" }}` reference, creating the secret first.
 *
 * Left alone: a parameter the form did not set, one already holding a
 * reference, one that is not a string, and every parameter of a policy that
 * declares nothing write-only. A path that crosses an array is skipped —
 * no catalogue policy declares one, and promoting by position would attach a
 * credential to whichever element happened to sit there.
 *
 * `params` is not modified; the rewritten map is returned. A failure to create
 * a secret is raised to the caller rather than silently falling back to saving
 * the credential as typed.
 */
export const promoteWriteOnlyParams = async <T extends Record<string, unknown>>(
  params: T,
  paths: WriteOnlyPath[],
  describe: SecretDescription,
): Promise<T> => {
  if (paths.length === 0) return params;

  let promoted: Record<string, unknown> = params;
  for (const path of paths) {
    if (path.includes(ARRAY_MARKER)) continue;

    const value = valueAt(promoted, path);
    if (typeof value !== 'string' || value === '' || isSecretPlaceholder(value)) {
      continue;
    }

    const secretHandle = generateSecretHandle();
    // eslint-disable-next-line no-await-in-loop -- one secret per credential;
    // the count is the number of credential parameters on a single policy.
    const secret = await createSecret({
      id: secretHandle,
      displayName: `${describe.artifactName} ${path[path.length - 1]}`,
      description: `Auto-generated secret for ${describe.policyName} on ${describe.artifactName}`,
      value,
      type: 'GENERIC',
    });
    logger.info('Created a secret for a write-only policy parameter', {
      secretHandle,
      policyName: describe.policyName,
      parameter: path.join('.'),
    });

    promoted = withValueAt(promoted, path, buildSecretPlaceholder(secret.id));
  }

  return promoted as T;
};
