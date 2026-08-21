import { fireEvent, render } from '@testing-library/react-native';
import { act } from 'react';

import { ClientLookupScreen } from './ClientLookupScreen';

it('navigates to AccountList with the entered client id', async () => {
  const navigate = jest.fn();
  const result = await render(
    <ClientLookupScreen navigation={{ navigate } as any} route={{} as any} />,
  );

  const input = result.getByPlaceholderText('Client ID');

  // Call the onChangeText callback directly to update the input
  await act(async () => {
    input.props.onChangeText('client-123');
  });

  // Find the button - it should now be enabled
  const button = result.getByText('View Accounts');

  // The Button component renders as View > View > Text
  // The outer View has the onClick handler (used in test environment)
  const buttonView = button.parent.parent;

  // Press the button by calling onClick
  await act(async () => {
    buttonView.props.onClick();
  });

  expect(navigate).toHaveBeenCalledWith('AccountList', { clientId: 'client-123' });
});
