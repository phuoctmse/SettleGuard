import { render } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { SettlementsScreen } from './SettlementsScreen';
import * as settlementApi from '../api/settlement';

jest.mock('../api/settlement');

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders settlements returned by listSettlements', async () => {
  (settlementApi.listSettlements as jest.Mock).mockResolvedValue([
    {
      id: 'settle-1',
      transaction_ids: ['txn-1', 'txn-2'],
      transaction_count: 2,
      total_amount: 1500,
      created_at: '2026-08-20T00:00:00Z',
    },
  ]);

  const { findByText } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByText('Transactions: 2 · Total: 1500')).toBeTruthy();
  expect(await findByText('2026-08-20T00:00:00Z')).toBeTruthy();
});

it('shows a failure message when listSettlements errors', async () => {
  (settlementApi.listSettlements as jest.Mock).mockRejectedValue(new Error('network error'));

  const { findByText } = await renderWithQuery(<SettlementsScreen />);

  expect(await findByText('Failed to load settlements.')).toBeTruthy();
});
