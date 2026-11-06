import js from "@eslint/js";
import globals from "globals";
import eslintReact from "@eslint-react/eslint-plugin";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";

export default [
  {
    ignores: ["dist"],
  },
  // TS support (recommended, non-type-checked): tsc --noEmit is the type
  // gate; lint stays structural. Scoped to ts/tsx so JS/JSX keep espree.
  ...tseslint.configs.recommended.map((config) => ({
    ...config,
    files: ["**/*.ts", "**/*.tsx"],
  })),
  // React rules run on TSX only; rules TypeScript enforces are disabled by
  // the preset, and the official react-hooks plugin owns the hooks rules.
  ...[
    eslintReact.configs["recommended-typescript"],
    eslintReact.configs["disable-conflict-eslint-plugin-react-hooks"],
  ].map((config) => ({ ...config, files: ["**/*.tsx"] })),
  {
    files: ["**/*.{js,jsx,ts,tsx}"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
      globals: globals.browser,
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...js.configs.recommended.rules,
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
    },
  },
  {
    // Core no-unused-vars cannot see type-only usage; the
    // @typescript-eslint/no-unused-vars rule from the recommended set above
    // owns this check for TS files.
    files: ["**/*.ts", "**/*.tsx"],
    rules: {
      "no-unused-vars": "off",
    },
  },
];
