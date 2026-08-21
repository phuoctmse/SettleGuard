import { fireEvent, render, waitFor } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { HeldTransactionsScreen } from './HeldTransactionsScreen';
import * as settlementApi from '../api/settlement';

jest.mock('../api/settlement');

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient();
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders held transactions returned by listHeldTransactions', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue([
    {
      id: 'txn-1',
      amount: 500,
      score: 0.9,
      decision: 'hold',
      status: 'held',
      triggered_rules: ['velocity_limit'],
      scored_at: '',
    },
  ]);

  const { findByText } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByText('Amount: 500 · Score: 0.9')).toBeTruthy();
  expect(await findByText('Triggered: velocity_limit')).toBeTruthy();
});

it('calls approveTransaction with the row id and removes the row after approval', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce([
      {
        id: 'txn-1',
        amount: 500,
        score: 0.9,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
    ])
    .mockResolvedValueOnce([]);
  (settlementApi.approveTransaction as jest.Mock).mockResolvedValue(undefined);

  const { findByText, getByText, queryByText } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByText('Amount: 500 · Score: 0.9')).toBeTruthy();

  await fireEvent.press(getByText('Approve'));

  expect((settlementApi.approveTransaction as jest.Mock).mock.calls[0][0]).toBe('txn-1');

  await waitFor(() => {
    expect(queryByText('Amount: 500 · Score: 0.9')).toBeNull();
  });
});
