import { render } from '@testing-library/react-native';

import { BalanceDisplay } from './BalanceDisplay';

it('renders a positive balance', async () => {
  const { findByText } = await render(<BalanceDisplay amount={500} />);
  expect(await findByText('Balance: 500')).toBeTruthy();
});

it('renders a negative balance', async () => {
  const { findByText } = await render(<BalanceDisplay amount={-50} />);
  expect(await findByText('Balance: -50')).toBeTruthy();
});
