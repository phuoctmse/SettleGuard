import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { listAccounts } from '../api/accounts';

type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountList'>;

export function AccountListScreen({ navigation, route }: Props) {
  const { clientId } = route.params;
  const { data, isLoading, error } = useQuery({
    queryKey: ['accounts', clientId],
    queryFn: () => listAccounts(clientId),
  });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;
  if (error) return <Text style={styles.pad}>Failed to load accounts.</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(a) => a.id}
      renderItem={({ item }) => (
        <Pressable style={styles.row} onPress={() => navigation.navigate('AccountDetail', { accountId: item.id })}>
          <Text>{item.external_ref ?? item.id}</Text>
          <Text>{item.status}</Text>
        </Pressable>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No accounts for this client.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { flexDirection: 'row', justifyContent: 'space-between', padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
