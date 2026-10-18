import { render } from '@testing-library/react-native';

import { LoadMoreFooter } from './LoadMoreFooter';

it('shows a loading line while the next page loads', async () => {
  const { getByText } = await render(<LoadMoreFooter loading failed={false} />);
  expect(getByText('Loading more…')).toBeTruthy();
});

it('shows a failure line when the next page failed', async () => {
  const { getByText } = await render(<LoadMoreFooter loading={false} failed />);
  expect(getByText('Failed to load more.')).toBeTruthy();
});

it('renders nothing otherwise', async () => {
  const { toJSON } = await render(<LoadMoreFooter loading={false} failed={false} />);
  expect(toJSON()).toBeNull();
});
