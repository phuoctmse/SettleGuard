import { fireEvent, render } from '@testing-library/react-native';

import { Button } from './Button';

it('calls onPress when tapped', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(<Button title="Approve" onPress={onPress} />);
  await fireEvent.press(getByText('Approve'));
  expect(onPress).toHaveBeenCalledTimes(1);
});

it('does not call onPress when disabled', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(<Button title="Approve" onPress={onPress} disabled />);
  await fireEvent.press(getByText('Approve'));
  expect(onPress).not.toHaveBeenCalled();
});

it('forwards testID so the button can be found and pressed by id', async () => {
  const onPress = jest.fn();
  const { getByTestId } = await render(
    <Button title="Approve" onPress={onPress} testID="approve-txn-1" />,
  );
  await fireEvent.press(getByTestId('approve-txn-1'));
  expect(onPress).toHaveBeenCalledTimes(1);
});
