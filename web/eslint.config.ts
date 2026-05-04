import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import { defineConfig, globalIgnores } from "eslint/config";
import eslintConfigPrettier from "eslint-config-prettier/flat";

export default defineConfig([
  reactHooks.configs.flat.recommended,
  tseslint.configs.recommended,
  eslintConfigPrettier,
  globalIgnores([".react-router/*", "build/*", "dist/*", "node_modules/*"]),
]);
