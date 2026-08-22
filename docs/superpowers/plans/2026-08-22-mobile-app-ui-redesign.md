# mobile-app UI/UX Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `mobile-app`'s default-looking UI (plain RN `Text`/`Button`/`FlatList`, no color system, 3 header buttons for nav) with an intentional "Sunset Coral" visual identity, a shared design-system component layer, and a bottom-tab navigation structure — no backend changes, no new screens, no new business functionality.

**Architecture:** A new `src/theme/` module (color/typography/spacing tokens) and `src/components/` module (6 small presentational components: `ScreenContainer`, `Card`, `StatusChip`, `BalanceDisplay`, `Button`, `EmptyState`) get built first, bottom-up. `RootNavigator.tsx` is restructured from one flat stack into a bottom-tab navigator wrapping 4 tabs (Accounts has a nested stack for its 3 screens; Held/Settlements/Alerts are flat). All 6 existing screens are then restyled to consume the new theme/components, preserving every string an existing test asserts on so no test file needs rewriting — only new tests for the new components.

**Tech Stack:** Same as existing `mobile-app` (Expo SDK 57, TypeScript, React Navigation, TanStack Query, Jest + RNTL). New dependencies: `@react-navigation/bottom-tabs`, `@expo/vector-icons`, `expo-linear-gradient`.

## Global Constraints

- **Design spec:** `docs/superpowers/specs/2026-08-22-mobile-app-ui-redesign-design.md` — read for full rationale; this plan carries every exact value forward, so implementers don't need to re-read it, but it's the source of truth if this plan and the spec ever disagree.
- **No backend/API changes.** No new screens, no new business functionality. Every existing screen keeps its current data-fetching logic (`useQuery`/`useMutation` calls) untouched — only rendering changes.
- **RNTL v14 is async:** `render()` and `fireEvent.*()` must be `await`ed. This is a hard-won lesson from this project's MVP build (a prior implementer wrote a broken workaround assuming otherwise) — every test in this plan already does this correctly; copy the pattern exactly.
- **New dependency install command:** this project's `package-lock.json` needs `--force` (not `--legacy-peer-deps`) for any `npm install`, due to a pre-existing `react-test-renderer` peer conflict (react 19.2.3 pinned vs. react-test-renderer's latest publish wanting 19.2.8). **Do not use `--legacy-peer-deps`** — it was tried during this plan's own execution (Task 4) and silently prunes `test-renderer`, an *undeclared* peer dependency of `@testing-library/react-native@14` that every test file needs (`--legacy-peer-deps` disables npm's peer auto-install entirely, so an undeclared, peer-only package gets dropped from the tree with no warning). `--force` still performs normal peer auto-install and only overrides the one known conflict. Always install new packages with:
  ```bash
  npm install <package>@<version> --registry=https://registry.npmjs.org --force
  ```
  The explicit `--registry` flag matters too — this machine's default npm registry is a mirror (`registry.npmmirror.com`); omitting it re-introduces mirror URLs into the committed lockfile (previously fixed and flagged as must-fix-before-merge in the MVP's whole-branch review). If a fresh install still leaves any `registry.npmmirror.com` URLs in `package-lock.json` (can happen from stale local npm cache), run `npm cache clean --force` and reinstall.
  Do **not** use `npx expo install` for new packages in this plan — it internally calls plain `npm install` without either flag and will fail with `ERESOLVE`.
- **No Claude/AI attribution trailers** in any commit message (repo-wide hard rule).
- **Preserve every string an existing test asserts on.** The screens being restyled have existing tests (`*.test.tsx` next to each screen) that assert on exact visible text (e.g. `'Balance: 500'`, `'Failed to load held transactions.'`, `'Approve'`). This plan's screen-restyle tasks are written so the rendered text is byte-identical to today — do not paraphrase, reformat, or "improve" any string a test checks. Run the existing test file after every screen change to confirm.
- **Colors/typography/spacing values below are exact** — copy them verbatim into the token files, do not approximate.

---

### Task 1: Theme tokens

**Files:**
- Create: `mobile-app/src/theme/colors.ts`
- Create: `mobile-app/src/theme/typography.ts`
- Create: `mobile-app/src/theme/spacing.ts`
- Create: `mobile-app/src/theme/index.ts`
- Test: `mobile-app/src/theme/theme.test.ts`

**Interfaces:**
- Consumes: nothing (pure constants, no new dependencies)
- Produces: `colors`, `typography`, `spacing` — imported by every later task as `import { colors, spacing, typography } from '../theme'` (from `src/components/` and `src/screens/`) or `'./theme'` (from `src/navigation/`)

- [ ] **Step 1: Write `src/theme/colors.ts`**

```ts
export const colors = {
  backgroundGradient: ['#FFF3EC', '#FFE4D6'] as const,
  surface: 'rgba(255,255,255,0.55)',
  surfaceSolid: '#FFFFFF',
  textPrimary: '#2B1A12',
  textSecondary: 'rgba(43,26,18,0.6)',
  accent: '#E8541F',
  accentMuted: 'rgba(232,84,31,0.14)',
  success: '#1A9E5C',
  successMuted: 'rgba(26,158,92,0.14)',
  danger: '#D13B3B',
  dangerMuted: 'rgba(209,59,59,0.14)',
  warning: '#C8420F',
  warningMuted: 'rgba(232,84,31,0.14)',
  neutral: '#8A7364',
  neutralMuted: 'rgba(138,115,100,0.14)',
} as const;
```

- [ ] **Step 2: Write `src/theme/typography.ts`**

```ts
import type { TextStyle } from 'react-native';

type TypographyKey = 'display' | 'heading' | 'label' | 'body' | 'caption' | 'chip';

export const typography: Record<TypographyKey, TextStyle> = {
  display: { fontSize: 32, fontWeight: '800', letterSpacing: -1 },
  heading: { fontSize: 17, fontWeight: '700', letterSpacing: -0.2 },
  label: { fontSize: 12, fontWeight: '700', letterSpacing: 0.5, textTransform: 'uppercase' },
  body: { fontSize: 14, fontWeight: '500' },
  caption: { fontSize: 12, fontWeight: '500' },
  chip: { fontSize: 10, fontWeight: '700', letterSpacing: 0.3 },
};
```

- [ ] **Step 3: Write `src/theme/spacing.ts`**

```ts
export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  cardRadius: 14,
} as const;
```

- [ ] **Step 4: Write `src/theme/index.ts`**

```ts
export * from './colors';
export * from './typography';
export * from './spacing';
```

- [ ] **Step 5: Write the test**

```ts
import { colors } from './colors';
import { typography } from './typography';
import { spacing } from './spacing';

it('exposes the accent color used throughout the app', () => {
  expect(colors.accent).toBe('#E8541F');
});

it('exposes a display typography style for balance figures', () => {
  expect(typography.display.fontSize).toBe(32);
  expect(typography.display.fontWeight).toBe('800');
});

it('exposes the spacing scale', () => {
  expect(spacing.lg).toBe(16);
  expect(spacing.cardRadius).toBe(14);
});
```

- [ ] **Step 6: Run the test**

Run: `npm test -- src/theme/theme.test.ts`
Expected: PASS, 3 tests.

- [ ] **Step 7: Commit**

```bash
git add src/theme/
git commit -m "feat(mobile-app): add design system theme tokens"
```

---

### Task 2: Presentational components — Card, StatusChip, BalanceDisplay, Button

**Files:**
- Create: `mobile-app/src/components/Card.tsx`
- Create: `mobile-app/src/components/Card.test.tsx`
- Create: `mobile-app/src/components/StatusChip.tsx`
- Create: `mobile-app/src/components/StatusChip.test.tsx`
- Create: `mobile-app/src/components/BalanceDisplay.tsx`
- Create: `mobile-app/src/components/BalanceDisplay.test.tsx`
- Create: `mobile-app/src/components/Button.tsx`
- Create: `mobile-app/src/components/Button.test.tsx`

**Interfaces:**
- Consumes: `colors`, `spacing`, `typography` from `../theme` (Task 1)
- Produces:
  - `Card({ children, onPress?, style? })` — renders a rounded white card; pressable (with press feedback) only if `onPress` given
  - `StatusChip({ status })` — renders a colored pill; `status` is any string, color looked up from a fixed map, falls back to neutral for unmapped values
  - `BalanceDisplay({ amount })` — renders `Balance: {amount}` in `display` typography, colored green if `amount >= 0` else red
  - `Button({ title, onPress, disabled?, variant? })` — `variant` defaults to `'primary'` (solid accent) vs `'secondary'` (muted accent, outline-style)

- [ ] **Step 1: Write `src/components/Card.tsx`**

```tsx
import type { ReactNode } from 'react';
import { Pressable, StyleSheet, View, type ViewStyle } from 'react-native';

import { colors, spacing } from '../theme';

interface Props {
  children: ReactNode;
  onPress?: () => void;
  style?: ViewStyle;
}

export function Card({ children, onPress, style }: Props) {
  if (onPress) {
    return (
      <Pressable
        onPress={onPress}
        style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}
      >
        {children}
      </Pressable>
    );
  }
  return <View style={[styles.card, style]}>{children}</View>;
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surfaceSolid,
    borderRadius: spacing.cardRadius,
    padding: spacing.lg,
    marginBottom: spacing.sm,
  },
  pressed: {
    opacity: 0.85,
    transform: [{ scale: 0.98 }],
  },
});
```

- [ ] **Step 2: Write `src/components/Card.test.tsx`**

```tsx
import { fireEvent, render } from '@testing-library/react-native';
import { Text } from 'react-native';

import { Card } from './Card';

it('renders its children', async () => {
  const { findByText } = await render(
    <Card>
      <Text>Row content</Text>
    </Card>,
  );
  expect(await findByText('Row content')).toBeTruthy();
});

it('calls onPress when tapped, if provided', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(
    <Card onPress={onPress}>
      <Text>Tap me</Text>
    </Card>,
  );
  await fireEvent.press(getByText('Tap me'));
  expect(onPress).toHaveBeenCalledTimes(1);
});
```

- [ ] **Step 3: Write `src/components/StatusChip.tsx`**

```tsx
import { StyleSheet, Text, View } from 'react-native';

import { colors, spacing, typography } from '../theme';

const STATUS_COLORS: Record<string, { bg: string; fg: string }> = {
  held: { bg: colors.warningMuted, fg: colors.warning },
  pending_settlement: { bg: colors.neutralMuted, fg: colors.neutral },
  settled: { bg: colors.successMuted, fg: colors.success },
  rejected: { bg: colors.dangerMuted, fg: colors.danger },
  risk_hold: { bg: colors.warningMuted, fg: colors.warning },
  settlement_finalized: { bg: colors.successMuted, fg: colors.success },
};

interface Props {
  status: string;
}

export function StatusChip({ status }: Props) {
  const tone = STATUS_COLORS[status] ?? { bg: colors.neutralMuted, fg: colors.neutral };
  return (
    <View style={[styles.chip, { backgroundColor: tone.bg }]}>
      <Text style={[typography.chip, { color: tone.fg }]}>{status}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  chip: {
    alignSelf: 'flex-start',
    borderRadius: 20,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
  },
});
```

- [ ] **Step 4: Write `src/components/StatusChip.test.tsx`**

```tsx
import { render } from '@testing-library/react-native';

import { StatusChip } from './StatusChip';

it('renders the status text', async () => {
  const { findByText } = await render(<StatusChip status="held" />);
  expect(await findByText('held')).toBeTruthy();
});

it('falls back to a neutral style for an unknown status', async () => {
  const { findByText } = await render(<StatusChip status="unknown_status" />);
  expect(await findByText('unknown_status')).toBeTruthy();
});
```

- [ ] **Step 5: Write `src/components/BalanceDisplay.tsx`**

```tsx
import { Text } from 'react-native';

import { colors, typography } from '../theme';

interface Props {
  amount: number;
}

export function BalanceDisplay({ amount }: Props) {
  const color = amount < 0 ? colors.danger : colors.success;
  return <Text style={[typography.display, { color }]}>Balance: {amount}</Text>;
}
```

- [ ] **Step 6: Write `src/components/BalanceDisplay.test.tsx`**

```tsx
import { render } from '@testing-library/react-native';

import { BalanceDisplay } from './BalanceDisplay';

it('renders a positive balance', async () => {
  const { findByText } = await render(<BalanceDisplay amount={500} />);
  expect(await findByText('Balance: 500')).toBeTruthy();
});

it('renders a negative balance', async () => {
  const { findByText } = await render(<BalanceDisplay amount={-50} />);
  expect(await findByText('Balance: -50')).toBeTruthy();
});
```

- [ ] **Step 7: Write `src/components/Button.tsx`**

```tsx
import { Pressable, StyleSheet, Text } from 'react-native';

import { colors, spacing, typography } from '../theme';

interface Props {
  title: string;
  onPress: () => void;
  disabled?: boolean;
  variant?: 'primary' | 'secondary';
}

export function Button({ title, onPress, disabled, variant = 'primary' }: Props) {
  const isPrimary = variant === 'primary';
  return (
    <Pressable
      disabled={disabled}
      onPress={onPress}
      style={({ pressed }) => [
        styles.button,
        isPrimary ? styles.primary : styles.secondary,
        disabled ? styles.disabled : null,
        pressed && !disabled ? styles.pressed : null,
      ]}
    >
      <Text style={[typography.body, isPrimary ? styles.primaryText : styles.secondaryText]}>{title}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    borderRadius: 20,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    alignItems: 'center',
    justifyContent: 'center',
  },
  primary: { backgroundColor: colors.accent },
  secondary: { backgroundColor: colors.accentMuted },
  disabled: { opacity: 0.5 },
  pressed: { opacity: 0.85, transform: [{ scale: 0.97 }] },
  primaryText: { color: '#FFFFFF', fontWeight: '700' },
  secondaryText: { color: colors.accent, fontWeight: '700' },
});
```

- [ ] **Step 8: Write `src/components/Button.test.tsx`**

```tsx
import { fireEvent, render } from '@testing-library/react-native';

import { Button } from './Button';

it('calls onPress when tapped', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(<Button title="Approve" onPress={onPress} />);
  await fireEvent.press(getByText('Approve'));
  expect(onPress).toHaveBeenCalledTimes(1);
});

it('does not call onPress when disabled', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(<Button title="Approve" onPress={onPress} disabled />);
  await fireEvent.press(getByText('Approve'));
  expect(onPress).not.toHaveBeenCalled();
});
```

- [ ] **Step 9: Run the tests**

Run: `npm test -- src/components/`
Expected: PASS, 4 suites, 6 tests.

- [ ] **Step 10: Commit**

```bash
git add src/components/Card.tsx src/components/Card.test.tsx src/components/StatusChip.tsx src/components/StatusChip.test.tsx src/components/BalanceDisplay.tsx src/components/BalanceDisplay.test.tsx src/components/Button.tsx src/components/Button.test.tsx
git commit -m "feat(mobile-app): add Card, StatusChip, BalanceDisplay, Button components"
```

---

### Task 3: EmptyState component

**Files:**
- Create: `mobile-app/src/components/EmptyState.tsx`
- Create: `mobile-app/src/components/EmptyState.test.tsx`

**Interfaces:**
- Consumes: `colors`, `spacing`, `typography` from `../theme` (Task 1)
- Produces: `EmptyState({ status: 'loading' | 'error' | 'empty', message? })` — for `status='loading'` always renders `"Loading…"` (message ignored); for `'error'`/`'empty'`, renders `message` verbatim. Every screen task (4-8) uses this for its loading/error/empty-list states, always passing the exact message string that today's inline `<Text>` renders, so existing tests keep passing unmodified.

- [ ] **Step 1: Write `src/components/EmptyState.tsx`**

```tsx
import { StyleSheet, Text } from 'react-native';

import { colors, spacing, typography } from '../theme';

interface Props {
  status: 'loading' | 'error' | 'empty';
  message?: string;
}

export function EmptyState({ status, message }: Props) {
  const text = status === 'loading' ? 'Loading…' : (message ?? '');
  return <Text style={[typography.body, styles.text]}>{text}</Text>;
}

const styles = StyleSheet.create({
  text: { padding: spacing.lg, color: colors.textSecondary },
});
```

- [ ] **Step 2: Write `src/components/EmptyState.test.tsx`**

```tsx
import { render } from '@testing-library/react-native';

import { EmptyState } from './EmptyState';

it('renders a loading message', async () => {
  const { findByText } = await render(<EmptyState status="loading" />);
  expect(await findByText('Loading…')).toBeTruthy();
});

it('renders the given error message', async () => {
  const { findByText } = await render(<EmptyState status="error" message="Failed to load accounts." />);
  expect(await findByText('Failed to load accounts.')).toBeTruthy();
});

it('renders the given empty message', async () => {
  const { findByText } = await render(<EmptyState status="empty" message="No accounts for this client." />);
  expect(await findByText('No accounts for this client.')).toBeTruthy();
});
```

- [ ] **Step 3: Run the tests**

Run: `npm test -- src/components/EmptyState.test.tsx`
Expected: PASS, 3 tests.

- [ ] **Step 4: Commit**

```bash
git add src/components/EmptyState.tsx src/components/EmptyState.test.tsx
git commit -m "feat(mobile-app): add EmptyState component"
```

---

### Task 4: ScreenContainer component (adds `expo-linear-gradient`)

**Files:**
- Modify: `mobile-app/package.json`, `mobile-app/package-lock.json` (new dependency)
- Modify: `mobile-app/jest.setup-env.js` (safe-area-context Jest mock)
- Create: `mobile-app/src/components/ScreenContainer.tsx`
- Create: `mobile-app/src/components/ScreenContainer.test.tsx`

**Interfaces:**
- Consumes: `colors`, `spacing` from `../theme` (Task 1); `react-native-safe-area-context` (already a dependency, unused directly until now)
- Produces: `ScreenContainer({ children, contentStyle? })` — gradient background + safe-area + horizontal padding wrapper. `contentStyle` is an escape hatch for per-screen layout tweaks (Task 5 uses it to vertically center `ClientLookupScreen`'s form). Every screen task (4-8) wraps its top-level return in this.

**Note on `SafeAreaView` vs. `useSafeAreaInsets`:** the code below uses the
`useSafeAreaInsets` hook, not the `SafeAreaView` component, specifically
because `react-native-safe-area-context`'s own official Jest mock (used in
Step 2) only implements the hook, not the component — using `SafeAreaView`
here would resolve to `undefined` under that mock and crash every test that
renders `ScreenContainer` with "Element type is invalid". This was
discovered the hard way during this plan's own execution; the code below is
the corrected, verified-working version.

- [ ] **Step 1: Install `expo-linear-gradient`**

```bash
npm install expo-linear-gradient@~57.0.1 --registry=https://registry.npmjs.org --force
```

- [ ] **Step 2: Add the safe-area-context Jest mock**

Append to `mobile-app/jest.setup-env.js`:

```js

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
```

- [ ] **Step 3: Write `src/components/ScreenContainer.tsx`**

```tsx
import type { ReactNode } from 'react';
import { LinearGradient } from 'expo-linear-gradient';
import { StyleSheet, View, type ViewStyle } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { colors, spacing } from '../theme';

interface Props {
  children: ReactNode;
  contentStyle?: ViewStyle;
}

export function ScreenContainer({ children, contentStyle }: Props) {
  const insets = useSafeAreaInsets();
  return (
    <LinearGradient colors={colors.backgroundGradient} style={styles.gradient}>
      <View
        style={[styles.content, { paddingTop: insets.top, paddingBottom: insets.bottom }, contentStyle]}
      >
        {children}
      </View>
    </LinearGradient>
  );
}

const styles = StyleSheet.create({
  gradient: { flex: 1 },
  content: { flex: 1, paddingHorizontal: spacing.lg },
});
```

- [ ] **Step 4: Write `src/components/ScreenContainer.test.tsx`**

```tsx
import { render } from '@testing-library/react-native';
import { Text } from 'react-native';

import { ScreenContainer } from './ScreenContainer';

it('renders its children', async () => {
  const { findByText } = await render(
    <ScreenContainer>
      <Text>Hello</Text>
    </ScreenContainer>,
  );
  expect(await findByText('Hello')).toBeTruthy();
});
```

- [ ] **Step 5: Run the test**

Run: `npm test -- src/components/ScreenContainer.test.tsx`
Expected: PASS, 1 test.

Two known contingencies may fire here, both already fixed in the code above
and in Step 2 — if you followed Steps 1-4 exactly as written you should not
hit either, but if you deviated and land on one of these errors, here's the
fix:

- **`SyntaxError: Cannot use import statement outside a module` mentioning
  `expo-linear-gradient`:** this project's Jest config only transforms a
  specific allowlist of `node_modules` packages (see the comment above
  `transformIgnorePatterns` in `jest.config.js`) — `expo-linear-gradient`
  ships untransformed ESM and needs to join that allowlist, the same fix
  already applied once before for `expo-modules-core` (see git history,
  commit `9fc47cb`). Add `|expo-linear-gradient` to the regex alternation.
- **Same error mentioning `react-native-safe-area-context/jest/mock.tsx`:**
  that package also isn't in the allowlist (a pre-existing gap that only
  surfaces once something actually imports it in a test, which Step 2 just
  did for the first time in this project). Add
  `|react-native-safe-area-context` to the same regex alternation.

Both together, the full updated regex is:

```js
'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@react-navigation/.*|expo-modules-core|expo-linear-gradient|react-native-safe-area-context)/)',
```

then rerun Step 5. (Editing `jest.config.js` invalidates Jest's transform
cache, so the very next run may be slower and can spuriously hit the
default 5000ms test timeout — if that happens, simply rerun once more; it's
a one-time cold-cache effect, not a real failure.)

- [ ] **Step 6: Commit**

```bash
git add package.json package-lock.json jest.setup-env.js src/components/ScreenContainer.tsx src/components/ScreenContainer.test.tsx jest.config.js
git commit -m "feat(mobile-app): add ScreenContainer component with gradient background"
```

---

### Task 5: Navigation restructure — bottom tabs + nested Accounts stack

**Files:**
- Modify: `mobile-app/package.json`, `mobile-app/package-lock.json` (new dependencies)
- Modify: `mobile-app/App.tsx`
- Modify: `mobile-app/src/navigation/RootNavigator.tsx`
- Modify: `mobile-app/src/screens/ClientLookupScreen.tsx` (type-only change)
- Modify: `mobile-app/src/screens/AccountListScreen.tsx` (type-only change)
- Modify: `mobile-app/src/screens/AccountDetailScreen.tsx` (type-only change)

**Interfaces:**
- Consumes: `colors` from `../theme` (Task 1)
- Produces: `AccountsStackParamList` (replaces `RootStackParamList` for the 3 Accounts-tab screens) and `RootTabParamList`, both exported from `src/navigation/RootNavigator.tsx`. Task 6 imports `AccountsStackParamList` — this task must land it first.

This task is a **structural, not visual** change — screens keep their current plain styling; Task 6 restyles them. Splitting it this way means Task 5's deliverable (app navigates correctly via tabs) is independently testable without also verifying new visual code, and Task 6 can't accidentally break navigation since the types are already settled.

- [ ] **Step 1: Install navigation and icon dependencies**

```bash
npm install @react-navigation/bottom-tabs@^7.18.9 @expo/vector-icons@^15.1.1 --registry=https://registry.npmjs.org --force
```

- [ ] **Step 2: Rewrite `src/navigation/RootNavigator.tsx`**

```tsx
import { NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { Feather } from '@expo/vector-icons';

import { ClientLookupScreen } from '../screens/ClientLookupScreen';
import { AccountListScreen } from '../screens/AccountListScreen';
import { AccountDetailScreen } from '../screens/AccountDetailScreen';
import { HeldTransactionsScreen } from '../screens/HeldTransactionsScreen';
import { SettlementsScreen } from '../screens/SettlementsScreen';
import { AlertsScreen } from '../screens/AlertsScreen';
import { colors } from '../theme';

export type AccountsStackParamList = {
  ClientLookup: undefined;
  AccountList: { clientId: string };
  AccountDetail: { accountId: string };
};

export type RootTabParamList = {
  Accounts: undefined;
  Held: undefined;
  Settlements: undefined;
  Alerts: undefined;
};

const AccountsStack = createNativeStackNavigator<AccountsStackParamList>();
const Tab = createBottomTabNavigator<RootTabParamList>();

function AccountsStackNavigator() {
  return (
    <AccountsStack.Navigator initialRouteName="ClientLookup">
      <AccountsStack.Screen name="ClientLookup" component={ClientLookupScreen} options={{ title: 'SettleGuard' }} />
      <AccountsStack.Screen name="AccountList" component={AccountListScreen} options={{ title: 'Accounts' }} />
      <AccountsStack.Screen name="AccountDetail" component={AccountDetailScreen} options={{ title: 'Account' }} />
    </AccountsStack.Navigator>
  );
}

export function RootNavigator() {
  return (
    <NavigationContainer>
      <Tab.Navigator
        screenOptions={{
          tabBarActiveTintColor: colors.accent,
          tabBarInactiveTintColor: colors.neutral,
        }}
      >
        <Tab.Screen
          name="Accounts"
          component={AccountsStackNavigator}
          options={{
            headerShown: false,
            tabBarIcon: ({ color, size }) => <Feather name="credit-card" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Held"
          component={HeldTransactionsScreen}
          options={{
            title: 'Held Transactions',
            tabBarIcon: ({ color, size }) => <Feather name="pause-circle" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Settlements"
          component={SettlementsScreen}
          options={{
            title: 'Settlements',
            tabBarIcon: ({ color, size }) => <Feather name="check-circle" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Alerts"
          component={AlertsScreen}
          options={{
            title: 'Alerts',
            tabBarIcon: ({ color, size }) => <Feather name="bell" size={size} color={color} />,
          }}
        />
      </Tab.Navigator>
    </NavigationContainer>
  );
}
```

This deletes the old `RootStackParamList` type and the single flat `Stack.Navigator` — including the 3-button `headerRight` on `AccountList`, which is no longer needed now that Held/Settlements/Alerts are reachable via their own tabs.

- [ ] **Step 3: Wrap `App.tsx` in `SafeAreaProvider`**

`@react-navigation/bottom-tabs` needs `SafeAreaProvider` (from the already-installed `react-native-safe-area-context`) somewhere above it in the tree to correctly pad the tab bar around the device's safe area (e.g. the iOS home indicator). Rewrite `mobile-app/App.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { RootNavigator } from './src/navigation/RootNavigator';

const queryClient = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <SafeAreaProvider>
        <RootNavigator />
      </SafeAreaProvider>
    </QueryClientProvider>
  );
}
```

- [ ] **Step 4: Update `ClientLookupScreen.tsx`'s type import (behavior unchanged)**

In `src/screens/ClientLookupScreen.tsx`, change:

```diff
-import type { RootStackParamList } from '../navigation/RootNavigator';
+import type { AccountsStackParamList } from '../navigation/RootNavigator';

-type Props = NativeStackScreenProps<RootStackParamList, 'ClientLookup'>;
+type Props = NativeStackScreenProps<AccountsStackParamList, 'ClientLookup'>;
```

- [ ] **Step 5: Update `AccountListScreen.tsx`'s type import (behavior unchanged)**

In `src/screens/AccountListScreen.tsx`, change:

```diff
-import type { RootStackParamList } from '../navigation/RootNavigator';
+import type { AccountsStackParamList } from '../navigation/RootNavigator';

-type Props = NativeStackScreenProps<RootStackParamList, 'AccountList'>;
+type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountList'>;
```

- [ ] **Step 6: Update `AccountDetailScreen.tsx`'s type import (behavior unchanged)**

In `src/screens/AccountDetailScreen.tsx`, change:

```diff
-import type { RootStackParamList } from '../navigation/RootNavigator';
+import type { AccountsStackParamList } from '../navigation/RootNavigator';

-type Props = NativeStackScreenProps<RootStackParamList, 'AccountDetail'>;
+type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountDetail'>;
```

- [ ] **Step 7: Type-check and run the full suite**

Run: `npx tsc --noEmit`
Expected: no new errors outside `*.test.ts(x)` files (the pre-existing missing-Jest-types gap is out of scope, see the original MVP's ledger).

Run: `npm test`
Expected: all existing suites still pass — nothing in this task changes any screen's rendered output or logic, only navigation plumbing and type imports.

- [ ] **Step 8: Manually verify tab navigation**

```bash
npm start
```

Press `a` (Android emulator) or `w` (web). Confirm: 4 tabs visible at the bottom (Accounts, Held, Settlements, Alerts) with icons; tapping each tab shows the right screen; the Accounts tab still lets you type a Client ID → view accounts → tap into an account detail, exactly as before.

- [ ] **Step 9: Commit**

```bash
git add package.json package-lock.json App.tsx src/navigation/RootNavigator.tsx src/screens/ClientLookupScreen.tsx src/screens/AccountListScreen.tsx src/screens/AccountDetailScreen.tsx
git commit -m "feat(mobile-app): restructure navigation into bottom tabs"
```

---

### Task 6: Restyle the Accounts-tab screens

**Files:**
- Modify: `mobile-app/src/screens/ClientLookupScreen.tsx`
- Modify: `mobile-app/src/screens/AccountListScreen.tsx`
- Modify: `mobile-app/src/screens/AccountDetailScreen.tsx`

**Interfaces:**
- Consumes: `ScreenContainer` (Task 4), `Card`, `Button`, `BalanceDisplay`, `EmptyState` (Task 2-3), `colors`/`spacing`/`typography` (Task 1), `AccountsStackParamList` (Task 5, already landed)
- Produces: nothing new — this is a leaf visual task; no later task depends on these screens' internals.

No test files change in this task — the existing `ClientLookupScreen.test.tsx`, `AccountListScreen.test.tsx`, `AccountDetailScreen.test.tsx` assert on exact visible text (`'View Accounts'`, `'Client ID'` placeholder, `'ext-1'`, `'Balance: 500'`, `'credit 500'`, `'Failed to load account.'`, `'Failed to load transaction history.'`) that this task's code preserves exactly.

- [ ] **Step 1: Rewrite `src/screens/ClientLookupScreen.tsx`**

```tsx
import { useState } from 'react';
import { StyleSheet, Text, TextInput } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { ScreenContainer } from '../components/ScreenContainer';
import { Button } from '../components/Button';
import { colors, spacing, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'ClientLookup'>;

export function ClientLookupScreen({ navigation }: Props) {
  const [clientId, setClientId] = useState('');

  return (
    <ScreenContainer contentStyle={styles.centered}>
      <Text style={[typography.label, styles.label]}>Enter a Client ID to view its accounts:</Text>
      <TextInput
        style={[typography.body, styles.input]}
        placeholder="Client ID"
        placeholderTextColor={colors.textSecondary}
        value={clientId}
        onChangeText={setClientId}
        autoCapitalize="none"
      />
      <Button
        title="View Accounts"
        disabled={clientId.trim().length === 0}
        onPress={() => navigation.navigate('AccountList', { clientId: clientId.trim() })}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  centered: { justifyContent: 'center' },
  label: { color: colors.textSecondary, marginBottom: spacing.sm },
  input: {
    borderWidth: 1,
    borderColor: colors.accentMuted,
    borderRadius: spacing.cardRadius,
    padding: spacing.md,
    marginBottom: spacing.lg,
    color: colors.textPrimary,
    backgroundColor: colors.surfaceSolid,
  },
});
```

- [ ] **Step 2: Run its test**

Run: `npm test -- src/screens/ClientLookupScreen.test.tsx`
Expected: PASS, unchanged.

- [ ] **Step 3: Rewrite `src/screens/AccountListScreen.tsx`**

```tsx
import { FlatList, Text } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { listAccounts } from '../api/accounts';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountList'>;

export function AccountListScreen({ navigation, route }: Props) {
  const { clientId } = route.params;
  const { data, isLoading, error } = useQuery({
    queryKey: ['accounts', clientId],
    queryFn: () => listAccounts(clientId),
  });

  if (isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load accounts." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(a) => a.id}
        renderItem={({ item }) => (
          <Card onPress={() => navigation.navigate('AccountDetail', { accountId: item.id })}>
            <Text style={[typography.body, { color: colors.textPrimary }]}>{item.external_ref ?? item.id}</Text>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.status}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No accounts for this client." />}
      />
    </ScreenContainer>
  );
}
```

- [ ] **Step 4: Run its test**

Run: `npm test -- src/screens/AccountListScreen.test.tsx`
Expected: PASS, unchanged.

- [ ] **Step 5: Rewrite `src/screens/AccountDetailScreen.tsx`**

```tsx
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { getAccount } from '../api/accounts';
import { listEntriesForAccount } from '../api/ledger';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { BalanceDisplay } from '../components/BalanceDisplay';
import { EmptyState } from '../components/EmptyState';
import { colors, spacing, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountDetail'>;

export function AccountDetailScreen({ route }: Props) {
  const { accountId } = route.params;
  const account = useQuery({ queryKey: ['account', accountId], queryFn: () => getAccount(accountId) });
  const entries = useQuery({ queryKey: ['entries', accountId], queryFn: () => listEntriesForAccount(accountId) });

  if (account.isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (account.error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load account." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      {account.data && (
        <View style={styles.header}>
          <BalanceDisplay amount={account.data.balance} />
          <Text style={[typography.caption, { color: colors.textSecondary }]}>Status: {account.data.status}</Text>
        </View>
      )}
      {entries.error ? (
        <EmptyState status="error" message="Failed to load transaction history." />
      ) : (
        <FlatList
          data={entries.data ?? []}
          keyExtractor={(e) => e.id}
          renderItem={({ item }) => (
            <Card>
              <Text style={[typography.body, { color: colors.textPrimary }]}>
                {item.direction} {item.amount}
              </Text>
              <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.reason}</Text>
            </Card>
          )}
          ListEmptyComponent={<EmptyState status="empty" message="No ledger entries yet." />}
        />
      )}
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  header: { marginBottom: spacing.lg },
});
```

- [ ] **Step 6: Run its test**

Run: `npm test -- src/screens/AccountDetailScreen.test.tsx`
Expected: PASS, unchanged (3 tests: renders balance+entries, account error, entries error).

- [ ] **Step 7: Run the full suite and type-check**

Run: `npm test`
Run: `npx tsc --noEmit`
Expected: all suites pass; no new non-test type errors.

- [ ] **Step 8: Commit**

```bash
git add src/screens/ClientLookupScreen.tsx src/screens/AccountListScreen.tsx src/screens/AccountDetailScreen.tsx
git commit -m "feat(mobile-app): restyle Accounts-tab screens with design system"
```

---

### Task 7: Restyle HeldTransactionsScreen

**Files:**
- Modify: `mobile-app/src/screens/HeldTransactionsScreen.tsx`

**Interfaces:**
- Consumes: `ScreenContainer`, `Card`, `StatusChip`, `Button`, `EmptyState` (Tasks 2-4), `colors`/`spacing`/`typography` (Task 1)
- Produces: nothing new.

The existing `HeldTransactionsScreen.test.tsx` (4 tests) asserts on exact text (`'Amount: 500 · Score: 0.9'`, `'Triggered: velocity_limit'`, `'Approve'`/`'Approving…'`, `'Reject'`/`'Rejecting…'`, `'Failed to load held transactions.'`, `'Failed to approve transaction. Please try again.'`) and on `fireEvent.press(getByText('Approve'))` triggering the mutation — all preserved verbatim below, including the `approve.variables === item.id && approve.isPending` per-row pending-state scoping (a deliberate fix from the MVP's whole-branch review — do not regress it back to a shared `isPending`).

- [ ] **Step 1: Rewrite `src/screens/HeldTransactionsScreen.tsx`**

```tsx
import { useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { StatusChip } from '../components/StatusChip';
import { Button } from '../components/Button';
import { EmptyState } from '../components/EmptyState';
import { colors, spacing, typography } from '../theme';

export function HeldTransactionsScreen() {
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useQuery({ queryKey: ['held-transactions'], queryFn: listHeldTransactions });
  const [actionError, setActionError] = useState<string | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['held-transactions'] });
  const approve = useMutation({
    mutationFn: approveTransaction,
    onMutate: () => setActionError(null),
    onSuccess: invalidate,
    onError: () => setActionError('Failed to approve transaction. Please try again.'),
  });
  const reject = useMutation({
    mutationFn: rejectTransaction,
    onMutate: () => setActionError(null),
    onSuccess: invalidate,
    onError: () => setActionError('Failed to reject transaction. Please try again.'),
  });

  if (isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load held transactions." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(t) => t.id}
        ListHeaderComponent={
          actionError ? <Text style={[typography.body, styles.errorText]}>{actionError}</Text> : null
        }
        renderItem={({ item }) => {
          const approving = approve.variables === item.id && approve.isPending;
          const rejecting = reject.variables === item.id && reject.isPending;
          return (
            <Card>
              <View style={styles.rowTop}>
                <Text style={[typography.body, { color: colors.textPrimary }]}>
                  Amount: {item.amount} · Score: {item.score}
                </Text>
                <StatusChip status={item.status} />
              </View>
              <Text style={[typography.caption, { color: colors.textSecondary }]}>
                Triggered: {item.triggered_rules.join(', ') || 'none'}
              </Text>
              <View style={styles.actions}>
                <Button
                  title={approving ? 'Approving…' : 'Approve'}
                  disabled={approving}
                  onPress={() => approve.mutate(item.id)}
                />
                <Button
                  title={rejecting ? 'Rejecting…' : 'Reject'}
                  disabled={rejecting}
                  variant="secondary"
                  onPress={() => reject.mutate(item.id)}
                />
              </View>
            </Card>
          );
        }}
        ListEmptyComponent={<EmptyState status="empty" message="No held transactions." />}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  rowTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: spacing.xs },
  actions: { flexDirection: 'row', gap: spacing.md, marginTop: spacing.sm },
  errorText: { padding: spacing.lg, color: colors.danger },
});
```

- [ ] **Step 2: Run its test**

Run: `npm test -- src/screens/HeldTransactionsScreen.test.tsx`
Expected: PASS, unchanged, 4 tests — including `'does not disable other rows when approve fails'`, which specifically exercises the per-row `approving`/`rejecting` scoping preserved above.

- [ ] **Step 3: Run the full suite and type-check**

Run: `npm test`
Run: `npx tsc --noEmit`

- [ ] **Step 4: Commit**

```bash
git add src/screens/HeldTransactionsScreen.tsx
git commit -m "feat(mobile-app): restyle Held Transactions screen with design system"
```

---

### Task 8: Restyle SettlementsScreen and AlertsScreen

**Files:**
- Modify: `mobile-app/src/screens/SettlementsScreen.tsx`
- Modify: `mobile-app/src/screens/AlertsScreen.tsx`

**Interfaces:**
- Consumes: `ScreenContainer`, `Card`, `StatusChip`, `EmptyState` (Tasks 2-4), `colors`/`typography` (Task 1)
- Produces: nothing new.

`SettlementsScreen.test.tsx` (2 tests) and `AlertsScreen.test.tsx` (3 tests, including the fake-timers polling test) assert on exact text (`'Transactions: 2 · Total: 1500'`, the raw `created_at` timestamp string, `'Failed to load settlements.'`, `'risk_hold · txn-1'`, `'Failed to load alerts.'`) — all preserved verbatim below. `AlertsScreen`'s `refetchInterval: 15000` is untouched, so the polling test's behavior is unaffected by this purely visual change.

- [ ] **Step 1: Rewrite `src/screens/SettlementsScreen.tsx`**

```tsx
import { FlatList, Text } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listSettlements } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

export function SettlementsScreen() {
  const { data, isLoading, error } = useQuery({ queryKey: ['settlements'], queryFn: listSettlements });

  if (isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load settlements." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(s) => s.id}
        renderItem={({ item }) => (
          <Card>
            <Text style={[typography.body, { color: colors.textPrimary }]}>
              Transactions: {item.transaction_count} · Total: {item.total_amount}
            </Text>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.created_at}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No settlements." />}
      />
    </ScreenContainer>
  );
}
```

- [ ] **Step 2: Run its test**

Run: `npm test -- src/screens/SettlementsScreen.test.tsx`
Expected: PASS, unchanged.

- [ ] **Step 3: Rewrite `src/screens/AlertsScreen.tsx`**

```tsx
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listNotifications } from '../api/notifications';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { StatusChip } from '../components/StatusChip';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

export function AlertsScreen() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['notifications'],
    queryFn: listNotifications,
    refetchInterval: 15000,
  });

  if (isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load alerts." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(n) => n.id}
        renderItem={({ item }) => (
          <Card>
            <View style={styles.rowTop}>
              <Text style={[typography.body, { color: colors.textPrimary }]}>
                {item.type} · {item.subject_id}
              </Text>
              <StatusChip status={item.type} />
            </View>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.created_at}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No alerts." />}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  rowTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
});
```

- [ ] **Step 4: Run its test**

Run: `npm test -- src/screens/AlertsScreen.test.tsx`
Expected: PASS, unchanged, 3 tests — including the fake-timers polling test.

- [ ] **Step 5: Run the full suite and type-check**

Run: `npm test`
Run: `npx tsc --noEmit`

- [ ] **Step 6: Commit**

```bash
git add src/screens/SettlementsScreen.tsx src/screens/AlertsScreen.tsx
git commit -m "feat(mobile-app): restyle Settlements and Alerts screens with design system"
```

---

### Task 9: Final verification

**Files:** none (verification only)

**Interfaces:**
- Consumes: the complete restyled app (Tasks 1-8)
- Produces: nothing — this is the plan's closing gate.

- [ ] **Step 1: Run the full test suite**

Run: `npm test`
Expected: every suite passes — all pre-existing screen tests unchanged, plus the new ones from Tasks 1-4 (`theme.test.ts`, `Card.test.tsx`, `StatusChip.test.tsx`, `BalanceDisplay.test.tsx`, `Button.test.tsx`, `EmptyState.test.tsx`, `ScreenContainer.test.tsx`).

- [ ] **Step 2: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors outside `*.test.ts(x)` files.

- [ ] **Step 3: Manual smoke test — full main flow**

```bash
npm start
```

Open on Android emulator (`a`) or web (`w`). Walk the whole app: Client Lookup → enter a client ID → Account List (cards, status text) → tap an account → Account Detail (balance figure, colored by sign, ledger entries as cards) → switch to the Held tab (status chips, Approve/Reject with press feedback) → Settlements tab → Alerts tab (chips for notification type). Confirm the gradient background, coral accent color, and bold typography are visible on every screen, and that no screen shows a layout glitch (overlapping text, off-screen content, unreadable contrast).

- [ ] **Step 4: Confirm no stray console errors**

While doing the manual walkthrough in Step 3, check the terminal running `npm start` (or the browser console, if testing via web) for red error output. Warnings are acceptable; errors are not.

No commit for this task — it's verification only, over commits already made in Tasks 1-8.
