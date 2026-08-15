import js from "@eslint/js"
import globals from "globals"
import reactHooks from "eslint-plugin-react-hooks"
import reactRefresh from "eslint-plugin-react-refresh"
import tseslint from "typescript-eslint"
import { defineConfig, globalIgnores } from "eslint/config"

export default defineConfig([
  globalIgnores(["dist/**", "node_modules/**"]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    rules: {
      // New in react-hooks v7 and it only flags pre-existing code here
      // (hooks/use-mobile.ts). Left as a warning rather than silenced so the
      // cleanup stays visible without failing the build.
      "react-hooks/set-state-in-effect": "warn",
    },
  },
  {
    // shadcn's generated components deliberately export cva variants next to
    // the component, and the providers export their hook. Both trip a rule
    // that only governs how granular hot reloads are, so it is not an error.
    files: [
      "components/ui/**/*.tsx",
      "lib/auth-context.tsx",
      "lib/i18n/context.tsx",
    ],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
])
