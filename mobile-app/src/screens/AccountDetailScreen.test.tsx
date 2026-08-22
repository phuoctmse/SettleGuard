import { render } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AccountDetailScreen } from './AccountDetailScreen';
import * as accountsApi from '../api/accounts';
import * as ledgerApi from '../api/ledger';

jest.mock('../api/accounts');
jest.mock('../api/ledger');

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders balance and ledger entries returned by the queries', async () => {
  (accountsApi.getAccount as jest.Mock).mockResolvedValue({
    id: 'acc-1',
    client_id: 'client-123',
    external_ref: 'ext-1',
    status: 'active',
    balance: 500,
    created_at: '',
  });
  (ledgerApi.listEntriesForAccount as jest.Mock).mockResolvedValue([
    { id: 'entry-1', account_id: 'acc-1', direction: 'credit', amount: 500, reason: 'payout', created_at: '' },
  ]);

  const { findByText } = await renderWithQuery(
    <AccountDetailScreen navigation={{} as any} route={{ params: { accountId: 'acc-1' } } as any} />,
  );

  expect(await findByText('Balance: 500')).toBeTruthy();
  expect(await findByText('credit 500')).toBeTruthy();
});

it('shows a failure message when the account query errors', async () => {
  (accountsApi.getAccount as jest.Mock).mockRejectedValue(new Error('network error'));
  (ledgerApi.listEntriesForAccount as jest.Mock).mockResolvedValue([]);

  const { findByText } = await renderWithQuery(
    <AccountDetailScreen navigation={{} as any} route={{ params: { accountId: 'acc-1' } } as any} />,
  );

  expect(await findByText('Failed to load account.')).toBeTruthy();
});

it('shows balance but a failure message for entries when the ledger query errors', async () => {
  (accountsApi.getAccount as jest.Mock).mockResolvedValue({
    id: 'acc-1',
    client_id: 'client-123',
    external_ref: 'ext-1',
    status: 'active',
    balance: 500,
    created_at: '',
  });
  (ledgerApi.listEntriesForAccount as jest.Mock).mockRejectedValue(new Error('network error'));

  const { findByText } = await renderWithQuery(
    <AccountDetailScreen navigation={{} as any} route={{ params: { accountId: 'acc-1' } } as any} />,
  );

  expect(await findByText('Balance: 500')).toBeTruthy();
  expect(await findByText('Failed to load transaction history.')).toBeTruthy();
});
