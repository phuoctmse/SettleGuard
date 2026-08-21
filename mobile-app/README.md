# mobile-app

Read-oriented client for accounts-service, ledger-service, settlement-engine,
and notification-service. Displays account balances, transaction history,
held transactions (with approve/reject actions), settlements, and real-time
alerts. Never a source of truth for domain data — all data is read from
backend services. See `docs/PROJECT_CHARTER.md` and
`docs/superpowers/specs/2026-08-19-mobile-app-design.md` for the full
design and system-wide context.

## Run locally

Requires Expo (`npm install -g expo-cli` or `npx expo`) and environment
configuration.

### Environment setup

Copy `.env.example` to `.env` (first time only):

```bash
cp .env.example .env
```

Then set the four backend base URLs in `.env`:

```
EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8081
EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8080
EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8082
EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8083
```

**Important for physical devices:** If testing on a physical phone (not
simulator/emulator), replace `localhost` with your development machine's
LAN IP address (e.g. `http://192.168.1.100:8080`). A physical device
cannot reach the dev machine's `localhost` directly — it must use the LAN
IP. Emulators and simulators can use `localhost` without change.

### Run the app

```bash
npx expo start
```

This opens the Expo CLI menu. Press:
- `i` to open in iOS simulator
- `a` to open in Android emulator
- `j` to open in web (browser)
- `s` to send the link to your phone (use Expo Go app)

## Run tests

```bash
npm test
```

Run a single test file:

```bash
npm test -- src/screens/AccountListScreen.test.tsx
```

## Configuration

The app requires four backend service base URLs, all set via environment
variables (in `.env`). Each maps to one backend service:

| Env var | Backend service | Default port |
|---|---|---|
| `EXPO_PUBLIC_ACCOUNTS_API_URL` | accounts-service | 8081 |
| `EXPO_PUBLIC_LEDGER_API_URL` | ledger-service | 8080 |
| `EXPO_PUBLIC_SETTLEMENT_API_URL` | settlement-engine | 8082 |
| `EXPO_PUBLIC_NOTIFICATION_API_URL` | notification-service | 8083 |

## Screens

The app has six main screens:

- **Client Lookup** — entry point; user enters a Client ID manually (no login
  yet) to fetch that client's accounts.
- **Account List** — displays all accounts for the selected client; tap to
  view details.
- **Account Detail** — shows account balance and full ledger entry history
  (debits/credits).
- **Held Transactions** — displays all transactions in `held` status (risk-hold
  decisions pending approval). User can approve or reject each transaction
  here — the **only write actions** in the app.
- **Settlements** — read-only list of finalized settlement batches.
- **Alerts** — displays recent risk-hold and settlement notifications. Updates
  automatically via polling (~15s refresh interval while the screen is open).
  No manual refresh needed.

## Known gaps

The following features are deferred from this MVP and documented for future
work:

- **Real push notifications** — Alerts screen currently uses polling
  (`refetchInterval` ~15s) to fetch new notifications from `GET /notifications`,
  not true push. Real push requires: (1) Expo push token registration on the
  mobile app, (2) a provider choice (e.g. Expo Push Service) and integration
  in notification-service to send push events. Provider has not been chosen
  yet.
- **Real auth/login** — currently a placeholder: users hand-type a Client ID
  with no authentication. No real auth flow, no JWT/token validation, no
  password. Deferred until a shared auth decision is made across all services.
- **Appium E2E suite** — end-to-end tests via Appium (in `tests/`) have not
  been written for mobile-app yet. Unit/component tests exist (`npm test`).
- **Production build** — EAS Build (Expo's cloud build service), app store
  publishing (Apple App Store, Google Play), and any related Kubernetes
  manifests (mobile-app is not a server-side service, so no k8s deployment
  is needed).
