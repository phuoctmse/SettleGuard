import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';

import { listNotifications } from '../api/notifications';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { StatusChip } from '../components/StatusChip';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

export function AlertsScreen() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['notifications'],
    queryFn: listNotifications,
    refetchInterval: 15000,
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
        <EmptyState status="error" message="Failed to load alerts." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(n) => n.id}
        renderItem={({ item }) => (
          <Card>
            <View style={styles.rowTop}>
              <Text style={[typography.body, { color: colors.textPrimary }]}>
                {item.type} · {item.subject_id}
              </Text>
              <StatusChip status={item.type} />
            </View>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.created_at}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No alerts." />}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  rowTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
});
