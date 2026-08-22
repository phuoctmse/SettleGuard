import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';
import { getAccount } from '../api/accounts';
import { listEntriesForAccount } from '../api/ledger';

type Props = NativeStackScreenProps<RootStackParamList, 'AccountDetail'>;

export function AccountDetailScreen({ route }: Props) {
  const { accountId } = route.params;
  const account = useQuery({ queryKey: ['account', accountId], queryFn: () => getAccount(accountId) });
  const entries = useQuery({ queryKey: ['entries', accountId], queryFn: () => listEntriesForAccount(accountId) });

  if (account.isLoading) return <Text style={styles.pad}>Loading…</Text>;
  if (account.error) return <Text style={styles.pad}>Failed to load account.</Text>;

  return (
    <View style={styles.container}>
      {account.data && (
        <View style={styles.header}>
          <Text style={styles.balance}>Balance: {account.data.balance}</Text>
          <Text>Status: {account.data.status}</Text>
        </View>
      )}
      {entries.error ? (
        <Text style={styles.pad}>Failed to load transaction history.</Text>
      ) : (
        <FlatList
          data={entries.data ?? []}
          keyExtractor={(e) => e.id}
          renderItem={({ item }) => (
            <View style={styles.row}>
              <Text>{item.direction} {item.amount}</Text>
              <Text>{item.reason}</Text>
            </View>
          )}
          ListEmptyComponent={<Text style={styles.pad}>No ledger entries yet.</Text>}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
  balance: { fontSize: 20, fontWeight: '600' },
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
