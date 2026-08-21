import { View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';

type Props = NativeStackScreenProps<RootStackParamList, 'AccountList'>;

export function AccountListScreen({ route }: Props) {
  // Stub component - to be implemented in Task 4
  return <View />;
}
