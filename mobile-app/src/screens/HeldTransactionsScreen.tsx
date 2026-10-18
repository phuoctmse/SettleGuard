import { useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';

import { approveTransaction, listHeldTransactions, rejectTransaction } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { StatusChip } from '../components/StatusChip';
import { Button } from '../components/Button';
import { EmptyState } from '../components/EmptyState';
import { LoadMoreFooter } from '../components/LoadMoreFooter';
import { colors, spacing, typography } from '../theme';

export function HeldTransactionsScreen() {
  const queryClient = useQueryClient();
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetching,
    isFetchingNextPage,
    isFetchNextPageError,
  } = useInfiniteQuery({
    queryKey: ['held-transactions'],
    queryFn: ({ pageParam }) => listHeldTransactions(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const items = data?.pages.flatMap((p) => p.items) ?? [];
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
  if (error && !data) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load held transactions." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        testID="held-list"
        data={items}
        keyExtractor={(t) => t.id}
        onEndReached={() => {
          if (hasNextPage && !isFetching) {
            void fetchNextPage();
          }
        }}
        onEndReachedThreshold={0.5}
        ListFooterComponent={
          <LoadMoreFooter loading={isFetchingNextPage} failed={isFetchNextPageError} />
        }
        ListHeaderComponent={
          actionError ? (
            <Text style={[typography.body, styles.errorText]}>{actionError}</Text>
          ) : null
        }
        renderItem={({ item }) => {
          const approving = approve.variables === item.id && approve.isPending;
          const rejecting = reject.variables === item.id && reject.isPending;
          return (
            <Card testID={`held-tx-${item.id}`}>
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
                  testID={`approve-${item.id}`}
                  title={approving ? 'Approving…' : 'Approve'}
                  disabled={approving}
                  onPress={() => approve.mutate(item.id)}
                />
                <Button
                  testID={`reject-${item.id}`}
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
  rowTop: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.xs,
  },
  actions: { flexDirection: 'row', gap: spacing.md, marginTop: spacing.sm },
  errorText: { padding: spacing.lg, color: colors.danger },
});
