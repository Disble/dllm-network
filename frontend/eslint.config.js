import js from '@eslint/js';
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript';
import checkFilePlugin from 'eslint-plugin-check-file';
import { createNodeResolver, importX } from 'eslint-plugin-import-x';
import jsdocPlugin from 'eslint-plugin-jsdoc';
import reactPlugin from 'eslint-plugin-react';
import reactDoctor from 'eslint-plugin-react-doctor';
import sonarjsPlugin from 'eslint-plugin-sonarjs';
import * as tsParser from '@typescript-eslint/parser';

import {
  appDeliverySyntaxRules,
  commonUiColocationSyntaxRules,
  contextProviderSyntaxRules,
  downgradeRuleSeverities,
  featureHookAnatomySyntaxRules,
  importXExtensions,
  publicConstantDocumentationContexts,
  publicHookDocumentationContexts,
  publicTypeContractDocumentationContexts,
  readonlyUiPropsBoundarySyntaxRules,
  tsxLayeringSyntaxRules,
  uiExportDocumentationContexts,
} from './eslint/architecture-rules.js';

const reactDoctorRecommendedWarnRules = downgradeRuleSeverities(reactDoctor.configs.recommended.rules);
const reactDoctorWebWarnRules = downgradeRuleSeverities(reactDoctor.configs.recommended.rules);

export default [
  js.configs.recommended,
  importX.flatConfigs.recommended,
  importX.flatConfigs.typescript,
  reactPlugin.configs.flat['jsx-runtime'],
  {
    ignores: ['dist/*', 'scripts/*', 'coverage/*', 'wailsjs/**/*'],
  },
  {
    files: ['**/*.{ts,tsx,js,jsx,mjs,cjs}'],
    languageOptions: {
      parser: tsParser,
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: {
        document: 'readonly',
        window: 'readonly',
      },
    },
    plugins: {
      sonarjs: sonarjsPlugin,
    },
    settings: {
      react: {
        version: 'detect',
      },
      'import-x/resolver-next': [
        createTypeScriptImportResolver({
          alwaysTryTypes: true,
          bun: true,
          project: './tsconfig.json',
        }),
        createNodeResolver({
          extensions: importXExtensions,
        }),
      ],
    },
    rules: {
      'import/default': 'off',
      'import/export': 'off',
      'import/named': 'off',
      'import/namespace': 'off',
      'import/no-duplicates': 'off',
      'import/no-named-as-default': 'off',
      'import/no-named-as-default-member': 'off',
      'import/no-unresolved': 'off',
      'no-redeclare': 'off',
      'import-x/no-cycle': ['error', { maxDepth: 1 }],
      'import-x/no-duplicates': 'error',
      'import-x/no-unresolved': 'error',
      'sonarjs/cognitive-complexity': ['warn', 15],
      'sonarjs/no-all-duplicated-branches': 'warn',
      'sonarjs/no-identical-functions': 'warn',
      'sonarjs/no-redundant-boolean': 'warn',
      'sonarjs/no-small-switch': 'warn',
    },
  },
  {
    files: ['**/*.ts', '**/*.tsx'],
    rules: {
      'max-lines': ['error', { max: 500, skipBlankLines: true, skipComments: true }],
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.ts', 'src/**/*.test.tsx', 'src/**/__tests__/**/*.{ts,tsx}'],
    plugins: {
      'react-doctor': reactDoctor,
    },
    rules: {
      ...reactDoctorRecommendedWarnRules,
      ...reactDoctorWebWarnRules,
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: {
      'check-file': checkFilePlugin,
    },
    rules: {
      'check-file/filename-blocklist': [
        'error',
        {
          'src/**/utils.ts': '*.helpers.ts',
          'src/**/Utils.ts': '*.helpers.ts',
        },
      ],
      'check-file/folder-match-with-fex': [
        'error',
        {
          'src/**/*.test.ts': '**/__tests__/',
          'src/**/*.test.tsx': '**/__tests__/',
        },
      ],
      'check-file/folder-naming-convention': [
        'error',
        {
          'src/features/*/': 'KEBAB_CASE',
        },
      ],
    },
  },
  {
    files: ['**/*.tsx'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['src/infrastructure/*', '**/infrastructure/*'],
              message:
                'Feature Boundary: UI components (.tsx) cannot directly import infrastructure layers. Use a feature hook or repository boundary instead.',
            },
          ],
        },
      ],
      'no-restricted-syntax': ['error', ...tsxLayeringSyntaxRules],
    },
  },
  {
    files: ['src/app/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['src/infrastructure/**', '@/src/infrastructure/**', '../infrastructure/**', '../../infrastructure/**', '**/infrastructure/**'],
              message:
                'Delivery Rule: app/ files cannot import infrastructure or runtime integration layers directly. Move runtime access behind a feature entrypoint.',
            },
            {
              group: ['**/features/**/use-*', '**/shared/**/use-*'],
              message: 'Delivery Rule: app/ files cannot import custom hooks directly. Move composition into a feature entrypoint.',
            },
          ],
        },
      ],
      'no-restricted-syntax': ['error', ...appDeliverySyntaxRules],
    },
  },
  {
    files: ['src/infrastructure/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['src/features/**', '@/src/features/**', '../features/**', '../../features/**', '../../../features/**', '**/features/**'],
              message: 'Infrastructure Boundary: infrastructure modules cannot import feature-layer code or contracts. Move shared contracts to src/shared/**.',
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/**/*.tsx', 'src/shared/**/*.tsx'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: 'zod',
              message: 'Strict Colocation: Zod schemas must live in a dedicated *.schema.ts file, never inside a component or hook.',
            },
          ],
        },
      ],
      'no-restricted-syntax': ['error', ...commonUiColocationSyntaxRules, ...readonlyUiPropsBoundarySyntaxRules],
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: uiExportDocumentationContexts,
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/features/**/use-*.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: 'zod',
              message: 'Strict Colocation: Zod schemas must live in a dedicated *.schema.ts file, never inside a component or hook.',
            },
          ],
        },
      ],
      'no-restricted-syntax': [
        'error',
        ...featureHookAnatomySyntaxRules,
        {
          selector: 'Program > VariableDeclaration',
          message:
            'Strict Colocation: Root-level variables are forbidden in feature components or hooks. Move constants to *.constants.ts and helper state to the function body or dedicated modules.',
        },
        {
          selector: 'Program > FunctionDeclaration',
          message:
            'Strict Colocation: Root-level helper functions are forbidden in feature components or hooks. Move them to *.helpers.ts or export the main component or hook function directly.',
        },
        {
          selector: 'Program > ExportNamedDeclaration > VariableDeclaration',
          message: 'Strict Colocation: Export feature components and hooks as function declarations, not root-level consts.',
        },
        {
          selector: 'Program > ExportDefaultDeclaration > ArrowFunctionExpression',
          message: 'Strict Colocation: Export feature components and hooks as named function declarations.',
        },
        {
          selector: 'TSInterfaceDeclaration',
          message: 'Strict Colocation: Interfaces must be declared in a separate .types.ts file, not inside the component or hook.',
        },
        {
          selector: 'TSTypeAliasDeclaration',
          message: 'Strict Colocation: Type aliases must be declared in a separate .types.ts file, not inside the component or hook.',
        },
      ],
    },
  },
  {
    files: ['src/features/**/use-*.ts', 'src/hooks/use-*.ts', 'src/shared/hooks/use-*.ts'],
    ignores: ['src/**/__tests__/**/*', 'src/**/*.test.ts', 'src/**/*.test.tsx'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: publicHookDocumentationContexts,
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/features/**/*.types.ts', 'src/shared/**/*.types.ts', 'src/contexts/**/*.types.ts'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'TSInterfaceDeclaration[id.name=/Props$/] TSPropertySignature[readonly!=true]',
          message: 'Type Contract Rule: every Props field must be declared as readonly.',
        },
      ],
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: publicTypeContractDocumentationContexts,
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/**/*.constants.ts'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: publicConstantDocumentationContexts,
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/**/*.schema.ts'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: [...publicConstantDocumentationContexts, ...publicTypeContractDocumentationContexts],
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/contexts/**/*.ts'],
    ignores: ['src/contexts/**/*.types.ts', 'src/contexts/**/*.constants.ts'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'no-restricted-syntax': ['error', ...contextProviderSyntaxRules, ...readonlyUiPropsBoundarySyntaxRules],
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: uiExportDocumentationContexts,
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
  {
    files: ['src/features/**/*.helpers.ts', 'src/shared/helpers/**/*.ts'],
    plugins: {
      jsdoc: jsdocPlugin,
    },
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'TSInterfaceDeclaration',
          message: 'Helper Contract Rule: interfaces must be declared in a separate .types.ts file, not inside helpers.',
        },
        {
          selector: 'TSTypeAliasDeclaration',
          message: 'Helper Contract Rule: type aliases must be declared in a separate .types.ts file, not inside helpers.',
        },
      ],
      'jsdoc/require-jsdoc': [
        'error',
        {
          contexts: [
            'ExportNamedDeclaration > VariableDeclaration > VariableDeclarator > ArrowFunctionExpression',
            'ExportNamedDeclaration > FunctionDeclaration',
          ],
          require: {
            FunctionDeclaration: false,
            ArrowFunctionExpression: false,
            FunctionExpression: false,
          },
        },
      ],
    },
  },
];
