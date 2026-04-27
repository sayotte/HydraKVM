# TypeScript style — local cheat sheet

Subset of Google's TypeScript Style Guide that is most load-bearing for this
project. The full guide is at
https://google.github.io/styleguide/tsguide.html and is the authority where
this file is silent.

This file captures the **taste rules** (naming, file structure, async style,
construct preferences) that ESLint cannot enforce mechanically. ESLint with
`@typescript-eslint/recommended` enforces the mechanical rules
(strictness, unused variables, type-assertion safety, etc.).

If you find yourself wanting to violate a rule here, ask first.

## Files and naming

- File names are `kebab-case.ts` (e.g. `key-event-handler.ts`).
- One module per file. The file name reflects the module's primary export.
- 2-space indent. File ends with a newline. No trailing whitespace.
- Identifier conventions:
  - `UpperCamelCase` for **types**: classes, interfaces, type aliases,
    enums, decorators, type parameters.
  - `lowerCamelCase` for **values**: variables, parameters, functions,
    methods, properties, module aliases.
  - `UPPER_SNAKE_CASE` for **module-level immutable constants** that are
    truly constant (config values, sentinels). Local `const` declarations
    use `lowerCamelCase`.
  - `UpperCamelCase` for enum values (Google explicitly deviates from
    UPPER_SNAKE_CASE here).

## Types

- **Strict mode is non-negotiable.** `tsconfig.json` enables `strict`,
  `noUncheckedIndexedAccess`, `noImplicitOverride`,
  `exactOptionalPropertyTypes`. Don't relax these locally.
- **Never `any`.** Use `unknown` if the type is genuinely unknown, then
  narrow before use.
- **Avoid `!` (non-null assertion).** Use a runtime check or restructure
  the code so the type system can prove non-nullness.
- **Prefer `interface` for object shapes.** Use `type` for unions,
  intersections, and aliases of primitives.
- **Array types**: `T[]` for simple types, `Array<T>` for complex ones
  (e.g. `Array<Map<string, unknown>>`).
- **Prefer named exports.** No default exports.

## Variables and constants

- `const` always. `let` only when reassignment is genuinely required.
  Never `var`.
- `??` (nullish coalescing) when you mean "null or undefined" — not `||`,
  which falsy-coerces.
- `?.` (optional chaining) over manual `&&` null checks.
- `===` and `!==`. Never `==` or `!=`.

## Strings

- Single quotes for string literals: `'foo'`.
- Template strings (`` `…${x}…` ``) for interpolation. Never use string
  concatenation for templating.

## Functions

- Don't mutate parameters. Return a new object if you need a modified
  version.
- Prefer `async`/`await` over `.then()` chains. `.then()` is acceptable
  for one-line transforms.
- Arrow functions for callbacks; `function` declarations for top-level
  named functions or where `this`-binding matters.
- Mark functions `async` only if they actually `await`.

## Imports and modules

- ES modules only (`import`/`export`). No CommonJS, no `require()`.
- Keep import paths short. Don't introduce module aliases without reason.
- Avoid `import * as foo` unless the package documents that as the
  intended use.
- Group imports: external packages first, then internal modules. ESLint
  may eventually enforce ordering; for now follow this manually.

## DOM and browser APIs

- Use `addEventListener`. Don't assign `onclick = …` or other on-handler
  properties.
- When narrowing query results, prefer `instanceof` over `as`:
  ```ts
  const el = document.querySelector('#foo');
  if (!(el instanceof HTMLButtonElement)) {
    throw new Error('expected #foo to be a button');
  }
  // el is HTMLButtonElement here
  ```
- Use `URL` and `URLSearchParams` over manual string-building for URLs.

## Comments and JSDoc

- Comments are rare by default — same rule as elsewhere in the project.
  Add a comment only when the *why* is non-obvious.
- For exported functions/classes/types that are part of a public-ish
  surface (used across modules), short JSDoc is appropriate. For private
  helpers, identifiers should be self-describing.
- No `// removed X`, no commented-out code, no `// TODO` without a
  matching tracked issue.

## Errors

- Throw `Error` (or a subclass), never strings or plain objects.
- Catch with `unknown`, then narrow:
  ```ts
  try { … } catch (err: unknown) {
    if (err instanceof Error) { … }
  }
  ```

## Anti-patterns

- No `eval`, no `new Function(string)`.
- No `var`.
- No `any`. (Use `unknown` and narrow.)
- No default exports.
- No mutation of function parameters.
- No `// @ts-ignore`. If you need to silence the type checker for a
  specific real reason, use `// @ts-expect-error <reason>` so the
  silencer goes away when the underlying issue is fixed.
