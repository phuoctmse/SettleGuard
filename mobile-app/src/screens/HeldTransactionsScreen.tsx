import { useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { StatusChip } from '../components/StatusChip';
import { Button } from '../components/Button';
import { EmptyState } from '../components/EmptyState';
import { colors, spacing, typography } from '../theme';

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

  if (isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load held transactions." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(t) => t.id}
        ListHeaderComponent={
          actionError ? <Text style={[typography.body, styles.errorText]}>{actionError}</Text> : null
        }
        renderItem={({ item }) => {
          const approving = approve.variables === item.id && approve.isPending;
          const rejecting = reject.variables === item.id && reject.isPending;
          return (
            <Card>
              <View style={styles.rowTop}>
                <Text style={[typography.body, { color: colors.textPrimary }]}>
                  Amount: {item.amount} · Score: {item.score}
                </Text>
                <StatusChip status={item.status} />
              </View>
              <Text style={[typography.caption, { color: colors.textSecondary }]}>
                Triggered: {item.triggered_rules.join(', ') || 'none'}
              </Text>
              <View style={styles.actions}>
                <Button
                  title={approving ? 'Approving…' : 'Approve'}
                  disabled={approving}
                  onPress={() => approve.mutate(item.id)}
                />
                <Button
                  title={rejecting ? 'Rejecting…' : 'Reject'}
                  disabled={rejecting}
                  variant="secondary"
                  onPress={() => reject.mutate(item.id)}
                />
              </View>
            </Card>
          );
        }}
        ListEmptyComponent={<EmptyState status="empty" message="No held transactions." />}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  rowTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: spacing.xs },
  actions: { flexDirection: 'row', gap: spacing.md, marginTop: spacing.sm },
  errorText: { padding: spacing.lg, color: colors.danger },
});
