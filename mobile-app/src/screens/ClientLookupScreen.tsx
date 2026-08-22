import { useState } from 'react';
import { Button, StyleSheet, Text, TextInput, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { AccountsStackParamList } from '../navigation/RootNavigator';

type Props = NativeStackScreenProps<AccountsStackParamList, 'ClientLookup'>;

export function ClientLookupScreen({ navigation }: Props) {
  const [clientId, setClientId] = useState('');

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Enter a Client ID to view its accounts:</Text>
      <TextInput
        style={styles.input}
        placeholder="Client ID"
        value={clientId}
        onChangeText={setClientId}
        autoCapitalize="none"
      />
      <Button
        title="View Accounts"
        disabled={clientId.trim().length === 0}
        onPress={() => navigation.navigate('AccountList', { clientId: clientId.trim() })}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 16, justifyContent: 'center' },
  label: { marginBottom: 8 },
  input: { borderWidth: 1, borderColor: '#ccc', borderRadius: 6, padding: 8, marginBottom: 12 },
});
