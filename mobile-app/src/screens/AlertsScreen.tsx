import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listNotifications } from '../api/notifications';

export function AlertsScreen() {
  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: listNotifications,
    refetchInterval: 15000,
  });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(n) => n.id}
      renderItem={({ item }) => (
        <View style={styles.row}>
          <Text>{item.type} · {item.subject_id}</Text>
          <Text>{item.created_at}</Text>
        </View>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No alerts.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
});
