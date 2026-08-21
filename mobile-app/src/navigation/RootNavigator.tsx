import { Button, View } from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';

import { ClientLookupScreen } from '../screens/ClientLookupScreen';
import { AccountListScreen } from '../screens/AccountListScreen';
import { AccountDetailScreen } from '../screens/AccountDetailScreen';
import { HeldTransactionsScreen } from '../screens/HeldTransactionsScreen';
import { SettlementsScreen } from '../screens/SettlementsScreen';
import { AlertsScreen } from '../screens/AlertsScreen';

export type RootStackParamList = {
  ClientLookup: undefined;
  AccountList: { clientId: string };
  AccountDetail: { accountId: string };
  HeldTransactions: undefined;
  Settlements: undefined;
  Alerts: undefined;
};

const Stack = createNativeStackNavigator<RootStackParamList>();

export function RootNavigator() {
  return (
    <NavigationContainer>
      <Stack.Navigator initialRouteName="ClientLookup">
        <Stack.Screen name="ClientLookup" component={ClientLookupScreen} options={{ title: 'SettleGuard' }} />
        <Stack.Screen
          name="AccountList"
          component={AccountListScreen}
          options={({ navigation }) => ({
            title: 'Accounts',
            headerRight: () => (
              <View style={{ flexDirection: 'row' }}>
                <Button title="Held" onPress={() => navigation.navigate('HeldTransactions')} />
                <Button title="Settlements" onPress={() => navigation.navigate('Settlements')} />
                <Button title="Alerts" onPress={() => navigation.navigate('Alerts')} />
              </View>
            ),
          })}
        />
        <Stack.Screen name="AccountDetail" component={AccountDetailScreen} options={{ title: 'Account' }} />
        <Stack.Screen name="HeldTransactions" component={HeldTransactionsScreen} options={{ title: 'Held Transactions' }} />
        <Stack.Screen name="Settlements" component={SettlementsScreen} options={{ title: 'Settlements' }} />
        <Stack.Screen name="Alerts" component={AlertsScreen} options={{ title: 'Alerts' }} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
