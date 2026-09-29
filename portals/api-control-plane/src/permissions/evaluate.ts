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

import { OPERATION_SCOPES, type ApScope, type OperationId } from '../api/core/spec';

/**
 * Re-exported so everything inside `permissions/` names scopes from one place,
 * rather than half of it reaching back into the API layer for the type.
 */
export type { ApScope } from '../api/core/spec';

/**
 * Decides what the signed-in user may be *offered*, from the scopes their token
 * carries and the scopes the spec says each operation accepts.
 *
 * This module is deliberately pure — no React, no network, no imports beyond
 * the generated map and its types — because every policy branch in the console
 * lives here and nowhere else. A page that renders its own opinion about a
 * missing scope is how a permission model drifts into forty inconsistent ones.
 *
 * **It is not a security boundary.** The Platform API's `ScopeEnforcer` is the
 * only thing that decides whether an operation happens; this decides whether a
 * button looks clickable. Every branch below is written on the assumption that
 * it may be wrong and that a 403 must still be handled gracefully — which is
 * also why the uncertain cases fail *open* rather than closed.
 */

/* -------------------------------------------------------------------------- */
/* Types                                                                       */
/* -------------------------------------------------------------------------- */

/**
 * How to treat a session whose granted scopes are unknown.
 *
 * `enforce` - absent scopes mean nothing is offered.
 * `permissive` - absent scopes mean everything is offered, and the server
 * decides. This is the correct posture for a deployment in role mode (where the
 * token carries roles, not scopes, and the expansion happens server-side) or
 * one running with `scope_validation = false`, where hiding the whole console
 * would be wrong in both directions.
 */
export type PermissionMode = 'enforce' | 'permissive';

/** Operation id (OpenAPI `operationId`). Writable as string to allow runtime
 * lookups for unknown or out-of-date generated maps. */
export type PermissionOperation = OperationId | (string & {});

/** Why a decision came out the way it did. */
export type DecisionReason =
  'granted' | 'missing-scope' | 'unknown-scopes' | 'unknown-operation' | 'loading';

export type Decision = {
  allowed: boolean;
  reason: DecisionReason;
  /**
   * The any-of scope list this decision was made against, for tooltips, tests
   * and the drift warning. Empty when the operation requires no scope, or when
   * the operation is unknown.
   */
  required: readonly string[];
};

/**
 * Everything a decision depends on besides the operation itself.
 *
 * Passed as one object so the provider assembles it once and memoizes it, and
 * so adding a future input (a per-organization scope set, say) does not change
 * the signature of every call site.
 */
export type PermissionInput = {
  /** `loading` until `GET /api/session` has resolved. */
  status: 'loading' | 'ready';
  /**
   * The scopes the token carries. `undefined` means the claim was absent;
   * which is emphatically not the same as an empty set, and is the distinction
   * that keeps a role-mode deployment from rendering an empty console.
   */
  granted?: ReadonlySet<string>;
  mode: PermissionMode;
};

/* -------------------------------------------------------------------------- */
/* Internals                                                                   */
/* -------------------------------------------------------------------------- */

/** Shared so a decision never allocates an array just to say "nothing". */
const NO_SCOPES: readonly string[] = Object.freeze([]);

/**
 * The generated map, widened for lookup by an arbitrary string.
 *
 * `OPERATION_SCOPES` is `as const satisfies Record<keyof operations, …>`, so
 * indexing it with a plain `string` is not assignable without this. The cast
 * widens the *index*, not the value — the returned type still admits
 * `undefined`, which is what forces the unknown-operation branch to be handled.
 *
 * It arrives via `api/core/spec`, the one module allowed to read the generated
 * output, rather than from `api/generated` directly.
 */
const scopeTable = OPERATION_SCOPES as Record<string, readonly ApScope[] | undefined>;

/**
 * Operations already warned about, so a component re-rendering on every
 * keystroke logs once rather than flooding the console and burying the warning
 * it was meant to surface.
 */
const warned = new Set<string>();

const warnOnce = (key: string, message: string) => {
  if (!import.meta.env.DEV || warned.has(key)) return;
  warned.add(key);
  console.warn(`[permissions] ${message}`);
};

/** Clears the warn-once memo. Tests only — never call this from app code. */
export const resetPermissionWarnings = (): void => {
  warned.clear();
};

/**
 * Reports that the server refused an operation the console had predicted would
 * be allowed.
 *
 * This is the signal that catches the failure this whole layer is exposed to: a
 * generated scope map that no longer matches the deployed backend. The user has
 * already seen a clear "you don't have permission" message, so nothing is broken
 * for them; but a control that offers an action the server rejects is a bug,
 * and without this it leaves no trace anywhere.
 *
 * Dev-only and once per operation, so it reads as a finding rather than noise.
 * A denial the console *predicted* is not reported: that is the system working.
 */
export const reportForbiddenDrift = (operation: string, predicted: Decision): void => {
  if (!predicted.allowed) return;
  const accepts =
    predicted.required.length > 0 ? `, accepts: ${predicted.required.join(' | ')}` : '';
  warnOnce(
    `drift:${operation}`,
    `the server returned 403 for "${operation}", which this console predicted was ` +
      `allowed (reason: ${predicted.reason}${accepts}). The generated scope map may ` +
      'not match the deployed Platform API — re-run `npm run api:codegen` against ' +
      'the matching spec version.',
  );
};

const decision = (
  allowed: boolean,
  reason: DecisionReason,
  required: readonly string[],
): Decision => ({ allowed, reason, required });

/* -------------------------------------------------------------------------- */
/* Public API                                                                  */
/* -------------------------------------------------------------------------- */

/**
 * Normalize session user scopes into a Set for permission evaluation.
 *
 * Null/undefined input returns undefined (no scope claim). An array returns a
 * Set; empty-string entries are removed.
 */
export const toGrantedSet = (
  scopes: readonly string[] | null | undefined,
): ReadonlySet<string> | undefined => {
  if (!scopes) return undefined;
  return new Set(scopes.filter((scope) => scope.trim() !== ''));
};

/** Short helper: returns the any-of scopes an operation accepts, or
 * `undefined` if the operation is not in the generated map. Used by tests
 * and the drift warning; callers should use decideOperation.
 */
export const requiredScopesFor = (operation: PermissionOperation): readonly string[] | undefined =>
  scopeTable[operation];

/** Decide whether the user may be offered `operation`.
 * Policy summary:
 * - While session is loading, allow (avoid flicker).
 * - Unknown operations are allowed (surface backend errors on use).
 * - If an operation requires no scopes, allow.
 * - If scopes are unknown, consult `mode`.
 * - Otherwise require exact any-of scope match (no wildcards or inheritance).
 */
export const decideOperation = (
  operation: PermissionOperation,
  input: PermissionInput,
): Decision => {
  const required = requiredScopesFor(operation);

  if (input.status === 'loading') {
    return decision(true, 'loading', required ?? NO_SCOPES);
  }

  if (required === undefined) {
    warnOnce(
      operation,
      `unknown operation "${operation}" — not in the generated scope map, so it ` +
        'cannot be gated. Check the spelling against the OpenAPI operationId, or ' +
        're-run `npm run api:codegen`. Offering it; the server will decide.',
    );
    return decision(true, 'unknown-operation', NO_SCOPES);
  }

  if (required.length === 0) {
    return decision(true, 'granted', NO_SCOPES);
  }

  if (input.granted === undefined) {
    return decision(input.mode === 'permissive', 'unknown-scopes', required);
  }

  const granted = input.granted;
  const allowed = required.some((scope) => granted.has(scope));
  return decision(allowed, allowed ? 'granted' : 'missing-scope', required);
};

/** Convenience wrapper for the common boolean question. */
export const canOperation = (operation: PermissionOperation, input: PermissionInput): boolean =>
  decideOperation(operation, input).allowed;

/**
 * Decide if any of `operations` is permitted (used e.g. to show an overflow
 * menu). If any allow, return that decision. If all deny, return the first
 * reason and the union of required scopes.
 */
export const decideAnyOperation = (
  operations: readonly PermissionOperation[],
  input: PermissionInput,
): Decision => {
  if (operations.length === 0) return decision(true, 'granted', NO_SCOPES);

  const decisions = operations.map((operation) => decideOperation(operation, input));
  const permitted = decisions.find((candidate) => candidate.allowed);
  if (permitted) return permitted;

  const union = [...new Set(decisions.flatMap((entry) => entry.required))];
  return decision(false, decisions[0].reason, union);
};

/** Check a single scope (used for special cases like ownership overrides).
 * Prefer decideOperation for normal permission checks. */
export const decideScope = (scope: ApScope | (string & {}), input: PermissionInput): Decision => {
  const required: readonly string[] = [scope];

  if (input.status === 'loading') return decision(true, 'loading', required);
  if (input.granted === undefined) {
    return decision(input.mode === 'permissive', 'unknown-scopes', required);
  }

  const allowed = input.granted.has(scope);
  return decision(allowed, allowed ? 'granted' : 'missing-scope', required);
};
