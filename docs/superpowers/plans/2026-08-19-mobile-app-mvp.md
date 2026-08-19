# mobile-app MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `mobile-app` (Expo + TypeScript) as the read-oriented
client for `accounts-service`/`ledger-service`/`settlement-engine`
described in
`docs/superpowers/specs/2026-08-19-mobile-app-design.md`. No auth, no
real push notifications (polling instead), no gateway — the app calls the
four backend services directly over four configurable base URLs.

**Blocking dependency — read before starting Tasks 5-7:** the Held
Transactions, Settlements, and Alerts screens call HTTP endpoints that do
**not exist yet** on `settlement-engine`/`notification-service` (see
design spec §4 for the exact contracts:
`GET/POST /transactions...`, `GET /settlements...`,
`GET /notifications...`). Those are separate plans on the existing
`service/settlement-engine` and `service/notification-service` branches —
**do not implement them as part of this plan**. Tasks 1-4 (scaffold, API
client shell, Client Lookup, Account List/Detail) have no such
dependency and can be done immediately.

**Tech Stack:** Expo (managed workflow) + TypeScript, React Navigation
(`native-stack`), TanStack Query (`@tanstack/react-query`) for
data-fetching/caching/polling, built-in `fetch` (no axios), Jest +
`@testing-library/react-native` for component/hook tests. No Redux/MobX —
all state is server state owned by React Query.

## Global Constraints

- Module/package layout: `App.tsx` (entrypoint) +
  `src/{api,screens,components,navigation,config}/` — mirrors the other
  services' `cmd/server/main.go` / `main.py` + `internal/{...}/` split,
  translated to a conventional RN layout.
- No ORM/gateway/auth — this app never writes to any datastore directly
  and never is the source of truth for domain data.
- Env vars (via `.env`, read through `expo-constants`/`app.config.ts` at
  build time, `EXPO_PUBLIC_` prefix so Expo inlines them client-side):
  `EXPO_PUBLIC_ACCOUNTS_API_URL`, `EXPO_PUBLIC_LEDGER_API_URL`,
  `EXPO_PUBLIC_SETTLEMENT_API_URL`, `EXPO_PUBLIC_NOTIFICATION_API_URL`.
  Defaults in `.env.example` point at the four services' local
  `docker-compose`/`go run` ports: `:8081`, `:8080`, `:8082`, `:8083`.
- Tests co-located with source as `<name>.test.ts(x)` (Jest default
  discovery), mirroring the `_test.go`/`_test.py` convention already used
  in this repo.
- Run all commands below from inside `mobile-app/`.

---

### Task 1: Project scaffold

**Files:**
- Create: `mobile-app/package.json`, `mobile-app/tsconfig.json`,
  `mobile-app/app.config.ts`, `mobile-app/App.tsx`,
  `mobile-app/.env.example`, `mobile-app/babel.config.js`,
  `mobile-app/jest.config.js`

**Interfaces:** None yet — this task only needs to produce a blank app
that boots.

- [ ] **Step 1: Scaffold via Expo's TypeScript template**

```bash
cd /home/user/SettleGuard
npx create-expo-app@latest mobile-app --template blank-typescript
cd mobile-app
```

- [ ] **Step 2: Install the fixed dependency set**

```bash
npx expo install @react-navigation/native @react-navigation/native-stack \
  react-native-screens react-native-safe-area-context
npm install @tanstack/react-query
npm install --save-dev jest @testing-library/react-native @testing-library/jest-native \
  jest-expo @types/jest
```

- [ ] **Step 3: Configure Jest**

`mobile-app/jest.config.js`:

```js
module.exports = {
  preset: 'jest-expo',
  setupFilesAfterEach: ['@testing-library/jest-native/extend-expect'],
  transformIgnorePatterns: [
    'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@react-navigation/.*)/)',
  ],
};
```

- [ ] **Step 4: Env config**

`mobile-app/.env.example`:

```
EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8081
EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8080
EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8082
EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8083
```

`mobile-app/src/config/env.ts`:

```ts
function requireEnv(name: string, value: string | undefined): string {
  if (!value) {
    throw new Error(`${name} environment variable is required`);
  }
  return value;
}

export const env = {
  accountsApiUrl: requireEnv('EXPO_PUBLIC_ACCOUNTS_API_URL', process.env.EXPO_PUBLIC_ACCOUNTS_API_URL),
  ledgerApiUrl: requireEnv('EXPO_PUBLIC_LEDGER_API_URL', process.env.EXPO_PUBLIC_LEDGER_API_URL),
  settlementApiUrl: requireEnv('EXPO_PUBLIC_SETTLEMENT_API_URL', process.env.EXPO_PUBLIC_SETTLEMENT_API_URL),
  notificationApiUrl: requireEnv('EXPO_PUBLIC_NOTIFICATION_API_URL', process.env.EXPO_PUBLIC_NOTIFICATION_API_URL),
};
```

Note: on real devices (not simulator), `localhost` in `.env.example` must
be swapped for the dev machine's LAN IP — document this in the README
(Task 8), not solved in code.

- [ ] **Step 5: Verify it boots**

```bash
cp .env.example .env
npx expo start
```

Expected: QR code / simulator option printed, app opens showing the
default blank-typescript screen, no red-box errors.

- [ ] **Step 6: Commit**

```bash
git add mobile-app/package.json mobile-app/package-lock.json mobile-app/tsconfig.json \
        mobile-app/app.config.ts mobile-app/App.tsx mobile-app/.env.example \
        mobile-app/babel.config.js mobile-app/jest.config.js mobile-app/src/config/
git commit -m "feat(mobile-app): scaffold Expo + TypeScript project"
```

---

### Task 2: API client layer

**Files:**
- Create: `src/api/types.ts`, `src/api/accounts.ts`, `src/api/ledger.ts`,
  `src/api/http.ts`
- Test: `src/api/http.test.ts`

**Interfaces:**
- Produces: `fetchJson<T>(url: string, init?: RequestInit): Promise<T>`
  (throws `ApiError` with `.status` on non-2xx), plus typed functions
  `listAccounts(clientId: string)`, `getAccount(id: string)`,
  `listEntries(accountId: string)` that every screen hook (Tasks 3-4)
  calls. Settlement-engine/notification-service clients are added in
  Tasks 5-7 once their endpoints exist.

- [ ] **Step 1: Write the failing test**

`src/api/http.test.ts`:

```ts
import { fetchJson, ApiError } from './http';

describe('fetchJson', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  it('returns parsed JSON on 200', async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ id: '1' }),
    });

    await expect(fetchJson('http://x/accounts/1')).resolves.toEqual({ id: '1' });
  });

  it('throws ApiError with status on non-2xx', async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({}),
    });

    await expect(fetchJson('http://x/accounts/missing')).rejects.toMatchObject(
      new ApiError(404, 'http://x/accounts/missing'),
    );
  });
});
```

- [ ] **Step 2: Run to verify it fails**

`npm test src/api/http.test.ts` — fails, `./http` doesn't exist yet.

- [ ] **Step 3: Implement `src/api/http.ts`**

```ts
export class ApiError extends Error {
  constructor(public status: number, public url: string) {
    super(`request to ${url} failed with status ${status}`);
    this.name = 'ApiError';
  }
}

export async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    throw new ApiError(res.status, url);
  }
  return res.json() as Promise<T>;
}
```

- [ ] **Step 4: Run to verify it passes**

`npm test src/api/http.test.ts` — passes.

- [ ] **Step 5: Define shared types and the accounts/ledger clients**

`src/api/types.ts`:

```ts
export type AccountStatus = 'active' | 'suspended' | 'closed';

export interface Account {
  id: string;
  client_id: string;
  external_ref: string | null;
  status: AccountStatus;
  balance: number;
  created_at: string;
}

export interface LedgerEntry {
  id: string;
  transaction_id: string;
  account_id: string;
  direction: 'debit' | 'credit';
  amount: number;
  reason: string;
  created_at: string;
}
```

`src/api/accounts.ts`:

```ts
import { env } from '../config/env';
import { fetchJson } from './http';
import type { Account } from './types';

export function listAccounts(clientId: string): Promise<Account[]> {
  const url = `${env.accountsApiUrl}/accounts?client_id=${encodeURIComponent(clientId)}`;
  return fetchJson<Account[]>(url);
}

export function getAccount(id: string): Promise<Account> {
  return fetchJson<Account>(`${env.accountsApiUrl}/accounts/${id}`);
}
```

`src/api/ledger.ts`:

```ts
import { env } from '../config/env';
import { fetchJson } from './http';
import type { LedgerEntry } from './types';

export function listEntriesForAccount(accountId: string): Promise<LedgerEntry[]> {
  const url = `${env.ledgerApiUrl}/entries?account_id=${encodeURIComponent(accountId)}`;
  return fetchJson<LedgerEntry[]>(url);
}
```

- [ ] **Step 6: Commit**

```bash
git add src/api/
git commit -m "feat(mobile-app): API client for accounts-service and ledger-service"
```

---

### Task 3: Navigation shell + Client Lookup screen

**Files:**
- Create: `src/navigation/RootNavigator.tsx`,
  `src/screens/ClientLookupScreen.tsx`
- Modify: `App.tsx`
- Test: `src/screens/ClientLookupScreen.test.tsx`

**Interfaces:**
- Produces: `RootNavigator`, mounted by `App.tsx` inside a
  `QueryClientProvider`. Defines the stack's route params type
  (`RootStackParamList`) that every later screen (Tasks 4-7) is typed
  against.

- [ ] **Step 1: Define the navigator and route params**

`src/navigation/RootNavigator.tsx`:

```tsx
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';

import { ClientLookupScreen } from '../screens/ClientLookupScreen';
import { AccountListScreen } from '../screens/AccountListScreen';
import { AccountDetailScreen } from '../screens/AccountDetailScreen';

export type RootStackParamList = {
  ClientLookup: undefined;
  AccountList: { clientId: string };
  AccountDetail: { accountId: string };
};

const Stack = createNativeStackNavigator<RootStackParamList>();

export function RootNavigator() {
  return (
    <NavigationContainer>
      <Stack.Navigator initialRouteName="ClientLookup">
        <Stack.Screen name="ClientLookup" component={ClientLookupScreen} options={{ title: 'SettleGuard' }} />
        <Stack.Screen name="AccountList" component={AccountListScreen} options={{ title: 'Accounts' }} />
        <Stack.Screen name="AccountDetail" component={AccountDetailScreen} options={{ title: 'Account' }} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
```

(`AccountListScreen`/`AccountDetailScreen` are stubbed as empty
components in this task, filled in by Task 4.)

- [ ] **Step 2: Write the Client Lookup screen + its test**

`src/screens/ClientLookupScreen.test.tsx`:

```tsx
import { fireEvent, render } from '@testing-library/react-native';

import { ClientLookupScreen } from './ClientLookupScreen';

it('navigates to AccountList with the entered client id', () => {
  const navigate = jest.fn();
  const { getByPlaceholderText, getByText } = render(
    <ClientLookupScreen navigation={{ navigate } as any} route={{} as any} />,
  );

  fireEvent.changeText(getByPlaceholderText('Client ID'), 'client-123');
  fireEvent.press(getByText('View Accounts'));

  expect(navigate).toHaveBeenCalledWith('AccountList', { clientId: 'client-123' });
});
```

`src/screens/ClientLookupScreen.tsx`:

```tsx
import { useState } from 'react';
import { Button, StyleSheet, Text, TextInput, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';

type Props = NativeStackScreenProps<RootStackParamList, 'ClientLookup'>;

export function ClientLookupScreen({ navigation }: Props) {
  const [clientId, setClientId] = useState('');

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Enter a Client ID to view its accounts:</Text>
      <TextInput
        style={styles.input}
        placeholder="Client ID"
        value={clientId}
        onChangeText={setClientId}
        autoCapitalize="none"
      />
      <Button
        title="View Accounts"
        disabled={clientId.trim().length === 0}
        onPress={() => navigation.navigate('AccountList', { clientId: clientId.trim() })}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 16, justifyContent: 'center' },
  label: { marginBottom: 8 },
  input: { borderWidth: 1, borderColor: '#ccc', borderRadius: 6, padding: 8, marginBottom: 12 },
});
```

- [ ] **Step 3: Wire `App.tsx`**

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { RootNavigator } from './src/navigation/RootNavigator';

const queryClient = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RootNavigator />
    </QueryClientProvider>
  );
}
```

- [ ] **Step 4: Run tests, verify pass**

`npm test` — `ClientLookupScreen.test.tsx` passes (stub
`AccountListScreen`/`AccountDetailScreen` just need to exist and export a
component, filled in next task).

- [ ] **Step 5: Commit**

```bash
git add App.tsx src/navigation/ src/screens/ClientLookupScreen.tsx src/screens/ClientLookupScreen.test.tsx
git commit -m "feat(mobile-app): navigation shell + Client Lookup screen"
```

---

### Task 4: Account List + Account Detail screens

**Files:**
- Create: `src/screens/AccountListScreen.tsx`, `src/screens/AccountDetailScreen.tsx`
- Test: `src/screens/AccountListScreen.test.tsx`

**Interfaces:**
- Consumes: `listAccounts`, `getAccount` (Task 2), `RootStackParamList`
  (Task 3).
- Produces: fully working read path for accounts + ledger history — the
  first end-to-end screen pair, runnable against real
  accounts-service/ledger-service today (no blocking dependency).

- [ ] **Step 1: Write the failing test**

`src/screens/AccountListScreen.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AccountListScreen } from './AccountListScreen';
import * as accountsApi from '../api/accounts';

jest.mock('../api/accounts');

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient();
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders accounts returned by listAccounts', async () => {
  (accountsApi.listAccounts as jest.Mock).mockResolvedValue([
    { id: 'acc-1', client_id: 'client-123', external_ref: 'ext-1', status: 'active', balance: 500, created_at: '' },
  ]);

  const { findByText } = renderWithQuery(
    <AccountListScreen
      navigation={{ navigate: jest.fn() } as any}
      route={{ params: { clientId: 'client-123' } } as any}
    />,
  );

  expect(await findByText('ext-1')).toBeTruthy();
  expect(accountsApi.listAccounts).toHaveBeenCalledWith('client-123');
});
```

- [ ] **Step 2: Run to verify it fails**, then implement.

`src/screens/AccountListScreen.tsx`:

```tsx
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';
import { listAccounts } from '../api/accounts';

type Props = NativeStackScreenProps<RootStackParamList, 'AccountList'>;

export function AccountListScreen({ navigation, route }: Props) {
  const { clientId } = route.params;
  const { data, isLoading, error } = useQuery({
    queryKey: ['accounts', clientId],
    queryFn: () => listAccounts(clientId),
  });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;
  if (error) return <Text style={styles.pad}>Failed to load accounts.</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(a) => a.id}
      renderItem={({ item }) => (
        <Pressable style={styles.row} onPress={() => navigation.navigate('AccountDetail', { accountId: item.id })}>
          <Text>{item.external_ref ?? item.id}</Text>
          <Text>{item.status}</Text>
        </Pressable>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No accounts for this client.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { flexDirection: 'row', justifyContent: 'space-between', padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
```

`src/screens/AccountDetailScreen.tsx`:

```tsx
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';
import { getAccount } from '../api/accounts';
import { listEntriesForAccount } from '../api/ledger';

type Props = NativeStackScreenProps<RootStackParamList, 'AccountDetail'>;

export function AccountDetailScreen({ route }: Props) {
  const { accountId } = route.params;
  const account = useQuery({ queryKey: ['account', accountId], queryFn: () => getAccount(accountId) });
  const entries = useQuery({ queryKey: ['entries', accountId], queryFn: () => listEntriesForAccount(accountId) });

  return (
    <View style={styles.container}>
      {account.data && (
        <View style={styles.header}>
          <Text style={styles.balance}>Balance: {account.data.balance}</Text>
          <Text>Status: {account.data.status}</Text>
        </View>
      )}
      <FlatList
        data={entries.data ?? []}
        keyExtractor={(e) => e.id}
        renderItem={({ item }) => (
          <View style={styles.row}>
            <Text>{item.direction} {item.amount}</Text>
            <Text>{item.reason}</Text>
          </View>
        )}
        ListEmptyComponent={<Text style={styles.pad}>No ledger entries yet.</Text>}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
  balance: { fontSize: 20, fontWeight: '600' },
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
```

- [ ] **Step 3: Run full test suite, verify pass**

`npm test`

- [ ] **Step 4: Manual smoke test against real services**

```bash
docker compose -f infra/docker/docker-compose.yml up -d accounts-postgres ledger-postgres nats
# in accounts-service/: go run ./cmd/server
# in ledger-service/: go run ./cmd/server
# in mobile-app/: npx expo start
```

Create a client + account + a couple of ledger entries via `curl` against
accounts-service/ledger-service directly (see their READMEs), then in the
app: enter that client ID → see the account → tap it → see balance and
entries.

- [ ] **Step 5: Commit**

```bash
git add src/screens/AccountListScreen.tsx src/screens/AccountListScreen.test.tsx src/screens/AccountDetailScreen.tsx
git commit -m "feat(mobile-app): Account List and Account Detail screens"
```

---

### Task 5: Held Transactions screen (blocked on settlement-engine plan)

**Precondition:** `settlement-engine` exposes
`GET /transactions?status=held`, `POST /transactions/{id}/approve`,
`POST /transactions/{id}/reject` (design spec §4). Do not start this task
until that lands.

**Files:**
- Create: `src/api/settlement.ts`, `src/screens/HeldTransactionsScreen.tsx`
- Modify: `src/navigation/RootNavigator.tsx` (add route)
- Test: `src/screens/HeldTransactionsScreen.test.tsx`

**Interfaces:**
- Produces: `listHeldTransactions()`, `approveTransaction(id)`,
  `rejectTransaction(id)` in `src/api/settlement.ts`, and the screen that
  calls them via `useMutation`, invalidating the `['held-transactions']`
  query key on success so the list drops the resolved row immediately.

- [ ] **Step 1: `src/api/settlement.ts`**

```ts
import { env } from '../config/env';
import { fetchJson } from './http';

export interface Transaction {
  id: string;
  amount: number;
  score: number;
  decision: 'pass' | 'hold';
  status: 'pending_settlement' | 'held' | 'settled' | 'rejected';
  triggered_rules: string[];
  scored_at: string;
}

export function listHeldTransactions(): Promise<Transaction[]> {
  return fetchJson<Transaction[]>(`${env.settlementApiUrl}/transactions?status=held`);
}

export function approveTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/approve`, { method: 'POST' });
}

export function rejectTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/reject`, { method: 'POST' });
}
```

- [ ] **Step 2: Test + implement the screen**

Test mirrors `AccountListScreen.test.tsx`'s mock-module pattern, plus one
case asserting that pressing "Approve" calls `approveTransaction` with the
row's id and the row disappears after the mutation resolves.

```tsx
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';

export function HeldTransactionsScreen() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ['held-transactions'], queryFn: listHeldTransactions });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['held-transactions'] });
  const approve = useMutation({ mutationFn: approveTransaction, onSuccess: invalidate });
  const reject = useMutation({ mutationFn: rejectTransaction, onSuccess: invalidate });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(t) => t.id}
      renderItem={({ item }) => (
        <View style={styles.row}>
          <Text>Amount: {item.amount} · Score: {item.score}</Text>
          <Text>Triggered: {item.triggered_rules.join(', ') || 'none'}</Text>
          <View style={styles.actions}>
            <Pressable onPress={() => approve.mutate(item.id)}><Text>Approve</Text></Pressable>
            <Pressable onPress={() => reject.mutate(item.id)}><Text>Reject</Text></Pressable>
          </View>
        </View>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No held transactions.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
  actions: { flexDirection: 'row', gap: 16, marginTop: 8 },
});
```

- [ ] **Step 3: Add the route to `RootNavigator`, add a way to reach it**
  (simplest: a button on `AccountListScreen` header, or a bottom tab —
  pick whichever is less code; a header button is enough for MVP).

- [ ] **Step 4: Run tests, manual smoke test against real
  settlement-engine (once the prerequisite endpoints exist), commit**

```bash
git add src/api/settlement.ts src/screens/HeldTransactionsScreen.tsx src/screens/HeldTransactionsScreen.test.tsx src/navigation/RootNavigator.tsx
git commit -m "feat(mobile-app): Held Transactions screen with approve/reject"
```

---

### Task 6: Settlements screen (blocked on settlement-engine plan)

**Precondition:** `GET /settlements` exists (design spec §4).

**Files:**
- Modify: `src/api/settlement.ts` (add `listSettlements`)
- Create: `src/screens/SettlementsScreen.tsx`
- Test: `src/screens/SettlementsScreen.test.tsx`

Same shape as Task 4's list screens: `useQuery(['settlements'],
listSettlements)`, render `FlatList` of `{ id, transaction_count,
total_amount, created_at }`, read-only (no mutations). Add the route to
`RootNavigator`.

- [ ] Steps: write failing test → implement `listSettlements` +
  `SettlementsScreen` → verify pass → add route → commit
  (`feat(mobile-app): Settlements screen`).

---

### Task 7: Alerts screen with polling (blocked on notification-service plan)

**Precondition:** `GET /notifications?type=&since=` exists (design spec
§4).

**Files:**
- Create: `src/api/notifications.ts`, `src/screens/AlertsScreen.tsx`
- Test: `src/screens/AlertsScreen.test.tsx`

**Interfaces:**
- Produces: `listNotifications(): Promise<Notification[]>` and a screen
  using `useQuery({ queryKey: ['notifications'], queryFn:
  listNotifications, refetchInterval: 15000 })` — this is the one query in
  the app with `refetchInterval` set, since it's the closest MVP
  approximation to "real-time alerts" without real push (see design spec
  §2).

```ts
// src/api/notifications.ts
import { env } from '../config/env';
import { fetchJson } from './http';

export interface Notification {
  id: string;
  type: 'risk_hold' | 'settlement_finalized';
  subject_id: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export function listNotifications(): Promise<Notification[]> {
  return fetchJson<Notification[]>(`${env.notificationApiUrl}/notifications`);
}
```

`AlertsScreen` mirrors `SettlementsScreen`'s read-only list shape, plus
`refetchInterval: 15000` in the `useQuery` options. Test asserts the query
is configured with `refetchInterval` (render, inspect via a spy on
`useQuery`, or simpler: assert `listNotifications` is called again after
advancing fake timers past 15s with `jest.useFakeTimers()`).

- [ ] Steps: write failing test → implement → verify pass → add route →
  commit (`feat(mobile-app): Alerts screen with polling`).

---

### Task 8: README

**Files:**
- Create: `mobile-app/README.md`

Mirror the structure of the other 4 services' READMEs: what the app does
and doesn't do (never a source of truth — read-only + the one
approve/reject write path), how to run locally (`npx expo start`, env
setup, note about `localhost` vs LAN IP on physical devices), how to run
tests (`npm test`), the four base-URL env vars and which backend service
each maps to, and an explicit "Known gaps" section listing what's
deferred (per design spec §7: real push, auth, Appium E2E, prod build) so
nobody mistakes the polling-based Alerts screen for real push later.

- [ ] **Step 1: Write the README** covering the sections above.
- [ ] **Step 2: Update root `CLAUDE.md`** — add `mobile-app`'s
  build/lint/test commands to the Project Status section (it's the last
  scaffold-with-no-code entry to clear).
- [ ] **Step 3: Commit**

```bash
git add mobile-app/README.md CLAUDE.md
git commit -m "docs(mobile-app): add README, update CLAUDE.md project status"
```
