import { NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { Feather } from '@expo/vector-icons';

import { ClientLookupScreen } from '../screens/ClientLookupScreen';
import { AccountListScreen } from '../screens/AccountListScreen';
import { AccountDetailScreen } from '../screens/AccountDetailScreen';
import { HeldTransactionsScreen } from '../screens/HeldTransactionsScreen';
import { SettlementsScreen } from '../screens/SettlementsScreen';
import { AlertsScreen } from '../screens/AlertsScreen';
import { colors } from '../theme';

export type AccountsStackParamList = {
  ClientLookup: undefined;
  AccountList: { clientId: string };
  AccountDetail: { accountId: string };
};

export type RootTabParamList = {
  Accounts: undefined;
  Held: undefined;
  Settlements: undefined;
  Alerts: undefined;
};

const AccountsStack = createNativeStackNavigator<AccountsStackParamList>();
const Tab = createBottomTabNavigator<RootTabParamList>();

function AccountsStackNavigator() {
  return (
    <AccountsStack.Navigator initialRouteName="ClientLookup">
      <AccountsStack.Screen name="ClientLookup" component={ClientLookupScreen} options={{ title: 'SettleGuard' }} />
      <AccountsStack.Screen name="AccountList" component={AccountListScreen} options={{ title: 'Accounts' }} />
      <AccountsStack.Screen name="AccountDetail" component={AccountDetailScreen} options={{ title: 'Account' }} />
    </AccountsStack.Navigator>
  );
}

export function RootNavigator() {
  return (
    <NavigationContainer>
      <Tab.Navigator
        screenOptions={{
          tabBarActiveTintColor: colors.accent,
          tabBarInactiveTintColor: colors.neutral,
        }}
      >
        <Tab.Screen
          name="Accounts"
          component={AccountsStackNavigator}
          options={{
            headerShown: false,
            tabBarIcon: ({ color, size }) => <Feather name="credit-card" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Held"
          component={HeldTransactionsScreen}
          options={{
            title: 'Held Transactions',
            tabBarIcon: ({ color, size }) => <Feather name="pause-circle" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Settlements"
          component={SettlementsScreen}
          options={{
            title: 'Settlements',
            tabBarIcon: ({ color, size }) => <Feather name="check-circle" size={size} color={color} />,
          }}
        />
        <Tab.Screen
          name="Alerts"
          component={AlertsScreen}
          options={{
            title: 'Alerts',
            tabBarIcon: ({ color, size }) => <Feather name="bell" size={size} color={color} />,
          }}
        />
      </Tab.Navigator>
    </NavigationContainer>
  );
}
