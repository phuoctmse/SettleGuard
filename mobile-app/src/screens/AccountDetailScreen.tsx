import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { getAccount } from '../api/accounts';
import { listEntriesForAccount } from '../api/ledger';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { BalanceDisplay } from '../components/BalanceDisplay';
import { EmptyState } from '../components/EmptyState';
import { colors, spacing, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountDetail'>;

export function AccountDetailScreen({ route }: Props) {
  const { accountId } = route.params;
  const account = useQuery({ queryKey: ['account', accountId], queryFn: () => getAccount(accountId) });
  const entries = useQuery({ queryKey: ['entries', accountId], queryFn: () => listEntriesForAccount(accountId) });

  if (account.isLoading) {
    return (
      <ScreenContainer>
        <EmptyState status="loading" />
      </ScreenContainer>
    );
  }
  if (account.error) {
    return (
      <ScreenContainer>
        <EmptyState status="error" message="Failed to load account." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      {account.data && (
        <View style={styles.header}>
          <BalanceDisplay amount={account.data.balance} />
          <Text style={[typography.caption, { color: colors.textSecondary }]}>Status: {account.data.status}</Text>
        </View>
      )}
      {entries.error ? (
        <EmptyState status="error" message="Failed to load transaction history." />
      ) : (
        <FlatList
          data={entries.data ?? []}
          keyExtractor={(e) => e.id}
          renderItem={({ item }) => (
            <Card>
              <Text style={[typography.body, { color: colors.textPrimary }]}>
                {item.direction} {item.amount}
              </Text>
              <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.reason}</Text>
            </Card>
          )}
          ListEmptyComponent={<EmptyState status="empty" message="No ledger entries yet." />}
        />
      )}
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  header: { marginBottom: spacing.lg },
});
