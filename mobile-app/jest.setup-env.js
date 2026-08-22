// Provides dummy values for the EXPO_PUBLIC_* env vars required by
// src/config/env.ts so tests can load modules that import the API client
// (env.ts throws at import time if these are unset). Real values live in
// .env, which is only loaded by Expo's Metro bundler at runtime, not by
// Jest — network calls in tests always go through mocked API modules, so
// the actual URL values here are irrelevant.
process.env.EXPO_PUBLIC_ACCOUNTS_API_URL ||= 'http://localhost:8081';
process.env.EXPO_PUBLIC_LEDGER_API_URL ||= 'http://localhost:8080';
process.env.EXPO_PUBLIC_SETTLEMENT_API_URL ||= 'http://localhost:8082';
process.env.EXPO_PUBLIC_NOTIFICATION_API_URL ||= 'http://localhost:8083';

// react-native-safe-area-context needs its dedicated Jest mock (its native
// module has no real insets in the test environment) — required as soon as
// any component uses useSafeAreaInsets, per the library's own setup docs.
// The mock file is `export default {...}`; requiring it directly (bypassing
// Babel's import-interop) yields { default: {...} }, so unwrap .default or
// named imports like useSafeAreaInsets resolve to undefined.
jest.mock('react-native-safe-area-context', () => {
  const mock = require('react-native-safe-area-context/jest/mock');
  return mock.default ?? mock;
});
