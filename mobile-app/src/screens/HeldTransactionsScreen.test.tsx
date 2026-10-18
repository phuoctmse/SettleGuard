import { act, fireEvent, render, waitFor } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { HeldTransactionsScreen } from './HeldTransactionsScreen';
import * as settlementApi from '../api/settlement';

jest.mock('../api/settlement');

afterEach(() => {
  jest.clearAllMocks();
});

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

const page = <T,>(items: T[], next_cursor: string | null = null) => ({ items, next_cursor });

const heldTx = (id: string, amount: number) => ({
  id,
  amount,
  score: 0.9,
  decision: 'hold',
  status: 'held',
  triggered_rules: [],
  scored_at: '',
});

it('renders held transactions returned by listHeldTransactions', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'txn-1',
        amount: 500,
        score: 0.9,
        decision: 'hold',
        status: 'held',
        triggered_rules: ['velocity_limit'],
        scored_at: '',
      },
    ]),
  );

  const { findByText } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByText('Amount: 500 · Score: 0.9')).toBeTruthy();
  expect(await findByText('Triggered: velocity_limit')).toBeTruthy();
});

it('calls approveTransaction with the row id and removes the row after approval', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(
      page([
        {
          id: 'txn-1',
          amount: 500,
          score: 0.9,
          decision: 'hold',
          status: 'held',
          triggered_rules: [],
          scored_at: '',
        },
      ]),
    )
    .mockResolvedValueOnce(page([]));
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
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'txn-1',
        amount: 500,
        score: 0.9,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
      {
        id: 'txn-2',
        amount: 200,
        score: 0.5,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
    ]),
  );
  (settlementApi.approveTransaction as jest.Mock).mockRejectedValue(new Error('conflict'));

  const { findByText, getAllByText } = await renderWithQuery(<HeldTransactionsScreen />);

  await findByText('Amount: 500 · Score: 0.9');

  await fireEvent.press(getAllByText('Approve')[0]);

  expect(await findByText('Failed to approve transaction. Please try again.')).toBeTruthy();
  expect(getAllByText('Approve')).toHaveLength(2);
});

it('only disables the row being approved, not other rows, while the mutation is in flight', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'txn-1',
        amount: 500,
        score: 0.9,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
      {
        id: 'txn-2',
        amount: 200,
        score: 0.5,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
    ]),
  );
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

it('exposes per-row testIDs and approves/rejects the right row by id', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(
    page([
      {
        id: 'txn-1',
        amount: 500,
        score: 0.9,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
      {
        id: 'txn-2',
        amount: 200,
        score: 0.5,
        decision: 'hold',
        status: 'held',
        triggered_rules: [],
        scored_at: '',
      },
    ]),
  );
  (settlementApi.approveTransaction as jest.Mock).mockResolvedValue(undefined);
  (settlementApi.rejectTransaction as jest.Mock).mockResolvedValue(undefined);

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);

  expect(await findByTestId('held-tx-txn-1')).toBeTruthy();
  expect(getByTestId('held-tx-txn-2')).toBeTruthy();

  await fireEvent.press(getByTestId('approve-txn-2'));
  expect((settlementApi.approveTransaction as jest.Mock).mock.calls[0][0]).toBe('txn-2');

  await fireEvent.press(getByTestId('reject-txn-1'));
  expect((settlementApi.rejectTransaction as jest.Mock).mock.calls[0][0]).toBe('txn-1');

  // The list mock never changes, so wait on the refetches themselves: one
  // initial load plus one invalidate-triggered refetch per mutation.
  await waitFor(() => {
    expect(settlementApi.listHeldTransactions).toHaveBeenCalledTimes(3);
  });
});

it('loads the next page with its cursor when the list reaches the end', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(page([heldTx('txn-1', 500)], 'cursor-2'))
    .mockResolvedValueOnce(page([heldTx('txn-2', 200)]));

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(await findByTestId('held-tx-txn-2')).toBeTruthy();
  expect(getByTestId('held-tx-txn-1')).toBeTruthy();
  expect((settlementApi.listHeldTransactions as jest.Mock).mock.calls[1][0]).toBe('cursor-2');
});

it('does not request another page when there is no next cursor', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(page([heldTx('txn-1', 500)]));

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(settlementApi.listHeldTransactions).toHaveBeenCalledTimes(1);
});

it('keeps loaded rows and shows a footer when the next page fails', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(page([heldTx('txn-1', 500)], 'cursor-2'))
    .mockRejectedValueOnce(new Error('network error'));

  const { findByTestId, findByText, getByTestId } = await renderWithQuery(
    <HeldTransactionsScreen />,
  );
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(await findByText('Failed to load more.')).toBeTruthy();
  expect(getByTestId('held-tx-txn-1')).toBeTruthy();
});

it('does not load the next page while an approve-triggered refetch is in flight', async () => {
  // Guards the onEndReached check: fetchNextPage() cancels a running refetch by default
  // (react-query v5), which would leave the approved row on screen.
  let resolveRefetch: (v: unknown) => void;
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(page([heldTx('txn-1', 500)], 'cursor-2'))
    .mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRefetch = resolve;
      }),
    )
    .mockResolvedValue(page([heldTx('txn-2', 200)]));
  (settlementApi.approveTransaction as jest.Mock).mockResolvedValue(undefined);

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent.press(getByTestId('approve-txn-1'));
  await waitFor(() => {
    expect(settlementApi.listHeldTransactions).toHaveBeenCalledTimes(2);
  });

  // Let react-query's batched notification re-render the screen with isFetching = true.
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  await fireEvent(getByTestId('held-list'), 'onEndReached');

  const calls = (settlementApi.listHeldTransactions as jest.Mock).mock.calls;
  expect(calls).toHaveLength(2);
  expect(calls.some((c) => c[0] === 'cursor-2')).toBe(false);

  resolveRefetch!(page([]));
  await waitFor(() => {
    expect(settlementApi.listHeldTransactions).toHaveBeenCalledTimes(2);
  });
});
