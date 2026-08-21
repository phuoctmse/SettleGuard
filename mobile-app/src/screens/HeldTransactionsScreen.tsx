import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';

export function HeldTransactionsScreen() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ['held-transactions'], queryFn: listHeldTransactions });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['held-transactions'] });
  const approve = useMutation({ mutationFn: approveTransaction, onSuccess: invalidate });
  const reject = useMutation({ mutationFn: rejectTransaction, onSuccess: invalidate });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(t) => t.id}
      renderItem={({ item }) => (
        <View style={styles.row}>
          <Text>Amount: {item.amount} · Score: {item.score}</Text>
          <Text>Triggered: {item.triggered_rules.join(', ') || 'none'}</Text>
          <View style={styles.actions}>
            <Pressable onPress={() => approve.mutate(item.id)}><Text>Approve</Text></Pressable>
            <Pressable onPress={() => reject.mutate(item.id)}><Text>Reject</Text></Pressable>
          </View>
        </View>
      )}
      ListEmptyComponent={<Text style={styles.pad}>No held transactions.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
  actions: { flexDirection: 'row', gap: 16, marginTop: 8 },
});
