import { fireEvent, render, waitFor } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { HeldTransactionsScreen } from './HeldTransactionsScreen';
import * as settlementApi from '../api/settlement';

jest.mock('../api/settlement');

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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

it('shows a failure message when listHeldTransactions errors', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockRejectedValue(new Error('network error'));

  const { findByText } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByText('Failed to load held transactions.')).toBeTruthy();
});

it('shows an error message and does not disable other rows when approve fails', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue([
    { id: 'txn-1', amount: 500, score: 0.9, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
    { id: 'txn-2', amount: 200, score: 0.5, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
  ]);
  (settlementApi.approveTransaction as jest.Mock).mockRejectedValue(new Error('conflict'));

  const { findByText, getAllByText } = await renderWithQuery(<HeldTransactionsScreen />);

  await findByText('Amount: 500 · Score: 0.9');

  await fireEvent.press(getAllByText('Approve')[0]);

  expect(await findByText('Failed to approve transaction. Please try again.')).toBeTruthy();
  expect(getAllByText('Approve')).toHaveLength(2);
});

it('only disables the row being approved, not other rows, while the mutation is in flight', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue([
    { id: 'txn-1', amount: 500, score: 0.9, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
    { id: 'txn-2', amount: 200, score: 0.5, decision: 'hold', status: 'held', triggered_rules: [], scored_at: '' },
  ]);
  let resolveApprove: () => void;
  (settlementApi.approveTransaction as jest.Mock).mockReturnValue(
    new Promise<void>((resolve) => {
      resolveApprove = () => resolve();
    }),
  );

  const { findByText, getAllByText } = await renderWithQuery(<HeldTransactionsScreen />);

  await findByText('Amount: 500 · Score: 0.9');

  await fireEvent.press(getAllByText('Approve')[0]);

  expect(await findByText('Approving…')).toBeTruthy();
  expect(getAllByText('Approve')).toHaveLength(1); // row 2's button must still read "Approve", not be caught by a shared isPending

  resolveApprove!();
  await findByText('Amount: 200 · Score: 0.5'); // wait for the invalidate/refetch cycle to settle before the test ends
});
