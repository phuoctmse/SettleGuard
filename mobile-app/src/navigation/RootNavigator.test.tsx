import { render } from '@testing-library/react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { RootNavigator } from './RootNavigator';

jest.mock('../screens/ClientLookupScreen', () => ({ ClientLookupScreen: () => null }));
jest.mock('../screens/AccountListScreen', () => ({ AccountListScreen: () => null }));
jest.mock('../screens/AccountDetailScreen', () => ({ AccountDetailScreen: () => null }));
jest.mock('../screens/HeldTransactionsScreen', () => ({ HeldTransactionsScreen: () => null }));
jest.mock('../screens/SettlementsScreen', () => ({ SettlementsScreen: () => null }));
jest.mock('../screens/AlertsScreen', () => ({ AlertsScreen: () => null }));

// @expo/vector-icons pulls in untransformed ESM (expo-font) that Jest cannot parse; icons are irrelevant here.
jest.mock('@expo/vector-icons', () => ({ Feather: () => null }));

const metrics = {
  frame: { x: 0, y: 0, width: 390, height: 844 },
  insets: { top: 0, left: 0, right: 0, bottom: 0 },
};

it('gives every bottom tab a stable testID', async () => {
  const { findByTestId } = await render(
    <SafeAreaProvider initialMetrics={metrics}>
      <RootNavigator />
    </SafeAreaProvider>,
  );

  for (const id of ['tab-accounts', 'tab-held', 'tab-settlements', 'tab-alerts']) {
    expect(await findByTestId(id)).toBeTruthy();
  }
});
