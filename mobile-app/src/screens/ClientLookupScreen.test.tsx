import { fireEvent, render } from '@testing-library/react-native';

import { ClientLookupScreen } from './ClientLookupScreen';

it('navigates to AccountList with the entered client id', async () => {
  const navigate = jest.fn();
  const { getByPlaceholderText, getByText } = await render(
    <ClientLookupScreen navigation={{ navigate } as any} route={{} as any} />,
  );

  await fireEvent.changeText(getByPlaceholderText('Client ID'), 'client-123');
  await fireEvent.press(getByText('View Accounts'));

  expect(navigate).toHaveBeenCalledWith('AccountList', { clientId: 'client-123' });
});

it('exposes the input and button by testID', async () => {
  const navigate = jest.fn();
  const { getByTestId } = await render(
    <ClientLookupScreen navigation={{ navigate } as any} route={{} as any} />,
  );

  await fireEvent.changeText(getByTestId('client-id-input'), 'client-123');
  await fireEvent.press(getByTestId('view-accounts-button'));

  expect(navigate).toHaveBeenCalledWith('AccountList', { clientId: 'client-123' });
});
