import js from '@eslint/js';
import globals from 'globals';

export default [
    {
        // Not application source — generated output, vendored libs (which carry
        // their own eslint config), samples, and non-JS asset directories.
        ignores: [
            'node_modules/**',
            'target/**',
            'coverage/**',
            'libs/**',
            'samples/**',
            'distribution/**',
            'production/**',
            'resources/**',
            'docs/**',
            'database/**',
            'bin/**',
        ],
    },
    js.configs.recommended,
    {
        // Server-side application code and build tooling (Node.js, CommonJS).
        files: ['src/**/*.js', 'tools/**/*.js'],
        languageOptions: {
            ecmaVersion: 2023,
            sourceType: 'commonjs',
            globals: { ...globals.node },
        },
        rules: {
            'no-unused-vars': ['warn', { args: 'none', caughtErrors: 'none', varsIgnorePattern: '^_' }],
            'no-empty': ['error', { allowEmptyCatch: true }],
        },
    },
    {
        // Integration tests: Cypress + Jest-style globals on top of Node.
        files: ['it/**/*.js'],
        languageOptions: {
            ecmaVersion: 2023,
            sourceType: 'commonjs',
            globals: {
                ...globals.node,
                ...globals.jest,
                ...globals.mocha,
                cy: 'readonly',
                Cypress: 'readonly',
            },
        },
        rules: {
            'no-unused-vars': ['warn', { args: 'none', caughtErrors: 'none', varsIgnorePattern: '^_' }],
            'no-empty': ['error', { allowEmptyCatch: true }],
        },
    },
    {
        // Cypress support files are authored as ES modules.
        files: ['it/ui/**/support/**/*.js'],
        languageOptions: { sourceType: 'module' },
    },
    {
        // Browser-side scripts served to the client. Their top-level functions
        // are invoked from Handlebars templates via inline handlers (onclick,
        // etc.), so they legitimately look "unused"; they also reference page
        // globals defined by sibling scripts and CDN libs (bootstrap, marked),
        // which the linter cannot resolve statically.
        files: ['src/scripts/**/*.js'],
        languageOptions: {
            ecmaVersion: 2023,
            sourceType: 'script',
            globals: { ...globals.browser },
        },
        rules: {
            'no-unused-vars': 'off',
            'no-undef': 'off',
            'no-empty': ['error', { allowEmptyCatch: true }],
        },
    },
];
