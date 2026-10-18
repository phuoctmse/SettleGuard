import { fireEvent, render } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { SettlementsScreen } from './SettlementsScreen';
import * as settlementApi from '../api/settlement';

jest.mock('../api/settlement');

afterEach(() => {
  jest.clearAllMocks();
});

const page = <T,>(items: T[], next_cursor: string | null = null) => ({ items, next_cursor });

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders settlements returned by listSettlements', async () => {
  (settlementApi.listSettlements as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'settle-1',
        transaction_ids: ['txn-1', 'txn-2'],
        transaction_count: 2,
        total_amount: 1500,
        created_at: '2026-08-20T00:00:00Z',
      },
    ]),
  );

  const { findByText } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByText('Transactions: 2 · Total: 1500')).toBeTruthy();
  expect(await findByText('2026-08-20T00:00:00Z')).toBeTruthy();
});

it('shows a failure message when listSettlements errors', async () => {
  (settlementApi.listSettlements as jest.Mock).mockRejectedValue(new Error('network error'));

  const { findByText } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByText('Failed to load settlements.')).toBeTruthy();
});

it('gives each settlement card a testID carrying the settlement id', async () => {
  (settlementApi.listSettlements as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'settle-1',
        transaction_ids: ['txn-1', 'txn-2'],
        transaction_count: 2,
        total_amount: 1500,
        created_at: '2026-08-20T00:00:00Z',
      },
    ]),
  );

  const { findByTestId } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByTestId('settlement-card-settle-1')).toBeTruthy();
});

it('loads the next page of settlements with its cursor', async () => {
  const settlementRow = (id: string) => ({
    id,
    transaction_ids: ['txn-1'],
    transaction_count: 1,
    total_amount: 100,
    created_at: '2026-08-20T00:00:00Z',
  });
  (settlementApi.listSettlements as jest.Mock)
    .mockResolvedValueOnce(page([settlementRow('settle-1')], 'cursor-2'))
    .mockResolvedValueOnce(page([settlementRow('settle-2')]));

  const { findByTestId, getByTestId } = await renderWithQuery(<SettlementsScreen />);
  await findByTestId('settlement-card-settle-1');

  await fireEvent(getByTestId('settlements-list'), 'onEndReached');

  expect(await findByTestId('settlement-card-settle-2')).toBeTruthy();
  expect((settlementApi.listSettlements as jest.Mock).mock.calls[1][0]).toBe('cursor-2');
});
