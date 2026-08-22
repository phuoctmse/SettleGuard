import { render } from '@testing-library/react-native';
import { Text } from 'react-native';

import { ScreenContainer } from './ScreenContainer';

it('renders its children', async () => {
  const { findByText } = await render(
    <ScreenContainer>
      <Text>Hello</Text>
    </ScreenContainer>,
  );
  expect(await findByText('Hello')).toBeTruthy();
});
