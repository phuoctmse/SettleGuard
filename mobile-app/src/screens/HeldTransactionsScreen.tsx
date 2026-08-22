import { useState } from 'react';
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';

export function HeldTransactionsScreen() {
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useQuery({ queryKey: ['held-transactions'], queryFn: listHeldTransactions });
  const [actionError, setActionError] = useState<string | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['held-transactions'] });
  const approve = useMutation({
    mutationFn: approveTransaction,
    onMutate: () => setActionError(null),
    onSuccess: invalidate,
    onError: () => setActionError('Failed to approve transaction. Please try again.'),
  });
  const reject = useMutation({
    mutationFn: rejectTransaction,
    onMutate: () => setActionError(null),
    onSuccess: invalidate,
    onError: () => setActionError('Failed to reject transaction. Please try again.'),
  });

  if (isLoading) return <Text style={styles.pad}>Loading…</Text>;
  if (error) return <Text style={styles.pad}>Failed to load held transactions.</Text>;

  return (
    <FlatList
      data={data}
      keyExtractor={(t) => t.id}
      ListHeaderComponent={actionError ? <Text style={styles.error}>{actionError}</Text> : null}
      renderItem={({ item }) => {
        const approving = approve.variables === item.id && approve.isPending;
        const rejecting = reject.variables === item.id && reject.isPending;
        return (
          <View style={styles.row}>
            <Text>Amount: {item.amount} · Score: {item.score}</Text>
            <Text>Triggered: {item.triggered_rules.join(', ') || 'none'}</Text>
            <View style={styles.actions}>
              <Pressable disabled={approving} onPress={() => approve.mutate(item.id)}>
                <Text>{approving ? 'Approving…' : 'Approve'}</Text>
              </Pressable>
              <Pressable disabled={rejecting} onPress={() => reject.mutate(item.id)}>
                <Text>{rejecting ? 'Rejecting…' : 'Reject'}</Text>
              </Pressable>
            </View>
          </View>
        );
      }}
      ListEmptyComponent={<Text style={styles.pad}>No held transactions.</Text>}
    />
  );
}

const styles = StyleSheet.create({
  pad: { padding: 16 },
  row: { padding: 16, borderBottomWidth: 1, borderColor: '#eee' },
  actions: { flexDirection: 'row', gap: 16, marginTop: 8 },
  error: { padding: 16, color: '#b00020' },
});
