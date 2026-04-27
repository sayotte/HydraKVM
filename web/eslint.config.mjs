// ESLint flat config (ESLint 9+).
// Enforces the mechanical subset of Google's TypeScript Style Guide;
// taste rules (naming, file structure, async style) live in STYLE.md.

import tseslint from 'typescript-eslint';

export default tseslint.config(
  ...tseslint.configs.recommended,
  {
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
      },
    },
    rules: {
      // Project-specific overrides (none yet — keep typescript-eslint defaults).
    },
  },
);
