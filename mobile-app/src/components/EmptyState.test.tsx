import { render } from '@testing-library/react-native';

import { EmptyState } from './EmptyState';

it('renders a loading message', async () => {
  const { findByText } = await render(<EmptyState status="loading" />);
  expect(await findByText('Loading…')).toBeTruthy();
});

it('renders the given error message', async () => {
  const { findByText } = await render(<EmptyState status="error" message="Failed to load accounts." />);
  expect(await findByText('Failed to load accounts.')).toBeTruthy();
});

it('renders the given empty message', async () => {
  const { findByText } = await render(<EmptyState status="empty" message="No accounts for this client." />);
  expect(await findByText('No accounts for this client.')).toBeTruthy();
});
