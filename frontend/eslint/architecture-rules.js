const appLayerReactHooksPattern =
  '^use(State|Reducer|Effect|LayoutEffect|InsertionEffect|SyncExternalStore|Memo|Callback|Ref|Context|Transition|DeferredValue|ImperativeHandle|DebugValue|Id|Optimistic|ActionState)?$';

const tsxLayeringSyntaxRules = [
  {
    selector: 'ImportDeclaration[source.value=/infrastructure/]',
    message:
      'Feature Boundary: UI components (.tsx) cannot directly import infrastructure layers. Use a feature hook or repository boundary instead.',
  },
  {
    selector: 'ImportDeclaration[source.value=/^@tauri-apps(?:\\/.*)?$/]',
    message:
      'Feature Boundary: UI components (.tsx) cannot directly import runtime integration adapters. Use a feature hook or app boundary instead.',
  },
];

const schemaPlacementSyntaxRules = [
  {
    selector: 'ImportDeclaration[source.value=/^zod(?:\\/.*)?$/]',
    message:
      'Strict Colocation: Zod schemas must live in a dedicated *.schema.ts file, never inside a component or hook.',
  },
];

const commonUiColocationSyntaxRules = [
  ...tsxLayeringSyntaxRules,
  ...schemaPlacementSyntaxRules,
  {
    selector: 'Program > VariableDeclaration',
    message:
      'Strict Colocation: Root-level variables are forbidden in feature/shared components and hooks. Move constants to *.constants.ts and helper state to the function body or dedicated modules.',
  },
  {
    selector: 'Program > FunctionDeclaration',
    message:
      'Strict Colocation: Root-level helper functions are forbidden in feature/shared components and hooks. Move them to *.helpers.ts or export the main component or hook function directly.',
  },
  {
    selector: 'Program > ExportNamedDeclaration > VariableDeclaration',
    message:
      'Strict Colocation: Export feature/shared components and hooks as function declarations, not root-level consts.',
  },
  {
    selector: 'Program > ExportDefaultDeclaration > ArrowFunctionExpression',
    message: 'Strict Colocation: Export feature/shared components and hooks as named function declarations.',
  },
  {
    selector: 'TSInterfaceDeclaration',
    message: 'Strict Colocation: Interfaces must be declared in a separate .types.ts file, not inside the component or hook.',
  },
  {
    selector: 'TSTypeAliasDeclaration',
    message: 'Strict Colocation: Type aliases must be declared in a separate .types.ts file, not inside the component or hook.',
  },
];

const appDeliverySyntaxRules = [
  ...tsxLayeringSyntaxRules.map((rule) => ({
    ...rule,
    message:
      'Delivery Rule: app/ files cannot import infrastructure or runtime integration layers directly. Move runtime access behind a feature entrypoint.',
  })),
  {
    selector: `ImportDeclaration[source.value='react'] ImportSpecifier[imported.name=/${appLayerReactHooksPattern}/]`,
    message: 'Delivery Rule: app/ files cannot import React hooks. Move screen logic into feature hooks or components.',
  },
  {
    selector: `MemberExpression[object.name='React'][property.name=/${appLayerReactHooksPattern}/]`,
    message:
      'Delivery Rule: app/ files cannot call React hooks through the React namespace. Move screen logic into feature hooks or components.',
  },
  {
    selector: 'ImportDeclaration[source.value=/^src\\/shared\\/.*use-/]',
    message: 'Delivery Rule: app/ files cannot import shared hooks directly. Compose through feature entrypoints.',
  },
  {
    selector: 'ImportDeclaration[source.value=/^src\\/shared\\//]',
    message: 'Delivery Rule: app/ files may consume shared contracts and presenters only through intentional composition seams rooted in src/shared.',
  },
];

const featureHookAnatomySyntaxRules = [
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration[id.name=/^use[A-Z0-9]/] > BlockStatement > ExpressionStatement:has(CallExpression[callee.name="useEffect"]) ~ VariableDeclaration:has(CallExpression[callee.name=/^use(Memo|Callback)$/])',
    message: 'Hook Anatomy Rule: useEffect must come after derived state and callbacks in feature hooks.',
  },
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration[id.name=/^use[A-Z0-9]/] > BlockStatement > ExpressionStatement:has(CallExpression[callee.object.name="React"][callee.property.name="useEffect"]) ~ VariableDeclaration:has(CallExpression[callee.name=/^use(Memo|Callback)$/])',
    message: 'Hook Anatomy Rule: React.useEffect must come after derived state and callbacks in feature hooks.',
  },
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration[id.name=/^use[A-Z0-9]/] > BlockStatement > VariableDeclaration:has(CallExpression[callee.name="useCallback"]) ~ VariableDeclaration:has(CallExpression[callee.name="useMemo"] )',
    message: 'Hook Anatomy Rule: useMemo derived state must come before useCallback callbacks in feature hooks.',
  },
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration[id.name=/^use[A-Z0-9]/] > BlockStatement > :not(ReturnStatement):last-child',
    message: 'Hook Anatomy Rule: feature hooks must end with a return statement.',
  },
];

const uiExportDocumentationContexts = ['ExportNamedDeclaration > FunctionDeclaration'];

const publicTypeContractDocumentationContexts = [
  'ExportNamedDeclaration > TSInterfaceDeclaration',
  'ExportNamedDeclaration > TSTypeAliasDeclaration',
];

const publicConstantDocumentationContexts = ['ExportNamedDeclaration[declaration.type="VariableDeclaration"]'];

const publicHookDocumentationContexts = ['ExportNamedDeclaration > FunctionDeclaration[id.name=/^use[A-Z0-9]/]'];

const contextProviderSyntaxRules = [
  {
    selector:
      'Program > VariableDeclaration:not(:has(CallExpression[callee.name="createContext"])):not(:has(CallExpression[callee.object.name="React"][callee.property.name="createContext"]))',
    message:
      'Context Provider Rule: root-level constants in context modules must be limited to createContext. Move config maps and constants to *.constants.ts.',
  },
  {
    selector: 'TSInterfaceDeclaration',
    message: 'Context Provider Rule: interfaces must be declared in a separate .types.ts file, not inside the context module.',
  },
  {
    selector: 'TSTypeAliasDeclaration',
    message: 'Context Provider Rule: type aliases must be declared in a separate .types.ts file, not inside the context module.',
  },
];

const readonlyUiPropsBoundarySyntaxRules = [
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration > Identifier[typeAnnotation.typeAnnotation.type="TSTypeReference"][typeAnnotation.typeAnnotation.typeName.name=/Props$/]',
    message: 'Type Contract Rule: component props parameters must use Readonly<Props> at the function boundary.',
  },
  {
    selector:
      'ExportNamedDeclaration > FunctionDeclaration > ObjectPattern[typeAnnotation.typeAnnotation.type="TSTypeReference"][typeAnnotation.typeAnnotation.typeName.name=/Props$/]',
    message: 'Type Contract Rule: destructured component props parameters must use Readonly<Props> at the function boundary.',
  },
];

const importXExtensions = ['.js', '.jsx', '.ts', '.tsx', '.d.ts'];

function downgradeRuleSeverities(rules) {
  return Object.fromEntries(
    Object.entries(rules).map(([ruleName, ruleValue]) => {
      if (ruleValue === 'off' || ruleValue === 0) {
        return [ruleName, 'off'];
      }

      if (Array.isArray(ruleValue)) {
        return [ruleName, ['warn', ...ruleValue.slice(1)]];
      }

      return [ruleName, 'warn'];
    }),
  );
}

export {
  appDeliverySyntaxRules,
  appLayerReactHooksPattern,
  commonUiColocationSyntaxRules,
  contextProviderSyntaxRules,
  downgradeRuleSeverities,
  featureHookAnatomySyntaxRules,
  importXExtensions,
  publicConstantDocumentationContexts,
  publicHookDocumentationContexts,
  publicTypeContractDocumentationContexts,
  readonlyUiPropsBoundarySyntaxRules,
  schemaPlacementSyntaxRules,
  tsxLayeringSyntaxRules,
  uiExportDocumentationContexts,
};
