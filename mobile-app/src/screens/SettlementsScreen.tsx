import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listSettlements } from '../api/settlement';

export function SettlementsScreen() {
  const { data, isLoading, error } = useQuery({ queryKey: ['settlements'], queryFn: listSettlements });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;
  if (error) return <Text style={styles.pad}>Failed to load settlements.</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(s) => s.id}
      renderItem={({ item }) => (
        <View style={styles.row}>
          <Text>Transactions: {item.transaction_count} · Total: {item.total_amount}</Text>
          <Text>{item.created_at}</Text>
        </View>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No settlements.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
