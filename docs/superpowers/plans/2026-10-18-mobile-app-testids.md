# mobile-app testIDs (dự án con 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gắn `testID` ổn định vào `mobile-app` theo §6 của `docs/superpowers/specs/2026-10-18-pentest-and-playwright-design.md`, không đổi hành vi, để Playwright (dự án con 3) và Appium có selector.

**Architecture:** Ba component dùng chung (`Card`, `Button`, `BalanceDisplay`) nhận prop `testID` tuỳ chọn và chuyển tiếp xuống phần tử gốc. Các screen truyền `testID` cụ thể (có id của bản ghi khi là phần tử trong danh sách). Tab dùng option `tabBarButtonTestID` của React Navigation. Trên web, `react-native-web` render `testID` thành `data-testid`. Mỗi `testID` được khoá bằng một test Jest dùng `getByTestId`.

**Tech Stack:** Expo 57, React Native 0.86, React 19.2, React Navigation 7 (`@react-navigation/bottom-tabs` có `tabBarButtonTestID`), `@testing-library/react-native` 14 (`render` là async), Jest qua `jest-expo`.

## Global Constraints

- Branch: `service/mobile-app` (đã tồn tại, phải giữ — không xoá, không viết lại lịch sử). Chạy `git branch --show-current` ngay trước **mỗi** commit.
- Commit message **không** có dòng `Co-Authored-By` hay "Generated with Claude".
- **Không đổi hành vi**: không đổi text hiển thị, style, điều hướng, hay lời gọi API. Chỉ thêm prop `testID`.
- Tên `testID` đúng nguyên văn bảng dưới (dự án con 3 dùng chúng):

| testID | Ở đâu |
|---|---|
| `client-id-input`, `view-accounts-button` | `ClientLookupScreen` |
| `account-card-<id>` | `AccountListScreen` (mỗi account) |
| `account-balance` | `AccountDetailScreen` (qua `BalanceDisplay`) |
| `held-tx-<id>`, `approve-<id>`, `reject-<id>` | `HeldTransactionsScreen` (mỗi giao dịch) |
| `settlement-card-<id>` | `SettlementsScreen` (mỗi settlement) |
| `alert-card-<id>` | `AlertsScreen` (mỗi notification) |
| `tab-accounts`, `tab-held`, `tab-settlements`, `tab-alerts` | `RootNavigator` |

- `<id>` là trường `id` của bản ghi API (account, transaction, settlement, notification).
- Mọi lệnh `npm` chạy từ `mobile-app/`. Kiểm tra cuối: `npm test`, `npm run lint`, `npm run format:check` đều pass.
- `README.md` của `mobile-app` viết tiếng Anh (giữ ngôn ngữ hiện có của file).

## File Structure

| File | Thay đổi |
|---|---|
| `mobile-app/src/components/Card.tsx`, `Button.tsx`, `BalanceDisplay.tsx` | Thêm prop `testID?: string`, chuyển tiếp xuống phần tử gốc |
| `mobile-app/src/components/Card.test.tsx`, `Button.test.tsx`, `BalanceDisplay.test.tsx` | Thêm test chuyển tiếp `testID` |
| `mobile-app/src/screens/*.tsx` (6 screen) | Truyền `testID` theo bảng |
| `mobile-app/src/screens/*.test.tsx` (6 file) | Thêm test `getByTestId` |
| `mobile-app/src/navigation/RootNavigator.tsx` | `tabBarButtonTestID` cho 4 tab |
| `mobile-app/src/navigation/RootNavigator.test.tsx` (tạo) | Test 4 tab có testID |
| `mobile-app/README.md` | Sửa đoạn `w` (web) về CORS; thêm mục testID |

---

### Task 1: Đưa branch về `main` và cho component dùng chung nhận `testID`

**Files:**
- Modify: `mobile-app/src/components/Card.tsx`, `mobile-app/src/components/Button.tsx`, `mobile-app/src/components/BalanceDisplay.tsx`
- Test: `mobile-app/src/components/Card.test.tsx`, `mobile-app/src/components/Button.test.tsx`, `mobile-app/src/components/BalanceDisplay.test.tsx`

**Interfaces:**
- Produces: `Card({ children, onPress?, style?, testID? })`, `Button({ title, onPress, disabled?, variant?, testID? })`, `BalanceDisplay({ amount, testID? })`. `testID` đi thẳng xuống `Pressable`/`View`/`Text` gốc của component.

- [ ] **Step 1: Cập nhật branch `service/mobile-app` lên `main` (không viết lại lịch sử)**

Branch đã tồn tại và đã bị squash-merge từ trước nên tách khỏi `main`. Merge `main` vào nó; nội dung cũ của branch đã có trong `main`, nên khi xung đột thì lấy bản của `main`.

```bash
cd D:/SettleGuard
git fetch origin
git checkout service/mobile-app
git merge -X theirs origin/main -m "Merge origin/main into service/mobile-app"
git diff origin/main --stat     # PHẢI rỗng
git branch --show-current       # service/mobile-app
```

Nếu `git diff origin/main --stat` không rỗng: dừng lại, báo cáo danh sách file khác (đừng tự sửa).

- [ ] **Step 2: Viết test thất bại cho cả ba component**

Thêm vào cuối `mobile-app/src/components/Card.test.tsx`:

```tsx
it('forwards testID to the pressable card', async () => {
  const { getByTestId } = await render(
    <Card testID="card-x" onPress={() => {}}>
      <Text>Body</Text>
    </Card>,
  );
  expect(getByTestId('card-x')).toBeTruthy();
});

it('forwards testID to the static card', async () => {
  const { getByTestId } = await render(
    <Card testID="card-y">
      <Text>Body</Text>
    </Card>,
  );
  expect(getByTestId('card-y')).toBeTruthy();
});
```

(Đảm bảo file đã import `render` từ `@testing-library/react-native` và `Text` từ `react-native`; thêm import nếu thiếu.)

Thêm vào cuối `mobile-app/src/components/Button.test.tsx`:

```tsx
it('forwards testID so the button can be found and pressed by id', async () => {
  const onPress = jest.fn();
  const { getByTestId } = await render(
    <Button title="Approve" onPress={onPress} testID="approve-txn-1" />,
  );
  await fireEvent.press(getByTestId('approve-txn-1'));
  expect(onPress).toHaveBeenCalledTimes(1);
});
```

Thêm vào cuối `mobile-app/src/components/BalanceDisplay.test.tsx`:

```tsx
it('forwards testID to the balance text', async () => {
  const { getByTestId } = await render(<BalanceDisplay amount={250} testID="account-balance" />);
  expect(getByTestId('account-balance')).toHaveTextContent('Balance: 250');
});
```

- [ ] **Step 3: Chạy để xác nhận fail**

Run: `cd mobile-app && npx jest src/components/Card.test.tsx src/components/Button.test.tsx src/components/BalanceDisplay.test.tsx`
Expected: 4 test mới FAIL (TypeScript qua Babel không chặn prop lạ; lỗi là `Unable to find an element with testID: ...`). Các test cũ vẫn PASS.

- [ ] **Step 4: Cài đặt**

`mobile-app/src/components/Card.tsx`:

```tsx
interface Props {
  children: ReactNode;
  onPress?: () => void;
  style?: ViewStyle;
  testID?: string;
}

export function Card({ children, onPress, style, testID }: Props) {
  if (onPress) {
    return (
      <Pressable
        testID={testID}
        onPress={onPress}
        accessibilityRole="button"
        style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}
      >
        {children}
      </Pressable>
    );
  }
  return (
    <View testID={testID} style={[styles.card, style]}>
      {children}
    </View>
  );
}
```

`mobile-app/src/components/Button.tsx`: thêm `testID?: string;` vào `Props`, nhận `testID` trong tham số, và đặt `testID={testID}` trên `Pressable` (dòng đầu tiên trong các prop của nó).

`mobile-app/src/components/BalanceDisplay.tsx`:

```tsx
interface Props {
  amount: number;
  testID?: string;
}

export function BalanceDisplay({ amount, testID }: Props) {
  const color = amount < 0 ? colors.danger : colors.success;
  return (
    <Text testID={testID} style={[typography.display, { color }]}>
      Balance: {amount}
    </Text>
  );
}
```

- [ ] **Step 5: Chạy lại**

Run: `cd mobile-app && npx jest src/components && npx prettier --check src/components && npx eslint src/components`
Expected: tất cả PASS, Prettier/ESLint sạch (nếu Prettier báo lỗi, chạy `npx prettier --write` trên đúng các file đã sửa rồi chạy lại).

- [ ] **Step 6: Commit**

```bash
git branch --show-current   # service/mobile-app
git add mobile-app/src/components/Card.tsx mobile-app/src/components/Button.tsx mobile-app/src/components/BalanceDisplay.tsx mobile-app/src/components/Card.test.tsx mobile-app/src/components/Button.test.tsx mobile-app/src/components/BalanceDisplay.test.tsx
git commit -m "feat(mobile-app): shared components forward testID"
```

---

### Task 2: testID cho luồng Accounts (ClientLookup, AccountList, AccountDetail)

**Files:**
- Modify: `mobile-app/src/screens/ClientLookupScreen.tsx`, `AccountListScreen.tsx`, `AccountDetailScreen.tsx`
- Test: `mobile-app/src/screens/ClientLookupScreen.test.tsx`, `AccountListScreen.test.tsx`, `AccountDetailScreen.test.tsx`

**Interfaces:**
- Consumes: `Card`/`Button`/`BalanceDisplay` nhận `testID` (Task 1).
- Produces: `client-id-input`, `view-accounts-button`, `account-card-<id>`, `account-balance`.

- [ ] **Step 1: Viết test thất bại**

Ba file đều đã có helper riêng (`renderWithQuery`, `jest.mock` của API). Test mới dùng lại chúng.

Thêm vào cuối `mobile-app/src/screens/ClientLookupScreen.test.tsx`:

```tsx
it('exposes the input and button by testID', async () => {
  const navigate = jest.fn();
  const { getByTestId } = await render(
    <ClientLookupScreen navigation={{ navigate } as any} route={{} as any} />,
  );

  await fireEvent.changeText(getByTestId('client-id-input'), 'client-123');
  await fireEvent.press(getByTestId('view-accounts-button'));

  expect(navigate).toHaveBeenCalledWith('AccountList', { clientId: 'client-123' });
});
```

Thêm vào cuối `mobile-app/src/screens/AccountListScreen.test.tsx` (đổi dòng import đầu file thành `import { fireEvent, render } from '@testing-library/react-native';`):

```tsx
it('gives each account card a testID carrying the account id', async () => {
  (accountsApi.listAccounts as jest.Mock).mockResolvedValue([
    { id: 'acc-1', client_id: 'client-123', external_ref: 'ext-1', status: 'active', balance: 0, created_at: '' },
    { id: 'acc-2', client_id: 'client-123', external_ref: 'ext-2', status: 'active', balance: 0, created_at: '' },
  ]);
  const navigate = jest.fn();

  const { findByTestId, getByTestId } = await renderWithQuery(
    <AccountListScreen
      navigation={{ navigate } as any}
      route={{ params: { clientId: 'client-123' } } as any}
    />,
  );

  expect(await findByTestId('account-card-acc-1')).toBeTruthy();
  await fireEvent.press(getByTestId('account-card-acc-2'));
  expect(navigate).toHaveBeenCalledWith('AccountDetail', { accountId: 'acc-2' });
});
```

Thêm vào cuối `mobile-app/src/screens/AccountDetailScreen.test.tsx`:

```tsx
it('exposes the balance by testID', async () => {
  (accountsApi.getAccount as jest.Mock).mockResolvedValue({
    id: 'acc-1',
    client_id: 'client-123',
    external_ref: 'ext-1',
    status: 'active',
    balance: 1500,
    created_at: '',
  });
  (ledgerApi.listEntriesForAccount as jest.Mock).mockResolvedValue([]);

  const { findByTestId } = await renderWithQuery(
    <AccountDetailScreen
      navigation={{} as any}
      route={{ params: { accountId: 'acc-1' } } as any}
    />,
  );

  expect(await findByTestId('account-balance')).toHaveTextContent('Balance: 1500');
});
```

- [ ] **Step 2: Chạy để xác nhận fail**

Run: `cd mobile-app && npx jest src/screens/ClientLookupScreen.test.tsx src/screens/AccountListScreen.test.tsx src/screens/AccountDetailScreen.test.tsx`
Expected: 3 test mới FAIL với `Unable to find an element with testID`.

- [ ] **Step 3: Cài đặt**

`ClientLookupScreen.tsx`: thêm `testID="client-id-input"` vào `TextInput`, `testID="view-accounts-button"` vào `Button`.

`AccountListScreen.tsx`: `<Card testID={`account-card-${item.id}`} onPress={...}>`.

`AccountDetailScreen.tsx`: `<BalanceDisplay amount={account.data.balance} testID="account-balance" />`.

- [ ] **Step 4: Chạy lại**

Run: `cd mobile-app && npx jest src/screens/ClientLookupScreen.test.tsx src/screens/AccountListScreen.test.tsx src/screens/AccountDetailScreen.test.tsx && npx prettier --check src/screens && npx eslint src/screens`
Expected: PASS, sạch (nếu Prettier báo lỗi định dạng ở test vừa thêm, chạy `npx prettier --write` trên đúng file đó rồi chạy lại).

- [ ] **Step 5: Commit**

```bash
git branch --show-current   # service/mobile-app
git add mobile-app/src/screens/ClientLookupScreen.tsx mobile-app/src/screens/AccountListScreen.tsx mobile-app/src/screens/AccountDetailScreen.tsx mobile-app/src/screens/ClientLookupScreen.test.tsx mobile-app/src/screens/AccountListScreen.test.tsx mobile-app/src/screens/AccountDetailScreen.test.tsx
git commit -m "feat(mobile-app): testIDs on the accounts flow"
```

---

### Task 3: testID cho Held, Settlements, Alerts

**Files:**
- Modify: `mobile-app/src/screens/HeldTransactionsScreen.tsx`, `SettlementsScreen.tsx`, `AlertsScreen.tsx`
- Test: `mobile-app/src/screens/HeldTransactionsScreen.test.tsx`, `SettlementsScreen.test.tsx`, `AlertsScreen.test.tsx`

**Interfaces:**
- Consumes: `Card`/`Button` nhận `testID` (Task 1).
- Produces: `held-tx-<id>`, `approve-<id>`, `reject-<id>`, `settlement-card-<id>`, `alert-card-<id>`.

- [ ] **Step 1: Viết test thất bại**

`HeldTransactionsScreen.test.tsx` — file dùng `renderWithQuery` và `jest.mock('../api/settlement')`. Thêm:

```tsx
it('exposes per-row testIDs and approves/rejects the right row by id', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue([
    { id: 'txn-1', amount: 500, score: 0.9, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
    { id: 'txn-2', amount: 200, score: 0.5, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
  ]);
  (settlementApi.approveTransaction as jest.Mock).mockResolvedValue(undefined);
  (settlementApi.rejectTransaction as jest.Mock).mockResolvedValue(undefined);

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByTestId('held-tx-txn-1')).toBeTruthy();
  expect(getByTestId('held-tx-txn-2')).toBeTruthy();

  await fireEvent.press(getByTestId('approve-txn-2'));
  expect((settlementApi.approveTransaction as jest.Mock).mock.calls[0][0]).toBe('txn-2');

  await fireEvent.press(getByTestId('reject-txn-1'));
  expect((settlementApi.rejectTransaction as jest.Mock).mock.calls[0][0]).toBe('txn-1');
});
```

Thêm vào cuối `mobile-app/src/screens/SettlementsScreen.test.tsx`:

```tsx
it('gives each settlement card a testID carrying the settlement id', async () => {
  (settlementApi.listSettlements as jest.Mock).mockResolvedValue([
    {
      id: 'settle-1',
      transaction_ids: ['txn-1', 'txn-2'],
      transaction_count: 2,
      total_amount: 1500,
      created_at: '2026-08-20T00:00:00Z',
    },
  ]);

  const { findByTestId } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByTestId('settlement-card-settle-1')).toBeTruthy();
});
```

Thêm vào cuối `mobile-app/src/screens/AlertsScreen.test.tsx`:

```tsx
it('gives each alert card a testID carrying the notification id', async () => {
  (notificationsApi.listNotifications as jest.Mock).mockResolvedValue([
    {
      id: 'notif-1',
      type: 'risk_hold',
      subject_id: 'txn-1',
      payload: {},
      created_at: '2026-08-20T00:00:00Z',
    },
  ]);

  const { findByTestId } = await renderWithQuery(<AlertsScreen />);

  expect(await findByTestId('alert-card-notif-1')).toBeTruthy();
});
```

- [ ] **Step 2: Chạy để xác nhận fail**

Run: `cd mobile-app && npx jest src/screens/HeldTransactionsScreen.test.tsx src/screens/SettlementsScreen.test.tsx src/screens/AlertsScreen.test.tsx`
Expected: 3 test mới FAIL (`Unable to find an element with testID`), các test cũ PASS.

- [ ] **Step 3: Cài đặt**

`HeldTransactionsScreen.tsx`, trong `renderItem`:
- `<Card testID={`held-tx-${item.id}`}>`
- Button Approve: thêm `testID={`approve-${item.id}`}`
- Button Reject: thêm `testID={`reject-${item.id}`}`

`SettlementsScreen.tsx`: `<Card testID={`settlement-card-${item.id}`}>`.

`AlertsScreen.tsx`: `<Card testID={`alert-card-${item.id}`}>`.

- [ ] **Step 4: Chạy lại**

Run: `cd mobile-app && npx jest src/screens && npx prettier --check src/screens && npx eslint src/screens`
Expected: PASS, sạch.

- [ ] **Step 5: Commit**

```bash
git branch --show-current   # service/mobile-app
git add mobile-app/src/screens/HeldTransactionsScreen.tsx mobile-app/src/screens/SettlementsScreen.tsx mobile-app/src/screens/AlertsScreen.tsx mobile-app/src/screens/HeldTransactionsScreen.test.tsx mobile-app/src/screens/SettlementsScreen.test.tsx mobile-app/src/screens/AlertsScreen.test.tsx
git commit -m "feat(mobile-app): testIDs on held transactions, settlements and alerts"
```

---

### Task 4: testID cho 4 tab

**Files:**
- Modify: `mobile-app/src/navigation/RootNavigator.tsx`
- Create: `mobile-app/src/navigation/RootNavigator.test.tsx`

**Interfaces:**
- Produces: `tab-accounts`, `tab-held`, `tab-settlements`, `tab-alerts` (option `tabBarButtonTestID` của `@react-navigation/bottom-tabs` v7).

- [ ] **Step 1: Viết test thất bại**

Test chỉ quan tâm thanh tab, nên thay 6 screen bằng component rỗng để không gọi API. Bottom tabs đọc safe-area insets, nên bọc `SafeAreaProvider` với `initialMetrics` cố định.

`mobile-app/src/navigation/RootNavigator.test.tsx`:

```tsx
import { render } from '@testing-library/react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { RootNavigator } from './RootNavigator';

jest.mock('../screens/ClientLookupScreen', () => ({ ClientLookupScreen: () => null }));
jest.mock('../screens/AccountListScreen', () => ({ AccountListScreen: () => null }));
jest.mock('../screens/AccountDetailScreen', () => ({ AccountDetailScreen: () => null }));
jest.mock('../screens/HeldTransactionsScreen', () => ({ HeldTransactionsScreen: () => null }));
jest.mock('../screens/SettlementsScreen', () => ({ SettlementsScreen: () => null }));
jest.mock('../screens/AlertsScreen', () => ({ AlertsScreen: () => null }));

const metrics = {
  frame: { x: 0, y: 0, width: 390, height: 844 },
  insets: { top: 0, left: 0, right: 0, bottom: 0 },
};

it('gives every bottom tab a stable testID', async () => {
  const { findByTestId } = await render(
    <SafeAreaProvider initialMetrics={metrics}>
      <RootNavigator />
    </SafeAreaProvider>,
  );

  for (const id of ['tab-accounts', 'tab-held', 'tab-settlements', 'tab-alerts']) {
    expect(await findByTestId(id)).toBeTruthy();
  }
});
```

- [ ] **Step 2: Chạy để xác nhận fail**

Run: `cd mobile-app && npx jest src/navigation/RootNavigator.test.tsx`
Expected: FAIL với `Unable to find an element with testID: tab-accounts`. Nếu thay vào đó render lỗi vì native module (vd. `react-native-screens`, `@expo/vector-icons`), đó là vấn đề môi trường test: thêm `jest.mock` tối thiểu cho đúng module đó trong file test này, ghi lý do bằng một dòng comment, và nêu trong báo cáo. Không sửa `RootNavigator` để né lỗi test.

- [ ] **Step 3: Cài đặt**

`mobile-app/src/navigation/RootNavigator.tsx`, thêm vào `options` của từng `Tab.Screen`:
- `Accounts`: `tabBarButtonTestID: 'tab-accounts',`
- `Held`: `tabBarButtonTestID: 'tab-held',`
- `Settlements`: `tabBarButtonTestID: 'tab-settlements',`
- `Alerts`: `tabBarButtonTestID: 'tab-alerts',`

- [ ] **Step 4: Chạy lại**

Run: `cd mobile-app && npx jest src/navigation && npx prettier --check src/navigation && npx eslint src/navigation`
Expected: PASS, sạch.

- [ ] **Step 5: Commit**

```bash
git branch --show-current   # service/mobile-app
git add mobile-app/src/navigation/RootNavigator.tsx mobile-app/src/navigation/RootNavigator.test.tsx
git commit -m "feat(mobile-app): testIDs on the bottom tabs"
```

---

### Task 5: README, kiểm tra toàn bộ, và xác nhận trên web

**Files:**
- Modify: `mobile-app/README.md`

- [ ] **Step 1: Sửa README**

Thay gạch đầu dòng `w` hiện tại (đoạn nói "the backend services need CORS support ... `w` will fail with CORS errors as of this MVP") bằng:

```markdown
- `w` to open in web (browser) — the four services have no CORS, so a
  browser cannot call them directly. Only the gateway (`services/gateway`)
  sends CORS headers, and it requires an API key the app does not send, so
  the web build is currently driven only by the Playwright suite, which
  points the API URLs at the gateway and injects a key (see
  `docs/superpowers/specs/2026-10-18-pentest-and-playwright-design.md` §7.1)
```

Thêm một mục mới ngay trước `## Run tests`:

```markdown
## Test IDs

Interactive and list elements carry stable `testID`s so E2E tests can find
them (on web, `react-native-web` renders them as `data-testid`; on native,
Appium reads the same IDs). Treat them as a public contract: rename one only
together with the tests that use it.

| testID | Screen |
| --- | --- |
| `client-id-input`, `view-accounts-button` | Client lookup |
| `account-card-<id>` | Account list |
| `account-balance` | Account detail |
| `held-tx-<id>`, `approve-<id>`, `reject-<id>` | Held transactions |
| `settlement-card-<id>` | Settlements |
| `alert-card-<id>` | Alerts |
| `tab-accounts`, `tab-held`, `tab-settlements`, `tab-alerts` | Bottom tabs |
```

- [ ] **Step 2: Kiểm tra toàn bộ**

Run (từ `mobile-app/`):

```bash
npm test -- --ci
npm run lint
npm run format:check
```

Expected: tất cả pass. Ghi số test trước/sau (trước Task 1 và bây giờ) vào báo cáo.

- [ ] **Step 3: Xác nhận testID ra `data-testid` trên bản web**

```bash
npx expo export --platform web --output-dir ../.expo-web-check
grep -rlo "client-id-input\|tab-held\|account-card-" ../.expo-web-check | head
```

Expected: bundle JS chứa các chuỗi testID (chứng minh chúng vào bản web build). Sau đó xoá thư mục: `rm -rf ../.expo-web-check`. Việc `react-native-web` đổi `testID` thành `data-testid` là hành vi của thư viện (`node_modules/react-native-web/dist/modules/createDOMProps/index.js`); dự án con 3 sẽ kiểm chứng nó trong trình duyệt thật.

Nếu `expo export` thiếu biến môi trường `EXPO_PUBLIC_*`, chạy với các giá trị trong `mobile-app/.env.example` (vd. `env $(cat .env.example | xargs) npx expo export ...`).

- [ ] **Step 4: Commit**

```bash
git branch --show-current   # service/mobile-app
git add mobile-app/README.md
git commit -m "docs(mobile-app): document test IDs and why the web build needs the gateway"
```

---

## Self-Review

**Spec coverage (§6):**

| Yêu cầu | Task |
|---|---|
| Mọi testID trong bảng §6 | 2 (accounts), 3 (held/settlements/alerts), 4 (tabs) |
| `Card`, `Button` chuyển tiếp `testID` | 1 (cộng `BalanceDisplay` cho `account-balance`) |
| Không đổi hành vi; `npm test`, lint, format pass | Global Constraints, 5 |
| README sửa đoạn CORS của web | 5 |
| Tiêu chí xong §8 (2) | 5 Step 2 |

**Lệch khỏi spec, có lý do:** `BalanceDisplay` cũng nhận `testID` (spec ghi "`AccountDetailScreen` / `BalanceDisplay`"), để giữ component dùng chung không biết tên testID cụ thể. Có thêm test cho `RootNavigator` (spec không yêu cầu) vì đó là cách duy nhất khoá được 4 testID của tab.
