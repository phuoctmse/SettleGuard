import { fireEvent, render } from '@testing-library/react-native';
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
    {
      id: 'acc-1',
      client_id: 'client-123',
      external_ref: 'ext-1',
      status: 'active',
      balance: 500,
      created_at: '',
    },
  ]);

  const { findByText } = await renderWithQuery(
    <AccountListScreen
      navigation={{ navigate: jest.fn() } as any}
      route={{ params: { clientId: 'client-123' } } as any}
    />,
  );

  expect(await findByText('ext-1')).toBeTruthy();
  expect(accountsApi.listAccounts).toHaveBeenCalledWith('client-123');
});

it('gives each account card a testID carrying the account id', async () => {
  (accountsApi.listAccounts as jest.Mock).mockResolvedValue([
    {
      id: 'acc-1',
      client_id: 'client-123',
      external_ref: 'ext-1',
      status: 'active',
      balance: 0,
      created_at: '',
    },
    {
      id: 'acc-2',
      client_id: 'client-123',
      external_ref: 'ext-2',
      status: 'active',
      balance: 0,
      created_at: '',
    },
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
