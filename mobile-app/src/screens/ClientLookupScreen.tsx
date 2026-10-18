import { useState } from 'react';
import { StyleSheet, Text, TextInput } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';
import { ScreenContainer } from '../components/ScreenContainer';
import { Button } from '../components/Button';
import { colors, spacing, typography } from '../theme';

type Props = NativeStackScreenProps<AccountsStackParamList, 'ClientLookup'>;

export function ClientLookupScreen({ navigation }: Props) {
  const [clientId, setClientId] = useState('');

  return (
    <ScreenContainer contentStyle={styles.centered}>
      <Text style={[typography.label, styles.label]}>Enter a Client ID to view its accounts:</Text>
      <TextInput
        style={[typography.body, styles.input]}
        placeholder="Client ID"
        placeholderTextColor={colors.textSecondary}
        value={clientId}
        onChangeText={setClientId}
        autoCapitalize="none"
      />
      <Button
        title="View Accounts"
        disabled={clientId.trim().length === 0}
        onPress={() => navigation.navigate('AccountList', { clientId: clientId.trim() })}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  centered: { justifyContent: 'center' },
  label: { color: colors.textSecondary, marginBottom: spacing.sm },
  input: {
    borderWidth: 1,
    borderColor: colors.accentMuted,
    borderRadius: spacing.cardRadius,
    padding: spacing.md,
    marginBottom: spacing.lg,
    color: colors.textPrimary,
    backgroundColor: colors.surfaceSolid,
  },
});
