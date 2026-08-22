import { fireEvent, render } from '@testing-library/react-native';
import { Text } from 'react-native';

import { Card } from './Card';

it('renders its children', async () => {
  const { findByText } = await render(
    <Card>
      <Text>Row content</Text>
    </Card>,
  );
  expect(await findByText('Row content')).toBeTruthy();
});

it('calls onPress when tapped, if provided', async () => {
  const onPress = jest.fn();
  const { getByText } = await render(
    <Card onPress={onPress}>
      <Text>Tap me</Text>
    </Card>,
  );
  await fireEvent.press(getByText('Tap me'));
  expect(onPress).toHaveBeenCalledTimes(1);
});
