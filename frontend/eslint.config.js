import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import pluginNoUnsanitized from 'eslint-plugin-no-unsanitized'
import eslintConfigPrettier from 'eslint-config-prettier'
import globals from 'globals'

export default tseslint.config(
  { ignores: ['dist', 'node_modules'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  // eslint-plugin-no-unsanitized blocks direct innerHTML / outerHTML /
  // insertAdjacentHTML writes, which are the primary DOM-XSS vectors once
  // the Vue template compiler is out of the loop (escape hatches, web-
  // component code, etc.). The broader rule set historically provided by
  // eslint-plugin-security is covered by the Semgrep workflow in
  // .github/workflows/semgrep.yml (p/owasp-top-ten + p/security-audit),
  // because upstream eslint-plugin-security hasn't migrated off the
  // removed context.getSourceCode API and breaks on ESLint 10.
  pluginNoUnsanitized.configs.recommended,
  {
    languageOptions: {
      globals: {
        ...globals.browser,
      },
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },
  eslintConfigPrettier,
  {
    files: ['e2e/**/*.spec.ts', 'e2e/**/*.ts'],
    rules: {
      'no-empty-pattern': 'off',
    },
  },
)
