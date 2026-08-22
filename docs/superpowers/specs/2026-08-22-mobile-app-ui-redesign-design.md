# mobile-app UI/UX redesign — design spec

Date: 2026-08-22
Status: approved by user (visual direction chosen via brainstorming companion), ready for planning

## Context

`mobile-app`'s MVP (merged to `main` in PR #14, 2026-08-22) is functionally
complete but visually plain: default React Native `Text`/`Button`/`FlatList`
styling, no color system, no icons, three text buttons crammed into a
header for navigation. The user asked to upgrade the whole product from
MVP to "complete product" across features and UI, and chose to start with
UI/UX visual design as the first sub-project (auth, push notifications, and
missing business features are separate future sub-projects, out of scope
here).

This spec covers a visual and structural redesign of the existing 6
screens — no new screens, no backend/API changes, no new business
functionality. Direction was chosen interactively via the
`superpowers:brainstorming` visual companion (color palette, typography,
and navigation structure each presented as 2-3 concrete mockup options).

## Goals

- Replace default-looking UI with an intentional visual identity ("sharp,
  modern fintech with personality" — user's stated reference: Revolut/Brex)
- Replace ad-hoc per-screen styling with a small shared design system
  (theme tokens + reusable components), removing the duplicated
  loading/error/empty-state pattern already flagged as a code-quality
  finding in the MVP's whole-branch review
- Replace the current single stack + 3 header buttons with a bottom tab
  bar — this also makes the existing navigation honest: Held Transactions,
  Settlements, and Alerts are not scoped to a client (a gap the MVP review
  already flagged), so surfacing them as always-accessible tabs matches
  their actual (unscoped) backend behavior instead of hiding that behind
  "you can only reach them after looking up a client"

## Non-goals (explicitly out of scope for this spec)

- Dark mode (deferred; single light theme only)
- Real auth/login, real push notifications, any new business feature —
  separate future sub-projects
- Any backend/API change
- New screens or new user-facing functionality beyond what already exists
- A general-purpose component library (React Native Paper, Tamagui,
  NativeBase, etc.) — the visual direction is specific and custom
  (see Visual Identity below); adopting a kit would mean fighting its
  defaults. Build a small bespoke design system instead.

## Visual Identity

**Direction chosen: "Sunset Coral"** — warm, light, energetic; distinct
from the colder "dark mode fintech" look (Revolut-style dark+neon) the
user explicitly passed over.

### Color tokens (`src/theme/colors.ts`)

| Token | Value | Usage |
|---|---|---|
| `background` | linear gradient `#FFF3EC → #FFE4D6` (160deg) | Screen background |
| `surface` | `rgba(255,255,255,0.55)` on gradient, `#FFFFFF` on flat contexts | Card/row backgrounds |
| `textPrimary` | `#2B1A12` | Headings, body text |
| `textSecondary` | `rgba(43,26,18,0.6-0.65)` | Labels, meta text |
| `accent` | `#E8541F` | Primary actions, active tab, balance figures, links |
| `accentMuted` | `rgba(232,84,31,0.12-0.18)` | Chip backgrounds, subtle highlights |
| `success` | `#1A9E5C` | Positive amounts, `settled` status |
| `danger` | `#D13B3B` | Negative amounts, `rejected` status, error text |
| `warning` | `#C8420F` (on `rgba(232,84,31,0.12)` bg) | `held` status |
| `neutral` | `#8A7364` (on `rgba(138,115,100,0.12)` bg) | `pending_settlement` status, inactive tab |

React Native has no native CSS gradient — `background` uses
`expo-linear-gradient` (already an implicit Expo SDK dependency; add
explicitly to `package.json` since it's directly imported) as the screen
container's background layer.

### Typography (`src/theme/typography.ts`)

System font only (`-apple-system, Roboto` via RN's platform default, no
custom font loading / `expo-font` needed — keeps this scoped to styling,
not a fonts pipeline).

| Style | Size | Weight | Letter-spacing | Usage |
|---|---|---|---|---|
| `display` | 32 | 800 | -0.03em | Balance figures |
| `heading` | 17 | 700 | -0.01em | Screen/section titles |
| `label` | 12 | 700 | 0.04em, uppercase | Field labels, header eyebrow text |
| `body` | 14 | 500 | normal | Row primary text |
| `caption` | 12 | 500 | normal | Row secondary text, meta |
| `chip` | 10 | 700 | 0.03em | Status chip text |

### Spacing (`src/theme/spacing.ts`)

4px base scale: `xs=4, sm=8, md=12, lg=16, xl=24`. Card corner radius
`14`. Screen horizontal padding `16` (`lg`).

## Navigation Architecture

Replace the current single `createNativeStackNavigator` (flat, all 6
screens as siblings) with a bottom tab navigator (`@react-navigation/bottom-tabs`
— **new dependency**, not currently installed) wrapping 4 tabs, each its
own stack:

```
RootNavigator (NavigationContainer)
└── Tab.Navigator (bottom tabs)
    ├── Tab "Accounts" → Stack.Navigator
    │   ├── ClientLookup
    │   ├── AccountList (params: { clientId })
    │   └── AccountDetail (params: { accountId })
    ├── Tab "Held" → HeldTransactionsScreen (flat, no nested stack)
    ├── Tab "Settlements" → SettlementsScreen (flat)
    └── Tab "Alerts" → AlertsScreen (flat)
```

- `AccountListScreen`'s `headerRight` with the 3 navigation buttons is
  deleted entirely — that navigation now happens via tabs.
- Tab bar icons: `@expo/vector-icons` Feather set (outline style, matches
  the "sharp/modern" direction) — `credit-card` for Accounts,
  `pause-circle` for Held, `check-circle` for Settlements, `bell` for
  Alerts. Active tab tinted `accent`, inactive `neutral`.
- `RootStackParamList` splits into a per-tab param list
  (`AccountsStackParamList` for the nested stack) plus a
  `RootTabParamList` for the tab navigator; existing `navigation.navigate`
  call sites inside the Accounts stack (`ClientLookupScreen` →
  `AccountList`, `AccountListScreen` → `AccountDetail`) are unaffected —
  only cross-tab navigation (previously `HeldTransactions`/`Settlements`/
  `Alerts` from the header) is removed, since those are now reached by
  tapping their tab, not `navigation.navigate`.

## Component System (`src/components/`, all new)

| Component | Purpose | Replaces |
|---|---|---|
| `ScreenContainer` | Gradient background + safe-area + horizontal padding wrapper | Ad-hoc `View`/`SafeAreaView` per screen |
| `Card` | Rounded, `surface`-colored container (the list-row look used throughout) | Each screen's own `styles.row`/`styles.pad` |
| `StatusChip` | Colored pill showing a status string, color from a fixed status→token map | Plain `<Text>{status}</Text>` in Held/Settlements/Alerts rows |
| `BalanceDisplay` | Large `display`-style figure, colored `success`/`danger` by sign | `AccountDetailScreen`'s inline balance text |
| `EmptyState` | One component handling all three of loading / error / empty-list, taking `{ status: 'loading' \| 'error' \| 'empty', message }` | The near-identical loading/error/empty-list guard block duplicated across `AccountListScreen`, `HeldTransactionsScreen`, `SettlementsScreen`, `AlertsScreen`, `AccountDetailScreen` (flagged in the MVP's whole-branch review as 4-5 near-identical copies) |
| `PrimaryButton` / `IconButton` | Pressable with `accent` background/text and press feedback (opacity/scale via `Pressable`'s `style` callback — no new animation library) | Approve/Reject buttons in `HeldTransactionsScreen`, native `Button` elsewhere |

`StatusChip`'s status→color map (single source of truth, used by Held
Transactions' `decision`/`status`, Settlements has no per-item status
today, Alerts' `type`):

```
held            → warning
pending_settlement → neutral
settled         → success
rejected        → danger
risk_hold       → warning   (Alerts' notification type)
settlement_finalized → success (Alerts' notification type)
```

## Motion

Subtle only, per user's scoping choice:
- `Pressable`'s `style={({ pressed }) => [...]}` for a light scale
  (0.97) + opacity (0.85) press-down feedback on `Card`, `PrimaryButton`,
  and tab bar items.
- Screen-to-screen transitions: react-navigation's built-in default
  (native-stack's platform slide, bottom-tabs' built-in fade) — no
  custom transition config, no `react-native-reanimated` dependency added.

## Data Flow / Error Handling

No change to data flow (TanStack Query hooks, API client modules
untouched). Error handling behavior is unchanged — every screen still
distinguishes loading/error/empty exactly as fixed in the MVP's
whole-branch-review fix wave — only the *rendering* of those three states
moves into the shared `EmptyState` component instead of each screen's own
inline JSX.

## Screens Touched

All 6 existing screens get restyled to use the new theme/components; none
gain new functionality:

1. `ClientLookupScreen` — `ScreenContainer`, styled input/button
2. `AccountListScreen` — `ScreenContainer`, `Card` rows, `EmptyState`;
   `headerRight` buttons removed
3. `AccountDetailScreen` — `ScreenContainer`, `BalanceDisplay`, `Card` rows
   for ledger entries, `EmptyState` for both its queries
4. `HeldTransactionsScreen` — `ScreenContainer`, `Card` rows, `StatusChip`,
   `PrimaryButton`/`IconButton` for approve/reject, `EmptyState`
5. `SettlementsScreen` — `ScreenContainer`, `Card` rows, `EmptyState`
6. `AlertsScreen` — `ScreenContainer`, `Card` rows, `StatusChip` for
   notification type, `EmptyState`

Plus `RootNavigator.tsx` — restructured per Navigation Architecture above.

## Testing Impact

Existing RNTL tests query primarily by visible text (`getByText`/
`findByText`), not by component type or snapshot, so restyling is
low-risk for most of them. Specific impacts to verify during
implementation:
- Tests asserting on the literal loading/error/empty message text must
  still pass once that text renders through `EmptyState` instead of
  inline `<Text>` — the message strings themselves are unchanged, so
  `getByText('Failed to load accounts.')`-style assertions keep working
  as long as `EmptyState` renders the passed message as plain text.
- `HeldTransactionsScreen.test.tsx`'s row-scoped pending-state test
  (`approve.variables === item.id`) must keep working once Approve/Reject
  become `PrimaryButton`/`IconButton` — the underlying `disabled`/
  `onPress` wiring is unchanged, only the visual wrapper changes.
- New components with actual logic (`StatusChip`'s status→color mapping,
  `EmptyState`'s 3-state branching) get their own small test file each.
- Navigation restructure requires updating/removing the parts of
  `RootNavigator`-adjacent tests (if any) that reference the old header
  buttons — confirm during planning by grep for `headerRight`/`Held`/
  `Settlements`/`Alerts` button text in test files.

## New Dependencies

- `@react-navigation/bottom-tabs` (navigation restructure)
- `@expo/vector-icons` (icons — transitively present via `expo`, add as a
  direct dependency since it's directly imported)
- `expo-linear-gradient` (gradient background)

No other new packages (no animation library, no UI kit, no font-loading
package).
