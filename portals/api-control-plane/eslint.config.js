import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  reactHooks.configs['recommended-latest'],
  {
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
  {
    files: ['src/**/*.{ts,tsx}', 'src/**/*.{js,jsx}'],
  },
  {
    // ── The API layer's public surface ──────────────────────────────────────
    //
    // Everything outside src/api reaches the backend through *hooks*, and
    // through nothing else. The layers below a hook (queries → endpoints →
    // transport) are implementation detail: a component that reaches past the
    // hook skips scope binding, `enabled` gating, cache-key construction and
    // invalidation, all of which live in the hook.
    //
    // Type-only imports are allowed where the types are the app's currency
    // (spec types, request/response shapes); the ban is on calling into a
    // layer, not on naming its types.
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': 'off',
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              // Legacy layer, removed as each resource is migrated. Resource
              // clients live one dir deep (api/<resource>/<x>Client); this
              // avoids matching api/client.ts, the token seam the auth
              // adapters legitimately import.
              // `!(core|resources)` matters: without it this pattern also
              // matches `api/core/queryClient`, which is not a legacy client
              // and which the composition root legitimately imports.
              group: ['**/api/!(core|resources)/*Client', '**/api/mvpApi'],
              allowTypeImports: true,
              message:
                'Do not import API clients directly. Use a hook from src/api/hooks (it resolves the client via useApiClient). See src/api/README.md.',
            },
            {
              group: ['**/api/core/http'],
              message:
                'The HTTP transport is internal to the API layer. Components use a hook from src/api/resources/<resource>; a new backend call needs an endpoint + query + hook, not a direct request.',
            },
            {
              group: ['**/api/core/queryKeys'],
              message:
                'Query keys are constructed inside the API layer so scope prefixing cannot be bypassed. If you need to invalidate or reset cache from the UI, expose a hook from src/api/resources/<resource> instead.',
            },
            {
              group: ['**/api/resources/**/*.endpoints'],
              allowTypeImports: true,
              message:
                'Endpoints are the raw transport layer — they carry no scope, no cache and no error surfacing. Use the matching hook. (Importing its types is fine.)',
            },
            {
              group: ['**/api/resources/**/*.queries'],
              allowTypeImports: true,
              message:
                'queryOptions are consumed by hooks, which own scope resolution and `enabled` gating. Use the hook. A route loader that genuinely needs queryOptions should get them from a hook-adjacent export added for that purpose.',
            },
            {
              group: ['**/api/generated/*'],
              message:
                'Generated spec types are reached through src/api/core/spec and re-exported by each resource module, so a generator change lands in one place. Import the type from the resource instead.',
            },
          ],
        },
      ],
    },
  },
  {
    // The legacy layer's modules import each other freely; that freedom ends at
    // src/api/core and src/api/resources, which are governed below.
    //
    // `useMockApi`/`usePlatformApi` are plain boolean mode checks, not React
    // hooks — they merely start with `use`, which rules-of-hooks would
    // otherwise flag as an out-of-component hook call.
    files: ['src/api/**'],
    rules: {
      '@typescript-eslint/no-restricted-imports': 'off',
      'react-hooks/rules-of-hooks': 'off',
    },
  },
  {
    // ── Layer discipline inside the API layer ───────────────────────────────
    //
    // A layer may only import from the layer directly below it. These paths are
    // relative (../../core/x), so they carry no `api/` segment; which is what
    // keeps these rules separate from the UI-facing ones above.
    files: ['src/api/core/**/*.ts', 'src/api/resources/**/*.ts'],
    rules: {
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              // One module owns the generator's output, so swapping generators
              // or patching a spec gap is a one-file change.
              group: ['**/generated/*'],
              message:
                'Only src/api/core/spec.ts may import the generated types. Use ResponseOf/BodyOf/QueryOf/PathOf/Schema from core/spec instead.',
            },
          ],
        },
      ],
    },
  },
  {
    // core/spec.ts is the one module allowed to read the generated output.
    files: ['src/api/core/spec.ts'],
    rules: { '@typescript-eslint/no-restricted-imports': 'off' },
  },
  {
    // Endpoints are pure transport: one function per spec operation, no cache
    // awareness and no React. Keeping them free of both is what lets them be
    // called from a loader, a test or a non-React caller.
    files: ['src/api/resources/**/*.endpoints.ts'],
    rules: {
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: 'react',
              message:
                'Endpoints are transport-only. Anything needing React belongs in the hooks layer.',
            },
            {
              name: '@tanstack/react-query',
              message:
                'Endpoints know nothing about the cache. Put queryOptions in <resource>.queries.ts.',
            },
          ],
          patterns: [
            {
              group: ['**/core/queryKeys', '**/core/queryClient', '**/core/scope'],
              message:
                'Endpoints sit below the cache. Keys, retry policy and scope resolution belong in the queries and hooks layers.',
            },
            {
              group: ['**/generated/*'],
              message:
                'Only src/api/core/spec.ts may import the generated types. Use the helpers from core/spec.',
            },
            {
              // The instance factories exist for composition and tests; a
              // resource module should only ever use the verb facade.
              group: ['**/core/http'],
              importNames: ['getHttpClient', 'createHttpClient', 'resetHttpClient', '_http'],
              message:
                'Use the `http` facade (http.get/post/put/delete). The instance factories are for the composition root and tests.',
            },
          ],
        },
      ],
    },
  },
  {
    // Query definitions bind an endpoint to a key and a freshness tier. They
    // are plain values on purpose — usable from a loader or a prefetch — so
    // they must not depend on React either.
    files: ['src/api/resources/**/*.queries.ts'],
    rules: {
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: 'react',
              message:
                'queryOptions must stay plain values so loaders and prefetch can use them. React belongs in <resource>.hooks.ts.',
            },
          ],
          patterns: [
            {
              group: ['**/core/scope'],
              message:
                'Scope is resolved by the hook and passed in as an argument, which keeps queryOptions usable outside a component.',
            },
            {
              group: ['**/generated/*'],
              message:
                'Only src/api/core/spec.ts may import the generated types. Use the helpers from core/spec.',
            },
          ],
        },
      ],
    },
  },
  {
    // Tests legitimately reach into the layer they cover — including the
    // instance factories and reset seams.
    files: ['src/**/*.test.{ts,tsx}', 'src/test/**'],
    rules: { '@typescript-eslint/no-restricted-imports': 'off' },
  },
  {
    ignores: [
      '**/build/**',
      '**/coverage/**',
      '**/node_modules/**',
      'src/api/generated/**',
    ],
  },
];
