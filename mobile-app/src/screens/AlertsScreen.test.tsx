import { act, render } from '@testing-library/react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AlertsScreen } from './AlertsScreen';
import * as notificationsApi from '../api/notifications';

jest.mock('../api/notifications');

afterEach(() => {
  jest.clearAllMocks();
  jest.useRealTimers();
});

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

it('renders notifications returned by listNotifications', async () => {
  (notificationsApi.listNotifications as jest.Mock).mockResolvedValue([
    {
      id: 'notif-1',
      type: 'risk_hold',
      subject_id: 'txn-1',
      payload: {},
      created_at: '2026-08-20T00:00:00Z',
    },
  ]);

  const { findByText } = await renderWithQuery(<AlertsScreen />);

  expect(await findByText('risk_hold · txn-1')).toBeTruthy();
  expect(await findByText('2026-08-20T00:00:00Z')).toBeTruthy();
});

it('shows a failure message when listNotifications errors', async () => {
  (notificationsApi.listNotifications as jest.Mock).mockRejectedValue(new Error('network error'));

  const { findByText } = await renderWithQuery(<AlertsScreen />);

  expect(await findByText('Failed to load alerts.')).toBeTruthy();
});

it('polls for new notifications every 15 seconds', async () => {
  jest.useFakeTimers();

  (notificationsApi.listNotifications as jest.Mock).mockResolvedValue([]);

  await renderWithQuery(<AlertsScreen />);

  await waitForCalls(1);

  await act(async () => {
    await jest.advanceTimersByTimeAsync(15000);
  });

  await waitForCalls(2);

  async function waitForCalls(count: number) {
    await act(async () => {
      await jest.advanceTimersByTimeAsync(0);
    });
    expect(notificationsApi.listNotifications).toHaveBeenCalledTimes(count);
  }
});
