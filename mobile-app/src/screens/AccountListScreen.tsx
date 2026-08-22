import { FlatList, Text } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { listAccounts } from '../api/accounts';
import { ScreenContainer } from '../components/ScreenContainer';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { colors, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'AccountList'>;

export function AccountListScreen({ navigation, route }: Props) {
  const { clientId } = route.params;
  const { data, isLoading, error } = useQuery({
    queryKey: ['accounts', clientId],
    queryFn: () => listAccounts(clientId),
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
        <EmptyState status="error" message="Failed to load accounts." />
      </ScreenContainer>
    );
  }

  return (
    <ScreenContainer>
      <FlatList
        data={data}
        keyExtractor={(a) => a.id}
        renderItem={({ item }) => (
          <Card onPress={() => navigation.navigate('AccountDetail', { accountId: item.id })}>
            <Text style={[typography.body, { color: colors.textPrimary }]}>{item.external_ref ?? item.id}</Text>
            <Text style={[typography.caption, { color: colors.textSecondary }]}>{item.status}</Text>
          </Card>
        )}
        ListEmptyComponent={<EmptyState status="empty" message="No accounts for this client." />}
      />
    </ScreenContainer>
  );
}
