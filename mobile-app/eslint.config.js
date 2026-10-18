// Flat config. `eslint-config-expo` is Expo's own preset for SDK 57 — it
// bundles the parser plus the react / react-hooks / import / expo rules
// already tuned for React Native, so we don't hand-roll them here.
// `eslint-config-prettier` comes after it to switch off the stylistic
// rules that would otherwise fight `npm run format`.
const expoConfig = require('eslint-config-expo/flat');
const prettierConfig = require('eslint-config-prettier');

module.exports = [
  {
    ignores: ['node_modules/**', '.expo/**', 'dist/**', 'web-build/**', 'expo-env.d.ts'],
  },
  ...expoConfig,
  prettierConfig,
  {
    // Jest globals for the test suite; the Expo preset only ships app globals.
    files: ['**/*.test.ts', '**/*.test.tsx', 'jest.setup-env.js', 'jest.config.js'],
    languageOptions: {
      globals: {
        jest: 'readonly',
        describe: 'readonly',
        it: 'readonly',
        test: 'readonly',
        expect: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
        beforeAll: 'readonly',
        afterAll: 'readonly',
      },
    },
  },
];
