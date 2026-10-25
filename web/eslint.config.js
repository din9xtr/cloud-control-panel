import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["src/api/schema.d.ts"] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  reactHooks.configs.flat.recommended,
  {
    languageOptions: { globals: globals.browser },
    rules: {
      "no-warning-comments": "error",
      "multiline-comment-style": "off",
    },
  },
  {
    files: ["**/*.test.ts", "**/*.test.tsx", "src/test/**"],
    rules: { "@typescript-eslint/no-non-null-assertion": "off" },
  },
);
