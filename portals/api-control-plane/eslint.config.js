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
    // Components/pages must access data through React Query hooks (which resolve
    // the client via useApiClient), never the API client modules directly. See
    // src/api/README.md. Type-only imports are allowed.
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': 'off',
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              // Resource clients live one dir deep (api/<resource>/<x>Client);
              // this avoids matching api/client.ts (the token seam the auth
              // adapters legitimately import).
              group: ['**/api/*/*Client', '**/api/mvpApi'],
              allowTypeImports: true,
              message:
                'Do not import API clients directly. Use a hook from src/api/hooks (it resolves the client via useApiClient). See src/api/README.md.',
            },
          ],
        },
      ],
    },
  },
  {
    // The hook layer + DI provider legitimately import the client modules.
    // `useMockApi`/`usePlatformApi` here are plain boolean mode-check
    // functions, not React hooks — they're just named with a `use` prefix,
    // which react-hooks/rules-of-hooks otherwise flags as an out-of-component
    // hook call.
    files: ['src/api/**'],
    rules: {
      '@typescript-eslint/no-restricted-imports': 'off',
      'react-hooks/rules-of-hooks': 'off',
    },
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
