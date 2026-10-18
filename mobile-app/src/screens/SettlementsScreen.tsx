import { FlatList, Text } from 'react-native';
import { useInfiniteQuery } from '@tanstack/react-query';

import { listSettlements } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { LoadMoreFooter } from '../components/LoadMoreFooter';
import { colors, typography } from '../theme';

export function SettlementsScreen() {
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
    queryKey: ['settlements'],
    queryFn: ({ pageParam }) => listSettlements(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const items = data?.pages.flatMap((p) => p.items) ?? [];

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
        <EmptyState status="error" message="Failed to load settlements." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        testID="settlements-list"
        data={items}
        keyExtractor={(s) => s.id}
        onEndReached={() => {
          if (hasNextPage && !isFetching) {
            void fetchNextPage();
          }
        }}
        onEndReachedThreshold={0.5}
        ListFooterComponent={
          <LoadMoreFooter loading={isFetchingNextPage} failed={isFetchNextPageError} />
        }
        renderItem={({ item }) => (
          <Card testID={`settlement-card-${item.id}`}>
            <Text style={[typography.body, { color: colors.textPrimary }]}>
              Transactions: {item.transaction_count} · Total: {item.total_amount}
            </Text>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>
              {item.created_at}
            </Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No settlements." />}
      />
    </ScreenContainer>
  );
}
