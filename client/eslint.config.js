import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import eslintConfigPrettier from 'eslint-config-prettier'

export default tseslint.config(
  { ignores: ['dist/**', 'coverage/**'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },
  {
    rules: {
      // TS/vue-tsc already fully type-checks identifier resolution
      // (including DOM lib globals like HTMLElement/MouseEvent in type
      // positions) — no-undef is redundant here and has known false
      // positives on TS-only syntax, per typescript-eslint's own docs.
      'no-undef': 'off',
    },
  },
  eslintConfigPrettier,
)
