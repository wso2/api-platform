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
 * Generates `src/api/generated/operationScopes.ts` from the Platform API's
 * OpenAPI spec: the machine-readable half of the console's permission layer.
 *
 * Run via `npm run api:codegen`, which also regenerates `platform.d.ts` from
 * the same file. The two must always be generated together: the emitted map is
 * constrained by `satisfies Record<keyof operations, ...>`, so a spec operation
 * present in one file and absent from the other is a typecheck failure rather
 * than a silent permission hole.
 *
 * This script is deliberately strict. It refuses to emit anything when the spec
 * violates an assumption the *evaluator* depends on, chiefly that each
 * operation's `security` block is a single requirement object, which is what
 * makes "hold at least one of these scopes" a correct reading. A spec that
 * grows AND-of semantics must be met with a new evaluator, not a map that
 * quietly loses the distinction.
 *
 * Node 24 runs this file directly (`node scripts/generateOperationScopes.ts`);
 * the type annotations are stripped, never checked, so nothing here may rely on
 * TypeScript-only runtime constructs (no enums, no parameter properties).
 */

import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import yaml from 'js-yaml';
import * as prettier from 'prettier';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SPEC_PATH = path.resolve(HERE, '../../../platform-api/resources/openapi.yaml');
const OUT_PATH = path.resolve(HERE, '../src/api/generated/operationScopes.ts');

/** The only security scheme this generator understands. */
const SCHEME = 'OAuth2Security';

/** OpenAPI 3.1 fixed fields on a Path Item that denote an operation. */
const HTTP_METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];

/**
 * Operations allowed to list a narrow action scope without its sibling
 * `:manage` — see `assertManageConvention`. Empty, and expected to stay empty:
 * the three operations that look like exceptions
 * (`GetGatewayManifest`, `ListApplicationAssociationAPIKeys`, `listUserAPIKeys`)
 * are not, because no `:manage` scope exists for those resource paths at all.
 */
const MANAGE_CONVENTION_EXCEPTIONS = new Set<string>([]);

type SecurityRequirement = Record<string, string[]>;

type SpecOperation = {
  operationId?: string;
  security?: SecurityRequirement[];
};

type Spec = {
  security?: SecurityRequirement[];
  paths?: Record<string, Record<string, unknown>>;
  components?: {
    securitySchemes?: Record<
      string,
      { flows?: Record<string, { scopes?: Record<string, string> }> }
    >;
  };
};

const problems: string[] = [];
const record = (message: string) => problems.push(message);

const abort = (): never => {
  console.error(`\noperationScopes: refusing to generate — ${problems.length} problem(s):\n`);
  for (const problem of problems) console.error(`  • ${problem}`);
  console.error('\nFix the spec (or this generator, if the contract genuinely changed).\n');
  process.exit(1);
};

/* -------------------------------------------------------------------------- */
/* Read                                                                        */
/* -------------------------------------------------------------------------- */

const spec = yaml.load(readFileSync(SPEC_PATH, 'utf8')) as Spec;

/**
 * The declared scope catalog, unioned across every flow. Unioning rather than
 * reading `clientCredentials` alone means adding an authorization-code flow to
 * the spec does not silently drop scopes only that flow declares.
 */
const catalog = new Map<string, string>();
const scheme = spec.components?.securitySchemes?.[SCHEME];
if (!scheme) {
  record(`security scheme "${SCHEME}" is missing from components.securitySchemes`);
} else {
  for (const flow of Object.values(scheme.flows ?? {})) {
    for (const [scope, description] of Object.entries(flow.scopes ?? {})) {
      catalog.set(scope, description);
    }
  }
  if (catalog.size === 0) record(`security scheme "${SCHEME}" declares no scopes`);
}

/**
 * The root requirement every operation without its own `security` inherits.
 * The walk below emits such an operation as "no scope required", which is only
 * true while the root is exactly `[{ OAuth2Security: [] }]`: authenticated, no
 * particular scope. A root that demands scopes, names another scheme, offers
 * alternatives, or is absent would make that reading wrong, and every
 * inheriting operation would be offered to every user. Refuse instead.
 */
const rootSecurity = spec.security;
const rootRequirement = rootSecurity?.length === 1 ? rootSecurity[0] : undefined;
const rootSchemes = rootRequirement ? Object.keys(rootRequirement) : [];
const rootIsAuthenticatedOnly =
  rootSchemes.length === 1 &&
  rootSchemes[0] === SCHEME &&
  Array.isArray(rootRequirement?.[SCHEME]) &&
  rootRequirement[SCHEME].length === 0;
if (!rootIsAuthenticatedOnly) {
  record(
    `root security must be exactly [{ ${SCHEME}: [] }], got ${JSON.stringify(rootSecurity ?? null)} — ` +
      'operations without their own security inherit it',
  );
}

/* -------------------------------------------------------------------------- */
/* Walk operations                                                             */
/* -------------------------------------------------------------------------- */

/** operationId -> accepted scopes, any-of. */
const operationScopes = new Map<string, string[]>();
/** Operations reachable with any authenticated token, reported for review. */
const unrestricted: string[] = [];

for (const [pathName, pathItem] of Object.entries(spec.paths ?? {})) {
  if ('$ref' in pathItem) {
    record(`${pathName}: $ref'd path items are not supported — resolve the spec first`);
    continue;
  }

  for (const [field, value] of Object.entries(pathItem)) {
    if (!HTTP_METHODS.includes(field)) continue; // parameters, summary, servers…

    const where = `${field.toUpperCase()} ${pathName}`;
    const operation = value as SpecOperation;
    const operationId = operation.operationId;

    if (!operationId) {
      record(`${where}: no operationId — the permission key cannot be derived`);
      continue;
    }
    if (operationScopes.has(operationId) || unrestricted.includes(operationId)) {
      record(`${where}: duplicate operationId "${operationId}"`);
      continue;
    }

    // `security` absent means "inherit the root requirement", validated above
    // to be `OAuth2Security: []` — authenticated, no particular scope. An
    // explicit `[]` means no scope is required either. Both are emitted as an
    // empty list, which the evaluator reads as "no scope required"; they are
    // reported because an operation becoming unrestricted by accident is
    // exactly the kind of change worth seeing in a diff.
    const security = operation.security;
    if (security === undefined && !rootIsAuthenticatedOnly) {
      record(
        `${where} (${operationId}): inherits a root security requirement that failed validation`,
      );
      continue;
    }
    if (security === undefined || security.length === 0) {
      unrestricted.push(operationId);
      operationScopes.set(operationId, []);
      continue;
    }

    // More than one requirement object is OR *between* objects, each of which
    // may itself demand several schemes at once. The evaluator models neither.
    if (security.length > 1) {
      record(
        `${where} (${operationId}): ${security.length} security requirement objects — ` +
          'the any-of evaluator assumes exactly one',
      );
      continue;
    }

    const requirement = security[0];
    const schemeNames = Object.keys(requirement);
    if (schemeNames.length !== 1 || schemeNames[0] !== SCHEME) {
      record(
        `${where} (${operationId}): expected the sole security scheme "${SCHEME}", got ` +
          `[${schemeNames.join(', ')}]`,
      );
      continue;
    }

    const scopes = requirement[SCHEME] ?? [];
    if (scopes.length === 0) {
      unrestricted.push(operationId);
      operationScopes.set(operationId, []);
      continue;
    }

    const undeclared = scopes.filter((scope) => !catalog.has(scope));
    if (undeclared.length > 0) {
      record(
        `${where} (${operationId}): scope(s) not in the ${SCHEME} catalog: ` +
          undeclared.join(', '),
      );
      continue;
    }

    // Sorted so the emitted file is a function of the spec's content, not of
    // the order scopes happen to appear in it.
    operationScopes.set(operationId, [...scopes].sort());
  }
}

/* -------------------------------------------------------------------------- */
/* Assert the convention the client relies on                                  */
/* -------------------------------------------------------------------------- */

/**
 * The console does no scope implication of its own: it never infers that
 * `ap:project:manage` covers `ap:project:read`. That is only safe because the
 * spec lists the `:manage` scope alongside every narrow one, so this checks it
 * rather than trusting it.
 *
 * The check is conditional on the sibling `:manage` scope *existing* in the
 * catalog. Some resource paths have no `:manage` scope at all
 * (`ap:gateway:manifest:read` has no `ap:gateway:manifest:manage`), and those
 * are not violations — there is nothing broader to have omitted.
 */
const assertManageConvention = () => {
  for (const [operationId, scopes] of operationScopes) {
    if (MANAGE_CONVENTION_EXCEPTIONS.has(operationId)) continue;

    for (const scope of scopes) {
      if (scope.endsWith(':manage')) continue;
      const sibling = scope.replace(/:[^:]+$/, ':manage');
      if (catalog.has(sibling) && !scopes.includes(sibling)) {
        record(
          `${operationId}: accepts "${scope}" but not "${sibling}", which exists in the ` +
            'catalog — a caller holding only the broader scope would be denied by the UI',
        );
      }
    }
  }
};
assertManageConvention();

if (problems.length > 0) abort();

/* -------------------------------------------------------------------------- */
/* Emit                                                                        */
/* -------------------------------------------------------------------------- */

const scopeUnion = [...catalog.keys()].sort();
const entries = [...operationScopes.entries()].sort(([a], [b]) => a.localeCompare(b));

const source = `/*
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
 * GENERATED FILE — do not edit.
 *
 * Produced by \`scripts/generateOperationScopes.ts\` (\`npm run api:codegen\`)
 * from platform-api/resources/openapi.yaml. Edit the spec, not this file.
 */

import type { operations } from './platform';

/** Every scope declared by the spec's OAuth2Security scheme. */
export type ApScope =
${scopeUnion.map((scope) => `  | '${scope}'`).join('\n')};

/**
 * The full scope catalog, for building test personas and for validating an
 * operator-supplied scope string. Sorted, so it is diff-stable.
 */
export const AP_SCOPES: readonly ApScope[] = [
${scopeUnion.map((scope) => `  '${scope}',`).join('\n')}
];

/**
 * Scopes accepted for each operation, **any-of**: a caller holding at least one
 * of them may invoke it. Mirrors each operation's \`security\` block verbatim,
 * including the broader \`:manage\` scope the spec lists beside each narrow one —
 * which is why no scope implication happens on the client.
 *
 * An empty list means the operation requires no particular scope.
 *
 * \`satisfies Record<keyof operations, ...>\` is the drift guard: an operation
 * added to the spec but missing here fails \`tsc\`.
 */
export const OPERATION_SCOPES = {
${entries
  .map(([operationId, scopes]) =>
    scopes.length === 0
      ? `  ${operationId}: [],`
      : `  ${operationId}: [${scopes.map((scope) => `'${scope}'`).join(', ')}],`,
  )
  .join('\n')}
} as const satisfies Record<keyof operations, readonly ApScope[]>;
`;

const prettierConfig = await prettier.resolveConfig(OUT_PATH);
const formatted = await prettier.format(source, {
  ...prettierConfig,
  filepath: OUT_PATH,
});

writeFileSync(OUT_PATH, formatted, 'utf8');

console.log(
  `operationScopes: ${entries.length} operations, ${scopeUnion.length} scopes → ` +
    path.relative(process.cwd(), OUT_PATH),
);
if (unrestricted.length > 0) {
  console.log(
    `operationScopes: ${unrestricted.length} operation(s) require no scope: ` +
      unrestricted.join(', '),
  );
}
