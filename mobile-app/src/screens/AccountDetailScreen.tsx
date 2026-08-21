import { View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../navigation/RootNavigator';

type Props = NativeStackScreenProps<RootStackParamList, 'AccountDetail'>;

export function AccountDetailScreen({ route }: Props) {
  // Stub component - to be implemented in Task 4
  return <View />;
}
