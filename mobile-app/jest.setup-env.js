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
