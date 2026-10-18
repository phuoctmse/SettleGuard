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
