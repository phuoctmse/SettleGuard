import { render } from '@testing-library/react-native';

import { StatusChip } from './StatusChip';

it('renders the status text', async () => {
  const { findByText } = await render(<StatusChip status="held" />);
  expect(await findByText('held')).toBeTruthy();
});

it('falls back to a neutral style for an unknown status', async () => {
  const { findByText } = await render(<StatusChip status="unknown_status" />);
  expect(await findByText('unknown_status')).toBeTruthy();
});
