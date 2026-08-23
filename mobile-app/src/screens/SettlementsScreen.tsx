import { FlatList, Text } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listSettlements } from '../api/settlement';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

export function SettlementsScreen() {
  const { data, isLoading, error } = useQuery({ queryKey: ['settlements'], queryFn: listSettlements });

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
        <EmptyState status="error" message="Failed to load settlements." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(s) => s.id}
        renderItem={({ item }) => (
          <Card>
            <Text style={[typography.body, { color: colors.textPrimary }]}>
              Transactions: {item.transaction_count} · Total: {item.total_amount}
            </Text>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.created_at}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No settlements." />}
      />
    </ScreenContainer>
  );
}
